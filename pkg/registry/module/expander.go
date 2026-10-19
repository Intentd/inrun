// pkg/module/expander.go
//
// Expander instantiates a Module by binding its inputs and expanding
// its resource blocks into a concrete HookTemplates value.
//
// Two modes:
//
// Static (inrun doctor init): inputs resolved from explicit with: bindings
// at generation time. The expanded resources are inlined into the generated
// Blueprint. No runtime dependency on the Module.
//
// Dynamic (Blueprint runtime): inputs resolved at Blueprint startup, before
// any reconcile. The Module is loaded once and its templates compiled with
// the input bindings.
package module

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"
	"text/template"

	"github.com/intentd/inrun/pkg/template/funcs"
	"github.com/intentd/inrun/pkg/types"
	"gopkg.in/yaml.v3"
)

// ExpandedModule holds the result of expanding a module.
type ExpandedModule struct {
	// Name is the module's metadata.name, used in conflict error messages.
	Name string
	// OnCreate contains resources from resources.onCreate: — merged into the CRD's OnCreate phase.
	OnCreate *types.HookTemplates
	// OnReconcile contains resources from the flat resources: fields — merged into OnReconcile.
	OnReconcile *types.HookTemplates
	Status      *types.StatusConfig
	Admission   *types.Admission
	// NotFunctionses carries user-defined functions declared in the module.
	// Merged into the Blueprint's KELFunctionRegistry during expandBlueprintImports.
	Functions types.KELFunctionRegistry
	// Profiles carries user-defined profiles declared in the module.
	// Merged into the Blueprint's ProfileRegistry during expandBlueprintImports.
	Profiles types.ProfileRegistry
}

// HasResources returns true when the module produced any resource templates.
func (e *ExpandedModule) HasResources() bool {
	return e.OnCreate != nil || e.OnReconcile != nil
}

// HasStatus reports whether the module defines status fields or conditions.
func (e *ExpandedModule) HasStatus() bool {
	return e.Status != nil
}

// HasAdmission reports whether the module includes admission rules.
func (e *ExpandedModule) HasAdmission() bool {
	return e.Admission != nil
}

// Expand instantiates a Module with the given input bindings and returns
// the expanded resources, status, and admission configuration.
//
// bindings maps input name → resolved value. Required inputs missing from
// bindings are a validation error. Unknown inputs in bindings are also an error.
// Optional inputs not in bindings use their Module-declared defaults.
//
// Expand replaces all `{{ .inputs.Name }}` and `{{ inputs.Name }}` expressions
// in the YAML of resources, status, and admission with the resolved binding values.
// Other template expressions (e.g., `{{ .children.* }}`) are left untouched
// and will be evaluated at runtime by the reconciler.
func Expand(m *types.Module, bindings map[string]string) (*ExpandedModule, error) {
	if err := validateBindings(m, bindings); err != nil {
		return nil, err
	}

	resolved := resolveDefaults(m, bindings)

	// ---- Expand resources ----
	var onCreate, onReconcile *types.HookTemplates
	if m.Resources != nil {
		resourceYAML, err := yaml.Marshal(m.Resources)
		if err != nil {
			return nil, fmt.Errorf("marshaling module resources: %w", err)
		}
		rendered, err := renderInputs(string(resourceYAML), resolved, m.Inputs)
		if err != nil {
			return nil, fmt.Errorf("rendering module %q resources: %w", m.Metadata.Name, err)
		}
		var mr types.ModuleResources
		if err := yaml.Unmarshal([]byte(rendered), &mr); err != nil {
			return nil, fmt.Errorf("parsing expanded module %q resources: %w", m.Metadata.Name, err)
		}
		if mr.OnCreate != nil {
			filterExpandedResources(mr.OnCreate)
			onCreate = mr.OnCreate
		}
		filterExpandedResources(&mr.HookTemplates)
		inline := mr.HookTemplates
		if !inline.Empty() {
			onReconcile = &inline
		}
	}

	// ---- Expand status ----
	var status *types.StatusConfig
	if m.Status != nil {
		statusYAML, err := yaml.Marshal(m.Status)
		if err != nil {
			return nil, fmt.Errorf("marshaling module status: %w", err)
		}
		rendered, err := renderInputs(string(statusYAML), resolved, m.Inputs)
		if err != nil {
			return nil, fmt.Errorf("rendering module %q status: %w", m.Metadata.Name, err)
		}
		var statusConfig types.StatusConfig
		if err := yaml.Unmarshal([]byte(rendered), &statusConfig); err != nil {
			return nil, fmt.Errorf("parsing expanded module %q status: %w", m.Metadata.Name, err)
		}
		status = &statusConfig
	}

	// ---- Expand admission (validation + mutation) ----
	var admission *types.Admission
	if m.Admission != nil {
		admissionYAML, err := yaml.Marshal(m.Admission)
		if err != nil {
			return nil, fmt.Errorf("marshaling module admission: %w", err)
		}
		rendered, err := renderInputs(string(admissionYAML), resolved, m.Inputs)
		if err != nil {
			return nil, fmt.Errorf("rendering module %q admission: %w", m.Metadata.Name, err)
		}
		var adm types.Admission
		if err := yaml.Unmarshal([]byte(rendered), &adm); err != nil {
			return nil, fmt.Errorf("parsing expanded module %q admission: %w", m.Metadata.Name, err)
		}
		admission = &adm
	}

	return &ExpandedModule{
		Name:        m.Metadata.Name,
		OnCreate:    onCreate,
		OnReconcile: onReconcile,
		Status:      status,
		Admission:   admission,
		Functions:   m.Functions,
		Profiles:    m.Profiles,
	}, nil
}

// validateBindings checks that all required inputs are provided and no
// unknown inputs are supplied.
func validateBindings(m *types.Module, bindings map[string]string) error {
	declared := make(map[string]*types.ModuleInput, len(m.Inputs))
	for i := range m.Inputs {
		declared[m.Inputs[i].Name] = &m.Inputs[i]
	}

	for _, input := range m.Inputs {
		if input.Required {
			if _, ok := bindings[input.Name]; !ok {
				return fmt.Errorf(
					"module %q: required input %q not provided in with:\n"+
						"  Module requires: %s\n"+
						"  Missing: %s",
					m.Metadata.Name, input.Name,
					strings.Join(requiredInputNames(m.Inputs), ", "),
					input.Name,
				)
			}
		}
	}

	for name := range bindings {
		if _, ok := declared[name]; !ok {
			return fmt.Errorf(
				"module %q: unknown input %q in with: — declared inputs: %s",
				m.Metadata.Name, name,
				strings.Join(inputNames(m.Inputs), ", "),
			)
		}
	}

	return nil
}

// resolveDefaults returns the full input map with all declared inputs present.
// Optional inputs not in bindings use their declared default (which may be "").
// All inputs are always included so the preprocessor has a complete context for
// complex expressions like {{ .inputs.loaderImage | default .inputs.image }}.
func resolveDefaults(m *types.Module, bindings map[string]string) map[string]string {
	resolved := make(map[string]string, len(m.Inputs))
	for _, input := range m.Inputs {
		if val, ok := bindings[input.Name]; ok {
			resolved[input.Name] = val
		} else {
			resolved[input.Name] = input.Default // "" for inputs with no default
		}
	}
	return resolved
}

// inputsExprRe matches any {{ ... }} block that references the inputs map.
// Used in the second pass of renderInputs to catch complex piped expressions
// like {{ .inputs.loaderImage | default .inputs.image }} that simple string
// replacement cannot handle.
var inputsExprRe = regexp.MustCompile(`\{\{-?\s*[^}]*\binputs\b[^}]*-?\}\}`)

// Built-in KEL functions
var kelFunctions = funcs.WithKEL()

// renderInputs is the module preprocessor: it fully resolves all {{ .inputs.* }}
// expressions so that only runtime expressions ({{ .metadata.* }}, {{ .spec.* }},
// {{ .children.* }}, etc.) remain in the output YAML.
//
// Two passes:
//  1. Fast exact-match replacement for the simple {{ .inputs.KEY }} pattern.
//  2. Go template evaluation (using the kelFunctions FuncMap) for any remaining
//     expressions that reference inputs — e.g. {{ .inputs.key | default .inputs.fallback }}.
//     Only blocks containing "inputs" are evaluated; all other template expressions
//     are left untouched for the runtime resolver.
func renderInputs(resourceYAML string, resolved map[string]string, inputs []types.ModuleInput) (string, error) {
	// Build a type index for post-substitution unquoting.
	inputTypes := make(map[string]string, len(inputs))
	for _, inp := range inputs {
		inputTypes[inp.Name] = strings.ToLower(inp.Type)
	}

	// Pass 1: exact-match simple patterns
	result := resourceYAML
	for key, val := range resolved {
		for _, pat := range []string{
			fmt.Sprintf("{{ .inputs.%s }}", key),
			fmt.Sprintf("{{ inputs.%s }}", key),
		} {
			result = strings.ReplaceAll(result, pat, val)
		}
	}

	// Pass 1b: strip YAML quotes around substituted scalar values for typed inputs.
	// YAML marshal quotes template expressions (e.g. '{{ .inputs.replicas }}'); after
	// pass 1 that becomes '1' — a YAML string — which Kubernetes rejects for integer fields.
	for key, val := range resolved {
		typ := inputTypes[key]
		if typ != "integer" && typ != "number" && typ != "bool" && typ != "boolean" {
			continue
		}
		for _, q := range []string{"'", `"`} {
			result = strings.ReplaceAll(result, q+val+q, val)
		}
	}

	// Pass 2: evaluate remaining complex input expressions with Go template.
	// Build an interface{} inputs map so | default and other pipeline funcs work.
	inputsData := make(map[string]interface{}, len(resolved))
	for k, v := range resolved {
		inputsData[k] = v
	}
	data := map[string]interface{}{"inputs": inputsData}

	var evalErr error
	result = inputsExprRe.ReplaceAllStringFunc(result, func(expr string) string {
		if evalErr != nil {
			return expr
		}
		tmpl, err := template.New("").
			Option("missingkey=zero").
			Funcs(kelFunctions).Parse(expr)

		if err != nil {
			return expr // malformed — leave as-is
		}
		var buf bytes.Buffer
		if err := tmpl.Execute(&buf, data); err != nil {
			return expr // leave failed expressions as-is for runtime
		}
		out := strings.TrimSpace(buf.String())
		return strings.ReplaceAll(out, "<no value>", "")
	})

	return result, evalErr
}

// ValidateModuleTemplates checks that all inputs.X references in the resource
// YAML correspond to declared input names. Returns a list of error strings.
func ValidateModuleTemplates(m *types.Module) []string {
	var errs []string
	declared := make(map[string]bool)
	for _, input := range m.Inputs {
		declared[input.Name] = true
	}

	resourceYAML, err := yaml.Marshal(m.Resources)
	if err != nil {
		return []string{"could not marshal resources for template validation"}
	}

	re := regexp.MustCompile(`\{\{\s*(?:index\s+)?\.?inputs\.?(\w+)`)
	matches := re.FindAllStringSubmatch(string(resourceYAML), -1)
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		inputName := match[1]
		if !declared[inputName] {
			errs = append(errs, fmt.Sprintf(
				"template references inputs.%s but no input named %q is declared",
				inputName, inputName,
			))
		}
	}
	return errs
}

// isRuntimeCondition reports whether a condition cannot be evaluated at module
// expansion time. A condition is runtime when its field or comparison value still
// contains a template expression ({{ }}) — meaning it references .spec.*, .metadata.*,
// .status.*, .external.*, or any other value only known during reconcile.
//
// After renderInputs runs, all {{ .inputs.* }} expressions have been replaced with
// their bound values. Any remaining {{ }} is a runtime expression that must be
// preserved on the resource for the reconciler to evaluate.
func isRuntimeCondition(cond types.Condition) bool {
	if types.IsTemplate(cond.Field) {
		return true
	}
	// Check comparison values
	_, val := types.ResolveConditionOp(cond)
	return types.IsTemplate(val)
}

// splitConditions partitions conditions into those that can be evaluated now
// (static — all inputs resolved, no remaining template expressions) and those
// that must be deferred to the reconciler (runtime — still contain {{ }}).
func splitConditions(conditions []types.Condition) (static, runtime []types.Condition) {
	for _, c := range conditions {
		if isRuntimeCondition(c) {
			runtime = append(runtime, c)
		} else {
			static = append(static, c)
		}
	}
	return
}

// evalModuleCondition evaluates a single module condition against already-resolved values.
// The field is treated as a literal value (not a dot-notation path to look up),
// because inputs have already been substituted by renderInputs.
func evalModuleCondition(cond types.Condition) bool {
	field := cond.Field

	// exists / notExists shorthands
	if cond.Exists != nil {
		return *cond.Exists == (field != "")
	}
	if cond.NotExists != nil {
		return *cond.NotExists == (field == "")
	}

	// operator or shorthand comparisons
	op, val := types.ResolveConditionOp(cond)
	switch op {
	case types.ConditionEquals:
		return field == val
	case types.ConditionNotEquals:
		return field != val
	case types.ConditionContains:
		return strings.Contains(field, val)
	case types.ConditionPrefix:
		return strings.HasPrefix(field, val)
	case types.ConditionSuffix:
		return strings.HasSuffix(field, val)
	}
	// Unknown or empty operator — treat as pass (don't silently drop resources)
	return true
}

// passesModuleConditions reports whether all conditions pass.
// Empty condition slice → unconditional (true).
func passesModuleConditions(conditions []types.Condition, or []types.Condition) bool {
	// AND conditions
	for _, c := range conditions {
		if !evalModuleCondition(c) {
			return false
		}
	}
	// or — if any pass, the block passes
	if len(or) > 0 {
		for _, c := range or {
			if evalModuleCondition(c) {
				return true
			}
		}
		return false
	}
	return true
}

// moduleConditionFilter is the standard fn passed to HookTemplates.FilterResources.
// Static conditions (no {{ }}, already input-substituted) gate the resource at
// expansion time. Runtime conditions (still contain {{ }}) are preserved on the
// resource for the reconciler to evaluate against live CR state.
func moduleConditionFilter(conditions, or []types.Condition) (bool, []types.Condition, []types.Condition) {
	static, runtime := splitConditions(conditions)
	staticOr, runtimeOr := splitConditions(or)
	return passesModuleConditions(static, staticOr), runtime, runtimeOr
}

// filterExpandedResources applies module condition filtering to ht using HookTemplates.FilterResources.
// Static conditions (no {{ }}, already input-substituted) gate inclusion at expansion time.
// Runtime conditions (still contain {{ }}) are preserved on the resource for the reconciler.
func filterExpandedResources(ht *types.HookTemplates) {
	filtered := ht.FilterResources(moduleConditionFilter)
	*ht = filtered
}

func inputNames(inputs []types.ModuleInput) []string {
	names := make([]string, len(inputs))
	for i, input := range inputs {
		names[i] = input.Name
	}
	return names
}

func requiredInputNames(inputs []types.ModuleInput) []string {
	var names []string
	for _, input := range inputs {
		if input.Required {
			names = append(names, input.Name)
		}
	}
	return names
}

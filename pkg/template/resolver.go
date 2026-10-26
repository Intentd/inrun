// pkg/template/resolver.go
package template

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"text/template"

	"github.com/intentd/inrun/domain"
	"github.com/intentd/inrun/pkg/template/funcs"
	"github.com/intentd/inrun/pkg/types"
)

// Resolver evaluates Go text/template expressions against a live CR object.
//
// Any field value containing "{{" is treated as a template expression and
// evaluated against the CR. Any value without "{{" is returned as-is.
//
// Template context is the CR's full object as map[string]interface{}:
//
//	"{{ .metadata.name }}"        → CR name
//	"{{ .metadata.namespace }}"   → CR namespace
//	"{{ .spec.image }}"           → spec.image field value
//	"{{ .spec.replicas }}"        → spec.replicas field value
//	"{{ .metadata.name }}-app"    → CR name with a static suffix
type Resolver struct {
	data           map[string]interface{}
	ownerName      string
	ownerNamespace string
	presets        *types.PresetRegistry
	// mergedFuncs is set by WithUserFunctions and contains kelFunctions + user-defined notes.
	// When nil, Resolve() uses the package-level kelFunctions FuncMap directly.
	mergedFuncs template.FuncMap
}

// WithSentinels adds the declared sentinel names to the resolver's FuncMap.
// Each sentinel is a no-arg function that returns its computed string value.
//
// Pass nil for values at validate time — sentinels return "" (stub for parse).
// Pass the computed values map at runtime — built by the informer UpdateFunc.
//
// Sentinels layer on top of any existing mergedFuncs (including user notes).
// Templates that reference an undeclared sentinel name fail to parse — this is
// how `inrun validate` catches misuse without executing templates.
func (r *Resolver) WithSentinels(declared []string, values map[string]string) *Resolver {
	if len(declared) == 0 {
		return r
	}
	base := kelFunctions
	if r.mergedFuncs != nil {
		base = r.mergedFuncs
	}
	merged := make(template.FuncMap, len(base)+len(declared))
	for k, v := range base {
		merged[k] = v
	}
	for _, name := range declared {
		n := name // capture
		merged[n] = func() string { return values[n] }
	}
	r.mergedFuncs = merged
	return r
}

// SentinelFuncMap returns a FuncMap containing stubs for each declared sentinel.
// All stubs return "" — only template parsing is checked, not execution.
// Used by validators that need to parse gate templates without a full Resolver.
func SentinelFuncMap(declared []string) template.FuncMap {
	fm := make(template.FuncMap, len(declared))
	for _, name := range declared {
		n := name
		fm[n] = func() string { return "" }
	}
	return fm
}

// WithPresets attaches a user-defined preset registry to the resolver.
// Call this after NewResolver when the blueprint declares a presets: block.
func (r *Resolver) WithPresets(reg *types.PresetRegistry) *Resolver {
	r.presets = reg
	return r
}

// WithUserFunctions registers user-defined notes from the Blueprint's KELFunctionRegistry.
// Each function's Expression is compiled as a Go template and registered under its
// Name in the resolver's FuncMap. Calling {{ functionName }} in any template
// evaluates the expression against the current CR's data.
// Built-in functions remain available; user functions may call built-in functions and
// each other inside their expressions.
func (r *Resolver) WithUserFunctions(reg types.KELFunctionRegistry) *Resolver {
	if reg.Empty() {
		return r
	}
	// merged holds built-ins + user functions. Closures capture it by reference
	// so all user functions see the complete FuncMap (including each other) at eval time.
	merged := make(template.FuncMap, len(kelFunctions)+len(reg.Definitions))
	for k, v := range kelFunctions {
		merged[k] = v
	}
	for _, n := range reg.Definitions {
		expr := n.Expression // capture by value, not loop variable
		merged[n.Name] = func() interface{} {
			tmpl, err := template.New("").Option("missingkey=zero").Funcs(merged).Parse(expr)
			if err != nil {
				return ""
			}
			var buf bytes.Buffer
			_ = tmpl.Execute(&buf, r.data)
			out := strings.TrimSpace(strings.ReplaceAll(buf.String(), "<no value>", ""))
			switch out {
			case "true":
				return true
			case "false":
				return false
			default:
				return out
			}
		}
	}
	r.mergedFuncs = merged
	return r
}

// Presets returns the user-defined preset registry attached to this resolver.
func (r *Resolver) Presets() *types.PresetRegistry {
	return r.presets
}

// NewResolver creates a Resolver from any domain.Object.
func NewResolver(ctx context.Context, obj domain.Object) (*Resolver, error) {
	data, err := objectToMap(obj) // converts any domain.Object to mapstringinterface{} for template execution.
	if err != nil {
		return nil, fmt.Errorf("template.NewResolver: %w", err)
	}

	// Inject empty inputs map so any unexpanded {{ .inputs.* }} module expressions
	// that survive module expansion return "" instead of nil-pointer-panicking.
	if _, ok := data["inputs"]; !ok {
		data["inputs"] = map[string]interface{}{}
	}

	// Inject runtime facts under .inrun.* from the context when available,
	// falling back to zero values so templates that reference .inrun.* never panic.
	data["inrun"] = inrunContextFromCtx(ctx).asMap()

	return &Resolver{
		data:           data,
		ownerName:      obj.GetName(),
		ownerNamespace: obj.GetNamespace(),
	}, nil
}

// WithInrunContext replaces the ".inrun" key in the resolver's data map with the
// provided runtime context. Call this after NewResolver when the full context
// (namespace, version) is available.
func (r *Resolver) WithInrunContext(ctx InrunContext) *Resolver {
	r.data["inrun"] = ctx.asMap()
	return r
}

// NewResolverFromMap creates a Resolver from a plain map[string]interface{}.
// Used by conversion webhooks where we work with unstructured JSON.
func NewResolverFromMap(data map[string]interface{}) *Resolver {
	return &Resolver{
		data: data,
		// ownerName/ownerNamespace are optional here; only needed for defaults.
	}
}

// NewResolverWithFunc creates a Resolver backed by a fresh template engine that
// exposes the functions from registry.
// Used where we need the resolver built from a function registry rather than
// unstructured JSON (e.g. CLI/template rendering paths).
func NewResolverWithFunc(registry types.KELFunctionRegistry) *Resolver {
	return NewResolverFromMap(map[string]interface{}{}).WithUserFunctions(registry)
}

// Built-in KEL functions
var kelFunctions = funcs.WithKEL()

// Resolve evaluates a single field value against the CR.
//
// If value contains "{{" it is a template expression — evaluated
// against the full CR map. Otherwise returned as-is (static value, no cost).
// Missing CR fields resolve to "" (missingkey=zero — no error on absent keys).
func (r *Resolver) Resolve(value string) (string, error) {
	out, _, err := r.resolve(value)
	return out, err
}

// Empty reports whether the Resolver has no meaningful settings
func (r *Resolver) Empty() bool {
	return r == nil
}

// ResolveStrict evaluates value like Resolve, but also reports whether any
// referenced field was missing (rendered "<no value>" before stripping).
// Callers that require every referenced field to be present — serve.name,
// serve.namespace — use this to catch composite templates (e.g.
// "{{ .team }}-{{ .environment }}") that would otherwise silently resolve
// to a non-empty but meaningless string like "-" when all fields are absent.
func (r *Resolver) ResolveStrict(value string) (out string, missing bool, err error) {
	return r.resolve(value)
}

func (r *Resolver) resolve(value string) (string, bool, error) {
	if value == "" {
		return "", false, nil
	}

	// Fast path — no template markers, static value
	if !types.IsTemplate(value) {
		return value, false, nil
	}

	funcs := kelFunctions
	if r.mergedFuncs != nil {
		funcs = r.mergedFuncs
	}
	tmpl, err := template.New("f").Option("missingkey=zero").
		Funcs(funcs).
		Parse(value)
	if err != nil {
		return "", false, fmt.Errorf("parsing %q: %w", value, err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, r.data); err != nil {
		return "", false, fmt.Errorf("executing %q: %w", value, err)
	}

	// missingkey=zero makes missing map keys produce nil (interface{} zero value).
	// Go's text/template renders nil interface{} as "<no value>", not "".
	// Replace all occurrences so callers get "" as documented.
	raw := strings.TrimSpace(buf.String())
	missing := strings.Contains(raw, "<no value>")
	out := strings.ReplaceAll(raw, "<no value>", "")
	return out, missing, nil
}

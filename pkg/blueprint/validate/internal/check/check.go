// Package check holds what the validate blocks share: the Checker every block
// embeds, the output marks, and the template, secret reference and field path
// checks more than one block needs.
package check

import (
	"fmt"
	"strings"
	texttemplate "text/template"

	"github.com/intentd/kel/core"

	"github.com/intentd/inrun/pkg/util/kube"
	"github.com/intentd/inrun/pkg/util/structured"
	"github.com/intentd/inrun/pkg/util/term"

	"github.com/go-playground/validator/v10"
	"k8s.io/apimachinery/pkg/api/validate/content"

	"github.com/intentd/inrun/pkg/blueprint"
	"github.com/intentd/inrun/pkg/config"
	"github.com/intentd/inrun/pkg/template"
	"github.com/intentd/inrun/pkg/template/funcs"

	"k8s.io/apimachinery/pkg/util/validation"

	"github.com/intentd/inrun/pkg/types"
)

// Checker is what every block's checks read: the Blueprint under validation and
// the process config. Each block embeds it in its own checker type.
type Checker struct {
	Blueprint *blueprint.Blueprint
	Cfg       *config.Config
}

// New returns the Checker for bp.
func New(bp *blueprint.Blueprint, cfg *config.Config) Checker {
	return Checker{Blueprint: bp, Cfg: cfg}
}

// Output marks and the parsers and predicates the blocks share.
var (
	Yellow      = term.Yellow
	Red         = term.Red
	FailureMark = term.FailureMark
	WarningMark = term.WarningMark

	ParseTimeDuration = core.ParseTimeDuration

	ToStringSet         = structured.ToStringSet
	IsNestedPath        = structured.IsNestedPath
	IsTemplate          = types.IsTemplate
	IsValidLabelKey     = content.IsLabelKey
	IsValidLabelValue   = content.IsLabelValue
	IsValidResolverName = template.ValidResolverName
	IsValidK8sName      = kube.ValidKubernetesName
)

func FuncMap(registry types.KELFunctionRegistry) texttemplate.FuncMap {
	builtins := funcs.WithKEL()
	funcMap := make(texttemplate.FuncMap, len(builtins)+len(registry.Definitions))
	for k, v := range builtins {
		funcMap[k] = v
	}
	for _, n := range registry.Definitions {
		funcMap[n.Name] = func() interface{} { return "" }
	}
	return funcMap
}

// validateSecretRef checks that ref has a name and a key. path is used in error messages.
func SecretRef(ref *types.APISecretRef, path string) error {
	if !ref.IsValid() {
		var missing []string
		if strings.TrimSpace(ref.Name) == "" {
			missing = append(missing, "name")
		}
		if strings.TrimSpace(ref.Key) == "" {
			missing = append(missing, "key")
		}
		return fmt.Errorf("%s %s: secretRef requires %s", FailureMark(), path, strings.Join(missing, " and "))
	}
	return nil
}

// validateSecretRefWithBlueprintWarning validates ref (name+key) and adds a blueprint-level
// warning when namespace is empty. Pass &e.bp.Warnings for gateway-level callers.
func SecretRefWithBlueprintWarning(ref *types.APISecretRef, path string, warnings *types.Warnings) error {
	return SecretRefNamespaceWarning(ref, path, warnings)
}

// validateSecretRefWithCRDWarning validates ref (name+key) and adds a CRD-level
// warning when namespace is empty. Pass &crd.Warnings for per-CRD callers.
func SecretRefWithCRDWarning(ref *types.APISecretRef, path string, warnings *types.Warnings) error {
	return SecretRefNamespaceWarning(ref, path, warnings)
}

func SecretRefNamespaceWarning(ref *types.APISecretRef, path string, warnings *types.Warnings) error {
	if err := SecretRef(ref, path); err != nil {
		return err
	}
	if strings.TrimSpace(ref.Namespace) == "" {
		warnings.AddWarning(fmt.Sprintf(
			"%s: secretRef.namespace is empty — will default to Inrun's namespace at runtime",
			path,
		))
	}
	return nil
}

func Template(caller, crdName, field, location, expr string, funcMap texttemplate.FuncMap) error {
	if _, err := texttemplate.New("").Funcs(funcMap).Parse(expr); err != nil {
		return fmt.Errorf("%s CRD %q: %s %q: %s: invalid template: %s", FailureMark(), crdName, caller, field, location, err.Error())
	}
	return nil
}

// PathFormat ensures the path is a valid dot-notation path:
//   - No empty segments (e.g., "app..repository")
//   - No leading or trailing dots
//   - Each segment is a valid Kubernetes qualified name
func PathFormat(path string) error {
	if path == "" {
		return fmt.Errorf("path cannot be empty")
	}

	if strings.HasPrefix(path, ".") || strings.HasSuffix(path, ".") {
		return fmt.Errorf("path cannot start or end with a dot")
	}

	parts := strings.Split(path, ".")
	for _, part := range parts {
		if part == "" {
			return fmt.Errorf("path contains empty segment (double dot)")
		}
		if errs := validation.IsQualifiedName(part); len(errs) > 0 {
			return fmt.Errorf("path segment %q is not a valid Kubernetes name: %s", part, strings.Join(errs, "; "))
		}
	}

	return nil
}

// First runs checks in order and returns the first error.
func First(checks ...func() error) error {
	for _, check := range checks {
		if err := check(); err != nil {
			return err
		}
	}
	return nil
}

// HandleValidationErrors prints struct-tag and custom validation errors.
func HandleValidationErrors(err error) {
	// Case 1: struct field validation errors (validator.v10)
	if errs, ok := err.(validator.ValidationErrors); ok {
		for _, e := range errs {
			fmt.Printf(
				"  • Field '%s' failed validation rule '%s'\n",
				e.Field(), e.Tag(),
			)
		}
		return
	}

	// Case 2: custom validation errors (like duration parsing)
	fmt.Printf("  • %s\n", err.Error())
}

// Package check holds what the validate blocks share: the Checker every block
// embeds, the output marks, and the template, secret reference and field path
// checks more than one block needs.
package check

import (
	"fmt"
	"github.com/go-playground/validator/v10"
	"github.com/intentd/inrun/pkg/catalog"
	"github.com/intentd/inrun/pkg/config"
	"github.com/intentd/inrun/pkg/template"
	"k8s.io/apimachinery/pkg/api/validate/content"
	"strings"
	texttemplate "text/template"

	"github.com/intentd/inrun/pkg/note"
	"github.com/intentd/inrun/pkg/types"
	"github.com/intentd/inrun/pkg/utils"
	"k8s.io/apimachinery/pkg/util/validation"
)

// Checker is what every block's checks read: the Catalog under validation and
// the process config. Each block embeds it in its own checker type.
type Checker struct {
	Cat *catalog.Catalog
	Cfg *config.Config
}

// New returns the Checker for cat.
func New(cat *catalog.Catalog, cfg *config.Config) Checker {
	return Checker{Cat: cat, Cfg: cfg}
}

// Output marks and the parsers and predicates the blocks share.
var (
	Yellow      = utils.Yellow
	Red         = utils.Red
	FailureMark = utils.FailureMark
	WarningMark = utils.WarningMark

	ParseTimeDuration = utils.ParseTimeDuration

	ToStringSet         = utils.ToStringSet
	IsNestedPath        = utils.IsNestedPath
	IsTemplate          = types.IsTemplate
	IsValidLabelKey     = content.IsLabelKey
	IsValidLabelValue   = content.IsLabelValue
	IsValidResolverName = template.ValidResolverName
	IsValidK8sName      = utils.ValidKubernetesName
)

func BoolPtr(b bool) *bool { return &b }

func FuncMap(notes types.NoteRegistry) texttemplate.FuncMap {
	builtins := note.Map()
	funcMap := make(texttemplate.FuncMap, len(builtins)+len(notes.Functions))
	for k, v := range builtins {
		funcMap[k] = v
	}
	for _, n := range notes.Functions {
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

// validateSecretRefWithCatalogWarning validates ref (name+key) and adds a catalog-level
// warning when namespace is empty. Pass &e.cat.Warnings for gateway-level callers.
func SecretRefWithCatalogWarning(ref *types.APISecretRef, path string, warnings *types.Warnings) error {
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

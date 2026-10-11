package validate

import (
	"fmt"
	"strings"
	texttemplate "text/template"

	"github.com/intentd/inrun/pkg/note"
	"github.com/intentd/inrun/pkg/template"
	"github.com/intentd/inrun/pkg/types"
	"github.com/intentd/inrun/pkg/utils"
	"k8s.io/apimachinery/pkg/api/validate/content"
)

var (
	yellow      = utils.Yellow
	red         = utils.Red
	failureMark = utils.FailureMark
	warningMark = utils.WarningMark

	parseTimeDuration = utils.ParseTimeDuration

	toStringSet         = utils.ToStringSet
	isNestedPath        = utils.IsNestedPath
	isTemplate          = types.IsTemplate
	isValidLabelKey     = content.IsLabelKey
	isValidLabelValue   = content.IsLabelValue
	isValidK8sName      = utils.ValidKubernetesName
	isValidResolverName = template.ValidResolverName
)

func boolPtr(b bool) *bool { return &b }

func buildFuncMapForValidation(notes types.NoteRegistry) texttemplate.FuncMap {
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
func validateSecretRef(ref *types.APISecretRef, path string) error {
	if !ref.IsValid() {
		var missing []string
		if strings.TrimSpace(ref.Name) == "" {
			missing = append(missing, "name")
		}
		if strings.TrimSpace(ref.Key) == "" {
			missing = append(missing, "key")
		}
		return fmt.Errorf("%s %s: secretRef requires %s", failureMark(), path, strings.Join(missing, " and "))
	}
	return nil
}

// validateSecretRefWithCatalogWarning validates ref (name+key) and adds a catalog-level
// warning when namespace is empty. Pass &e.k.Warnings for gateway-level callers.
func validateSecretRefWithCatalogWarning(ref *types.APISecretRef, path string, warnings *types.Warnings) error {
	return secretRefNamespaceWarning(ref, path, warnings)
}

// validateSecretRefWithCRDWarning validates ref (name+key) and adds a CRD-level
// warning when namespace is empty. Pass &crd.Warnings for per-CRD callers.
func validateSecretRefWithCRDWarning(ref *types.APISecretRef, path string, warnings *types.Warnings) error {
	return secretRefNamespaceWarning(ref, path, warnings)
}

func secretRefNamespaceWarning(ref *types.APISecretRef, path string, warnings *types.Warnings) error {
	if err := validateSecretRef(ref, path); err != nil {
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

func validateTemplate(caller, crdName, field, location, expr string, funcMap texttemplate.FuncMap) error {
	if _, err := texttemplate.New("").Funcs(funcMap).Parse(expr); err != nil {
		return fmt.Errorf("%s CRD %q: %s %q: %s: invalid template: %s", failureMark(), crdName, caller, field, location, err.Error())
	}
	return nil
}

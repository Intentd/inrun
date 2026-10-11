package catalog

import (
	"fmt"
	"text/template"

	"github.com/intentd/inrun/pkg/note"
	"github.com/intentd/inrun/pkg/types"
	"github.com/intentd/inrun/pkg/utils"
	"k8s.io/apimachinery/pkg/api/validate/content"
)

// ── utils aliases ────────────────────────────────────────────────────────────
// Import utils once here. All other files in this package use these names
// directly — no per-file utils import needed.

var (
	// colors / styles
	yellow = utils.Yellow
	red    = utils.Red

	// marks and icons
	failureMark = utils.FailureMark
	warningMark = utils.WarningMark

	// duration parsing
	parseTimeDuration = utils.ParseTimeDuration

	// helpers
	exit              = utils.Exit
	toStringSet       = utils.ToStringSet
	isNestedPath      = utils.IsNestedPath
	isTemplate        = types.IsTemplate
	isValidLabelKey   = content.IsLabelKey
	isValidLabelValue = content.IsLabelValue
	isValidK8sName    = utils.ValidKubernetesName
	readLocal         = utils.ReadLocal
)

func boolPtr(b bool) *bool { return &b }

// buildFuncMapForValidation builds a stub FuncMap that includes all registered
// note functions (so cross-note references compile) and all built-in functions.
// User-defined notes from the Catalog are added as stubs — only parsing is
// checked, not execution.
func buildFuncMapForValidation(notes types.NoteRegistry) template.FuncMap {
	builtins := note.Map()
	funcMap := make(template.FuncMap, len(builtins)+len(notes.Functions))
	for k, v := range builtins {
		funcMap[k] = v
	}
	for _, n := range notes.Functions {
		funcMap[n.Name] = func() interface{} { return "" }
	}
	return funcMap
}

func validateTemplate(caller, crdName, field, location, expr string, funcMap template.FuncMap) error {
	if _, err := template.New("").Funcs(funcMap).Parse(expr); err != nil {
		return fmt.Errorf("%s CRD %q: %s %q: %s: invalid template: %s", failureMark(), crdName, caller, field, location, err.Error())
	}
	return nil
}

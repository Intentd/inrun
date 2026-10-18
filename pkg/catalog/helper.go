package catalog

import (
	"fmt"
	"text/template"

	"github.com/intentd/inrun/pkg/template/funcs"
	"github.com/intentd/inrun/pkg/types"
	"github.com/intentd/inrun/pkg/utils"
)

// ── utils aliases ────────────────────────────────────────────────────────────
// Import utils once here. All other files in this package use these names
// directly — no per-file utils import needed.

var (
	failureMark       = utils.FailureMark
	warningMark       = utils.WarningMark
	parseTimeDuration = utils.ParseTimeDuration
	isTemplate        = types.IsTemplate
	readLocal         = utils.ReadLocal
)

func boolPtr(b bool) *bool { return &b }

// buildFuncMapForValidation builds a stub FuncMap that includes all registered
// functions (so cross-function references compile) and all built-in functions.
// User-defined functions from the Catalog are added as stubs — only parsing is
// checked, not execution.
func buildFuncMapForValidation(registry types.KELFunctionRegistry) template.FuncMap {
	builtins := funcs.WithKEL()
	funcMap := make(template.FuncMap, len(builtins)+len(registry.Definitions))
	for k, v := range builtins {
		funcMap[k] = v
	}
	for _, n := range registry.Definitions {
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

package serve

import "github.com/intentd/inrun/pkg/catalog/validate/internal/check"

// Shared helpers, under the names the checks use.
var (
	boolPtr                   = check.BoolPtr
	buildFuncMapForValidation = check.FuncMap
	failureMark               = check.FailureMark
	isNestedPath              = check.IsNestedPath
	isTemplate                = check.IsTemplate
	isValidLabelKey           = check.IsValidLabelKey
	toStringSet               = check.ToStringSet
	validatePathFormat        = check.PathFormat
	validateTemplate          = check.Template
	warningMark               = check.WarningMark
	yellow                    = check.Yellow
)

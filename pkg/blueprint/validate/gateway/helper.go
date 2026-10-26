package gateway

import "github.com/intentd/inrun/pkg/blueprint/validate/internal/check"

// Shared helpers, under the names the checks use.
var (
	buildFuncMapForValidation             = check.FuncMap
	failureMark                           = check.FailureMark
	isTemplate                            = check.IsTemplate
	red                                   = check.Red
	validateSecretRefWithBlueprintWarning = check.SecretRefWithBlueprintWarning
	validateTemplate                      = check.Template
)

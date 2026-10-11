package gateway

import "github.com/intentd/inrun/pkg/catalog/validate/internal/check"

// Shared helpers, under the names the checks use.
var (
	buildFuncMapForValidation           = check.FuncMap
	failureMark                         = check.FailureMark
	isTemplate                          = check.IsTemplate
	red                                 = check.Red
	validateSecretRefWithCatalogWarning = check.SecretRefWithCatalogWarning
	validateTemplate                    = check.Template
)

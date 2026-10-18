package reconcile

import "github.com/intentd/inrun/pkg/catalog/validate/internal/check"

// Shared helpers, under the names the checks use.
var (
	buildFuncMapForValidation       = check.FuncMap
	failureMark                     = check.FailureMark
	isTemplate                      = check.IsTemplate
	isValidK8sName                  = check.IsValidK8sName
	isValidLabelKey                 = check.IsValidLabelKey
	isValidLabelValue               = check.IsValidLabelValue
	parseTimeDuration               = check.ParseTimeDuration
	validatePathFormat              = check.PathFormat
	validateSecretRefWithCRDWarning = check.SecretRefWithCRDWarning
	validateTemplate                = check.Template
)

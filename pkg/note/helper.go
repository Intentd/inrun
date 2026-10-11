package note

import "github.com/intentd/inrun/pkg/utils"

var (
	expandCronMacro   = utils.ExpandCronMacro
	validateCronField = utils.ValidateCronField
	isValidCronExpr   = utils.IsValidCronExpr

	semverValid   = utils.SemverValid
	semverMajor   = utils.SemverMajor
	semverMinor   = utils.SemverMinor
	semverPatch   = utils.SemverPatch
	semverCompare = utils.SemverCompare
	semverCheck   = utils.SemverCheck
	semverBump    = utils.SemverBump
)

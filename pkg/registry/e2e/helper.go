package e2e

import (
	"github.com/intentd/inrun/pkg/types"
	"github.com/intentd/inrun/pkg/utils"
)

var (
	readLocal         = utils.ReadLocal
	strictUnmarshal   = utils.StrictUnmarshal
	startSpinner      = utils.StartSpinner
	successMark       = utils.SuccessMark
	failureMark       = utils.FailureMark
	parseTimeDuration = utils.ParseTimeDuration
)

// assertionListMsg is appended to every "at least one assertion required"
// validation error so the accepted field names stay in sync with hasAssertion.
const assertionListMsg = "at least one assertion required (equals, notEquals, outputContains, outputNotContains, regex, greaterThan, lessThan, greaterThanOrEqual, lessThanOrEqual, between, notBetween, oneOf, notOneOf, exists, notExists)"

// The types.E2EKubectl* subcommand types and E2ECommand each carry their
// own copy of the same 15 assertion fields (needed so they round-trip
// through YAML as flat, self-describing structs). These functions are the
// single place that knows how to pull those fields into an assertions
// value — every hasAssertion/applyAssertions call site in validate.go and
// verify.go goes through one of these instead of re-listing all 15 fields
// itself.

func commandAssertions(c types.E2ECommand) assertions {
	return assertions{Equals: c.Equals, NotEquals: c.NotEquals, OutputContains: c.OutputContains, OutputNotContains: c.OutputNotContains, Regex: c.Regex, GreaterThan: c.GreaterThan, LessThan: c.LessThan, GreaterThanOrEqual: c.GreaterThanOrEqual, LessThanOrEqual: c.LessThanOrEqual, Between: c.Between, NotBetween: c.NotBetween, OneOf: c.OneOf, NotOneOf: c.NotOneOf, Exists: c.Exists, NotExists: c.NotExists}
}

func kubectlApplyAssertions(e types.E2EKubectlApply) assertions {
	return assertions{Equals: e.Equals, NotEquals: e.NotEquals, OutputContains: e.OutputContains, OutputNotContains: e.OutputNotContains, Regex: e.Regex, GreaterThan: e.GreaterThan, LessThan: e.LessThan, GreaterThanOrEqual: e.GreaterThanOrEqual, LessThanOrEqual: e.LessThanOrEqual, Between: e.Between, NotBetween: e.NotBetween, OneOf: e.OneOf, NotOneOf: e.NotOneOf, Exists: e.Exists, NotExists: e.NotExists}
}

func kubectlGetAssertions(e types.E2EKubectlGet) assertions {
	return assertions{Equals: e.Equals, NotEquals: e.NotEquals, OutputContains: e.OutputContains, OutputNotContains: e.OutputNotContains, Regex: e.Regex, GreaterThan: e.GreaterThan, LessThan: e.LessThan, GreaterThanOrEqual: e.GreaterThanOrEqual, LessThanOrEqual: e.LessThanOrEqual, Between: e.Between, NotBetween: e.NotBetween, OneOf: e.OneOf, NotOneOf: e.NotOneOf, Exists: e.Exists, NotExists: e.NotExists}
}

func kubectlLogsAssertions(e types.E2EKubectlLogs) assertions {
	return assertions{Equals: e.Equals, NotEquals: e.NotEquals, OutputContains: e.OutputContains, OutputNotContains: e.OutputNotContains, Regex: e.Regex, GreaterThan: e.GreaterThan, LessThan: e.LessThan, GreaterThanOrEqual: e.GreaterThanOrEqual, LessThanOrEqual: e.LessThanOrEqual, Between: e.Between, NotBetween: e.NotBetween, OneOf: e.OneOf, NotOneOf: e.NotOneOf, Exists: e.Exists, NotExists: e.NotExists}
}

func kubectlDescribeAssertions(e types.E2EKubectlDescribe) assertions {
	return assertions{Equals: e.Equals, NotEquals: e.NotEquals, OutputContains: e.OutputContains, OutputNotContains: e.OutputNotContains, Regex: e.Regex, GreaterThan: e.GreaterThan, LessThan: e.LessThan, GreaterThanOrEqual: e.GreaterThanOrEqual, LessThanOrEqual: e.LessThanOrEqual, Between: e.Between, NotBetween: e.NotBetween, OneOf: e.OneOf, NotOneOf: e.NotOneOf, Exists: e.Exists, NotExists: e.NotExists}
}

func kubectlExecAssertions(e types.E2EKubectlExec) assertions {
	return assertions{Equals: e.Equals, NotEquals: e.NotEquals, OutputContains: e.OutputContains, OutputNotContains: e.OutputNotContains, Regex: e.Regex, GreaterThan: e.GreaterThan, LessThan: e.LessThan, GreaterThanOrEqual: e.GreaterThanOrEqual, LessThanOrEqual: e.LessThanOrEqual, Between: e.Between, NotBetween: e.NotBetween, OneOf: e.OneOf, NotOneOf: e.NotOneOf, Exists: e.Exists, NotExists: e.NotExists}
}

func kubectlPortForwardAssertions(e types.E2EKubectlPortForward) assertions {
	return assertions{Equals: e.Equals, NotEquals: e.NotEquals, OutputContains: e.OutputContains, OutputNotContains: e.OutputNotContains, Regex: e.Regex, GreaterThan: e.GreaterThan, LessThan: e.LessThan, GreaterThanOrEqual: e.GreaterThanOrEqual, LessThanOrEqual: e.LessThanOrEqual, Between: e.Between, NotBetween: e.NotBetween, OneOf: e.OneOf, NotOneOf: e.NotOneOf, Exists: e.Exists, NotExists: e.NotExists}
}

func kubectlEventsAssertions(e types.E2EKubectlEvents) assertions {
	return assertions{Equals: e.Equals, NotEquals: e.NotEquals, OutputContains: e.OutputContains, OutputNotContains: e.OutputNotContains, Regex: e.Regex, GreaterThan: e.GreaterThan, LessThan: e.LessThan, GreaterThanOrEqual: e.GreaterThanOrEqual, LessThanOrEqual: e.LessThanOrEqual, Between: e.Between, NotBetween: e.NotBetween, OneOf: e.OneOf, NotOneOf: e.NotOneOf, Exists: e.Exists, NotExists: e.NotExists}
}

func kubectlAuthAssertions(e types.E2EKubectlAuth) assertions {
	return assertions{Equals: e.Equals, NotEquals: e.NotEquals, OutputContains: e.OutputContains, OutputNotContains: e.OutputNotContains, Regex: e.Regex, GreaterThan: e.GreaterThan, LessThan: e.LessThan, GreaterThanOrEqual: e.GreaterThanOrEqual, LessThanOrEqual: e.LessThanOrEqual, Between: e.Between, NotBetween: e.NotBetween, OneOf: e.OneOf, NotOneOf: e.NotOneOf, Exists: e.Exists, NotExists: e.NotExists}
}

func kubectlCpAssertions(e types.E2EKubectlCp) assertions {
	return assertions{Equals: e.Equals, NotEquals: e.NotEquals, OutputContains: e.OutputContains, OutputNotContains: e.OutputNotContains, Regex: e.Regex, GreaterThan: e.GreaterThan, LessThan: e.LessThan, GreaterThanOrEqual: e.GreaterThanOrEqual, LessThanOrEqual: e.LessThanOrEqual, Between: e.Between, NotBetween: e.NotBetween, OneOf: e.OneOf, NotOneOf: e.NotOneOf, Exists: e.Exists, NotExists: e.NotExists}
}

func kubectlTopAssertions(e types.E2EKubectlTop) assertions {
	return assertions{Equals: e.Equals, NotEquals: e.NotEquals, OutputContains: e.OutputContains, OutputNotContains: e.OutputNotContains, Regex: e.Regex, GreaterThan: e.GreaterThan, LessThan: e.LessThan, GreaterThanOrEqual: e.GreaterThanOrEqual, LessThanOrEqual: e.LessThanOrEqual, Between: e.Between, NotBetween: e.NotBetween, OneOf: e.OneOf, NotOneOf: e.NotOneOf, Exists: e.Exists, NotExists: e.NotExists}
}

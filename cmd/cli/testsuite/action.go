//go:build !runtime && !gateway

package testsuite

import (
	"fmt"

	"github.com/intentd/inrun/pkg/util/fileio"
)

// testAction is what the flags select for the given argument.
type testAction int

const (
	actionRun testAction = iota
	actionValidate
	actionList
	actionDryRun
)

// pickAction chooses what to do from --validate, --list, and --dry-run, for
// arg (a file path or a discovery pattern like ./...). Returns an error when
// the flags don't apply to the argument — --list on a single file, for
// instance, has nothing to list.
func pickAction(arg string, validate, list, dryRun bool) (testAction, error) {
	_, isDiscovery := fileio.IsDiscoveryArg(arg)

	switch {
	case list && !isDiscovery:
		return actionRun, fmt.Errorf("--list applies to ./... discovery, not a single file")
	case validate && list:
		return actionRun, fmt.Errorf("--validate and --list are mutually exclusive")
	case validate && dryRun:
		return actionRun, fmt.Errorf("--validate and --dry-run are mutually exclusive")
	case list && dryRun:
		return actionRun, fmt.Errorf("--list and --dry-run are mutually exclusive")
	case validate:
		return actionValidate, nil
	case list:
		return actionList, nil
	case dryRun:
		return actionDryRun, nil
	default:
		return actionRun, nil
	}
}

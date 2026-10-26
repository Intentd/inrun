package expect

import (
	"fmt"

	"github.com/intentd/inrun/pkg/external"
	"github.com/intentd/inrun/pkg/types"
)

// ValidateExternal checks every external: call across all expect entries —
// each step's, and each one's onFailure and finally — with the validator a
// Blueprint's calls go through. Run it after ExpandIncludes, so included
// calls are checked too.
func ValidateExternal(expects types.E2EExpectations) []error {
	var errs []error
	for i, exp := range expects {
		loc := fmt.Sprintf("expect[%d] %q", i, exp.Name)
		errs = append(errs, ValidateExternalCalls(loc, exp.External)...)
		if exp.OnFailure != nil {
			errs = append(errs, ValidateExternalCalls(loc+" onFailure", exp.OnFailure.External)...)
		}
		if exp.Finally != nil {
			errs = append(errs, ValidateExternalCalls(loc+" finally", exp.Finally.External)...)
		}
	}
	return errs
}

// ValidateExternalCalls checks one external: list; loc prefixes each error.
// Names are required and unique within the list.
func ValidateExternalCalls(loc string, calls []types.E2EExternalCheck) []error {
	var errs []error
	seen := map[string]bool{}
	for j, c := range calls {
		at := fmt.Sprintf("%s external[%d]", loc, j)
		if c.Name == "" {
			errs = append(errs, fmt.Errorf("%s: name is required", at))
			continue
		}
		if seen[c.Name] {
			errs = append(errs, fmt.Errorf("%s: duplicate call name %q", at, c.Name))
		}
		seen[c.Name] = true
		if err := external.ValidateCall(at, c.ExternalCallSpec); err != nil {
			errs = append(errs, err)
		}
	}
	return errs
}

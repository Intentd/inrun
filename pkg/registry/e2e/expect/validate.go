package expect

import (
	"fmt"

	"github.com/intentd/inrun/pkg/external"
	"github.com/intentd/inrun/pkg/types"
)

// ValidateExternal checks every external: call across all expect entries
// with the validator a Blueprint's calls go through. Run it after
// ExpandIncludes, so included calls are checked too.
func ValidateExternal(expects []types.E2EExpectation) []error {
	var errs []error
	for i, exp := range expects {
		seen := map[string]bool{}
		for j, c := range exp.External {
			loc := fmt.Sprintf("expect[%d] %q external[%d]", i, exp.Name, j)
			if c.Name == "" {
				errs = append(errs, fmt.Errorf("%s: name is required", loc))
				continue
			}
			if seen[c.Name] {
				errs = append(errs, fmt.Errorf("%s: duplicate call name %q", loc, c.Name))
			}
			seen[c.Name] = true
			if err := external.ValidateCall(loc, c.ExternalCallSpec); err != nil {
				errs = append(errs, err)
			}
		}
	}
	return errs
}

package e2e

import (
	"fmt"
	"strings"
)

// validateImports is a backstop that delegates to the exported ValidateImports.
// inrun e2e always calls this at startup so malformed imports are caught before
// the cluster is provisioned. inrun validate calls ValidateImports directly for
// earlier, friendlier feedback.
func (r *Runner) validateImports() error {
	errs := ValidateImports(r.e2eDir, r.e2e.Imports)
	if len(errs) == 0 {
		return nil
	}
	msgs := make([]string, len(errs))
	for i, e := range errs {
		msgs[i] = e.Error()
	}
	return fmt.Errorf("invalid imports: %s", strings.Join(msgs, "; "))
}

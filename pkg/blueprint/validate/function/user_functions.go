package function

import (
	"fmt"
	"text/template"

	"github.com/intentd/inrun/pkg/template/funcs"
)

// validateUserFunctions checks the functions: block declared on the Blueprint.
//
// Enforces:
//  1. Each function must have a non-empty name.
//  2. Each function must have a non-empty expression.
//  3. Expression must be a valid Go template.
//  4. No duplicate names.
//  5. Warns when a function name shadows a built-in unless shadow: true is set.
func (c *checker) validateUserFunctions() error {
	reg := c.Blueprint.Functions
	if reg.Empty() {
		return nil
	}

	builtins := funcs.WithKEL()

	// Pre-build stubs for every user-defined function so cross-references resolve
	// during template parsing. Matches runtime behaviour where the full funcmap
	// (built-ins + all user functions) is assembled before any expression runs.
	funcMap := make(template.FuncMap, len(builtins)+len(reg.Definitions))
	for k, v := range builtins {
		funcMap[k] = v
	}
	for _, n := range reg.Definitions {
		funcMap[n.Name] = func() string { return "" }
	}

	seen := make(map[string]bool, len(reg.Definitions))

	for i, n := range reg.Definitions {
		if n.Name == "" {
			return fmt.Errorf("%s functions[%d]: name must not be empty", failureMark(), i)
		}
		if n.Expression == "" {
			return fmt.Errorf("%s functions[%d] %q: expression must not be empty", failureMark(), i, n.Name)
		}
		if seen[n.Name] {
			return fmt.Errorf("%s functions: duplicate function name %q — names must be unique", failureMark(), n.Name)
		}
		seen[n.Name] = true

		if _, err := template.New("").Funcs(funcMap).Parse(n.Expression); err != nil {
			return fmt.Errorf("%s functions %q: invalid expression: %w", failureMark(), n.Name, err)
		}

		if _, isBuiltin := builtins[n.Name]; isBuiltin && !n.Shadow {
			c.Blueprint.Warnings.AddWarning(fmt.Sprintf("functions %q shadows a built-in Inrun function — add `shadow: true` to acknowledge",
				n.Name))
		}
	}
	return nil
}

package types

import (
	"fmt"
	"strings"

	"github.com/intentd/inrun/pkg/sentinel"
)

// UserDefinedFunction is a named template expression declared in a Blueprint or Module.
// Once registered, the function is callable by name in any template expression
// across the Blueprint — status fields, when: conditions, normalize, e2e assertions.
type UserDefinedFunction struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description,omitempty"`
	// Expression is a Go template string evaluated by the Resolver.
	// Built-in functions and other user-defined functions (same Blueprint or imported) are
	// available inside the expression. The result is always a string.
	Expression string `yaml:"expression"`
	// Shadow: true acknowledges that this function intentionally overrides a built-in.
	Shadow bool `yaml:"shadow,omitempty"`
}

// KELFunctionRegistry holds user-defined functions merged from a Blueprint and its spec.imports Modules.
type KELFunctionRegistry struct {
	Include     string                `yaml:"include,omitempty"`
	Definitions []UserDefinedFunction `yaml:"definitions,omitempty"`
}

// Merge merges src into nr. Local (nr) entries override src entries with the same name.
// Two src entries (from different Modules) with the same name are a hard error.
func (nr KELFunctionRegistry) Merge(src KELFunctionRegistry, srcLabel string) (KELFunctionRegistry, error) {
	if len(src.Definitions) == 0 {
		return nr, nil
	}
	existing := make(map[string]bool, len(nr.Definitions))
	for _, n := range nr.Definitions {
		existing[n.Name] = true
	}
	result := KELFunctionRegistry{Definitions: make([]UserDefinedFunction, len(nr.Definitions))}
	copy(result.Definitions, nr.Definitions)
	for _, n := range src.Definitions {
		if existing[n.Name] {
			// Local function with same name — local wins silently (intentional override).
			continue
		}
		existing[n.Name] = true
		result.Definitions = append(result.Definitions, n)
	}
	return result, nil
}

// MergeImport merges src into nr detecting conflicts between two non-local sources.
// Used when accumulating functions from multiple spec.imports Modules.
func (nr KELFunctionRegistry) MergeImport(src KELFunctionRegistry, srcLabel string, seen map[string]string) (KELFunctionRegistry, error) {
	result := KELFunctionRegistry{Definitions: make([]UserDefinedFunction, len(nr.Definitions))}
	copy(result.Definitions, nr.Definitions)
	for _, n := range src.Definitions {
		if prev, seenBefore := seen[n.Name]; seenBefore {
			// The same definition arriving twice (e.g. one functions.include shared by
			// two imported Blueprints) is reuse, not a conflict: keep one copy.
			if existing, ok := result.find(n.Name); ok && existing.Expression == n.Expression && existing.Shadow == n.Shadow {
				continue
			}
			return KELFunctionRegistry{}, fmt.Errorf(
				"function conflict: %q is declared differently in %s and %s — rename one or declare a local override in the Blueprint's functions: block",
				n.Name, prev, srcLabel,
			)
		}
		seen[n.Name] = srcLabel
		result.Definitions = append(result.Definitions, n)
	}
	return result, nil
}

// IsKelFuncRef reports whether field is a user-defined function call — {{ functionName }}
// where functionName does not start with a dot (data ref) and is not a sentinel.
func IsKelFuncRef(field string) bool {
	if !strings.HasPrefix(field, "{{") {
		return false
	}
	inner := strings.TrimSpace(strings.TrimPrefix(field, "{{"))
	if strings.HasPrefix(inner, ".") {
		return false
	}
	name := strings.TrimSpace(strings.TrimSuffix(inner, "}}"))
	return name != "" && !sentinel.IsValid(name)
}

// FuncRefName extracts the bare function name from a {{ functionName }} field expression.
// Assumes IsKelFuncRef returned true for the same field.
func FuncRefName(field string) string {
	inner := strings.TrimSpace(strings.TrimPrefix(field, "{{"))
	return strings.TrimSpace(strings.TrimSuffix(inner, "}}"))
}

// ExpressionFor returns the template expression body for the named function,
// or an empty string if no function with that name exists in the registry.
func (nr KELFunctionRegistry) ExpressionFor(name string) string {
	for _, n := range nr.Definitions {
		if n.Name == name {
			return n.Expression
		}
	}
	return ""
}

// References reports whether expr reads the data root (for example
// "health", "external" or "args.quota"): as a field path ("health.status" or
// ".health.status"), inside a template ("{{ .health.status }}"), or through a
// function the template calls, followed through the functions that function calls.
// Sentinels are never functions. Cycle-safe.
func (nr KELFunctionRegistry) References(expr, root string) bool {
	expr = strings.TrimSpace(expr)
	if expr == "" || root == "" {
		return false
	}
	if !strings.Contains(expr, "{{") {
		path := strings.TrimPrefix(expr, ".")
		return path == root || strings.HasPrefix(path, root+".")
	}
	return nr.templateReferences(expr, root, make(map[string]bool))
}

func (nr KELFunctionRegistry) templateReferences(expr, root string, visited map[string]bool) bool {
	if readsRoot(expr, root) {
		return true
	}
	for _, n := range nr.Definitions {
		if visited[n.Name] || sentinel.IsValid(n.Name) || !isWordIn(expr, n.Name) {
			continue
		}
		visited[n.Name] = true
		if nr.templateReferences(n.Expression, root, visited) {
			return true
		}
	}
	return false
}

// readsRoot reports whether the template expr reads ".root" as the start of a
// path: not part of a longer path (.spec.health) or name (.healthCheck).
func readsRoot(expr, root string) bool {
	needle := "." + root
	for i := strings.Index(expr, needle); i >= 0; {
		end := i + len(needle)
		startsPath := i == 0 || !(isIdentByte(expr[i-1]) || expr[i-1] == '.' || expr[i-1] == ')' || expr[i-1] == ']')
		endsName := end == len(expr) || !isIdentByte(expr[end])
		if startsPath && endsName {
			return true
		}
		next := strings.Index(expr[i+1:], needle)
		if next < 0 {
			break
		}
		i = i + 1 + next
	}
	return false
}

// isWordIn reports whether word appears in s as a whole identifier token —
// not as a substring of a longer identifier on either side.
func isWordIn(s, word string) bool {
	if word == "" {
		return false
	}
	i := strings.Index(s, word)
	for i >= 0 {
		end := i + len(word)
		leftOk := i == 0 || !isIdentByte(s[i-1])
		rightOk := end == len(s) || !isIdentByte(s[end])
		if leftOk && rightOk {
			return true
		}
		next := strings.Index(s[i+1:], word)
		if next < 0 {
			break
		}
		i = i + 1 + next
	}
	return false
}

// isIdentByte reports whether b is a valid Go identifier continuation byte.
func isIdentByte(b byte) bool {
	return b == '_' || (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9')
}

// Empty reports whether the registry contains no functions.
func (nr KELFunctionRegistry) Empty() bool {
	return len(nr.Definitions) == 0
}

// find returns the function named name, if present.
func (nr KELFunctionRegistry) find(name string) (UserDefinedFunction, bool) {
	for _, n := range nr.Definitions {
		if n.Name == name {
			return n, true
		}
	}
	return UserDefinedFunction{}, false
}

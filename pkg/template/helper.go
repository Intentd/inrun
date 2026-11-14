package template

import (
	"fmt"

	"github.com/intentd/inrun/domain"
	"github.com/intentd/inrun/pkg/types"
	"github.com/intentd/tools/resolver"
)

// ResolveMap evaluates template expressions in each value of a map[string]string —
// used for labels, annotations, and match selectors alike.
// Keys are never template expressions — only values are resolved.
//
// Example:
//
//	name: {{ .metadata.name }}
//	app: {{ .metadata.labels.app }}
func (r *Resolver) ResolveMap(m map[string]string) (map[string]string, error) {
	return r.core().ResolveMap(m)
}

// ResolveMapLenient is ResolveMap that never fails: a value that fails to
// resolve, or resolves to nothing, becomes "". Used for labels and
// annotations, where "" is valid and a raw template is not.
func (r *Resolver) ResolveMapLenient(m map[string]string) map[string]string {
	return r.core().ResolveMapLenient(m)
}

// ResolveStringSlice resolves template expressions in each element of a string slice.
// Each element is resolved independently — one failing element does not affect others.
// Used for toNamespaces where each namespace may be a template expression.
//
// Example:
//
//	input:  ["{{ .metadata.namespace }}", "monitoring", "{{ .spec.extraNamespace }}"]
//	output: ["my-app", "monitoring", "platform"]
func (r *Resolver) ResolveStringSlice(values []string) ([]string, error) {
	return r.core().ResolveStringSlice(values)
}

// ResolveArgs resolves a command or args list element by element. Unlike
// ResolveStringSlice it keeps elements that resolve to "", since dropping one
// would shift the arguments that follow it. field names the list in errors.
func (r *Resolver) ResolveArgs(values []string, field string) ([]string, error) {
	return r.core().ResolveArgs(values, field)
}

// ResolveNamespace resolves a child's namespace; an empty one is the CR's.
func (r *Resolver) ResolveNamespace(ns string) (string, error) {
	if ns == "" {
		ns = "{{ .metadata.namespace }}"
	}
	return r.Resolve(ns)
}

// ResolveEnvVarsLenient resolves each env var's Value; Name is copied as-is.
// A value that fails to resolve becomes "".
func (r *Resolver) ResolveEnvVarsLenient(vars []types.EnvVar) []types.EnvVar {
	if len(vars) == 0 {
		return vars
	}
	out := make([]types.EnvVar, 0, len(vars))
	for _, v := range vars {
		rv, _ := r.Resolve(v.Value)
		out = append(out, types.EnvVar{Name: v.Name, Value: rv})
	}
	return out
}

// ResolveEnvFromRefs resolves Name/Prefix/Suffix template expressions in each
// EnvFromRef. Keys and Optional are not template expressions — copied as-is.
func (r *Resolver) ResolveEnvFromRefs(refs []types.EnvFromRef, field string) ([]types.EnvFromRef, error) {
	if len(refs) == 0 {
		return nil, nil
	}
	resolved := make([]types.EnvFromRef, 0, len(refs))
	for _, ref := range refs {
		name, err := r.Resolve(ref.Name)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", field, err)
		}
		prefix, err := r.Resolve(ref.Prefix)
		if err != nil {
			return nil, fmt.Errorf("%s: prefix: %w", field, err)
		}
		suffix, err := r.Resolve(ref.Suffix)
		if err != nil {
			return nil, fmt.Errorf("%s: suffix: %w", field, err)
		}
		resolved = append(resolved, types.EnvFromRef{
			Name:     name,
			Prefix:   prefix,
			Suffix:   suffix,
			Optional: ref.Optional,
			Keys:     ref.Keys,
		})
	}
	return resolved, nil
}

// ObjectToMap converts any domain.Object to the map templates see (see
// resolver.ObjectToMap): .metadata, .spec and .status read the same for typed
// and unstructured objects.
func ObjectToMap(obj domain.Object) (map[string]interface{}, error) {
	return resolver.ObjectToMap(obj)
}

// ResolveRaw returns the raw value at the path in a "{{ .a.b }}" expression,
// keeping its type ([]interface{}, map, string, float64, bool), or nil when
// the path does not exist. Used where a template names a list, such as
// toNamespaces: "{{ .spec.targetNamespaces }}".
func (r *Resolver) ResolveRaw(expr string) interface{} {
	return r.core().ResolveRaw(expr)
}

// ValidResolverName reports whether name is a valid normalized resolver name.
//
// Resolver names use a simple camelCase-style convention:
//   - must be non-empty
//   - must start with an ASCII letter
//   - may contain ASCII letters, digits, and underscores
//   - must not contain spaces, punctuation, Unicode characters, or other
//     special characters
func ValidResolverName(name string) error {
	if name == "" {
		return fmt.Errorf("event name must not be empty")
	}

	for i, r := range name {
		if i == 0 {
			if !isASCIILetter(r) {
				return fmt.Errorf(
					"event name %q must start with a letter",
					name,
				)
			}
			continue
		}

		if isASCIILetter(r) || isASCIIDigit(r) || r == '_' {
			continue
		}

		return fmt.Errorf(
			"event name %q contains invalid character %q at position %d; only letters, digits, and underscores are allowed",
			name,
			r,
			i,
		)
	}

	return nil
}

func isASCIILetter(r rune) bool {
	return (r >= 'a' && r <= 'z') ||
		(r >= 'A' && r <= 'Z')
}

func isASCIIDigit(r rune) bool {
	return r >= '0' && r <= '9'
}

package structured

import "sort"

// ToStringSet converts a slice of strings to a map[string]bool for O(1) lookups.
//
// Useful for:
//   - Permission checks (e.g., "is this operation in the allowed list?")
//   - Deduplication checks
//   - Set operations (intersection, subset, difference)
//
// Returns an empty map when ops is nil or empty.
func ToStringSet(ops []string) map[string]bool {
	s := make(map[string]bool, len(ops))
	for _, op := range ops {
		s[op] = true
	}
	return s
}

// SetContains is a nil-safe struct{} map membership check.
func SetContains(s map[string]struct{}, key string) bool {
	_, ok := s[key]
	return ok
}

// MapContains is a nil-safe map membership check for any value type.
func MapContains[V any](m map[string]V, key string) bool {
	if m == nil {
		return false
	}
	_, ok := m[key]
	return ok
}

// SortedKeys returns a sorted slice of all keys from the given map.
//
// The keys are sorted in ascending lexicographical order (standard string ordering).
// This is useful for:
//   - Producing deterministic output from map iteration (which is nondeterministic in Go)
//   - Displaying keys in a consistent order in logs, error messages, or user interfaces
//   - Comparing two maps for equality by comparing their sorted key slices
//   - Generating stable test output
//
// Returns an empty slice (not nil) for nil or empty maps.
// The function handles any map with string keys and arbitrary value types.
//
// Example:
//
//	m := map[string]int{"b": 2, "a": 1, "c": 3}
//	keys := SortedKeys(m)  // returns ["a", "b", "c"]
func SortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// DedupStrings removes duplicate strings from a slice while preserving the original order
// (first occurrence is kept). Returns a new slice without modifying the input.
func DedupStrings(ss []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(ss))
	for _, s := range ss {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}

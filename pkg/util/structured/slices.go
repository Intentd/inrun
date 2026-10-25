package structured

// Reversed reverses any given slice
func Reversed[T any](s []T) []T {
	out := make([]T, len(s))
	for i := range s {
		out[len(s)-1-i] = s[i]
	}
	return out
}

// Intersect returns the elements of a that are also in b, preserving a's order.
func Intersect(a, b []string) []string {
	set := make(map[string]bool, len(b))
	for _, v := range b {
		set[v] = true
	}
	var out []string
	for _, v := range a {
		if set[v] {
			out = append(out, v)
		}
	}
	return out
}

// UnionStrings returns a ∪ b, deduplicated, preserving a's order then b's.
func UnionStrings(a, b []string) []string {
	seen := make(map[string]bool, len(a)+len(b))
	var out []string
	for _, list := range [][]string{a, b} {
		for _, v := range list {
			if !seen[v] {
				seen[v] = true
				out = append(out, v)
			}
		}
	}
	return out
}

// IsSubset reports whether every element of a is in b.
func IsSubset(a, b []string) bool {
	set := make(map[string]bool, len(b))
	for _, v := range b {
		set[v] = true
	}
	for _, v := range a {
		if !set[v] {
			return false
		}
	}
	return true
}

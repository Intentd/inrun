package structured

// Reversed reverses any given slice
func Reversed[T any](s []T) []T {
	out := make([]T, len(s))
	for i := range s {
		out[len(s)-1-i] = s[i]
	}
	return out
}

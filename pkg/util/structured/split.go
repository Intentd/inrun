package structured

import (
	"strings"
)

// SplitBySeparator splits a string by the given separator into a slice of trimmed strings.
// Empty elements are omitted from the result.
//
// Examples:
//
//	SplitBySeparator("a,b,c", ",")           // → ["a", "b", "c"]
//	SplitBySeparator("a, b, c", ",")         // → ["a", "b", "c"]
//	SplitBySeparator("a,,b", ",")            // → ["a", "b"]
//	SplitBySeparator("a|b|c", "|")           // → ["a", "b", "c"]
//	SplitBySeparator("a | b | c", "|")       // → ["a", "b", "c"]
//	SplitBySeparator("", ",")                // → [] (empty slice)
//	SplitBySeparator("  ", ",")              // → [] (empty slice)
//	SplitBySeparator("a, , b", ",")          // → ["a", "b"]
func SplitBySeparator(s, separator string) []string {
	if s == "" || separator == "" {
		return []string{}
	}

	parts := strings.Split(s, separator)
	result := make([]string, 0, len(parts))

	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}

	return result
}

// SplitCommaSeparated splits a comma-separated string into a slice of trimmed strings.
// Empty elements are omitted from the result.
//
// Examples:
//
//	SplitCommaSeparated("a,b,c")        // → ["a", "b", "c"]
//	SplitCommaSeparated("a, b, c")      // → ["a", "b", "c"]
//	SplitCommaSeparated("a,,b")         // → ["a", "b"]
//	SplitCommaSeparated("")             // → [] (empty slice)
//	SplitCommaSeparated("  ")           // → [] (empty slice)
//	SplitCommaSeparated("a, , b")       // → ["a", "b"]
func SplitCommaSeparated(s string) []string {
	return SplitBySeparator(s, ",")
}

// SplitCommaList flattens repeated string flags, each of which may hold
// comma-separated values, into a single list of trimmed, non-empty values.
// Useful for flags like --skip, --include, or --tag that are repeatable and
// accept comma-separated entries.
//
// Examples:
//
//	SplitCommaList([]string{"a,b", " c ,, d "}) // → ["a", "b", "c", "d"]
//	SplitCommaList([]string{"a", "b", "c"})      // → ["a", "b", "c"]
//	SplitCommaList(nil)                          // → [] (empty slice)
//	SplitCommaList([]string{"", "  "})           // → [] (empty slice)
func SplitCommaList(values []string) []string {
	out := []string{}
	for _, v := range values {
		out = append(out, SplitCommaSeparated(v)...)
	}
	return out
}

// SplitPipeSeparated splits a pipe-separated string into a slice of trimmed strings.
// Empty elements are omitted from the result.
//
// Examples:
//
//	SplitPipeSeparated("a|b|c")        // → ["a", "b", "c"]
//	SplitPipeSeparated("a | b | c")    // → ["a", "b", "c"]
//	SplitPipeSeparated("a||b")         // → ["a", "b"]
func SplitPipeSeparated(s string) []string {
	return SplitBySeparator(s, "|")
}

// SplitColonSeparated splits a colon-separated string into a slice of trimmed strings.
// Empty elements are omitted from the result.
//
// Examples:
//
//	SplitColonSeparated("a:b:c")        // → ["a", "b", "c"]
//	SplitColonSeparated("a: b: c")      // → ["a", "b", "c"]
func SplitColonSeparated(s string) []string {
	return SplitBySeparator(s, ":")
}

// SplitSemicolonSeparated splits a semicolon-separated string into a slice of trimmed strings.
// Empty elements are omitted from the result.
//
// Examples:
//
//	SplitSemicolonSeparated("a;b;c")        // → ["a", "b", "c"]
//	SplitSemicolonSeparated("a; b; c")      // → ["a", "b", "c"]
func SplitSemicolonSeparated(s string) []string {
	return SplitBySeparator(s, ";")
}

// SplitField splits a dot-notation field path into keys. A leading dot is
// ignored, and `\.` is a literal dot inside a key:
//
//	metadata.labels.app\.kubernetes\.io/name → [metadata labels app.kubernetes.io/name]
func SplitField(field string) []string {
	field = strings.TrimPrefix(field, ".")
	if field == "" {
		return nil
	}
	if !strings.Contains(field, `\.`) {
		return strings.Split(field, ".")
	}
	var parts []string
	var key strings.Builder
	for i := 0; i < len(field); i++ {
		switch {
		case field[i] == '\\' && i+1 < len(field) && field[i+1] == '.':
			key.WriteByte('.')
			i++
		case field[i] == '.':
			parts = append(parts, key.String())
			key.Reset()
		default:
			key.WriteByte(field[i])
		}
	}
	return append(parts, key.String())
}

// EscapePathKey escapes the dots in one key, so SplitField keeps it whole:
// "app.kubernetes.io/name" → `app\.kubernetes\.io/name`.
func EscapePathKey(key string) string {
	return strings.ReplaceAll(key, ".", `\.`)
}

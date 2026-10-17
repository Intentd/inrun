package note

import (
	"strings"
	"text/template"
)

func stringNotes() template.FuncMap {
	return template.FuncMap{
		"toLower":      strings.ToLower,
		"toUpper":      strings.ToUpper,
		"trimSpace":    strings.TrimSpace,
		"trim":         strings.Trim,
		"trimPrefix":   strings.TrimPrefix,
		"trimSuffix":   strings.TrimSuffix,
		"hasPrefix":    strings.HasPrefix,
		"hasSuffix":    strings.HasSuffix,
		"contains":     strings.Contains,
		"replace":      strings.ReplaceAll,
		"split":        strSplit,
		"join":         strings.Join,
		"repeat":       strings.Repeat,
		"camelToKebab": camelToKebab,
		"truncate":     strTruncate,
		"concat":       strConcat,
	}
}

// strSplit splits s by sep. Returns empty slice for empty input.
// Compose with index to extract specific elements:
//
//	{{ index (split .spec.tags ",") 0 }}   →  first tag
func strSplit(s, sep string) []string {
	if s == "" {
		return []string{}
	}
	return strings.Split(s, sep)
}

// camelToKebab converts CamelCase or PascalCase to kebab-case.
//
//	{{ camelToKebab "WebsiteOperator" }}   →  "website-operator"
//	{{ camelToKebab "myAppName" }}         →  "my-app-name"
//	{{ camelToKebab "HTTPRequest" }}         →  "http-request"
func camelToKebab(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		// Convert to lowercase
		lower := c
		if c >= 'A' && c <= 'Z' {
			lower = c - 'A' + 'a'
		}
		// Add hyphen before uppercase if:
		// - it's not the first character,
		// - and the previous character is lowercase (word boundary),
		// - or the previous character is uppercase but the next is lowercase (acronym boundary).
		if i > 0 && c >= 'A' && c <= 'Z' {
			prev := s[i-1]
			next := byte(0)
			if i+1 < len(s) {
				next = s[i+1]
			}
			// Insert hyphen when:
			//   prev is lowercase (new word)
			//   OR prev is uppercase and next is lowercase (end of acronym)
			if (prev >= 'a' && prev <= 'z') || (prev >= 'A' && prev <= 'Z' && next >= 'a' && next <= 'z') {
				b.WriteByte('-')
			}
		}
		b.WriteByte(lower)
	}
	return b.String()
}

// strTruncate truncates s to at most n characters, appending "..." if truncated.
// Label values in Kubernetes have a 63-character limit — use this to enforce it.
//
//	{{ truncate .metadata.name 63 }}
func strTruncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	if n <= 3 {
		return s[:n]
	}
	return s[:n-3] + "..."
}

// strConcat joins all arguments into a single string with no separator.
//
//	{{ concat "*." .spec.domain }}   →  "*.api.example.com"
func strConcat(parts ...string) string {
	var b strings.Builder
	for _, p := range parts {
		b.WriteString(p)
	}
	return b.String()
}

// helper
// join concatenates the elements of a slice into a single string,
// separated by the provided separator.
// placed here to be reused across all notes
func join(slice []string, sep string) string {
	return strings.Join(slice, sep)
}

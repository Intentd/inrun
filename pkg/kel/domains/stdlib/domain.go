package note

import (
	"strings"
	"text/template"
)

// domainNotes registers helpers for cleaning protocol-prefixed domain/URL strings
// into bare hostnames. Useful in normalize blocks where spec fields may contain
// "https://acme.com/", "http://acme.com", or just "acme.com".
//
// Usage:
//
//	tmpl.Funcs(note.domainNotes())
//
// Template examples:
//
//	{{ domainHost .spec.domain }}   → "acme.example.com"
//	{{ domainBare .spec.domain }}   → "example.com"  (also strips www.)
//
// Equivalent long-form chains:
//
//	domainHost: {{ trimSuffix (trimPrefix (trimPrefix (trimSpace .spec.domain) "https://") "http://") "/" }}
//	domainBare: {{ trimPrefix (domainHost .spec.domain) "www." }}
//
// Both functions are nil-safe and return "" for non-string input.
func domainNotes() template.FuncMap {
	return template.FuncMap{
		"domainHost": noteDomainHost,
		"domainBare": noteDomainBare,
	}
}

// noteDomainHost strips the protocol prefix (http:// or https://), trims
// surrounding whitespace, and removes a trailing slash. Returns "" for nil
// or non-string input.
//
//	{{ domainHost "https://acme.example.com/" }}  → "acme.example.com"
//	{{ domainHost " http://acme.example.com " }}  → "acme.example.com"
//	{{ domainHost "acme.example.com" }}           → "acme.example.com"
func noteDomainHost(v interface{}) string {
	if v == nil {
		return ""
	}
	s, ok := v.(string)
	if !ok {
		return ""
	}
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "https://")
	s = strings.TrimPrefix(s, "http://")
	s = strings.TrimSuffix(s, "/")
	return s
}

// noteDomainBare strips the protocol prefix, whitespace, trailing slash, and
// a leading "www." subdomain. Returns "" for nil or non-string input.
//
//	{{ domainBare "https://www.acme.com/" }}  → "acme.com"
//	{{ domainBare "https://acme.com/" }}      → "acme.com"
//	{{ domainBare "www.acme.com" }}           → "acme.com"
func noteDomainBare(v interface{}) string {
	s := noteDomainHost(v)
	if s == "" {
		return ""
	}
	return strings.TrimPrefix(s, "www.")
}

package web

import (
	"fmt"
	"html/template"
	"strings"
	"time"
)

func init() {
	for k, v := range crTemplateFuncs {
		templateFuncs[k] = v
	}
}

// templateFuncs provides helper functions for HTML templates
var templateFuncs = template.FuncMap{
	"mul": func(a, b int) int {
		return a * b
	},
	"mulFloat": func(a, b float64) float64 {
		return a * b
	},
	"div": func(a, b int) int {
		if b == 0 {
			return 0
		}
		return a / b
	},
	"sub": func(a, b int) int {
		return a - b
	},
	"title": strings.Title,
	"truncate": func(s string, n int) string {
		if len(s) <= n {
			return s
		}
		return s[:n-3] + "..."
	},
	// formatNumber formats large numbers with K/M/B suffixes
	"formatNumber": func(n int) string {
		if n >= 1_000_000_000 {
			return fmt.Sprintf("%.1fB", float64(n)/1_000_000_000)
		}
		if n >= 1_000_000 {
			return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
		}
		if n >= 10_000 {
			return fmt.Sprintf("%.1fK", float64(n)/1_000)
		}
		return fmt.Sprintf("%d", n)
	},
	"add": func(a, b int) int {
		return a + b
	},
	"min": func(a, b int) int {
		if a < b {
			return a
		}
		return b
	},
	// format a time string to a readable format
	"formatTime": func(timeStr string) string {
		if timeStr == "" {
			return "never"
		}
		t, err := time.Parse(time.RFC3339, timeStr)
		if err != nil {
			return timeStr
		}
		return t.Format("2006-01-02 15:04:05")
	},
	// join a slice of strings with a separator
	"join": func(sep string, items []string) string {
		if len(items) == 0 {
			return ""
		}
		return strings.Join(items, sep)
	},
	// Helper to check if a value is in a slice (for dependency states)
	"contains": func(slice []string, item string) bool {
		for _, s := range slice {
			if s == item {
				return true
			}
		}
		return false
	},
	// default returns the first non-empty value
	"default": func(def interface{}, val interface{}) interface{} {
		// Check if val is nil
		if val == nil {
			return def
		}

		// Check for empty string
		if s, ok := val.(string); ok && s == "" {
			return def
		}

		// Check for zero int
		if i, ok := val.(int); ok && i == 0 {
			return def
		}

		// Check for empty slice
		if slice, ok := val.([]interface{}); ok && len(slice) == 0 {
			return def
		}

		// Check for empty map
		if m, ok := val.(map[string]interface{}); ok && len(m) == 0 {
			return def
		}

		return val
	},
	// mapGet retrieves a key from an interface{} that is a map[string]interface{}.
	// Returns nil when the value is not a map or the key is absent.
	// Used in templates to safely navigate OperatorBox and other JSON-decoded maps.
	"mapGet": func(m interface{}, key string) interface{} {
		if m == nil {
			return nil
		}
		if mm, ok := m.(map[string]interface{}); ok {
			return mm[key]
		}
		return nil
	},
	// asBool coerces an interface{} to bool.
	"asBool": func(v interface{}) bool {
		if b, ok := v.(bool); ok {
			return b
		}
		return false
	},
	// asString coerces an interface{} to string.
	"asString": func(v interface{}) string {
		if s, ok := v.(string); ok {
			return s
		}
		return ""
	},
	// asSlice coerces an interface{} to []interface{}.
	"asSlice": func(v interface{}) []interface{} {
		if s, ok := v.([]interface{}); ok {
			return s
		}
		return nil
	},
	// asMap coerces an interface{} to map[string]interface{}.
	"asMap": func(v interface{}) map[string]interface{} {
		if m, ok := v.(map[string]interface{}); ok {
			return m
		}
		return nil
	},
	// hasPrefix checks if a string has a given prefix
	"hasPrefix": func(s, prefix string) bool {
		return strings.HasPrefix(s, prefix)
	},
	// json marshals a value to JSON string
	"json": func(v interface{}) string {
		return fmt.Sprintf("%v", v)
	},
	"toLower": func(s string) string {
		return strings.ToLower(s)
	},
	// phaseBadge returns safe HTML for a phase badge without CSS variables in template actions
	"phaseBadge": func(phase string) template.HTML {
		switch {
		case phase == "Succeeded":
			return template.HTML(`<span class="console-badge console-badge-healthy">✓ ` + template.HTMLEscapeString(phase) + `</span>`)
		case phase == "Failed":
			return template.HTML(`<span class="console-badge console-badge-degraded">✗ ` + template.HTMLEscapeString(phase) + `</span>`)
		case strings.HasPrefix(phase, "Running"):
			return template.HTML(`<span class="console-badge console-badge-started">◌ ` + template.HTMLEscapeString(phase) + `</span>`)
		case phase == "Pending":
			return template.HTML(`<span class="console-badge console-badge-pending">◷ ` + template.HTMLEscapeString(phase) + `</span>`)
		default:
			return template.HTML(`<span class="console-badge console-badge-neutral">` + template.HTMLEscapeString(phase) + `</span>`)
		}
	},
}

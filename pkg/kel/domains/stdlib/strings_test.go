// pkg/note/string_test.go
package note

import "testing"

func TestStrSplit(t *testing.T) {
	tests := []struct {
		name string
		s    string
		sep  string
		want []string
	}{
		{"empty string", "", ",", []string{}},
		{"single element", "a", ",", []string{"a"}},
		{"multiple", "a,b,c", ",", []string{"a", "b", "c"}},
		{"with empty elements", "a,,b", ",", []string{"a", "", "b"}},
		{"no sep", "abc", "", []string{"a", "b", "c"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := strSplit(tt.s, tt.sep)
			if len(got) != len(tt.want) {
				t.Errorf("strSplit(%q, %q) length = %d, want %d", tt.s, tt.sep, len(got), len(tt.want))
				return
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("strSplit(%q, %q) = %v, want %v", tt.s, tt.sep, got, tt.want)
					return
				}
			}
		})
	}
}

func TestCamelToKebab(t *testing.T) {
	tests := []struct {
		name string
		s    string
		want string
	}{
		{"simple Pascal", "WebsiteOperator", "website-operator"},
		{"camelCase", "myAppName", "my-app-name"},
		{"all caps", "HTTPRequest", "http-request"},
		{"single word", "Hello", "hello"},
		{"already kebab", "my-app", "my-app"},
		{"empty", "", ""},
		{"with numbers", "AppV2", "app-v2"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := camelToKebab(tt.s); got != tt.want {
				t.Errorf("camelToKebab(%q) = %q, want %q", tt.s, got, tt.want)
			}
		})
	}
}

func TestStrTruncate(t *testing.T) {
	tests := []struct {
		name string
		s    string
		n    int
		want string
	}{
		{"shorter than limit", "hello", 10, "hello"},
		{"equal to limit", "12345", 5, "12345"},
		{"longer than limit", "hello world", 8, "hello..."},
		{"limit less than 3", "abcde", 2, "ab"},
		{"limit 3, exact", "abc", 3, "abc"},
		{"limit 3, longer", "abcd", 3, "abc"},
		{"empty string", "", 5, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := strTruncate(tt.s, tt.n); got != tt.want {
				t.Errorf("strTruncate(%q, %d) = %q, want %q", tt.s, tt.n, got, tt.want)
			}
		})
	}
}

func TestStringNotes(t *testing.T) {
	// Test that all registered functions work as expected.
	fm := stringNotes()

	// toLower
	if fn, ok := fm["toLower"].(func(string) string); ok {
		if got := fn("HELLO"); got != "hello" {
			t.Errorf("toLower(HELLO) = %q, want hello", got)
		}
	} else {
		t.Error("toLower not registered")
	}

	// toUpper
	if fn, ok := fm["toUpper"].(func(string) string); ok {
		if got := fn("hello"); got != "HELLO" {
			t.Errorf("toUpper(hello) = %q, want HELLO", got)
		}
	} else {
		t.Error("toUpper not registered")
	}

	// trimSpace
	if fn, ok := fm["trimSpace"].(func(string) string); ok {
		if got := fn("  hi  "); got != "hi" {
			t.Errorf("trimSpace() = %q, want hi", got)
		}
	} else {
		t.Error("trimSpace not registered")
	}

	// trim
	if fn, ok := fm["trim"].(func(string, string) string); ok {
		if got := fn("!hello!", "!"); got != "hello" {
			t.Errorf("trim() = %q, want hello", got)
		}
	} else {
		t.Error("trim not registered")
	}

	// trimPrefix
	if fn, ok := fm["trimPrefix"].(func(string, string) string); ok {
		if got := fn("prefix-suffix", "prefix-"); got != "suffix" {
			t.Errorf("trimPrefix() = %q, want suffix", got)
		}
	} else {
		t.Error("trimPrefix not registered")
	}

	// trimSuffix
	if fn, ok := fm["trimSuffix"].(func(string, string) string); ok {
		if got := fn("prefix-suffix", "-suffix"); got != "prefix" {
			t.Errorf("trimSuffix() = %q, want prefix", got)
		}
	} else {
		t.Error("trimSuffix not registered")
	}

	// hasPrefix
	if fn, ok := fm["hasPrefix"].(func(string, string) bool); ok {
		if !fn("prefix-suffix", "prefix") {
			t.Error("hasPrefix() should be true")
		}
	} else {
		t.Error("hasPrefix not registered")
	}

	// hasSuffix
	if fn, ok := fm["hasSuffix"].(func(string, string) bool); ok {
		if !fn("prefix-suffix", "suffix") {
			t.Error("hasSuffix() should be true")
		}
	} else {
		t.Error("hasSuffix not registered")
	}

	// contains
	if fn, ok := fm["contains"].(func(string, string) bool); ok {
		if !fn("hello world", "world") {
			t.Error("contains() should be true")
		}
	} else {
		t.Error("contains not registered")
	}

	// replace
	if fn, ok := fm["replace"].(func(string, string, string) string); ok {
		if got := fn("foo bar foo", "foo", "baz"); got != "baz bar baz" {
			t.Errorf("replace() = %q, want 'baz bar baz'", got)
		}
	} else {
		t.Error("replace not registered")
	}

	// split
	if fn, ok := fm["split"].(func(string, string) []string); ok {
		got := fn("a,b,c", ",")
		if len(got) != 3 || got[0] != "a" || got[1] != "b" || got[2] != "c" {
			t.Errorf("split() = %v, want [a b c]", got)
		}
	} else {
		t.Error("split not registered")
	}

	// join
	if fn, ok := fm["join"].(func([]string, string) string); ok {
		got := fn([]string{"a", "b", "c"}, "-")
		if got != "a-b-c" {
			t.Errorf("join() = %q, want a-b-c", got)
		}
	} else {
		t.Error("join not registered")
	}

	// repeat
	if fn, ok := fm["repeat"].(func(string, int) string); ok {
		got := fn("ab", 3)
		if got != "ababab" {
			t.Errorf("repeat() = %q, want ababab", got)
		}
	} else {
		t.Error("repeat not registered")
	}

	// camelToKebab and truncate are tested separately.
}

func TestStrConcat(t *testing.T) {
	tests := []struct {
		name  string
		parts []string
		want  string
	}{
		{"no parts", []string{}, ""},
		{"single part", []string{"hello"}, "hello"},
		{"two parts", []string{"*.", "api.example.com"}, "*.api.example.com"},
		{"three parts", []string{"webapp", "-", "prod"}, "webapp-prod"},
		{"empty part in middle", []string{"a", "", "b"}, "ab"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := strConcat(tt.parts...); got != tt.want {
				t.Errorf("strConcat(%v) = %q, want %q", tt.parts, got, tt.want)
			}
		})
	}
}

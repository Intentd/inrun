// pkg/note/data_test.go
package note

import (
	"encoding/base64"
	"testing"
)

func TestNoteToBase64(t *testing.T) {
	tests := []struct {
		name string
		s    string
		want string
	}{
		{"empty", "", ""},
		{"simple", "hello", base64.StdEncoding.EncodeToString([]byte("hello"))},
		{"with special chars", "a/b+c", base64.StdEncoding.EncodeToString([]byte("a/b+c"))},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := noteToBase64(tt.s); got != tt.want {
				t.Errorf("noteToBase64(%q) = %q, want %q", tt.s, got, tt.want)
			}
		})
	}
}

func TestNoteFromBase64(t *testing.T) {
	tests := []struct {
		name string
		s    string
		want string
	}{
		{"empty", "", ""},
		{"valid", base64.StdEncoding.EncodeToString([]byte("hello")), "hello"},
		{"invalid", "!!!", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := noteFromBase64(tt.s); got != tt.want {
				t.Errorf("noteFromBase64(%q) = %q, want %q", tt.s, got, tt.want)
			}
		})
	}
}

func TestNoteToJSON(t *testing.T) {
	tests := []struct {
		name string
		v    interface{}
		want string
	}{
		{"map", map[string]int{"a": 1}, `{"a":1}`},
		{"slice", []string{"x", "y"}, `["x","y"]`},
		{"string", "hello", `"hello"`},
		{"nil", nil, "null"},
		{"struct", struct{ A int }{42}, `{"A":42}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := noteToJSON(tt.v); got != tt.want {
				t.Errorf("noteToJSON(%v) = %q, want %q", tt.v, got, tt.want)
			}
		})
	}
}

func TestNoteSHA256Sum(t *testing.T) {
	tests := []struct {
		name    string
		s       string
		wantLen int
	}{
		{"empty", "", 8},
		{"hello", "hello", 8},
		{"long", "this is a longer string that is hashed", 8},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := noteSHA256Sum(tt.s)
			if len(got) != tt.wantLen {
				t.Errorf("noteSHA256Sum(%q) length = %d, want %d", tt.s, len(got), tt.wantLen)
			}
			// deterministic
			if got != noteSHA256Sum(tt.s) {
				t.Errorf("noteSHA256Sum not deterministic: %q != %q", got, noteSHA256Sum(tt.s))
			}
		})
	}
}

func TestNoteTruncate(t *testing.T) {
	tests := []struct {
		name   string
		s      string
		maxLen int
		want   string
	}{
		{"shorter", "hello", 10, "hello"},
		{"exact", "world", 5, "world"},
		{"longer", "hello world", 5, "hello"},
		{"zero length", "test", 0, ""},
		{"empty", "", 10, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := noteTruncate(tt.s, tt.maxLen); got != tt.want {
				t.Errorf("noteTruncate(%q, %d) = %q, want %q", tt.s, tt.maxLen, got, tt.want)
			}
		})
	}
}

func TestNoteSlugify(t *testing.T) {
	tests := []struct {
		name string
		s    string
		want string
	}{
		{"empty", "", ""},
		{"simple", "My App", "my-app"},
		{"with slash", "My App / Service", "my-app-service"},
		{"multiple spaces", "a   b", "a-b"},
		{"leading dash", "-hello-", "hello"},
		{"special chars", "foo@bar$baz", "foo-bar-baz"},
		{"underscores", "hello_world", "hello-world"},
		{"numbers", "app123", "app123"},
		{"already slug", "my-app", "my-app"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := noteSlugify(tt.s); got != tt.want {
				t.Errorf("noteSlugify(%q) = %q, want %q", tt.s, got, tt.want)
			}
		})
	}
}

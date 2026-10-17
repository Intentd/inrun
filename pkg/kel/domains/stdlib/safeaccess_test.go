// pkg/note/safeaccess_test.go
package note

import (
	"reflect"
	"testing"
)

func TestNoteGetOr(t *testing.T) {
	tests := []struct {
		name string
		val  interface{}
		def  interface{}
		want interface{}
	}{
		{"non‑empty string", "hello", "default", "hello"},
		{"empty string", "", "default", "default"},
		{"zero int", 0, 42, 42},
		{"non‑zero int", 5, 42, 5},
		{"nil", nil, "fallback", "fallback"},
		{"false bool", false, true, true},
		{"true bool", true, false, true},
		{"empty slice", []interface{}{}, "default", "default"},
		{"non‑empty slice", []interface{}{1, 2}, "default", []interface{}{1, 2}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := noteGetOr(tt.val, tt.def); !deepEqual(got, tt.want) {
				t.Errorf("noteGetOr(%v, %v) = %v, want %v", tt.val, tt.def, got, tt.want)
			}
		})
	}
}

func TestNoteGetStringOr(t *testing.T) {
	tests := []struct {
		name string
		val  interface{}
		def  string
		want string
	}{
		{"valid string", "foo", "default", "foo"},
		{"empty string", "", "default", "default"},
		{"non‑string", 123, "default", "default"},
		{"nil", nil, "default", "default"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := noteGetStringOr(tt.val, tt.def); got != tt.want {
				t.Errorf("noteGetStringOr(%v, %q) = %q, want %q", tt.val, tt.def, got, tt.want)
			}
		})
	}
}

func TestNoteGetIntOr(t *testing.T) {
	tests := []struct {
		name string
		val  interface{}
		def  int
		want int
	}{
		{"int", 42, 100, 42},
		{"int64", int64(99), 100, 99},
		{"float64", 3.14, 100, 3},
		{"string", "42", 100, 100},
		{"nil", nil, 100, 100},
		{"empty string", "", 100, 100},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := noteGetIntOr(tt.val, tt.def); got != tt.want {
				t.Errorf("noteGetIntOr(%v, %d) = %d, want %d", tt.val, tt.def, got, tt.want)
			}
		})
	}
}

func TestNoteGetBoolOr(t *testing.T) {
	tests := []struct {
		name string
		val  interface{}
		def  bool
		want bool
	}{
		{"bool true", true, false, true},
		{"bool false", false, true, false},
		{"int 1", 1, false, false},
		{"string true", "true", false, false},
		{"nil", nil, true, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := noteGetBoolOr(tt.val, tt.def); got != tt.want {
				t.Errorf("noteGetBoolOr(%v, %v) = %v, want %v", tt.val, tt.def, got, tt.want)
			}
		})
	}
}

func deepEqual(a, b interface{}) bool {
	return reflect.DeepEqual(a, b)
}

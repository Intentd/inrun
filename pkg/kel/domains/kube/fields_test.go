// pkg/note/kube_fields_test.go
package note

import (
	"testing"
)

func TestNoteResourceName(t *testing.T) {
	tests := []struct {
		name string
		obj  interface{}
		want string
	}{
		{"valid", map[string]interface{}{"metadata": map[string]interface{}{"name": "my-app"}}, "my-app"},
		{"missing name", map[string]interface{}{"metadata": map[string]interface{}{}}, ""},
		{"missing metadata", map[string]interface{}{}, ""},
		{"not a map", "string", ""},
		{"nil", nil, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := noteResourceName(tt.obj); got != tt.want {
				t.Errorf("noteResourceName() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNoteResourceNamespace(t *testing.T) {
	tests := []struct {
		name string
		obj  interface{}
		want string
	}{
		{"valid", map[string]interface{}{"metadata": map[string]interface{}{"namespace": "default"}}, "default"},
		{"missing namespace", map[string]interface{}{"metadata": map[string]interface{}{}}, ""},
		{"missing metadata", map[string]interface{}{}, ""},
		{"not a map", "string", ""},
		{"nil", nil, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := noteResourceNamespace(tt.obj); got != tt.want {
				t.Errorf("noteResourceNamespace() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNoteResourceUID(t *testing.T) {
	tests := []struct {
		name string
		obj  interface{}
		want string
	}{
		{"valid", map[string]interface{}{"metadata": map[string]interface{}{"uid": "abc-123"}}, "abc-123"},
		{"missing uid", map[string]interface{}{"metadata": map[string]interface{}{}}, ""},
		{"missing metadata", map[string]interface{}{}, ""},
		{"not a map", "string", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := noteResourceUID(tt.obj); got != tt.want {
				t.Errorf("noteResourceUID() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNoteResourceVersion(t *testing.T) {
	tests := []struct {
		name string
		obj  interface{}
		want string
	}{
		{"valid", map[string]interface{}{"metadata": map[string]interface{}{"resourceVersion": "12345"}}, "12345"},
		{"missing", map[string]interface{}{"metadata": map[string]interface{}{}}, ""},
		{"missing metadata", map[string]interface{}{}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := noteResourceVersion(tt.obj); got != tt.want {
				t.Errorf("noteResourceVersion() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNoteCreationTimestamp(t *testing.T) {
	tests := []struct {
		name string
		obj  interface{}
		want string
	}{
		{"valid", map[string]interface{}{"metadata": map[string]interface{}{"creationTimestamp": "2024-01-01T00:00:00Z"}}, "2024-01-01T00:00:00Z"},
		{"missing", map[string]interface{}{"metadata": map[string]interface{}{}}, ""},
		{"missing metadata", map[string]interface{}{}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := noteCreationTimestamp(tt.obj); got != tt.want {
				t.Errorf("noteCreationTimestamp() = %q, want %q", got, tt.want)
			}
		})
	}
}

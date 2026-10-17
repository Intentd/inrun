// pkg/note/as_test.go
package note

import (
	"testing"
)

func TestAsList(t *testing.T) {
	tests := []struct {
		name  string
		input any
		want  []any
	}{
		{
			name:  "already slice",
			input: []any{1, "two", 3.0},
			want:  []any{1, "two", 3.0},
		},
		{
			name:  "YAML list string",
			input: "- one\n- two\n- three",
			want:  []any{"one", "two", "three"},
		},
		{
			name:  "JSON array string",
			input: `["a","b","c"]`,
			want:  []any{"a", "b", "c"},
		},
		{
			name:  "invalid YAML/JSON string",
			input: "not a list",
			want:  []any{},
		},
		{
			name:  "empty string",
			input: "",
			want:  []any{},
		},
		{
			name:  "non-string non-slice",
			input: 42,
			want:  []any{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := asList(tt.input)
			if len(got) != len(tt.want) {
				t.Errorf("asList() length = %d, want %d", len(got), len(tt.want))
				return
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("asList() at index %d = %v, want %v", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestAsMap(t *testing.T) {
	tests := []struct {
		name  string
		input any
		want  map[string]any
	}{
		{
			name:  "already map",
			input: map[string]any{"key": "value", "num": 123},
			want:  map[string]any{"key": "value", "num": 123},
		},
		{
			name:  "YAML map string",
			input: "key: value\nnum: 123",
			want:  map[string]any{"key": "value", "num": 123},
		},
		{
			name:  "JSON object string",
			input: `{"key":"value","num":123}`,
			want:  map[string]any{"key": "value", "num": 123},
		},
		{
			name:  "invalid YAML/JSON string",
			input: "not a map",
			want:  map[string]any{},
		},
		{
			name:  "empty string",
			input: "",
			want:  map[string]any{},
		},
		{
			name:  "non-string non-map",
			input: []int{1, 2},
			want:  map[string]any{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := asMap(tt.input)
			if len(got) != len(tt.want) {
				t.Errorf("asMap() length = %d, want %d", len(got), len(tt.want))
				return
			}
			for k, v := range tt.want {
				if gv, ok := got[k]; !ok || gv != v {
					t.Errorf("asMap()[%q] = %v, want %v", k, gv, v)
				}
			}
		})
	}
}

func TestAsString(t *testing.T) {
	tests := []struct {
		name  string
		input any
		want  string
	}{
		{
			name:  "string input",
			input: "hello",
			want:  "hello",
		},
		{
			name:  "byte slice",
			input: []byte("world"),
			want:  "world",
		},
		{
			name:  "integer",
			input: 123,
			want:  "123",
		},
		{
			name:  "float",
			input: 45.67,
			want:  "45.67",
		},
		{
			name:  "bool true",
			input: true,
			want:  "true",
		},
		{
			name:  "bool false",
			input: false,
			want:  "false",
		},
		{
			name:  "nil",
			input: nil,
			want:  "null",
		},
		{
			name:  "map (JSON)",
			input: map[string]int{"a": 1},
			want:  `{"a":1}`,
		},
		{
			name:  "slice (JSON)",
			input: []string{"x", "y"},
			want:  `["x","y"]`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := asString(tt.input)
			if got != tt.want {
				t.Errorf("asString(%v) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

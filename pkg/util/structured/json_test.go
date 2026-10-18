package structured

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestJSONNormalize(t *testing.T) {
	cases := []struct {
		name string
		in   interface{}
		want interface{}
	}{
		// ── numeric types collapse to float64 ────────────────────────────
		{"int", 42, float64(42)},
		{"int64", int64(42), float64(42)},
		{"int32", int32(42), float64(42)},
		{"uint", uint(42), float64(42)},
		{"float32", float32(1.5), float64(1.5)},
		{"float64", 1.5, float64(1.5)},
		{"negative int", -7, float64(-7)},
		{"zero", 0, float64(0)},

		// ── non-numeric scalars pass through ─────────────────────────────
		{"string", "hello", "hello"},
		{"bool true", true, true},
		{"bool false", false, false},
		{"nil", nil, nil},

		// ── containers ───────────────────────────────────────────────────
		{
			name: "map with int values",
			in:   map[string]interface{}{"replicas": 3, "port": 8080},
			want: map[string]interface{}{"replicas": float64(3), "port": float64(8080)},
		},
		{
			name: "nested map",
			in:   map[string]interface{}{"scaling": map[string]interface{}{"min": 2}},
			want: map[string]interface{}{"scaling": map[string]interface{}{"min": float64(2)}},
		},
		{
			name: "slice of ints",
			in:   []interface{}{1, 2, 3},
			want: []interface{}{float64(1), float64(2), float64(3)},
		},
		{
			name: "empty map",
			in:   map[string]interface{}{},
			want: map[string]interface{}{},
		},
		{
			name: "empty slice",
			in:   []interface{}{},
			want: []interface{}{},
		},

		// ── the comparison this function exists for ──────────────────────
		{
			name: "int64 from API server matches float64 from JSON",
			in:   int64(5),
			want: float64(5),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := JSONNormalize(tc.in)
			assert.Equal(t, tc.want, got)
		})
	}
}

// TestJSONNormalize_EqualAfterNormalize is the property the migrator relies
// on: values that came from different sources compare equal after
// normalization even when their Go types differ.
func TestJSONNormalize_EqualAfterNormalize(t *testing.T) {
	fromAPI := map[string]interface{}{
		"replicas": int64(3),
		"image":    "nginx:1.25",
	}
	fromAnnotation := map[string]interface{}{
		"replicas": float64(3),
		"image":    "nginx:1.25",
	}

	// Without normalization, the types differ.
	assert.NotEqual(t, fromAPI, fromAnnotation)

	// With normalization, they match.
	assert.Equal(t,
		JSONNormalize(fromAPI),
		JSONNormalize(fromAnnotation),
	)
}

// TestJSONNormalize_UnmarshalableInputReturnsOriginal documents the fallback:
// when the value cannot be marshalled (channels, funcs, cycles), the original
// is returned unchanged rather than nil or a partial value.
func TestJSONNormalize_UnmarshalableInputReturnsOriginal(t *testing.T) {
	ch := make(chan int)
	got := JSONNormalize(ch)

	// Channels can't marshal, so the original value comes back.
	assert.Equal(t, ch, got)
}

func BenchmarkJSONNormalize(b *testing.B) {
	v := map[string]interface{}{"replicas": int64(3), "image": "nginx:1.25"}

	for b.Loop() {
		JSONNormalize(v)
	}
}

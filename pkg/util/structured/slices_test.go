package structured

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestReversed_Empty(t *testing.T) {
	out := Reversed([]int{})
	if len(out) != 0 {
		t.Error("reversed empty slice must be empty")
	}
}

func TestReversed_Single(t *testing.T) {
	out := Reversed([]string{"a"})
	if len(out) != 1 || out[0] != "a" {
		t.Errorf("unexpected: %v", out)
	}
}

func TestReversed_Multiple(t *testing.T) {
	in := []int{1, 2, 3, 4, 5}
	out := Reversed(in)
	want := []int{5, 4, 3, 2, 1}
	for i, v := range want {
		if out[i] != v {
			t.Errorf("index %d: expected %d, got %d", i, v, out[i])
		}
	}
	// Original must be unchanged
	if in[0] != 1 {
		t.Error("Reversed must not modify the original slice")
	}
}

func TestReversed_Strings(t *testing.T) {
	out := Reversed([]string{"a", "b", "c"})
	if out[0] != "c" || out[1] != "b" || out[2] != "a" {
		t.Errorf("unexpected: %v", out)
	}
}

func TestIntersect(t *testing.T) {
	cases := []struct {
		name string
		a    []string
		b    []string
		want []string
	}{
		// ── basic intersection ───────────────────────────────────────────
		{"overlap", []string{"a", "b", "c"}, []string{"b", "c", "d"}, []string{"b", "c"}},
		{"identical", []string{"a", "b"}, []string{"a", "b"}, []string{"a", "b"}},
		{"disjoint", []string{"a", "b"}, []string{"c", "d"}, nil},
		{"a subset of b", []string{"a"}, []string{"a", "b", "c"}, []string{"a"}},
		{"b subset of a", []string{"a", "b", "c"}, []string{"a"}, []string{"a"}},

		// ── emptiness ────────────────────────────────────────────────────
		{"both empty", []string{}, []string{}, nil},
		{"a empty", []string{}, []string{"a", "b"}, nil},
		{"b empty", []string{"a", "b"}, []string{}, nil},
		{"both nil", nil, nil, nil},
		{"a nil", nil, []string{"a"}, nil},
		{"b nil", []string{"a"}, nil, nil},

		// ── order is a's order ───────────────────────────────────────────
		{"order preserved from a", []string{"c", "a", "b"}, []string{"a", "b", "c"}, []string{"c", "a", "b"}},
		{"order from a, not b", []string{"z", "y", "x"}, []string{"x", "y", "z"}, []string{"z", "y", "x"}},

		// ── duplicates ───────────────────────────────────────────────────
		{"duplicates in a preserved", []string{"a", "a", "b"}, []string{"a", "b"}, []string{"a", "a", "b"}},
		{"duplicates in b collapsed", []string{"a", "b"}, []string{"a", "a", "b"}, []string{"a", "b"}},

		// ── namespaces-shaped data (the actual use case) ─────────────────
		{
			"namespace intersection",
			[]string{"platform-01", "platform-02", "platform-03"},
			[]string{"platform-02", "platform-03", "platform-04"},
			[]string{"platform-02", "platform-03"},
		},
		{
			"no namespace overlap",
			[]string{"platform-01"},
			[]string{"platform-02"},
			nil,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Intersect(tc.a, tc.b)
			assert.Equal(t, tc.want, got)
		})
	}
}

// ── UnionStrings ─────────────────────────────────────────────────────────────

func TestUnionStrings(t *testing.T) {
	cases := []struct {
		name string
		a    []string
		b    []string
		want []string
	}{
		// ── basic union ──────────────────────────────────────────────────
		{"overlap", []string{"a", "b"}, []string{"b", "c"}, []string{"a", "b", "c"}},
		{"identical", []string{"a", "b"}, []string{"a", "b"}, []string{"a", "b"}},
		{"disjoint", []string{"a", "b"}, []string{"c", "d"}, []string{"a", "b", "c", "d"}},
		{"a subset of b", []string{"a"}, []string{"a", "b"}, []string{"a", "b"}},
		{"b subset of a", []string{"a", "b"}, []string{"a"}, []string{"a", "b"}},

		// ── emptiness ────────────────────────────────────────────────────
		{"both empty", []string{}, []string{}, nil},
		{"a empty", []string{}, []string{"a", "b"}, []string{"a", "b"}},
		{"b empty", []string{"a", "b"}, []string{}, []string{"a", "b"}},
		{"both nil", nil, nil, nil},
		{"a nil", nil, []string{"a"}, []string{"a"}},
		{"b nil", []string{"a"}, nil, []string{"a"}},

		// ── order: a first, then b's new elements ────────────────────────
		{"order a then b", []string{"c", "a"}, []string{"b", "a"}, []string{"c", "a", "b"}},
		{"order preserved within a", []string{"z", "y", "x"}, []string{"w"}, []string{"z", "y", "x", "w"}},

		// ── duplicates ───────────────────────────────────────────────────
		{"duplicates in a collapsed", []string{"a", "a", "b"}, []string{"c"}, []string{"a", "b", "c"}},
		{"duplicates in b collapsed", []string{"a"}, []string{"a", "a", "b"}, []string{"a", "b"}},
		{"duplicates across both collapsed", []string{"a", "a"}, []string{"a", "a"}, []string{"a"}},

		// ── namespace-shaped (the actual use case) ───────────────────────
		{
			"namespace union",
			[]string{"platform-01"},
			[]string{"platform-01", "platform-02"},
			[]string{"platform-01", "platform-02"},
		},
		{
			"disjoint namespaces",
			[]string{"platform-01"},
			[]string{"platform-02"},
			[]string{"platform-01", "platform-02"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := UnionStrings(tc.a, tc.b)
			assert.Equal(t, tc.want, got)
		})
	}
}

// ── IsSubset ─────────────────────────────────────────────────────────────────

func TestIsSubset(t *testing.T) {
	cases := []struct {
		name string
		a    []string
		b    []string
		want bool
	}{
		// ── basic subset ─────────────────────────────────────────────────
		{"true subset", []string{"a"}, []string{"a", "b", "c"}, true},
		{"equal sets", []string{"a", "b"}, []string{"a", "b"}, true},
		{"not a subset", []string{"a", "z"}, []string{"a", "b"}, false},
		{"disjoint", []string{"x", "y"}, []string{"a", "b"}, false},
		{"superset is not subset", []string{"a", "b", "c"}, []string{"a"}, false},

		// ── emptiness: the empty set is a subset of everything ───────────
		{"empty a, non-empty b", []string{}, []string{"a"}, true},
		{"nil a, non-empty b", nil, []string{"a"}, true},
		{"empty a, empty b", []string{}, []string{}, true},
		{"both nil", nil, nil, true},
		{"non-empty a, empty b", []string{"a"}, []string{}, false},
		{"non-empty a, nil b", []string{"a"}, nil, false},

		// ── duplicates don't affect subset semantics ─────────────────────
		{"duplicates in a", []string{"a", "a", "b"}, []string{"a", "b"}, true},
		{"duplicates in b", []string{"a", "b"}, []string{"a", "a", "b"}, true},

		// ── namespace-shaped (the actual use case) ───────────────────────
		{
			"cli namespaces subset of declared",
			[]string{"platform-01"},
			[]string{"platform-01", "platform-02"},
			true,
		},
		{
			"cli namespaces widen declared",
			[]string{"platform-01", "platform-03"},
			[]string{"platform-01", "platform-02"},
			false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := IsSubset(tc.a, tc.b)
			assert.Equal(t, tc.want, got)
		})
	}
}

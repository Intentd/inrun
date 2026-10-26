package types

import (
	"os"
	"reflect"
	"sort"
	"testing"
)

// withEnv sets key=value for the duration of the test, restoring the prior
// value (or unsetting it) on cleanup.
func withEnv(t *testing.T, key, value string) {
	t.Helper()
	old, had := os.LookupEnv(key)
	os.Setenv(key, value)
	t.Cleanup(func() {
		if had {
			os.Setenv(key, old)
		} else {
			os.Unsetenv(key)
		}
	})
}

func TestEnvResolver_Lookup(t *testing.T) {
	withEnv(t, "INRUN_TEST_PROCESS", "from-process")

	spec := &E2ESpec{Env: map[string]string{
		"INRUN_TEST_SPEC":     "from-spec",
		"INRUN_TEST_OVERRIDE": "from-spec",
	}}
	exp := E2EExpectation{Env: map[string]string{
		"INRUN_TEST_EXPECT":   "from-expect",
		"INRUN_TEST_OVERRIDE": "from-expect",
	}}
	r := spec.ResolveEnv(exp)

	cases := []struct {
		key  string
		want string
	}{
		{"INRUN_TEST_EXPECT", "from-expect"},   // expectation env
		{"INRUN_TEST_SPEC", "from-spec"},       // spec env
		{"INRUN_TEST_OVERRIDE", "from-expect"}, // expectation wins over spec
		{"INRUN_TEST_PROCESS", "from-process"}, // process env fallback
		{"INRUN_TEST_MISSING", ""},             // absent → empty, no panic
	}
	for _, tc := range cases {
		if got := r.Lookup(tc.key); got != tc.want {
			t.Fatalf("Lookup(%q) = %q, want %q", tc.key, got, tc.want)
		}
	}
}

func TestEnvResolver_LookupNilResolverFallsThrough(t *testing.T) {
	withEnv(t, "INRUN_TEST_PROCESS", "from-process")

	var r EnvResolver // zero value
	if got := r.Lookup("INRUN_TEST_PROCESS"); got != "from-process" {
		t.Fatalf("zero Lookup = %q, want from-process", got)
	}
}

func TestEnvResolver_LookupNilSpec(t *testing.T) {
	withEnv(t, "INRUN_TEST_PROCESS", "from-process")

	var spec *E2ESpec
	r := spec.ResolveEnv(E2EExpectation{Env: map[string]string{"A": "exp"}})

	if got := r.Lookup("A"); got != "exp" {
		t.Fatalf("Lookup(A) = %q, want exp", got)
	}
	if got := r.Lookup("INRUN_TEST_PROCESS"); got != "from-process" {
		t.Fatalf("Lookup(process) = %q, want from-process", got)
	}
}

func TestEnvResolver_Pairs(t *testing.T) {
	spec := &E2ESpec{Env: map[string]string{"A": "spec", "B": "spec"}}
	exp := E2EExpectation{Env: map[string]string{"B": "exp", "C": "exp"}}
	r := spec.ResolveEnv(exp)

	got := r.Pairs()
	sort.Strings(got)

	want := []string{"A=spec", "B=exp", "C=exp"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Pairs() = %v, want %v", got, want)
	}
}

func TestEnvResolver_PairsExcludesProcessEnv(t *testing.T) {
	withEnv(t, "INRUN_TEST_PROCESS", "from-process")

	r := (&E2ESpec{Env: map[string]string{"A": "1"}}).ResolveEnv(E2EExpectation{})
	for _, p := range r.Pairs() {
		if p == "INRUN_TEST_PROCESS=from-process" {
			t.Fatalf("Pairs() leaked the process env: %v", r.Pairs())
		}
	}
}

func TestEnvResolver_PairsEmpty(t *testing.T) {
	r := (&E2ESpec{}).ResolveEnv(E2EExpectation{})
	if got := r.Pairs(); len(got) != 0 {
		t.Fatalf("Pairs() = %v, want empty", got)
	}

	var zero EnvResolver
	if got := zero.Pairs(); len(got) != 0 {
		t.Fatalf("zero Pairs() = %v, want empty", got)
	}
}

func TestE2ESpec_InheritEnv(t *testing.T) {
	cases := []struct {
		name   string
		spec   map[string]string
		parent map[string]string
		want   map[string]string
	}{
		{
			name:   "nil spec, nil parent stays nil",
			spec:   nil,
			parent: nil,
			want:   nil,
		},
		{
			name:   "nil parent leaves spec unchanged",
			spec:   map[string]string{"A": "1"},
			parent: nil,
			want:   map[string]string{"A": "1"},
		},
		{
			name:   "empty parent leaves nil spec nil",
			spec:   nil,
			parent: map[string]string{},
			want:   nil,
		},
		{
			name:   "nil spec inherits parent",
			spec:   nil,
			parent: map[string]string{"A": "1", "B": "2"},
			want:   map[string]string{"A": "1", "B": "2"},
		},
		{
			name:   "disjoint keys are unioned",
			spec:   map[string]string{"A": "1"},
			parent: map[string]string{"B": "2"},
			want:   map[string]string{"A": "1", "B": "2"},
		},
		{
			name:   "leaf wins on collision",
			spec:   map[string]string{"A": "leaf"},
			parent: map[string]string{"A": "parent", "B": "2"},
			want:   map[string]string{"A": "leaf", "B": "2"},
		},
		{
			name:   "leaf empty value still wins over parent",
			spec:   map[string]string{"A": ""},
			parent: map[string]string{"A": "parent"},
			want:   map[string]string{"A": ""},
		},
		{
			name:   "parent empty value inherited when leaf absent",
			spec:   map[string]string{},
			parent: map[string]string{"A": ""},
			want:   map[string]string{"A": ""},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := &E2ESpec{Env: tc.spec}
			s.InheritEnv(tc.parent)

			if !reflect.DeepEqual(s.Env, tc.want) {
				t.Fatalf("Env = %#v, want %#v", s.Env, tc.want)
			}
		})
	}
}

func TestE2ESpec_InheritEnvDoesNotMutateParent(t *testing.T) {
	parent := map[string]string{"A": "parent"}
	s := &E2ESpec{Env: map[string]string{"B": "leaf"}}
	s.InheritEnv(parent)

	if !reflect.DeepEqual(parent, map[string]string{"A": "parent"}) {
		t.Fatalf("parent mutated: %#v", parent)
	}
	if !reflect.DeepEqual(s.Env, map[string]string{"A": "parent", "B": "leaf"}) {
		t.Fatalf("Env = %#v", s.Env)
	}
}

func TestE2ESpec_InheritEnvIdempotent(t *testing.T) {
	parent := map[string]string{"A": "parent", "B": "parent"}
	s := &E2ESpec{Env: map[string]string{"A": "leaf"}}

	s.InheritEnv(parent)
	first := map[string]string{"A": "leaf", "B": "parent"}
	if !reflect.DeepEqual(s.Env, first) {
		t.Fatalf("first InheritEnv: Env = %#v, want %#v", s.Env, first)
	}

	// Second call must not change anything — the leaf still wins.
	s.InheritEnv(parent)
	if !reflect.DeepEqual(s.Env, first) {
		t.Fatalf("second InheritEnv changed Env: %#v, want %#v", s.Env, first)
	}
}

package types

import (
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/intentd/tools/cluster"
	"github.com/intentd/tools/resolver/env"
	"github.com/intentd/tools/structured"
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
		if got, _ := r.Lookup(tc.key); got != tc.want {
			t.Fatalf("Lookup(%q) = %q, want %q", tc.key, got, tc.want)
		}
	}
}

func TestEnvResolver_LookupNilResolverFallsThrough(t *testing.T) {
	withEnv(t, "INRUN_TEST_PROCESS", "from-process")

	var r EnvResolver // zero value
	if got, _ := r.Lookup("INRUN_TEST_PROCESS"); got != "from-process" {
		t.Fatalf("zero Lookup = %q, want from-process", got)
	}
}

func TestEnvResolver_LookupNilSpec(t *testing.T) {
	withEnv(t, "INRUN_TEST_PROCESS", "from-process")

	var spec *E2ESpec
	r := spec.ResolveEnv(E2EExpectation{Env: map[string]string{"A": "exp"}})

	if got, _ := r.Lookup("A"); got != "exp" {
		t.Fatalf("Lookup(A) = %q, want exp", got)
	}
	if got, _ := r.Lookup("INRUN_TEST_PROCESS"); got != "from-process" {
		t.Fatalf("Lookup(process) = %q, want from-process", got)
	}
}

// A nil spec resolves to the expectation's env alone. The merge itself is
// jig's (spec.Expectation.ResolveEnv).
func TestE2ESpec_ResolveEnvNilSpec(t *testing.T) {
	var spec *E2ESpec
	got := spec.ResolveEnv(E2EExpectation{Env: map[string]string{"A": "exp"}}).Vars()
	if !reflect.DeepEqual(got, env.Vars{"A": "exp"}) {
		t.Fatalf("Vars() = %v, want only the expectation's", got)
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

func TestEnvResolver_LookupEnv(t *testing.T) {
	t.Setenv("INRUN_TEST_PROCESS", "from-process")
	r := (&E2ESpec{Env: map[string]string{"EMPTY": ""}}).ResolveEnv(E2EExpectation{})
	if v, ok := r.Lookup("EMPTY"); !ok || v != "" {
		t.Errorf("declared empty: got %q, %v; want \"\", true", v, ok)
	}
	if v, ok := r.Lookup("INRUN_TEST_PROCESS"); !ok || v != "from-process" {
		t.Errorf("process: got %q, %v", v, ok)
	}
	if _, ok := r.Lookup("INRUN_TEST_UNSET"); ok {
		t.Error("unset: want false")
	}
}

func TestEnvResolver_Expand(t *testing.T) {
	t.Setenv("INRUN_TEST_PROCESS", "p")
	r := (&E2ESpec{Env: map[string]string{"A": "a"}}).ResolveEnv(E2EExpectation{Env: map[string]string{"B": "b"}})
	cases := map[string]string{
		"${A}-${B}-${INRUN_TEST_PROCESS}": "a-b-p",
		"${INRUN_TEST_UNSET}":             "${INRUN_TEST_UNSET}",
		`{"$patch":"delete"}`:             `{"$patch":"delete"}`,
		"pa$$word $A":                     "pa$$word $A",
	}
	for in, want := range cases {
		if got := r.Expand(in); got != want {
			t.Errorf("Expand(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestEffectiveLoadImages(t *testing.T) {
	tests := []struct {
		name      string
		env       *E2EEnvironment
		cli       []string
		want      []string
		wantErr   bool
		wantInErr string
	}{
		{
			name: "nil environment, no cli",
			env:  nil,
			cli:  nil,
			want: nil,
		},
		{
			name: "nil environment, cli only",
			env:  nil,
			cli:  []string{"app:1"},
			want: []string{"app:1"},
		},
		{
			name: "nil environment, cli deduped",
			env:  nil,
			cli:  []string{"app:1", "app:1", " "},
			want: []string{"app:1"},
		},
		{
			name: "schema only",
			env:  &E2EEnvironment{Images: cluster.Images{{Image: "app:1"}}},
			cli:  nil,
			want: []string{"app:1"},
		},
		{
			name: "schema and cli subset",
			env: &E2EEnvironment{Images: cluster.Images{
				{Image: "app:1"},
				{Image: "sidecar:1"},
			}},
			cli:  []string{"app:1"},
			want: []string{"app:1"},
		},
		{
			name:      "cli widens the schema",
			env:       &E2EEnvironment{Images: cluster.Images{{Image: "app:1"}}},
			cli:       []string{"other:1"},
			wantErr:   true,
			wantInErr: `--images "other:1" is not declared in the schema`,
		},
		{
			name: "build entry resolves to its image reference",
			env: &E2EEnvironment{Images: cluster.Images{
				{Image: "built:1", Build: &cluster.Build{Context: "./app"}},
			}},
			cli:  nil,
			want: []string{"built:1"},
		},
		{
			name: "build entry without image gets the default name",
			env: &E2EEnvironment{Images: cluster.Images{
				{Build: &cluster.Build{Context: "./app"}},
			}},
			cli:  nil,
			want: []string{"jig-myapp-0"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := tc.env.EffectiveLoadImages("myapp", tc.cli)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if tc.wantInErr != "" && !strings.Contains(err.Error(), tc.wantInErr) {
					t.Errorf("error = %q, want it to contain %q", err, tc.wantInErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !structured.StringSliceEqual(got, tc.want) {
				t.Errorf("images = %v, want %v", got, tc.want)
			}
		})
	}
}

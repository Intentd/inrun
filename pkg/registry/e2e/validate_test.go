package e2e

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/intentd/inrun/pkg/types"
	"github.com/intentd/tools/cluster"
)

func validSpec() *types.E2ESpec {
	return &types.E2ESpec{
		Environment: &types.E2EEnvironment{
			Cluster: types.E2ECluster{Provider: types.ProviderKind},
		},
	}
}

func TestValidateCluster(t *testing.T) {
	cases := []struct {
		name      string
		spec      *types.E2ESpec
		wantErrs  int
		wantInErr []string // substrings expected across the errors
	}{
		{
			name:     "nil spec",
			spec:     nil,
			wantErrs: 0,
		},
		{
			name:     "kind with defaults",
			spec:     validSpec(),
			wantErrs: 0,
		},
		{
			name:     "empty provider defaults to kind",
			spec:     &types.E2ESpec{Environment: &types.E2EEnvironment{Cluster: types.E2ECluster{}}},
			wantErrs: 0,
		},
		{
			name:     "unknown provider",
			spec:     &types.E2ESpec{Environment: &types.E2EEnvironment{Cluster: types.E2ECluster{Provider: "eks"}}},
			wantErrs: 1,
			wantInErr: []string{
				`cluster.provider "eks" is not supported`,
				"kind, none",
			},
		},
		{
			name: "none with no operator and no cluster checks",
			spec: &types.E2ESpec{
				Environment: &types.E2EEnvironment{Cluster: types.E2ECluster{Provider: types.ProviderNone}},
			},
			wantErrs: 0,
		},
		{
			name: "none with a blueprint",
			spec: &types.E2ESpec{
				Environment: &types.E2EEnvironment{Cluster: types.E2ECluster{Provider: types.ProviderNone}},
				Blueprint:   "blueprint.yaml",
			},
			wantErrs: 1,
			wantInErr: []string{
				"cannot install an operator",
				"blueprint",
			},
		},
		{
			name: "none with a crd",
			spec: &types.E2ESpec{
				Environment: &types.E2EEnvironment{Cluster: types.E2ECluster{Provider: types.ProviderNone}},
				CRD:         "crd.yaml",
			},
			wantErrs:  1,
			wantInErr: []string{"cannot install an operator"},
		},
		{
			name: "none with crdFiles",
			spec: &types.E2ESpec{
				Environment: &types.E2EEnvironment{Cluster: types.E2ECluster{Provider: types.ProviderNone}},
				CRDFiles:    []string{"a.yaml", "b.yaml"},
			},
			wantErrs:  1,
			wantInErr: []string{"cannot install an operator"},
		},
		{
			name: "none with crFiles",
			spec: &types.E2ESpec{
				Environment: &types.E2EEnvironment{Cluster: types.E2ECluster{Provider: types.ProviderNone}},
				CRFiles:     []string{"cr.yaml"},
			},
			wantErrs:  1,
			wantInErr: []string{"cannot install an operator"},
		},
		{
			name: "none with kubectl expectation",
			spec: &types.E2ESpec{
				Environment: &types.E2EEnvironment{Cluster: types.E2ECluster{Provider: types.ProviderNone}},
				Expect: []types.E2EExpectation{
					{Name: "check pod", Kubectl: &types.E2EKubectl{}},
				},
			},
			wantErrs: 1,
			wantInErr: []string{
				"expect[0]",
				"kubectl requires a cluster",
			},
		},
		{
			name: "none with resources expectation",
			spec: &types.E2ESpec{
				Environment: &types.E2EEnvironment{Cluster: types.E2ECluster{Provider: types.ProviderNone}},
				Expect: []types.E2EExpectation{
					{Name: "check res", Resources: []types.E2EResourceCheck{{Kind: "Pod"}}},
				},
			},
			wantErrs: 1,
			wantInErr: []string{
				"expect[0]",
				"resources requires a cluster",
			},
		},
		{
			name: "none with external expectation",
			spec: &types.E2ESpec{
				Environment: &types.E2EEnvironment{Cluster: types.E2ECluster{Provider: types.ProviderNone}},
				Expect: []types.E2EExpectation{
					{Name: "call api", External: []types.E2EExternalCheck{{Name: "x"}}},
				},
			},
			wantErrs: 1,
			wantInErr: []string{
				"expect[0]",
				"external requires a cluster",
			},
		},
		{
			name: "none with commands only — valid",
			spec: &types.E2ESpec{
				Environment: &types.E2EEnvironment{Cluster: types.E2ECluster{Provider: types.ProviderNone}},
				Expect: []types.E2EExpectation{
					{Name: "local", Commands: []types.E2ECommand{{Run: "true"}}},
				},
			},
			wantErrs: 0,
		},
		{
			name: "none accumulates every violation",
			spec: &types.E2ESpec{
				Environment: &types.E2EEnvironment{Cluster: types.E2ECluster{Provider: types.ProviderNone}},
				Blueprint:   "blueprint.yaml",
				Expect: []types.E2EExpectation{
					{Name: "a", Kubectl: &types.E2EKubectl{}},
					{Name: "b", Resources: []types.E2EResourceCheck{{Kind: "Pod"}}},
					{Name: "c", External: []types.E2EExternalCheck{{Name: "x"}}},
				},
			},
			wantErrs: 4, // blueprint + 3 expectations
			wantInErr: []string{
				"cannot install an operator",
				"expect[0]", "kubectl",
				"expect[1]", "resources",
				"expect[2]", "external",
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			errs := ValidateCluster(tc.spec)
			if len(errs) != tc.wantErrs {
				t.Fatalf("got %d errors, want %d: %v", len(errs), tc.wantErrs, errs)
			}
			if len(tc.wantInErr) == 0 {
				return
			}
			joined := errorsText(errs)
			for _, want := range tc.wantInErr {
				if !strings.Contains(joined, want) {
					t.Fatalf("errors missing %q:\n%s", want, joined)
				}
			}
		})
	}
}

func errorsText(errs []error) string {
	var b strings.Builder
	for _, err := range errs {
		b.WriteString(err.Error())
		b.WriteString("\n")
	}
	return b.String()
}

// Build paths are checked as Compose resolves them: the context against the
// suite's directory, the dockerfile against the context.
func TestValidatePaths_ImageBuild(t *testing.T) {
	dir := t.TempDir()
	for _, f := range []string{"app/Dockerfile", "app/docker/Dockerfile.dev", "Dockerfile.root"} {
		path := filepath.Join(dir, f)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}

	cases := []struct {
		name  string
		build cluster.Build
		want  string // substring of the single error; "" means valid
	}{
		{"default dockerfile in the context", cluster.Build{Context: "./app"}, ""},
		{"dockerfile relative to the context", cluster.Build{Context: "./app", Dockerfile: "docker/Dockerfile.dev"}, ""},
		{"dockerfile outside the context", cluster.Build{Context: "./app", Dockerfile: "../Dockerfile.root"}, ""},
		{"absolute dockerfile", cluster.Build{Context: "./app", Dockerfile: filepath.Join(dir, "Dockerfile.root")}, ""},
		{
			// Relative to the suite directory it exists, relative to the context it does not.
			"dockerfile is not resolved against the suite directory",
			cluster.Build{Context: "./app", Dockerfile: "Dockerfile.root"},
			`spec.environment.images[0].build.dockerfile "` + filepath.Join(dir, "app", "Dockerfile.root") + `"`,
		},
		{
			"missing context names the field and the path as written",
			cluster.Build{Context: "./nope", Dockerfile: filepath.Join(dir, "Dockerfile.root")},
			`spec.environment.images[0].build.context "./nope"`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			b := tc.build
			spec := &types.E2ESpec{Environment: &types.E2EEnvironment{
				Images: cluster.Images{{Image: "app:1", Build: &b}},
			}}
			errs := ValidatePaths(dir, spec)
			if tc.want == "" {
				if len(errs) != 0 {
					t.Fatalf("errs = %v, want none", errs)
				}
				return
			}
			if len(errs) != 1 || !strings.Contains(errs[0].Error(), tc.want) {
				t.Fatalf("errs = %v, want one containing %s", errs, tc.want)
			}
		})
	}
}

func TestValidatePaths_PrepareFieldsAreNamedPrepare(t *testing.T) {
	spec := &types.E2ESpec{Prepare: &types.Prepare{Provision: types.Provision{
		Apply: []types.ProvisionApplyEntry{{Path: "./missing.yaml"}},
	}}}
	errs := ValidatePaths(t.TempDir(), spec)
	if len(errs) != 1 || !strings.Contains(errs[0].Error(), "spec.prepare.apply[0].path") {
		t.Fatalf("errs = %v, want one naming spec.prepare.apply[0].path", errs)
	}
}

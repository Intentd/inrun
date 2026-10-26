package e2e

import (
	"strings"
	"testing"

	"github.com/intentd/inrun/pkg/types"
)

func validSpec() *types.E2ESpec {
	return &types.E2ESpec{
		Cluster: types.E2ECluster{Provider: types.ProviderKind},
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
			spec:     &types.E2ESpec{Cluster: types.E2ECluster{}},
			wantErrs: 0,
		},
		{
			name:     "unknown provider",
			spec:     &types.E2ESpec{Cluster: types.E2ECluster{Provider: "eks"}},
			wantErrs: 1,
			wantInErr: []string{
				`cluster.provider "eks" is not supported`,
				"kind, none",
			},
		},
		{
			name: "none with no operator and no cluster checks",
			spec: &types.E2ESpec{
				Cluster: types.E2ECluster{Provider: types.ProviderNone},
			},
			wantErrs: 0,
		},
		{
			name: "none with a blueprint",
			spec: &types.E2ESpec{
				Cluster:   types.E2ECluster{Provider: types.ProviderNone},
				Blueprint: "blueprint.yaml",
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
				Cluster: types.E2ECluster{Provider: types.ProviderNone},
				CRD:     "crd.yaml",
			},
			wantErrs:  1,
			wantInErr: []string{"cannot install an operator"},
		},
		{
			name: "none with crdFiles",
			spec: &types.E2ESpec{
				Cluster:  types.E2ECluster{Provider: types.ProviderNone},
				CRDFiles: []string{"a.yaml", "b.yaml"},
			},
			wantErrs:  1,
			wantInErr: []string{"cannot install an operator"},
		},
		{
			name: "none with crFiles",
			spec: &types.E2ESpec{
				Cluster: types.E2ECluster{Provider: types.ProviderNone},
				CRFiles: []string{"cr.yaml"},
			},
			wantErrs:  1,
			wantInErr: []string{"cannot install an operator"},
		},
		{
			name: "none with kubectl expectation",
			spec: &types.E2ESpec{
				Cluster: types.E2ECluster{Provider: types.ProviderNone},
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
				Cluster: types.E2ECluster{Provider: types.ProviderNone},
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
				Cluster: types.E2ECluster{Provider: types.ProviderNone},
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
				Cluster: types.E2ECluster{Provider: types.ProviderNone},
				Expect: []types.E2EExpectation{
					{Name: "local", Commands: []types.E2ECommand{{Run: "true"}}},
				},
			},
			wantErrs: 0,
		},
		{
			name: "none accumulates every violation",
			spec: &types.E2ESpec{
				Cluster:   types.E2ECluster{Provider: types.ProviderNone},
				Blueprint: "blueprint.yaml",
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

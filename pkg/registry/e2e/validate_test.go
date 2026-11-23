package e2e

import (
	"strings"
	"testing"

	"github.com/intentd/inrun/pkg/types"
)

func TestNoClusterInstall(t *testing.T) {
	none := func() *types.E2EEnvironment {
		return &types.E2EEnvironment{Cluster: types.E2ECluster{Provider: types.ProviderNone}}
	}
	cases := []struct {
		name string
		spec *types.E2ESpec
		want bool // an error is expected
	}{
		{"nil spec", nil, false},
		{"kind with a blueprint", &types.E2ESpec{Blueprint: "blueprint.yaml"}, false},
		{"none with nothing to install", &types.E2ESpec{Environment: none()}, false},
		{"none with a blueprint", &types.E2ESpec{Environment: none(), Blueprint: "blueprint.yaml"}, true},
		{"none with a crd", &types.E2ESpec{Environment: none(), CRD: "crd.yaml"}, true},
		{"none with crdFiles", &types.E2ESpec{Environment: none(), CRDFiles: []string{"a.yaml"}}, true},
		{"none with crFiles", &types.E2ESpec{Environment: none(), CRFiles: []string{"cr.yaml"}}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			errs := noClusterInstall(tc.spec)
			if got := len(errs) > 0; got != tc.want {
				t.Fatalf("errs = %v, want an error: %v", errs, tc.want)
			}
			if tc.want && !strings.Contains(errorsText(errs), "cannot install an operator") {
				t.Fatalf("errs = %v", errs)
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

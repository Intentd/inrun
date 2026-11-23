package types_test

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/intentd/tools/structured/structuredtest"

	"github.com/intentd/inrun/pkg/types"
)

func TestSimulateSpec_ValidateSetup(t *testing.T) {
	apply := &types.SimulateSpec{Prepare: &types.Prepare{Apply: []types.ProvisionApplyEntry{{Path: "./ns.yaml"}}}}
	assert.Empty(t, apply.ValidateSetup(), "manifests are what simulate applies")
	assert.Equal(t, []string{"./ns.yaml"}, apply.PreparePaths())
	assert.Empty(t, (&types.SimulateSpec{}).ValidateSetup())

	beyond := &types.SimulateSpec{Prepare: &types.Prepare{
		Apply: []types.ProvisionApplyEntry{{Path: "./ns.yaml", Wait: []types.ProvisionWait{{Kind: "Namespace", Name: "a"}}}},
		Helm:  []types.ProvisionHelmInstall{{}},
		Wait:  []types.ProvisionWait{{Kind: "Namespace", Name: "a"}},
	}}
	errs := beyond.ValidateSetup()
	assert.Len(t, errs, 3)
	assert.Contains(t, errs[0], "spec.prepare.helm")
	assert.Contains(t, errs[1], "spec.prepare.wait")
	assert.Contains(t, errs[2], "spec.prepare.apply[0].wait")
}

// prepare.apply expands its includes, paths resolved from the include file;
// helm is left as written, for ValidateSetup to report.
func TestSimulateSpec_ExpandIncludes(t *testing.T) {
	dir := t.TempDir()
	structuredtest.WriteFile(t, filepath.Join(dir, "prepare", "apply.yaml"), "apply:\n  - ./ns.yaml\n")
	s := &types.SimulateSpec{Prepare: &types.Prepare{
		Apply: []types.ProvisionApplyEntry{{Path: "./own.yaml"}, {Include: "./prepare/apply.yaml"}},
		Helm:  []types.ProvisionHelmInstall{{Include: "./prepare/helm.yaml"}},
	}}
	require.NoError(t, types.ExpandSimulatePrepareIncludes(s.Prepare, dir))
	assert.Equal(t, []string{"./own.yaml", filepath.Join(dir, "prepare", "ns.yaml")}, s.PreparePaths())
	assert.Equal(t, "./prepare/helm.yaml", s.Prepare.Helm[0].Include)
	assert.NotEmpty(t, s.ValidateSetup(), "a helm entry is still an error for simulate")
	assert.NoError(t, types.ExpandSimulatePrepareIncludes(nil, dir))
}

// A Blueprint's prepare expands apply and helm includes, each path resolved
// from the file that holds it.
func TestExpandPrepareIncludes(t *testing.T) {
	dir := t.TempDir()
	structuredtest.WriteFile(t, filepath.Join(dir, "prepare", "apply.yaml"), "apply:\n  - ./ns.yaml\n")
	structuredtest.WriteFile(t, filepath.Join(dir, "prepare", "helm.yaml"), "helm:\n  - { chart: ../charts/db }\n")
	p := &types.Prepare{}
	p.Apply = []types.ProvisionApplyEntry{{Include: "./prepare/apply.yaml"}}
	p.Helm = []types.ProvisionHelmInstall{{Include: "./prepare/helm.yaml"}}
	require.NoError(t, types.ExpandPrepareIncludes(p, dir))
	require.Len(t, p.Apply, 1)
	assert.Equal(t, filepath.Join(dir, "prepare", "ns.yaml"), p.Apply[0].Path)
	require.Len(t, p.Helm, 1)
	assert.Equal(t, filepath.Join(dir, "charts", "db"), p.Helm[0].Chart)
	assert.NoError(t, types.ExpandPrepareIncludes(nil, dir))
}

package types_test

import (
	"testing"

	"github.com/intentd/inrun/pkg/types"
	"github.com/stretchr/testify/assert"
)

func crdWithReconcile(r *types.ReconcileConfig) types.CRDEntry {
	return types.CRDEntry{OperatorBox: &types.OperatorBoxConfig{Reconcile: r}}
}

func TestReconcilerKind(t *testing.T) {
	cases := map[string]struct {
		reconcile *types.ReconcileConfig
		want      types.ReconcilerKind
	}{
		"templates only": {nil, types.KindDeclarative},
		"hooks":          {&types.ReconcileConfig{Hooks: &types.HookDeclaration{Location: "github.com/x/hooks", Function: "New"}}, types.KindHooks},
		"remote":         {&types.ReconcileConfig{Default: boolp(false), Remote: &types.RemoteReconcilerDeclaration{Endpoint: "http://svc/reconcile"}}, types.KindRemote},
		"constructor":    {&types.ReconcileConfig{Default: boolp(false), ConstructorDecl: &types.ConstructorDeclaration{Location: "github.com/x/rec", Function: "New"}}, types.KindConstructor},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			crd := crdWithReconcile(tc.reconcile)
			assert.Equal(t, tc.want, crd.ReconcilerKind())
		})
	}
}

func TestTargetReconcilerKinds_OnlyTargetsThatDiffer(t *testing.T) {
	crd := crdWithReconcile(nil)
	crd.Serve = &types.ServeConfig{Enabled: true}
	crd.Serve.Target.Entries = map[string]*types.ServeTargetConfig{
		"argsOnly": {OperatorBox: &types.OperatorBoxConfig{Reconcile: &types.ReconcileConfig{Args: map[string]interface{}{"a": "b"}}}},
		"remote": {OperatorBox: &types.OperatorBoxConfig{Reconcile: &types.ReconcileConfig{
			Default: boolp(false), Remote: &types.RemoteReconcilerDeclaration{Endpoint: "http://svc/reconcile"},
		}}},
		"go": {OperatorBox: &types.OperatorBoxConfig{Reconcile: &types.ReconcileConfig{
			Default: boolp(false), ConstructorDecl: &types.ConstructorDeclaration{Location: "github.com/x/rec", Function: "New"},
		}}},
	}
	assert.Equal(t, map[string]types.ReconcilerKind{
		"remote": types.KindRemote,
		"go":     types.KindConstructor,
	}, crd.TargetReconcilerKinds())
}

func TestTargetReconcilerKinds_NoneWithoutTargets(t *testing.T) {
	crd := crdWithReconcile(nil)
	assert.Nil(t, crd.TargetReconcilerKinds())
}

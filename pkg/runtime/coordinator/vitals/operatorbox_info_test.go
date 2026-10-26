package vitals

import (
	"testing"

	"github.com/intentd/inrun/pkg/types"
	"github.com/stretchr/testify/assert"
)

func TestOperatorBoxInfo_KindAndRuntimeFinalizers(t *testing.T) {
	no := false
	crd := types.CRDEntry{OperatorBox: &types.OperatorBoxConfig{
		Reconcile: &types.ReconcileConfig{Default: &no, Remote: &types.RemoteReconcilerDeclaration{Endpoint: "http://svc/reconcile"}},
		Runtime:   &types.RuntimeConfig{},
	}}
	crd.Box().Runtime.SetFinalizerNames([]string{"inrun.dev/cleanup"})

	info := operatorBoxInfoStruct(crd)
	assert.Equal(t, "remote", info.Type)
	assert.Nil(t, info.Targets)
	assert.Equal(t, FinalizersInfo{Source: "runtime", Values: []string{"inrun.dev/cleanup"}}, info.Finalizers)
}

func TestOperatorBoxInfo_RemovedFinalizers(t *testing.T) {
	crd := types.CRDEntry{OperatorBox: &types.OperatorBoxConfig{Runtime: &types.RuntimeConfig{}}}
	crd.Box().Runtime.SetFinalizerNames([]string{"inrun.dev/cleanup"})
	crd.Box().Runtime.Finalizers.Remove = true

	info := operatorBoxInfoStruct(crd)
	assert.Equal(t, "declarative", info.Type)
	assert.Equal(t, FinalizersInfo{Source: "remove", Values: []string{}}, info.Finalizers)
}

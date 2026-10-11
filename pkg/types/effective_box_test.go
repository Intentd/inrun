package types

import "testing"

func TestEffectiveOperatorBox_TargetTemplates(t *testing.T) {
	crdCreate, crdReconcile := &HookTemplates{}, &HookTemplates{}
	targetCreate := &HookTemplates{}

	for _, tc := range []struct {
		name string
		crd  *ReconcileConfig
	}{
		{name: "CRD-level reconcile block", crd: &ReconcileConfig{Workers: 2, OnCreate: crdCreate, OnReconcile: crdReconcile}},
		{name: "no CRD-level reconcile block", crd: nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := CRDEntry{
				OperatorBox: &OperatorBoxConfig{Reconcile: tc.crd},
				Serve: &ServeConfig{Target: ServeTargetValue{Entries: map[string]*ServeTargetConfig{
					"t": {OperatorBox: &OperatorBoxConfig{Reconcile: &ReconcileConfig{OnCreate: targetCreate}}},
				}}},
			}
			box := c.EffectiveOperatorBox("t")
			if box.EffectiveOnCreate() != targetCreate {
				t.Error("target onCreate not used")
			}
			if tc.crd != nil {
				if box.EffectiveOnReconcile() != crdReconcile {
					t.Error("CRD-level onReconcile not inherited when the target omits it")
				}
				if box.Reconcile.Workers != 2 {
					t.Error("CRD-level workers not inherited")
				}
			}
		})
	}
}

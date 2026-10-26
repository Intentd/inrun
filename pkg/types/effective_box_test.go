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

func TestEffectivePrune(t *testing.T) {
	crdPrune := &PruneConfig{Enabled: true}
	targetPrune := &PruneConfig{Enabled: true, Keep: []string{"persistentvolumeclaim"}}
	c := CRDEntry{
		OperatorBox: &OperatorBoxConfig{Reconcile: &ReconcileConfig{Prune: crdPrune}},
		Serve: &ServeConfig{Target: ServeTargetValue{Entries: map[string]*ServeTargetConfig{
			"inherits":  {OperatorBox: &OperatorBoxConfig{Reconcile: &ReconcileConfig{Workers: 1}}},
			"overrides": {OperatorBox: &OperatorBoxConfig{Reconcile: &ReconcileConfig{Prune: targetPrune}}},
		}}},
	}
	if got := c.EffectivePrune(""); got != crdPrune {
		t.Error("no target: want the CRD-level prune")
	}
	if got := c.EffectivePrune("inherits"); got != crdPrune {
		t.Error("target without prune: want the CRD-level prune")
	}
	if got := c.EffectivePrune("overrides"); got != targetPrune {
		t.Error("target with prune: want the target's prune")
	}
	if (&CRDEntry{}).EffectivePrune("").IsEnabled() {
		t.Error("no reconcile block: want pruning off")
	}
}

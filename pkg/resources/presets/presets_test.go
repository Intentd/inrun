package presets_test

import (
	"testing"

	"github.com/intentd/inrun/pkg/resources/core/types"
	"github.com/intentd/inrun/pkg/resources/presets"
)

func TestBuiltin_Values(t *testing.T) {
	b := presets.Builtin()

	if r, ok := b.Resource("medium"); !ok || r.Requests["cpu"] != "250m" || r.Limits["memory"] != "1Gi" {
		t.Errorf("medium resource = %+v, %v", r, ok)
	}
	if p, ok := b.Probe("standard"); !ok || p.InitialDelaySeconds != 15 || p.PeriodSeconds != 20 || p.TimeoutSeconds != 10 {
		t.Errorf("standard probe = %+v, %v", p, ok)
	}
	if c, ok := b.ContainerSecurity("restricted"); !ok || !*c.RunAsNonRoot || c.Capabilities.Drop[0] != "ALL" {
		t.Errorf("restricted container security = %+v, %v", c, ok)
	}
	if p, ok := b.PodSecurity("hardened"); !ok || *p.RunAsUser != 65534 || *p.FSGroup != 65534 {
		t.Errorf("hardened pod security = %+v, %v", p, ok)
	}
	if r, ok := b.RollingUpdate("safe"); !ok || r.MaxSurge != "1" || r.MaxUnavailable != "0" {
		t.Errorf("safe rolling update = %+v, %v", r, ok)
	}
	if h, ok := b.HPA("web"); !ok || h.CPUTarget() != 70 || h.Behavior.ScaleDown.StabilizationWindowSeconds != 300 {
		t.Errorf("web hpa = %+v, %v", h, ok)
	}
	if p, ok := b.PDB("zero-downtime"); !ok || p.MinAvailable != "100%" {
		t.Errorf("zero-downtime pdb = %+v, %v", p, ok)
	}
	if n, ok := b.NetworkPolicy("deny-all"); !ok || len(n.PolicyTypes) != 2 || n.Ingress == nil || n.Egress == nil {
		t.Errorf("deny-all network policy = %+v, %v", n, ok)
	}
	if q, ok := b.ResourceQuota("xlarge"); !ok || q.Hard["pods"] != "100" {
		t.Errorf("xlarge quota = %+v, %v", q, ok)
	}
	if len(b.LimitRanges) != 0 {
		t.Errorf("limit ranges have no built-ins, got %d", len(b.LimitRanges))
	}
}

func TestBuiltin_ReturnsACopy(t *testing.T) {
	b := presets.Builtin()
	b.Resources[0].Name = "changed"
	if r, _ := presets.Builtin().Resource("tiny"); r.Name != "tiny" {
		t.Error("changing Builtin's result changed the built-ins")
	}
}

func TestRegistry_LookupIsCaseInsensitive(t *testing.T) {
	b := presets.Builtin()
	if _, ok := b.Probe("Slow-Start"); !ok {
		t.Error("probe lookup should ignore case")
	}
	if _, ok := b.HPA("WEB"); !ok {
		t.Error("hpa lookup should ignore case")
	}
	if _, ok := b.Resource("unknown"); ok {
		t.Error("unknown name should not be found")
	}
}

func TestRegistry_With(t *testing.T) {
	own := presets.Registry{
		Resources: []presets.Resource{
			{Name: "Medium", Requests: map[string]string{"cpu": "300m"}},
			{Name: "gpu", Requests: map[string]string{"nvidia.com/gpu": "1"}},
		},
		LimitRanges: []presets.LimitRange{{Name: "team", Limits: []types.LimitRangeItem{{Type: "Container"}}}},
	}
	reg := presets.Builtin().With(own)

	if r, _ := reg.Resource("medium"); r.Requests["cpu"] != "300m" {
		t.Errorf("own medium should replace the built-in, got %+v", r)
	}
	if _, ok := reg.Resource("gpu"); !ok {
		t.Error("own preset missing after With")
	}
	if _, ok := reg.Resource("tiny"); !ok {
		t.Error("built-in kept after With")
	}
	if got := len(reg.Resources); got != len(presets.Builtin().Resources)+1 {
		t.Errorf("resources after With = %d, want built-ins + 1", got)
	}
	if _, ok := reg.LimitRange("team"); !ok {
		t.Error("own limit range missing after With")
	}
}

func TestRegistry_Empty(t *testing.T) {
	if !(presets.Registry{}).Empty() {
		t.Error("zero Registry should be empty")
	}
	if presets.Builtin().Empty() {
		t.Error("Builtin should not be empty")
	}
}

func TestConversions(t *testing.T) {
	if got := (presets.HPA{TargetCPUUtilizationPercentage: "x"}).CPUTarget(); got != 0 {
		t.Errorf("CPUTarget of a non-number = %d, want 0", got)
	}
	if got := (presets.RollingUpdate{MaxSurge: "2"}).Behavior().MaxSurge; got != "2" {
		t.Errorf("Behavior().MaxSurge = %q", got)
	}
	r := presets.Resource{Requests: map[string]string{"cpu": "1"}}.Requirements()
	if r.Requests["cpu"] != "1" || r.Preset != "" {
		t.Errorf("Requirements() = %+v", r)
	}
}

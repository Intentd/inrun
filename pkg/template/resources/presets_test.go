package resources

import (
	"testing"

	"github.com/intentd/inrun/pkg/kubeclient/kubefake/object"
	"github.com/intentd/inrun/pkg/types"
	"github.com/intentd/kubewright/presets"
)

func TestHPASpecReplicaBounds(t *testing.T) {
	reg := &types.PresetRegistry{Registry: presets.Registry{HPAs: []presets.HPA{
		{Name: "bounded", MinReplicas: "2", MaxReplicas: "10"},
	}}}
	bounded := &types.HPABehavior{Preset: "bounded"}

	cases := []struct {
		name     string
		src      types.HPATemplateSource
		min, max int32
	}{
		{"from the preset", types.HPATemplateSource{Behavior: bounded}, 2, 10},
		{"the entry's own min wins", types.HPATemplateSource{Behavior: bounded, MinReplicas: "3"}, 3, 10},
		{"no preset, no bounds", types.HPATemplateSource{}, 1, 1},
		{"min only: max follows it", types.HPATemplateSource{MinReplicas: "3"}, 3, 3},
	}
	for _, c := range cases {
		spec := HPASpec(c.src, object.Owner(), reg)
		if spec.MinReplicas != c.min || spec.MaxReplicas != c.max {
			t.Errorf("%s: got %d..%d, want %d..%d", c.name, spec.MinReplicas, spec.MaxReplicas, c.min, c.max)
		}
	}
}

func TestNetworkPolicySpecPresetPodSelector(t *testing.T) {
	reg := &types.PresetRegistry{Registry: presets.Registry{NetworkPolicies: []presets.NetworkPolicy{
		{Name: "web-only", PodSelector: map[string]string{"app": "web"}, PolicyTypes: []string{"Ingress"}},
	}}}

	spec := NetworkPolicySpec(types.NetworkPolicyTemplateSource{Preset: "web-only"}, object.Owner(), reg)
	if spec.PodSelector["app"] != "web" {
		t.Errorf("pod selector = %v; want the preset's app=web", spec.PodSelector)
	}
	own := NetworkPolicySpec(types.NetworkPolicyTemplateSource{Preset: "web-only", PodSelector: map[string]string{"app": "api"}}, object.Owner(), reg)
	if own.PodSelector["app"] != "api" {
		t.Errorf("pod selector = %v; the entry's own selector should win", own.PodSelector)
	}
}

// TestPresetsReachTheSpec checks every preset kind is expanded by the spec
// builder that uses it, with the built-in presets (and a custom LimitRange
// preset: there are no built-in ones).
func TestPresetsReachTheSpec(t *testing.T) {
	reg := &types.PresetRegistry{Registry: presets.Registry{LimitRanges: []presets.LimitRange{
		{Name: "team-defaults", Limits: []types.LimitRangeItem{{Type: "Container", Default: map[string]string{"cpu": "250m"}}}},
	}}}
	owner := object.Owner()

	d := DeploymentSpec(types.DeploymentTemplateSource{
		Image:           "app:1",
		Resources:       &types.ResourceRequirements{Preset: "small"},
		Probes:          &types.ProbesConfig{Readiness: &types.ProbeConfig{Path: "/ready", Preset: "fast"}},
		SecurityContext: &types.ContainerSecurityContext{Preset: "restricted"},
		PodSecurity:     &types.PodSecurityContext{Preset: "restricted"},
		RollingUpdate:   &types.RollingUpdateBehavior{Preset: "safe"},
		Containers: []types.Container{{Name: "app", Image: "app:1",
			Resources: &types.ResourceRequirements{Preset: "small"}}},
	}, owner, reg)
	if d.Resources == nil || d.Resources.Requests["cpu"] != "100m" {
		t.Errorf("resources = %+v; want small's cpu request 100m", d.Resources)
	}
	if d.Probes == nil || d.Probes.Readiness == nil || d.Probes.Readiness.PeriodSeconds == nil || *d.Probes.Readiness.PeriodSeconds != 10 {
		t.Errorf("readiness probe = %+v; want fast's period 10s", d.Probes)
	}
	if d.SecurityContext == nil || d.SecurityContext.RunAsNonRoot == nil || !*d.SecurityContext.RunAsNonRoot {
		t.Errorf("security context = %+v; want restricted's runAsNonRoot", d.SecurityContext)
	}
	if d.PodSecurity == nil || d.PodSecurity.RunAsNonRoot == nil || !*d.PodSecurity.RunAsNonRoot {
		t.Errorf("pod security = %+v; want restricted's runAsNonRoot", d.PodSecurity)
	}
	if d.RollingUpdate == nil || d.RollingUpdate.MaxSurge != "1" || d.RollingUpdate.MaxUnavailable != "0" {
		t.Errorf("rolling update = %+v; want safe's 1 and 0", d.RollingUpdate)
	}
	if len(d.Containers) != 1 || d.Containers[0].Resources == nil || d.Containers[0].Resources.Requests["cpu"] != "100m" {
		t.Errorf("container resources = %+v; want small's, expanded per container", d.Containers)
	}

	if p := PDBSpec(types.PDBTemplateSource{Behavior: &types.PDBBehavior{Preset: "rolling"}}, owner, reg); p.MaxUnavailable != "1" {
		t.Errorf("pdb maxUnavailable = %q; want rolling's 1", p.MaxUnavailable)
	}
	if q := ResourceQuotaSpec(types.ResourceQuotaTemplateSource{Preset: "small"}, owner, reg); q.Hard["pods"] != "10" {
		t.Errorf("quota hard = %v; want small's 10 pods", q.Hard)
	}
	if l := LimitRangeSpec(types.LimitRangeTemplateSource{Preset: "team-defaults"}, owner, reg); len(l.Limits) != 1 || l.Limits[0].Default["cpu"] != "250m" {
		t.Errorf("limit range = %+v; want team-defaults' 250m cpu", l.Limits)
	}
	if h := HPASpec(types.HPATemplateSource{Behavior: &types.HPABehavior{Preset: "web"}}, owner, reg); h.Behavior == nil || h.TargetCPUUtilizationPercentage != 70 {
		t.Errorf("hpa = %+v, %d; want web's behavior and 70%% CPU", h.Behavior, h.TargetCPUUtilizationPercentage)
	}
}

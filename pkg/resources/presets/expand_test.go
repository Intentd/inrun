package presets_test

import (
	"testing"

	"github.com/intentd/inrun/pkg/resources/core/types"
	"github.com/intentd/inrun/pkg/resources/presets"
)

func TestExpandResources(t *testing.T) {
	reg := presets.Builtin()

	got, err := presets.ExpandResources(&types.ResourceRequirements{Preset: "small"}, reg)
	if err != nil || got.Requests["cpu"] != "100m" {
		t.Errorf("small = %+v, %v; want the small preset's requests", got, err)
	}

	explicit := &types.ResourceRequirements{Requests: map[string]string{"cpu": "1"}}
	if got, _ := presets.ExpandResources(explicit, reg); got != explicit {
		t.Error("explicit requirements should be returned as they are")
	}
	if got, err := presets.ExpandResources(&types.ResourceRequirements{}, reg); got != nil || err != nil {
		t.Errorf("empty requirements = %+v, %v; want nil", got, err)
	}
	if got, err := presets.ExpandResources(nil, reg); got != nil || err != nil {
		t.Errorf("nil = %+v, %v; want nil", got, err)
	}
	if _, err := presets.ExpandResources(&types.ResourceRequirements{Preset: "huge"}, reg); err == nil {
		t.Error("an unknown preset should be an error")
	}
}

func TestExpandSecurity(t *testing.T) {
	reg := presets.Builtin()

	c, err := presets.ExpandContainerSecurity(&types.ContainerSecurityContext{Preset: "restricted"}, reg)
	if err != nil || !*c.RunAsNonRoot {
		t.Errorf("restricted container = %+v, %v; want runAsNonRoot", c, err)
	}
	p, err := presets.ExpandPodSecurity(&types.PodSecurityContext{Preset: "hardened"}, reg)
	if err != nil || *p.RunAsUser != 65534 {
		t.Errorf("hardened pod = %+v, %v; want runAsUser 65534", p, err)
	}
	if _, err := presets.ExpandPodSecurity(&types.PodSecurityContext{Preset: "unknown"}, reg); err == nil {
		t.Error("an unknown preset should be an error")
	}
}

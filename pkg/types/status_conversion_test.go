// Tests for StatusConfig (status.go) and ConversionRules (conversion.go).
package types_test

import (
	"testing"

	"github.com/intentd/inrun/pkg/types"
	"github.com/stretchr/testify/assert"
)

// ── StatusConfig.ConditionsEnabled ───────────────────────────────────────────

func TestStatusConfig_ConditionsEnabled_Nil(t *testing.T) {
	var s *types.StatusConfig
	assert.True(t, s.ConditionsEnabled())
}

func TestStatusConfig_ConditionsEnabled_NilField(t *testing.T) {
	s := &types.StatusConfig{}
	assert.True(t, s.ConditionsEnabled())
}

func TestStatusConfig_ConditionsEnabled_ExplicitTrue(t *testing.T) {
	t_ := true
	s := &types.StatusConfig{Conditions: &t_}
	assert.True(t, s.ConditionsEnabled())
}

func TestStatusConfig_ConditionsEnabled_ExplicitFalse(t *testing.T) {
	f := false
	s := &types.StatusConfig{Conditions: &f}
	assert.False(t, s.ConditionsEnabled())
}

// ── StatusConfig.HasFields ────────────────────────────────────────────────────

func TestStatusConfig_HasFields_Nil(t *testing.T) {
	var s *types.StatusConfig
	assert.False(t, s.HasFields())
}

func TestStatusConfig_HasFields_Empty(t *testing.T) {
	s := &types.StatusConfig{}
	assert.False(t, s.HasFields())
}

func TestStatusConfig_HasFields_WithFields(t *testing.T) {
	s := &types.StatusConfig{
		Fields: []types.StatusFieldSpec{{Path: "phase", Value: "Running"}},
	}
	assert.True(t, s.HasFields())
}

// ── ConversionRules.FindPath ──────────────────────────────────────────────────

func TestConversionRules_FindPath_Found(t *testing.T) {
	rules := types.ConversionRules{
		Paths: []types.ConversionPath{
			{From: "v1", To: "v2"},
			{From: "v2", To: "v3"},
		},
	}
	p := rules.FindPath("v1", "v2")
	assert.NotNil(t, p)
	assert.Equal(t, "v1", p.From)
	assert.Equal(t, "v2", p.To)
}

func TestConversionRules_FindPath_NotFound(t *testing.T) {
	rules := types.ConversionRules{
		Paths: []types.ConversionPath{
			{From: "v1", To: "v2"},
		},
	}
	assert.Nil(t, rules.FindPath("v2", "v1"))
	assert.Nil(t, rules.FindPath("v1", "v3"))
}

func TestConversionRules_FindPath_Empty(t *testing.T) {
	rules := types.ConversionRules{}
	assert.Nil(t, rules.FindPath("v1", "v2"))
}

func TestConversionRules_FindPath_ReturnsPointerToSliceElement(t *testing.T) {
	rules := types.ConversionRules{
		Paths: []types.ConversionPath{
			{From: "v1", To: "v2"},
		},
	}
	p := rules.FindPath("v1", "v2")
	// Modifying via pointer must affect the slice
	p.From = "changed"
	assert.Equal(t, "changed", rules.Paths[0].From)
}

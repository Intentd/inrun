package metadata

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/intentd/inrun/pkg/blueprint"
	"github.com/intentd/inrun/pkg/config"
	"github.com/intentd/inrun/pkg/types"
)

// helpers

func blueprintWithLifecycle(lc *types.BlueprintLifecycle) *checker {
	return newExec(blueprint.NewBlueprintWithLifecycleForTest(config.BlueprintKind(), lc))
}

func stackWithLifecycle(lc *types.BlueprintLifecycle) *checker {
	return newExec(blueprint.NewBlueprintWithLifecycleForTest(config.StackKind(), lc))
}

func blueprintWithPolicy(p *types.BlueprintPolicy) *checker {
	return newExec(blueprint.NewBlueprintWithPolicyForTest(config.BlueprintKind(), p))
}

// ── maturity ─────────────────────────────────────────────────────────────────

func TestValidateLifecycle_NoLifecycle(t *testing.T) {
	e := blueprintWithLifecycle(nil)
	assert.NoError(t, e.validateLifecycle())
}

func TestValidateLifecycle_StableMaturity(t *testing.T) {
	e := blueprintWithLifecycle(&types.BlueprintLifecycle{
		Maturity: types.MaturityStable,
	})
	assert.NoError(t, e.validateLifecycle())
	assert.False(t, e.Blueprint.Warnings.HasWarnings())
}

func TestValidateLifecycle_AlphaMaturityWarns(t *testing.T) {
	e := blueprintWithLifecycle(&types.BlueprintLifecycle{
		Maturity: types.MaturityAlpha,
	})
	assert.NoError(t, e.validateLifecycle())
	assert.True(t, e.Blueprint.Warnings.HasWarnings())
	assert.Contains(t, e.Blueprint.Warnings[0], "alpha")
}

func TestValidateLifecycle_BetaMaturityWarns(t *testing.T) {
	e := blueprintWithLifecycle(&types.BlueprintLifecycle{
		Maturity: types.MaturityBeta,
	})
	assert.NoError(t, e.validateLifecycle())
	assert.True(t, e.Blueprint.Warnings.HasWarnings())
	assert.Contains(t, e.Blueprint.Warnings[0], "beta")
}

// maturity: deprecated without a deprecation block is now a warning, not an error.
// The deprecation block is the primary signal.
func TestValidateLifecycle_DeprecatedWithoutBlockWarns(t *testing.T) {
	e := blueprintWithLifecycle(&types.BlueprintLifecycle{
		Maturity: types.MaturityDeprecated,
	})
	assert.NoError(t, e.validateLifecycle())
	assert.True(t, e.Blueprint.Warnings.HasWarnings())
	assert.Contains(t, e.Blueprint.Warnings[0], "lifecycle.deprecation is not set")
}

// A deprecation block alone is sufficient — maturity: deprecated is not required.
func TestValidateLifecycle_DeprecationBlockWithoutMaturityIsValid(t *testing.T) {
	e := blueprintWithLifecycle(&types.BlueprintLifecycle{
		Deprecation: &types.BlueprintDeprecation{
			Message: "use v2",
		},
	})
	assert.NoError(t, e.validateLifecycle())
	assert.False(t, e.Blueprint.Warnings.HasWarnings())
}

func TestValidateLifecycle_DeprecatedWithDeprecationBlock(t *testing.T) {
	e := blueprintWithLifecycle(&types.BlueprintLifecycle{
		Maturity: types.MaturityDeprecated,
		Deprecation: &types.BlueprintDeprecation{
			Message: "use v2",
		},
	})
	assert.NoError(t, e.validateLifecycle())
	assert.False(t, e.Blueprint.Warnings.HasWarnings())
}

func TestValidateLifecycle_UnknownMaturity(t *testing.T) {
	e := blueprintWithLifecycle(&types.BlueprintLifecycle{
		Maturity: types.LifecycleMaturity("experimental"),
	})
	err := e.validateLifecycle()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not valid")
	assert.Contains(t, err.Error(), "experimental")
}

// ── compatibility ─────────────────────────────────────────────────────────────

func TestValidateLifecycle_ValidCompatibility(t *testing.T) {
	e := blueprintWithLifecycle(&types.BlueprintLifecycle{
		Compatibility: &types.LifecycleCompat{
			Kubernetes: ">=1.31",
			Inrun:      ">=0.7.14",
		},
	})
	assert.NoError(t, e.validateLifecycle())
}

func TestValidateLifecycle_InvalidKubernetesRange(t *testing.T) {
	e := blueprintWithLifecycle(&types.BlueprintLifecycle{
		Compatibility: &types.LifecycleCompat{
			Kubernetes: "!!!invalid",
		},
	})
	err := e.validateLifecycle()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "lifecycle.compatibility.kubernetes")
}

func TestValidateLifecycle_InvalidInrunRange(t *testing.T) {
	e := blueprintWithLifecycle(&types.BlueprintLifecycle{
		Compatibility: &types.LifecycleCompat{
			Inrun: "!!!invalid",
		},
	})
	err := e.validateLifecycle()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "lifecycle.compatibility.inrun")
}

// ── blueprint Deprecation ────────────────────────────────────────────────────

func blueprintWithDeprecation(d *types.BlueprintDeprecation) *checker {
	if d == nil {
		return newExec(blueprint.NewEmptyBlueprint())
	}
	return newExec(blueprint.NewBlueprintWithLifecycleForTest("", &types.BlueprintLifecycle{
		Deprecation: d,
	}))
}

func TestValidateLifecycleDeprecation_Nil(t *testing.T) {
	e := blueprintWithDeprecation(nil)
	assert.NoError(t, e.validateLifecycleDeprecation())
}

func TestValidateLifecycleDeprecation_MessageOnly(t *testing.T) {
	e := blueprintWithDeprecation(&types.BlueprintDeprecation{
		Message: "use v2",
	})
	assert.NoError(t, e.validateLifecycleDeprecation())
}

func TestValidateLifecycleDeprecation_MissingMessage(t *testing.T) {
	e := blueprintWithDeprecation(&types.BlueprintDeprecation{
		MigratedTo: "mypattern:v2",
	})
	err := e.validateLifecycleDeprecation()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "message is required")
}

func TestValidateLifecycleDeprecation_ValidTimeline(t *testing.T) {
	e := blueprintWithDeprecation(&types.BlueprintDeprecation{
		Message: "use v2",
		Timeline: &types.DeprecationTimeline{
			From: "2026-01-01",
			To:   "2027-01-01",
		},
	})
	assert.NoError(t, e.validateLifecycleDeprecation())
}

func TestValidateLifecycleDeprecation_BadFromDate(t *testing.T) {
	e := blueprintWithDeprecation(&types.BlueprintDeprecation{
		Message: "use v2",
		Timeline: &types.DeprecationTimeline{
			From: "not-a-date",
			To:   "2027-01-01",
		},
	})
	err := e.validateLifecycleDeprecation()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "timeline.from")
	assert.Contains(t, err.Error(), "YYYY-MM-DD")
}

func TestValidateLifecycleDeprecation_BadToDate(t *testing.T) {
	e := blueprintWithDeprecation(&types.BlueprintDeprecation{
		Message: "use v2",
		Timeline: &types.DeprecationTimeline{
			From: "2026-01-01",
			To:   "01/01/2027",
		},
	})
	err := e.validateLifecycleDeprecation()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "timeline.to")
	assert.Contains(t, err.Error(), "YYYY-MM-DD")
}

func TestValidateLifecycleDeprecation_FromAfterTo(t *testing.T) {
	e := blueprintWithDeprecation(&types.BlueprintDeprecation{
		Message: "use v2",
		Timeline: &types.DeprecationTimeline{
			From: "2027-06-01",
			To:   "2027-01-01",
		},
	})
	err := e.validateLifecycleDeprecation()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "from")
	assert.Contains(t, err.Error(), "before")
	assert.Contains(t, err.Error(), "to")
}

func TestValidateLifecycleDeprecation_FromEqualTo(t *testing.T) {
	e := blueprintWithDeprecation(&types.BlueprintDeprecation{
		Message: "use v2",
		Timeline: &types.DeprecationTimeline{
			From: "2027-01-01",
			To:   "2027-01-01",
		},
	})
	err := e.validateLifecycleDeprecation()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "before")
}

func TestValidateLifecycleDeprecation_TimelineFromOnly(t *testing.T) {
	e := blueprintWithDeprecation(&types.BlueprintDeprecation{
		Message: "use v2",
		Timeline: &types.DeprecationTimeline{
			From: "2026-01-01",
		},
	})
	assert.NoError(t, e.validateLifecycleDeprecation())
}

func TestValidateLifecycleDeprecation_TimelineToOnly(t *testing.T) {
	e := blueprintWithDeprecation(&types.BlueprintDeprecation{
		Message: "use v2",
		Timeline: &types.DeprecationTimeline{
			To: "2027-01-01",
		},
	})
	assert.NoError(t, e.validateLifecycleDeprecation())
}

// ── kind boundary — accept ────────────────────────────────────────────────────

func TestValidateLifecycle_AcceptOnBlueprintIsError(t *testing.T) {
	e := blueprintWithLifecycle(&types.BlueprintLifecycle{
		Accept: &types.StackAccept{
			Patterns: []types.StackAcceptEntry{{Name: "some-pattern"}},
		},
	})
	err := e.validateLifecycle()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "lifecycle.accept is only valid in a Stack")
}

func TestValidateLifecycle_AcceptOnStackIsValid(t *testing.T) {
	e := stackWithLifecycle(&types.BlueprintLifecycle{
		Accept: &types.StackAccept{
			Patterns: []types.StackAcceptEntry{
				{Name: "old-operator"},
				{Name: "alpha-import", Author: "myorg"},
			},
		},
	})
	assert.NoError(t, e.validateLifecycle())
}

// ── accept.patterns version field ────────────────────────────────────────────

func TestValidateLifecycle_AcceptPatternVersionValid(t *testing.T) {
	e := stackWithLifecycle(&types.BlueprintLifecycle{
		Accept: &types.StackAccept{
			Patterns: []types.StackAcceptEntry{
				{Name: "webapp-operator", Version: "=1.0.0"},
				{Name: "cache-operator", Version: ">=0.1.0, <1.0.0"},
			},
		},
	})
	assert.NoError(t, e.validateLifecycle())
}

func TestValidateLifecycle_AcceptPatternVersionInvalid(t *testing.T) {
	e := stackWithLifecycle(&types.BlueprintLifecycle{
		Accept: &types.StackAccept{
			Patterns: []types.StackAcceptEntry{
				{Name: "webapp-operator", Version: "!!!invalid"},
			},
		},
	})
	err := e.validateLifecycle()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "lifecycle.accept.patterns[\"webapp-operator\"].version")
}

func TestValidateLifecycle_AcceptPatternNoVersionIsValid(t *testing.T) {
	e := stackWithLifecycle(&types.BlueprintLifecycle{
		Accept: &types.StackAccept{
			Patterns: []types.StackAcceptEntry{
				{Name: "webapp-operator"}, // no version = accept all versions
			},
		},
	})
	assert.NoError(t, e.validateLifecycle())
}

// ── StackAccept.Accepts helper ─────────────────────────────────────────────

func TestStackAccept_Accepts(t *testing.T) {
	a := &types.StackAccept{
		Patterns: []types.StackAcceptEntry{
			{Name: "old-operator"},
			{Name: "alpha-lib", Author: "myorg"},
		},
	}

	assert.True(t, a.Accepts("old-operator", ""))
	assert.True(t, a.Accepts("old-operator", "anyorg")) // no author filter when entry has no author
	assert.True(t, a.Accepts("alpha-lib", "myorg"))
	assert.False(t, a.Accepts("alpha-lib", "otherorg")) // author mismatch
	assert.False(t, a.Accepts("unknown-pattern", ""))
	assert.False(t, (*types.StackAccept)(nil).Accepts("any", ""))
}

// ── policy ────────────────────────────────────────────────────────────────────

func TestValidatePolicy_NilPolicy(t *testing.T) {
	e := blueprintWithPolicy(nil)
	assert.NoError(t, e.validatePolicy())
}

func TestValidatePolicy_ValidMinMaturity(t *testing.T) {
	for _, m := range []types.LifecycleMaturity{
		types.MaturityAlpha,
		types.MaturityBeta,
		types.MaturityStable,
	} {
		e := blueprintWithPolicy(&types.BlueprintPolicy{
			Lifecycle: &types.BlueprintLifecyclePolicy{MinMaturity: m},
		})
		assert.NoError(t, e.validatePolicy(), "expected no error for minMaturity: %s", m)
	}
}

func TestValidatePolicy_DeprecatedFloorIsError(t *testing.T) {
	e := blueprintWithPolicy(&types.BlueprintPolicy{
		Lifecycle: &types.BlueprintLifecyclePolicy{MinMaturity: types.MaturityDeprecated},
	})
	err := e.validatePolicy()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "policy.lifecycle.minMaturity")
	assert.Contains(t, err.Error(), "deprecated")
}

func TestValidatePolicy_UnknownFloorIsError(t *testing.T) {
	e := blueprintWithPolicy(&types.BlueprintPolicy{
		Lifecycle: &types.BlueprintLifecyclePolicy{MinMaturity: types.LifecycleMaturity("production")},
	})
	err := e.validatePolicy()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "policy.lifecycle.minMaturity")
}

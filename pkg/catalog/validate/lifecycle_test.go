package validate

import (
	"testing"

	"github.com/intentd/inrun/pkg/catalog"
	"github.com/intentd/inrun/pkg/config"
	"github.com/intentd/inrun/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// helpers

func catalogWithLifecycle(lc *types.CatalogLifecycle) *executor {
	return newExec(catalog.NewCatalogWithLifecycleForTest(config.CatalogKind(), lc))
}

func stackWithLifecycle(lc *types.CatalogLifecycle) *executor {
	return newExec(catalog.NewCatalogWithLifecycleForTest(config.StackKind(), lc))
}

func catalogWithPolicy(p *types.CatalogPolicy) *executor {
	return newExec(catalog.NewCatalogWithPolicyForTest(config.CatalogKind(), p))
}

// ── maturity ─────────────────────────────────────────────────────────────────

func TestValidateLifecycle_NoLifecycle(t *testing.T) {
	k := catalogWithLifecycle(nil)
	assert.NoError(t, k.validateLifecycle())
}

func TestValidateLifecycle_StableMaturity(t *testing.T) {
	k := catalogWithLifecycle(&types.CatalogLifecycle{
		Maturity: types.MaturityStable,
	})
	assert.NoError(t, k.validateLifecycle())
	assert.False(t, k.k.Warnings.HasWarnings())
}

func TestValidateLifecycle_AlphaMaturityWarns(t *testing.T) {
	k := catalogWithLifecycle(&types.CatalogLifecycle{
		Maturity: types.MaturityAlpha,
	})
	assert.NoError(t, k.validateLifecycle())
	assert.True(t, k.k.Warnings.HasWarnings())
	assert.Contains(t, k.k.Warnings[0], "alpha")
}

func TestValidateLifecycle_BetaMaturityWarns(t *testing.T) {
	k := catalogWithLifecycle(&types.CatalogLifecycle{
		Maturity: types.MaturityBeta,
	})
	assert.NoError(t, k.validateLifecycle())
	assert.True(t, k.k.Warnings.HasWarnings())
	assert.Contains(t, k.k.Warnings[0], "beta")
}

// maturity: deprecated without a deprecation block is now a warning, not an error.
// The deprecation block is the primary signal.
func TestValidateLifecycle_DeprecatedWithoutBlockWarns(t *testing.T) {
	k := catalogWithLifecycle(&types.CatalogLifecycle{
		Maturity: types.MaturityDeprecated,
	})
	assert.NoError(t, k.validateLifecycle())
	assert.True(t, k.k.Warnings.HasWarnings())
	assert.Contains(t, k.k.Warnings[0], "lifecycle.deprecation is not set")
}

// A deprecation block alone is sufficient — maturity: deprecated is not required.
func TestValidateLifecycle_DeprecationBlockWithoutMaturityIsValid(t *testing.T) {
	k := catalogWithLifecycle(&types.CatalogLifecycle{
		Deprecation: &types.CatalogDeprecation{
			Message: "use v2",
		},
	})
	assert.NoError(t, k.validateLifecycle())
	assert.False(t, k.k.Warnings.HasWarnings())
}

func TestValidateLifecycle_DeprecatedWithDeprecationBlock(t *testing.T) {
	k := catalogWithLifecycle(&types.CatalogLifecycle{
		Maturity: types.MaturityDeprecated,
		Deprecation: &types.CatalogDeprecation{
			Message: "use v2",
		},
	})
	assert.NoError(t, k.validateLifecycle())
	assert.False(t, k.k.Warnings.HasWarnings())
}

func TestValidateLifecycle_UnknownMaturity(t *testing.T) {
	k := catalogWithLifecycle(&types.CatalogLifecycle{
		Maturity: types.LifecycleMaturity("experimental"),
	})
	err := k.validateLifecycle()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not valid")
	assert.Contains(t, err.Error(), "experimental")
}

// ── compatibility ─────────────────────────────────────────────────────────────

func TestValidateLifecycle_ValidCompatibility(t *testing.T) {
	k := catalogWithLifecycle(&types.CatalogLifecycle{
		Compatibility: &types.LifecycleCompat{
			Kubernetes: ">=1.31",
			Inrun:      ">=0.7.14",
		},
	})
	assert.NoError(t, k.validateLifecycle())
}

func TestValidateLifecycle_InvalidKubernetesRange(t *testing.T) {
	k := catalogWithLifecycle(&types.CatalogLifecycle{
		Compatibility: &types.LifecycleCompat{
			Kubernetes: "!!!invalid",
		},
	})
	err := k.validateLifecycle()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "lifecycle.compatibility.kubernetes")
}

func TestValidateLifecycle_InvalidInrunRange(t *testing.T) {
	k := catalogWithLifecycle(&types.CatalogLifecycle{
		Compatibility: &types.LifecycleCompat{
			Inrun: "!!!invalid",
		},
	})
	err := k.validateLifecycle()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "lifecycle.compatibility.inrun")
}

// ── catalog Deprecation ────────────────────────────────────────────────────

func catalogWithDeprecation(d *types.CatalogDeprecation) *executor {
	if d == nil {
		return newExec(catalog.NewEmptyCatalog())
	}
	return newExec(catalog.NewCatalogWithLifecycleForTest("", &types.CatalogLifecycle{
		Deprecation: d,
	}))
}

func TestValidateLifecycleDeprecation_Nil(t *testing.T) {
	k := catalogWithDeprecation(nil)
	assert.NoError(t, k.validateLifecycleDeprecation())
}

func TestValidateLifecycleDeprecation_MessageOnly(t *testing.T) {
	k := catalogWithDeprecation(&types.CatalogDeprecation{
		Message: "use v2",
	})
	assert.NoError(t, k.validateLifecycleDeprecation())
}

func TestValidateLifecycleDeprecation_MissingMessage(t *testing.T) {
	k := catalogWithDeprecation(&types.CatalogDeprecation{
		MigratedTo: "mypattern:v2",
	})
	err := k.validateLifecycleDeprecation()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "message is required")
}

func TestValidateLifecycleDeprecation_ValidTimeline(t *testing.T) {
	k := catalogWithDeprecation(&types.CatalogDeprecation{
		Message: "use v2",
		Timeline: &types.DeprecationTimeline{
			From: "2026-01-01",
			To:   "2027-01-01",
		},
	})
	assert.NoError(t, k.validateLifecycleDeprecation())
}

func TestValidateLifecycleDeprecation_BadFromDate(t *testing.T) {
	k := catalogWithDeprecation(&types.CatalogDeprecation{
		Message: "use v2",
		Timeline: &types.DeprecationTimeline{
			From: "not-a-date",
			To:   "2027-01-01",
		},
	})
	err := k.validateLifecycleDeprecation()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "timeline.from")
	assert.Contains(t, err.Error(), "YYYY-MM-DD")
}

func TestValidateLifecycleDeprecation_BadToDate(t *testing.T) {
	k := catalogWithDeprecation(&types.CatalogDeprecation{
		Message: "use v2",
		Timeline: &types.DeprecationTimeline{
			From: "2026-01-01",
			To:   "01/01/2027",
		},
	})
	err := k.validateLifecycleDeprecation()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "timeline.to")
	assert.Contains(t, err.Error(), "YYYY-MM-DD")
}

func TestValidateLifecycleDeprecation_FromAfterTo(t *testing.T) {
	k := catalogWithDeprecation(&types.CatalogDeprecation{
		Message: "use v2",
		Timeline: &types.DeprecationTimeline{
			From: "2027-06-01",
			To:   "2027-01-01",
		},
	})
	err := k.validateLifecycleDeprecation()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "from")
	assert.Contains(t, err.Error(), "before")
	assert.Contains(t, err.Error(), "to")
}

func TestValidateLifecycleDeprecation_FromEqualTo(t *testing.T) {
	k := catalogWithDeprecation(&types.CatalogDeprecation{
		Message: "use v2",
		Timeline: &types.DeprecationTimeline{
			From: "2027-01-01",
			To:   "2027-01-01",
		},
	})
	err := k.validateLifecycleDeprecation()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "before")
}

func TestValidateLifecycleDeprecation_TimelineFromOnly(t *testing.T) {
	k := catalogWithDeprecation(&types.CatalogDeprecation{
		Message: "use v2",
		Timeline: &types.DeprecationTimeline{
			From: "2026-01-01",
		},
	})
	assert.NoError(t, k.validateLifecycleDeprecation())
}

func TestValidateLifecycleDeprecation_TimelineToOnly(t *testing.T) {
	k := catalogWithDeprecation(&types.CatalogDeprecation{
		Message: "use v2",
		Timeline: &types.DeprecationTimeline{
			To: "2027-01-01",
		},
	})
	assert.NoError(t, k.validateLifecycleDeprecation())
}

// ── kind boundary — accept ────────────────────────────────────────────────────

func TestValidateLifecycle_AcceptOnCatalogIsError(t *testing.T) {
	k := catalogWithLifecycle(&types.CatalogLifecycle{
		Accept: &types.StackAccept{
			Patterns: []types.StackAcceptEntry{{Name: "some-pattern"}},
		},
	})
	err := k.validateLifecycle()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "lifecycle.accept is only valid in a Stack")
}

func TestValidateLifecycle_AcceptOnStackIsValid(t *testing.T) {
	k := stackWithLifecycle(&types.CatalogLifecycle{
		Accept: &types.StackAccept{
			Patterns: []types.StackAcceptEntry{
				{Name: "old-operator"},
				{Name: "alpha-import", Author: "myorg"},
			},
		},
	})
	assert.NoError(t, k.validateLifecycle())
}

// ── accept.patterns version field ────────────────────────────────────────────

func TestValidateLifecycle_AcceptPatternVersionValid(t *testing.T) {
	k := stackWithLifecycle(&types.CatalogLifecycle{
		Accept: &types.StackAccept{
			Patterns: []types.StackAcceptEntry{
				{Name: "webapp-operator", Version: "=1.0.0"},
				{Name: "cache-operator", Version: ">=0.1.0, <1.0.0"},
			},
		},
	})
	assert.NoError(t, k.validateLifecycle())
}

func TestValidateLifecycle_AcceptPatternVersionInvalid(t *testing.T) {
	k := stackWithLifecycle(&types.CatalogLifecycle{
		Accept: &types.StackAccept{
			Patterns: []types.StackAcceptEntry{
				{Name: "webapp-operator", Version: "!!!invalid"},
			},
		},
	})
	err := k.validateLifecycle()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "lifecycle.accept.patterns[\"webapp-operator\"].version")
}

func TestValidateLifecycle_AcceptPatternNoVersionIsValid(t *testing.T) {
	k := stackWithLifecycle(&types.CatalogLifecycle{
		Accept: &types.StackAccept{
			Patterns: []types.StackAcceptEntry{
				{Name: "webapp-operator"}, // no version = accept all versions
			},
		},
	})
	assert.NoError(t, k.validateLifecycle())
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
	k := catalogWithPolicy(nil)
	assert.NoError(t, k.validatePolicy())
}

func TestValidatePolicy_ValidMinMaturity(t *testing.T) {
	for _, m := range []types.LifecycleMaturity{
		types.MaturityAlpha,
		types.MaturityBeta,
		types.MaturityStable,
	} {
		k := catalogWithPolicy(&types.CatalogPolicy{
			Lifecycle: &types.CatalogLifecyclePolicy{MinMaturity: m},
		})
		assert.NoError(t, k.validatePolicy(), "expected no error for minMaturity: %s", m)
	}
}

func TestValidatePolicy_DeprecatedFloorIsError(t *testing.T) {
	k := catalogWithPolicy(&types.CatalogPolicy{
		Lifecycle: &types.CatalogLifecyclePolicy{MinMaturity: types.MaturityDeprecated},
	})
	err := k.validatePolicy()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "policy.lifecycle.minMaturity")
	assert.Contains(t, err.Error(), "deprecated")
}

func TestValidatePolicy_UnknownFloorIsError(t *testing.T) {
	k := catalogWithPolicy(&types.CatalogPolicy{
		Lifecycle: &types.CatalogLifecyclePolicy{MinMaturity: types.LifecycleMaturity("production")},
	})
	err := k.validatePolicy()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "policy.lifecycle.minMaturity")
}

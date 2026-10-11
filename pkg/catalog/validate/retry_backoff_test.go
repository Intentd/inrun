package validate

import (
	"testing"

	"time"

	"github.com/intentd/inrun/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func catalogWithRetryBackoff(crdName string, rec *types.ReconcileConfig, box *types.OperatorBoxConfig) *executor {
	if rec != nil {
		if box.Reconcile == nil {
			box.Reconcile = rec
		} else {
			// merge reconcile-level settings into the existing block (preserves OnReconcile etc.)
			if rec.Resync.Duration != 0 {
				box.Reconcile.Resync = rec.Resync
			}
			if rec.Queue.RetryBackoff != nil || rec.Queue.MaxDepth != 0 {
				box.Reconcile.Queue = rec.Queue
			}
		}
	}
	return newCatalogExec(map[string]types.CRDEntry{
		crdName: {OperatorBox: box},
	})
}

func dur(d time.Duration) types.Duration { return types.Duration{Duration: d} }

// ── queue.retryBackoff ────────────────────────────────────────────────────────

func TestValidateRetryBackoff_NoConfig(t *testing.T) {
	e := catalogWithRetryBackoff("myapp", nil, &types.OperatorBoxConfig{})
	assert.NoError(t, e.validateRetryBackoff())
}

func TestValidateRetryBackoff_ValidShorthand(t *testing.T) {
	e := catalogWithRetryBackoff("myapp", &types.ReconcileConfig{
		Resync: dur(10 * time.Minute),
		Queue:  types.Queue{RetryBackoff: &types.RetryBackoffConfig{Initial: dur(5 * time.Second)}},
	}, &types.OperatorBoxConfig{})
	assert.NoError(t, e.validateRetryBackoff())
}

func TestValidateRetryBackoff_ValidFullForm(t *testing.T) {
	e := catalogWithRetryBackoff("myapp", &types.ReconcileConfig{
		Resync: dur(10 * time.Minute),
		Queue: types.Queue{RetryBackoff: &types.RetryBackoffConfig{
			Initial:     dur(500 * time.Millisecond),
			Max:         dur(30 * time.Second),
			Multiplier:  2.0,
			MaxAttempts: 3,
		}},
	}, &types.OperatorBoxConfig{})
	assert.NoError(t, e.validateRetryBackoff())
}

func TestValidateRetryBackoff_NegativeMultiplierErrors(t *testing.T) {
	e := catalogWithRetryBackoff("myapp", &types.ReconcileConfig{
		Queue: types.Queue{RetryBackoff: &types.RetryBackoffConfig{
			Initial:    dur(500 * time.Millisecond),
			Multiplier: -1.0,
		}},
	}, &types.OperatorBoxConfig{})
	err := e.validateRetryBackoff()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "multiplier must be >= 0")
}

func TestValidateRetryBackoff_MaxLessThanInitialErrors(t *testing.T) {
	e := catalogWithRetryBackoff("myapp", &types.ReconcileConfig{
		Queue: types.Queue{RetryBackoff: &types.RetryBackoffConfig{
			Initial: dur(30 * time.Second),
			Max:     dur(1 * time.Second),
		}},
	}, &types.OperatorBoxConfig{})
	err := e.validateRetryBackoff()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "max")
	assert.Contains(t, err.Error(), "must be >= initial")
}

func TestValidateRetryBackoff_WorstCaseExceedsResyncWarns(t *testing.T) {
	// maxAttempts=5, initial=10s, multiplier=2 → delays: 10s+20s+40s+80s = 150s > 30s resync
	e := catalogWithRetryBackoff("myapp", &types.ReconcileConfig{
		Resync: dur(30 * time.Second),
		Queue: types.Queue{RetryBackoff: &types.RetryBackoffConfig{
			Initial:     dur(10 * time.Second),
			Max:         dur(5 * time.Minute),
			Multiplier:  2.0,
			MaxAttempts: 5,
		}},
	}, &types.OperatorBoxConfig{})
	assert.NoError(t, e.validateRetryBackoff())
	crd := e.cat.EnabledCRDs()["myapp"]
	assert.True(t, crd.Warnings.Contains("worst-case delay"))
}

func TestValidateRetryBackoff_WorstCaseWithinResyncNoWarning(t *testing.T) {
	// maxAttempts=3, initial=500ms, multiplier=2 → delays: 500ms+1s = 1.5s < 10m resync
	e := catalogWithRetryBackoff("myapp", &types.ReconcileConfig{
		Resync: dur(10 * time.Minute),
		Queue: types.Queue{RetryBackoff: &types.RetryBackoffConfig{
			Initial:     dur(500 * time.Millisecond),
			Max:         dur(30 * time.Second),
			Multiplier:  2.0,
			MaxAttempts: 3,
		}},
	}, &types.OperatorBoxConfig{})
	assert.NoError(t, e.validateRetryBackoff())
	crd := e.cat.EnabledCRDs()["myapp"]
	assert.False(t, crd.Warnings.Contains("worst-case delay"))
}

// ── external[].retryBackoff ───────────────────────────────────────────────────

func TestValidateRetryBackoff_ExternalValidShorthand(t *testing.T) {
	e := catalogWithRetryBackoff("myapp", &types.ReconcileConfig{
		Resync: dur(10 * time.Minute),
	}, &types.OperatorBoxConfig{
		Reconcile: &types.ReconcileConfig{
			OnReconcile: &types.HookTemplates{
				External: []types.ExternalCallSpec{
					{Name: "health", URL: "http://svc/health", RetryBackoff: &types.RetryBackoffConfig{
						Initial: dur(1 * time.Second),
					}},
				},
			},
		},
	})
	assert.NoError(t, e.validateRetryBackoff())
}

func TestValidateRetryBackoff_ExternalNegativeMultiplierErrors(t *testing.T) {
	e := catalogWithRetryBackoff("myapp", nil, &types.OperatorBoxConfig{
		Reconcile: &types.ReconcileConfig{
			OnReconcile: &types.HookTemplates{
				External: []types.ExternalCallSpec{
					{Name: "health", URL: "http://svc/health", RetryBackoff: &types.RetryBackoffConfig{
						Multiplier: -2.0,
					}},
				},
			},
		},
	})
	err := e.validateRetryBackoff()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "multiplier must be >= 0")
}

func TestValidateRetryBackoff_ExternalWorstCaseExceedsResyncWarns(t *testing.T) {
	e := catalogWithRetryBackoff("myapp", &types.ReconcileConfig{
		Resync: dur(5 * time.Second),
	}, &types.OperatorBoxConfig{
		Reconcile: &types.ReconcileConfig{
			OnReconcile: &types.HookTemplates{
				External: []types.ExternalCallSpec{
					{Name: "db", URL: "postgres://svc/db", RetryBackoff: &types.RetryBackoffConfig{
						Initial:     dur(3 * time.Second),
						Max:         dur(1 * time.Minute),
						Multiplier:  2.0,
						MaxAttempts: 4,
					}},
				},
			},
		},
	})
	assert.NoError(t, e.validateRetryBackoff())
	crd := e.cat.EnabledCRDs()["myapp"]
	assert.True(t, crd.Warnings.Contains("worst-case delay"))
}

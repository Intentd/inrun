package validate

import (
	"testing"

	"github.com/intentd/inrun/domain"
	"github.com/intentd/inrun/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// catalogWithObserve builds an executor with a single CRD containing the
// supplied observe configuration.
func catalogWithObserve(crdName string, observe *types.Observe) *executor {
	return newCatalogExec(map[string]types.CRDEntry{
		crdName: {OperatorBox: &types.OperatorBoxConfig{
			Observe: observe,
		},
		}})
}

func TestValidateEventEntries_ObserveNil(t *testing.T) {
	e := catalogWithObserve("myapp", nil)
	assert.NoError(t, e.validateEventEntries())
}

func TestValidateEventEntries_Empty(t *testing.T) {
	e := catalogWithObserve("myapp", &types.Observe{})
	assert.NoError(t, e.validateEventEntries())
}

func TestValidateEventEntries_Valid(t *testing.T) {
	e := catalogWithObserve("myapp", &types.Observe{
		Events: map[string]*types.EventEntry{
			"dbReady": {
				Reason: "DatabaseReady",
			},
			"sync": {
				Action: "Sync",
				Type:   "Normal",
			},
			"operator": {
				ReportingController: "example.com/operator",
				ReportingInstance:   "operator-1",
			},
			"regarding": {
				Regarding: &domain.ManagedResource{
					Kind: "Database",
					Name: "my-db",
				},
			},
			"related": {
				Related: &domain.ManagedResource{
					Kind: "ConfigMap",
					Name: "config",
				},
			},
			"namespace": {
				Namespace: "default",
			},
			"keyFrom": {
				KeyFrom: &types.WatchKeyFrom{Name: "myapp"},
			},
		},
	})
	assert.NoError(t, e.validateEventEntries())
}

func TestValidateEventEntries_NoMatcher(t *testing.T) {
	e := catalogWithObserve("myapp", &types.Observe{
		Events: map[string]*types.EventEntry{
			"empty": {},
		},
	})
	err := e.validateEventEntries()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "event entry must declare at least one matching or routing field")
}

func TestValidateEventEntries_DifferentMatchers(t *testing.T) {
	e := catalogWithObserve("myapp", &types.Observe{
		Events: map[string]*types.EventEntry{
			"ready":  {Reason: "DatabaseReady"},
			"failed": {Reason: "DatabaseFailed"},
			"sync":   {Action: "Sync"},
			"warn":   {Type: "Warning"},
		},
	})
	assert.NoError(t, e.validateEventEntries())
}

func TestValidateEventEntries_Regarding(t *testing.T) {
	e := catalogWithObserve("myapp", &types.Observe{
		Events: map[string]*types.EventEntry{
			"database": {
				Regarding: &domain.ManagedResource{
					APIVersion: "databases.example.com/v1",
					Kind:       "Database",
					Name:       "my-db",
					Namespace:  "default",
				},
			},
		},
	})
	assert.NoError(t, e.validateEventEntries())
}

func TestValidateEventEntries_Related(t *testing.T) {
	e := catalogWithObserve("myapp", &types.Observe{
		Events: map[string]*types.EventEntry{
			"config": {
				Related: &domain.ManagedResource{
					APIVersion: "v1",
					Kind:       "ConfigMap",
					Name:       "shared-config",
				},
			},
		},
	})
	assert.NoError(t, e.validateEventEntries())
}

func TestValidateEventEntries_KeyFromValidLabel(t *testing.T) {
	e := catalogWithObserve("myapp", &types.Observe{
		Events: map[string]*types.EventEntry{
			"dbReady": {
				Reason:  "DatabaseReady",
				KeyFrom: &types.WatchKeyFrom{Label: "app.kubernetes.io/cr-owner"},
			},
		},
	})
	require.NoError(t, e.validateEventEntries())
}

func TestValidateEventEntries_KeyFromValidName(t *testing.T) {
	e := catalogWithObserve("myapp", &types.Observe{
		Events: map[string]*types.EventEntry{
			"dbReady": {
				Reason:  "DatabaseReady",
				KeyFrom: &types.WatchKeyFrom{Name: "myapp"},
			},
		},
	})
	require.NoError(t, e.validateEventEntries())
}

func TestValidateEventEntries_KeyFromBothLabelAndName(t *testing.T) {
	e := catalogWithObserve("myapp", &types.Observe{
		Events: map[string]*types.EventEntry{
			"dbReady": {
				Reason: "DatabaseReady",
				KeyFrom: &types.WatchKeyFrom{
					Label: "some-label",
					Name:  "some-name",
				},
			},
		},
	})
	err := e.validateEventEntries()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exactly one of label or name")
}

func TestValidateEventEntries_KeyFromNeitherLabelNorName(t *testing.T) {
	e := catalogWithObserve("myapp", &types.Observe{
		Events: map[string]*types.EventEntry{
			"dbReady": {
				Reason:  "DatabaseReady",
				KeyFrom: &types.WatchKeyFrom{},
			},
		},
	})
	err := e.validateEventEntries()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "neither label nor name")
}

func TestValidateEventEntries_KeyFromNamespaceWithLabelRejected(t *testing.T) {
	e := catalogWithObserve("myapp", &types.Observe{
		Events: map[string]*types.EventEntry{
			"dbReady": {
				Reason: "DatabaseReady",
				KeyFrom: &types.WatchKeyFrom{
					Label:     "some-label",
					Namespace: "default",
				},
			},
		},
	})
	err := e.validateEventEntries()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "namespace has no effect")
}

func TestValidateEventEntries_OnValid(t *testing.T) {
	e := catalogWithObserve("myapp", &types.Observe{
		Events: map[string]*types.EventEntry{
			"dbReady": {
				Reason: "DatabaseReady",
				On:     []string{"create", "update", "delete"},
			},
		},
	})
	assert.NoError(t, e.validateEventEntries())
}

func TestValidateEventEntries_OnInvalid(t *testing.T) {
	e := catalogWithObserve("myapp", &types.Observe{
		Events: map[string]*types.EventEntry{
			"dbReady": {
				Reason: "DatabaseReady",
				On:     []string{"create", "modify"},
			},
		},
	})
	err := e.validateEventEntries()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown on: value(s) [modify]")
}

func TestValidateEventEntries_OnAllValid(t *testing.T) {
	e := catalogWithObserve("myapp", &types.Observe{
		Events: map[string]*types.EventEntry{
			"dbReady": {
				Reason: "DatabaseReady",
				On:     types.ValidObserveEvents(),
			},
		},
	})
	assert.NoError(t, e.validateEventEntries())
}

func TestValidateEventEntries_EventNameCamelCase(t *testing.T) {
	tests := []struct {
		name      string
		eventName string
		valid     bool
	}{
		{
			name:      "lower camel case",
			eventName: "dbReady",
			valid:     true,
		},
		{
			name:      "multiple words",
			eventName: "databaseReady",
			valid:     true,
		},
		{
			name:      "acronym",
			eventName: "dbAPIReady",
			valid:     true,
		},
		{
			name:      "hyphenated",
			eventName: "db-ready",
			valid:     false,
		},
		{
			name:      "snake case",
			eventName: "db_ready",
			valid:     true,
		},
		{
			name:      "with space",
			eventName: "Db Ready",
			valid:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			observe := &types.Observe{
				Events: map[string]*types.EventEntry{
					tt.eventName: {
						Reason: "DatabaseReady",
					},
				},
			}

			err := validateCRDEventEntries(
				"app",
				types.CRDEntry{
					OperatorBox: &types.OperatorBoxConfig{
						Observe: observe,
					},
				},
			)

			if tt.valid && err != nil {
				t.Fatalf("expected valid event name, got error: %v", err)
			}
			if !tt.valid && err == nil {
				t.Fatal("expected invalid event name")
			}
		})
	}
}

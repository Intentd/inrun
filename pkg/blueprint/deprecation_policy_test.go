package blueprint

import (
	"testing"

	"github.com/intentd/inrun/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func blueprintWithDeprecation(d *types.BlueprintDeprecation) *Blueprint {
	if d == nil {
		return NewEmptyBlueprint()
	}
	return &Blueprint{lifecycle: &types.BlueprintLifecycle{Deprecation: d}}
}

func TestCheckDeprecationPolicy_NilBlock(t *testing.T) {
	bp := blueprintWithDeprecation(nil)
	assert.NoError(t, bp.CheckDeprecationPolicy())
}

func TestCheckDeprecationPolicy_NoTimeline_Deprecated(t *testing.T) {
	bp := blueprintWithDeprecation(&types.BlueprintDeprecation{
		Message: "use v2",
	})
	err := bp.CheckDeprecationPolicy()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "deprecated")
	assert.Contains(t, err.Error(), "lifecycle:")
}

func TestCheckDeprecationPolicy_WarningWindow(t *testing.T) {
	bp := blueprintWithDeprecation(&types.BlueprintDeprecation{
		Message: "use v2",
		Timeline: &types.DeprecationTimeline{
			From: "2020-01-01", // in the past — window is open
			To:   "2099-01-01",
		},
	})
	err := bp.CheckDeprecationPolicy()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "lifecycle:")
}

func TestCheckDeprecationPolicy_BeforeFrom_NoAcceptNeeded(t *testing.T) {
	bp := blueprintWithDeprecation(&types.BlueprintDeprecation{
		Message: "use v2",
		Timeline: &types.DeprecationTimeline{
			From: "2099-01-01", // not yet reached
			To:   "2099-06-01",
		},
	})
	assert.NoError(t, bp.CheckDeprecationPolicy())
}

func TestCheckDeprecationPolicy_EOL(t *testing.T) {
	bp := blueprintWithDeprecation(&types.BlueprintDeprecation{
		Message: "use v2",
		Timeline: &types.DeprecationTimeline{
			From: "2020-01-01",
			To:   "2020-06-01", // past EOL
		},
	})
	err := bp.CheckDeprecationPolicy()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "end-of-life")
	assert.Contains(t, err.Error(), "lifecycle:")
}

func TestCheckDeprecationPolicy_MigrationTargetInError(t *testing.T) {
	bp := blueprintWithDeprecation(&types.BlueprintDeprecation{
		Message:    "use v2",
		MigratedTo: "my-pattern:v2.0.0",
	})
	err := bp.CheckDeprecationPolicy()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "my-pattern:v2.0.0")
}

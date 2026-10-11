// Tests for NeedsRotation (secret_rotation.go). ParseTimeDuration itself now
// lives in pkg/utils — see pkg/utils/duration_test.go.
package types_test

import (
	"testing"
	"time"

	"github.com/intentd/inrun/pkg/types"
	"github.com/stretchr/testify/assert"
)

// ── NeedsRotation ─────────────────────────────────────────────────────────────

func TestNeedsRotation_NoRotateAfter(t *testing.T) {
	past := time.Now().Add(-400 * 24 * time.Hour).Format(time.RFC3339)
	assert.False(t, types.NeedsRotation(past, ""))
}

func TestNeedsRotation_NotYetExpired(t *testing.T) {
	recent := time.Now().Add(-30 * 24 * time.Hour).Format(time.RFC3339)
	assert.False(t, types.NeedsRotation(recent, "90d"))
}

func TestNeedsRotation_Expired(t *testing.T) {
	old := time.Now().Add(-91 * 24 * time.Hour).Format(time.RFC3339)
	assert.True(t, types.NeedsRotation(old, "90d"))
}

func TestNeedsRotation_ExactThreshold(t *testing.T) {
	// Just at the boundary: age == threshold means rotate (>=)
	exact := time.Now().Add(-365 * 24 * time.Hour).Format(time.RFC3339)
	assert.True(t, types.NeedsRotation(exact, "1y"))
}

func TestNeedsRotation_InvalidTimestamp(t *testing.T) {
	// Unparseable annotation → rotate to be safe
	assert.True(t, types.NeedsRotation("not-a-timestamp", "30d"))
}

func TestNeedsRotation_InvalidDuration(t *testing.T) {
	// Invalid duration → do not rotate unexpectedly
	recent := time.Now().Add(-5 * 24 * time.Hour).Format(time.RFC3339)
	assert.False(t, types.NeedsRotation(recent, "invalid"))
}

func TestNeedsRotation_WeekRotation(t *testing.T) {
	old := time.Now().Add(-8 * 24 * time.Hour).Format(time.RFC3339)
	assert.True(t, types.NeedsRotation(old, "1w"))
}

func TestNeedsRotation_MonthRotation_NotYet(t *testing.T) {
	recent := time.Now().Add(-15 * 24 * time.Hour).Format(time.RFC3339)
	assert.False(t, types.NeedsRotation(recent, "1mo"))
}

package types_test

import (
	"testing"

	"github.com/intentd/inrun/pkg/types"
	"github.com/stretchr/testify/assert"
)

// ── IsNoteRef ─────────────────────────────────────────────────────────────────

func TestIsNoteRef(t *testing.T) {
	tests := []struct {
		field string
		want  bool
	}{
		// note calls — should be true
		{"{{ inBusinessHours }}", true},
		{"{{ isBusy }}", true},
		{"{{myNote}}", true},
		{"{{ combinedCheck }}", true},

		// data refs — dot-path after {{ is not a note
		{"{{ .health.status }}", false},
		{"{{ .metrics.queueDepth }}", false},
		{"{{.health.readyCount}}", false},
		{".health.status", false},
		{".health.status", false},
		{"spec.image", false},

		// sentinels — valid sentinel names are not notes
		{"{{ generationChanged }}", false},
		{"{{ labelsChanged }}", false},
		{"{{ annotationsChanged }}", false},

		// malformed / empty
		{"", false},
		{"{{}}", false},
		{"{{ }}", false},
	}
	for _, tc := range tests {
		t.Run(tc.field, func(t *testing.T) {
			assert.Equal(t, tc.want, types.IsNoteRef(tc.field))
		})
	}
}

// ── NoteRefName ───────────────────────────────────────────────────────────────

func TestNoteRefName(t *testing.T) {
	assert.Equal(t, "inBusinessHours", types.NoteRefName("{{ inBusinessHours }}"))
	assert.Equal(t, "isBusy", types.NoteRefName("{{isBusy}}"))
	assert.Equal(t, "myNote", types.NoteRefName("{{  myNote  }}"))
}

// ── NoteRegistry.ExpressionFor ────────────────────────────────────────────────

func TestNoteRegistry_ExpressionFor(t *testing.T) {
	nr := types.NoteRegistry{Functions: []types.UserDefinedNote{
		{Name: "isHealthy", Expression: `{{ eq .health.status "healthy" }}`},
		{Name: "isBusy", Expression: `{{ gt .metrics.workersBusyPercent 80 }}`},
	}}

	assert.Equal(t, `{{ eq .health.status "healthy" }}`, nr.ExpressionFor("isHealthy"))
	assert.Equal(t, `{{ gt .metrics.workersBusyPercent 80 }}`, nr.ExpressionFor("isBusy"))
	assert.Equal(t, "", nr.ExpressionFor("unknown"))
}

// ── NoteRegistry.ContainsInExpression ────────────────────────────────────────

func TestNoteRegistry_ContainsInExpression(t *testing.T) {
	t.Run("direct reference to .health in expression", func(t *testing.T) {
		nr := types.NoteRegistry{Functions: []types.UserDefinedNote{
			{Name: "isHealthy", Expression: `{{ eq .health.status "healthy" }}`},
		}}
		assert.True(t, nr.ContainsInExpression("isHealthy", ".health."))
		assert.False(t, nr.ContainsInExpression("isHealthy", ".metrics."))
	})

	t.Run("transitive — note calls another note that references .health", func(t *testing.T) {
		nr := types.NoteRegistry{Functions: []types.UserDefinedNote{
			{Name: "isHealthy", Expression: `{{ eq .health.status "healthy" }}`},
			{Name: "isReady", Expression: `{{ and isHealthy (gt .metrics.readyCount 0) }}`},
			{Name: "canAccept", Expression: `{{ and isReady (lt .metrics.queueDepth 500) }}`},
		}}
		assert.True(t, nr.ContainsInExpression("canAccept", ".health."))
		assert.True(t, nr.ContainsInExpression("canAccept", ".metrics."))
	})

	t.Run("word boundary — note named 'is' does not match inside 'isHealthy'", func(t *testing.T) {
		nr := types.NoteRegistry{Functions: []types.UserDefinedNote{
			{Name: "is", Expression: `{{ eq .health.status "healthy" }}`},
			{Name: "isHealthy", Expression: `{{ gt .metrics.workersBusyPercent 80 }}`},
		}}
		// canAccept calls isHealthy (whole word), not is (which is a prefix of isHealthy)
		assert.False(t, nr.ContainsInExpression("isHealthy", ".health."))
		// "is" is a whole word in a plain expression
		nr2 := types.NoteRegistry{Functions: []types.UserDefinedNote{
			{Name: "is", Expression: `{{ eq .health.status "healthy" }}`},
			{Name: "outer", Expression: `{{ is }}`},
		}}
		assert.True(t, nr2.ContainsInExpression("outer", ".health."))
	})

	t.Run("cycle — mutual recursion does not loop", func(t *testing.T) {
		nr := types.NoteRegistry{Functions: []types.UserDefinedNote{
			{Name: "alpha", Expression: `{{ beta }}`},
			{Name: "beta", Expression: `{{ alpha }}`},
		}}
		assert.False(t, nr.ContainsInExpression("alpha", ".health."))
	})

	t.Run("note not in registry returns false", func(t *testing.T) {
		nr := types.NoteRegistry{}
		assert.False(t, nr.ContainsInExpression("unknown", ".health."))
	})

	t.Run("note composing both .health and .metrics", func(t *testing.T) {
		nr := types.NoteRegistry{Functions: []types.UserDefinedNote{
			{Name: "combined", Expression: `{{ and (eq .health.status "healthy") (lt .metrics.queueDepth 100) }}`},
		}}
		assert.True(t, nr.ContainsInExpression("combined", ".health."))
		assert.True(t, nr.ContainsInExpression("combined", ".metrics."))
	})
}

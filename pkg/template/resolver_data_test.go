package template

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWithExternal_AddsToEarlierCalls(t *testing.T) {
	r := NewResolverFromMap(map[string]interface{}{}).
		WithExternal(map[string]interface{}{"plan": map[string]interface{}{"status": "200"}}).
		WithExternal(map[string]interface{}{"audit": map[string]interface{}{"status": "201"}})

	plan, err := r.Resolve("{{ .external.plan.status }}")
	require.NoError(t, err)
	assert.Equal(t, "200", plan, "reconcile.external stays in scope beside a block's own calls")
	audit, err := r.Resolve("{{ .external.audit.status }}")
	require.NoError(t, err)
	assert.Equal(t, "201", audit)
}

func TestWithExternal_LaterCallOfTheSameNameWins(t *testing.T) {
	first := NewResolverFromMap(map[string]interface{}{}).
		WithExternal(map[string]interface{}{"plan": map[string]interface{}{"status": "200"}})
	second := first.WithExternal(map[string]interface{}{"plan": map[string]interface{}{"status": "503"}})

	out, err := second.Resolve("{{ .external.plan.status }}")
	require.NoError(t, err)
	assert.Equal(t, "503", out)
	out, err = first.Resolve("{{ .external.plan.status }}")
	require.NoError(t, err)
	assert.Equal(t, "200", out, "the earlier resolver is unchanged")
}

func TestWithEnv(t *testing.T) {
	base := NewResolverFromMap(map[string]interface{}{})
	r := base.WithEnv(map[string]string{"TOKEN": "t0k"})
	got, err := r.Resolve("Bearer {{ .env.TOKEN }}")
	if err != nil {
		t.Fatal(err)
	}
	if got != "Bearer t0k" {
		t.Fatalf("got %q", got)
	}
	if _, ok := base.Data()["env"]; ok {
		t.Fatal("WithEnv changed the receiver")
	}
}

package webhook

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/inrundev/inrun/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// At admission, the rules generated for serve labels set the labels, and the
// patch reaches them through a correctly escaped pointer.
func TestServeLabelRules_Admission(t *testing.T) {
	crd := types.CRDEntry{Serve: &types.ServeConfig{Labels: map[string]types.ServeFieldConfig{
		"app.kubernetes.io/name": {Default: "web"},
	}}}
	original := map[string]interface{}{"metadata": map[string]interface{}{"name": "site"}, "spec": map[string]interface{}{}}
	mutated := deepCopyMap(original)

	changes, err := (&WebhookServer{}).applyMutationRules(context.Background(), mutated,
		&types.MutationConfig{Rules: crd.ServeFieldMutationRules()}, "Website")
	require.NoError(t, err)
	require.Len(t, changes, 1)

	labels := mutated["metadata"].(map[string]interface{})["labels"].(map[string]interface{})
	assert.Equal(t, "web", labels["app.kubernetes.io/name"])

	raw, err := buildJSONPatch(original, mutated)
	require.NoError(t, err)
	var ops []map[string]interface{}
	require.NoError(t, json.Unmarshal(raw, &ops))
	require.Len(t, ops, 1)
	assert.Equal(t, "/metadata/labels", ops[0]["path"])
	assert.Equal(t, map[string]interface{}{"app.kubernetes.io/name": "web"}, ops[0]["value"])
}

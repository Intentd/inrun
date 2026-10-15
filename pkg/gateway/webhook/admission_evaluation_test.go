// Tests for the pure evaluation logic in admission_evaluation.go and
// the shared helper functions in admission.go.
package webhook

import (
	"context"
	"encoding/json"
	"github.com/intentd/inrun/pkg/utils"
	"testing"

	"github.com/intentd/inrun/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ── setFieldPath ──────────────────────────────────────────────────────────────

func TestSetFieldPath(t *testing.T) {
	t.Run("top-level field", func(t *testing.T) {
		obj := map[string]interface{}{}
		setFieldPath(obj, "name", "inrun")
		assert.Equal(t, "inrun", obj["name"])
	})

	t.Run("nested field creates intermediate map", func(t *testing.T) {
		obj := map[string]interface{}{}
		setFieldPath(obj, "spec.image", "myorg/app:latest")
		spec, ok := obj["spec"].(map[string]interface{})
		require.True(t, ok, "spec should be a map")
		assert.Equal(t, "myorg/app:latest", spec["image"])
	})

	t.Run("deeply nested creates all levels", func(t *testing.T) {
		obj := map[string]interface{}{}
		setFieldPath(obj, "spec.db.host", "localhost")
		spec := obj["spec"].(map[string]interface{})
		db := spec["db"].(map[string]interface{})
		assert.Equal(t, "localhost", db["host"])
	})

	t.Run("overwrites existing scalar", func(t *testing.T) {
		obj := map[string]interface{}{
			"spec": map[string]interface{}{"replicas": "1"},
		}
		setFieldPath(obj, "spec.replicas", "3")
		spec := obj["spec"].(map[string]interface{})
		assert.Equal(t, "3", spec["replicas"])
	})

	t.Run("sibling keys preserved", func(t *testing.T) {
		obj := map[string]interface{}{
			"spec": map[string]interface{}{"image": "nginx"},
		}
		setFieldPath(obj, "spec.replicas", "2")
		spec := obj["spec"].(map[string]interface{})
		assert.Equal(t, "nginx", spec["image"])
		assert.Equal(t, "2", spec["replicas"])
	})
}

func specObj(fields map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{"spec": fields}
}

// ── evaluateValidationRules: multi-rule ───────────────────────────────────────

func TestEvaluateValidationRules_MixedDenyAndWarn(t *testing.T) {
	h := &WebhookServer{}

	obj := map[string]interface{}{
		"spec": map[string]interface{}{
			"image":    "docker.io/nginx:1.25", // violates deny rule
			"replicas": float64(15),            // violates deny rule
		},
		// metadata.labels.team absent → violates warn rule
	}
	cfg := &types.ValidationConfig{
		Rules: []types.ValidationRule{
			{Field: "spec.image", Prefix: "myorg/", Message: "wrong registry", Action: types.ValidationActionDeny},
			{Field: "spec.replicas", Max: "10", Message: "too many replicas", Action: types.ValidationActionDeny},
			{Field: "metadata.labels.team", Exists: utils.BoolPtr(true), Message: "team label required", Action: types.ValidationActionWarn},
		},
	}

	denials, warnings := h.evaluateValidationRules(context.Background(), obj, cfg, "Website")

	assert.Len(t, denials, 2, "should collect all deny violations — not fail-fast")
	assert.Len(t, warnings, 1)
	assert.Equal(t, "spec.image", denials[0].Field)
	assert.Equal(t, "spec.replicas", denials[1].Field)
	assert.Equal(t, "metadata.labels.team", warnings[0].Field)
}

func TestEvaluateValidationRules_AllPass(t *testing.T) {
	h := &WebhookServer{}

	obj := map[string]interface{}{
		"spec": map[string]interface{}{
			"image":    "myorg/nginx:1.25",
			"replicas": float64(3),
		},
		"metadata": map[string]interface{}{
			"labels": map[string]interface{}{"team": "platform"},
		},
	}
	cfg := &types.ValidationConfig{
		Rules: []types.ValidationRule{
			{Field: "spec.image", Prefix: "myorg/", Message: "wrong registry", Action: types.ValidationActionDeny},
			{Field: "spec.replicas", Max: "10", Message: "too many", Action: types.ValidationActionDeny},
			{Field: "metadata.labels.team", Exists: utils.BoolPtr(true), Message: "team required", Action: types.ValidationActionWarn},
		},
	}

	denials, warnings := h.evaluateValidationRules(context.Background(), obj, cfg, "Website")

	assert.Empty(t, denials)
	assert.Empty(t, warnings)
}

func TestEvaluateValidationRules_EmptyActionDefaultsToDeny(t *testing.T) {
	h := &WebhookServer{}

	obj := specObj(map[string]interface{}{"image": "bad-registry/nginx"})
	cfg := &types.ValidationConfig{
		Rules: []types.ValidationRule{
			// Action field not set — should default to deny
			{Field: "spec.image", Prefix: "myorg/", Message: "wrong registry"},
		},
	}

	denials, warnings := h.evaluateValidationRules(context.Background(), obj, cfg, "Website")

	assert.Len(t, denials, 1)
	assert.Empty(t, warnings)
}

// ── applyMutationRules ────────────────────────────────────────────────────────

func TestApplyMutationRules_NilConfig(t *testing.T) {
	h := &WebhookServer{}
	obj := specObj(map[string]interface{}{})

	changes, err := h.applyMutationRules(context.Background(), obj, nil, "Website")

	require.NoError(t, err)
	assert.Empty(t, changes)
}

func TestApplyMutationRules_EmptyRules(t *testing.T) {
	h := &WebhookServer{}
	obj := specObj(map[string]interface{}{})
	cfg := &types.MutationConfig{Rules: []types.MutationRule{}}

	changes, err := h.applyMutationRules(context.Background(), obj, cfg, "Website")

	require.NoError(t, err)
	assert.Empty(t, changes)
}

func TestApplyMutationRules_DefaultAppliedWhenAbsent(t *testing.T) {
	h := &WebhookServer{}
	obj := map[string]interface{}{
		"metadata": map[string]interface{}{"name": "my-site"},
		"spec":     map[string]interface{}{},
	}
	cfg := &types.MutationConfig{
		Rules: []types.MutationRule{
			{Field: "spec.replicas", Default: "2"},
		},
	}

	changes, err := h.applyMutationRules(context.Background(), obj, cfg, "Website")

	require.NoError(t, err)
	require.Len(t, changes, 1)
	assert.Equal(t, "spec.replicas", changes[0].Field)
	assert.Equal(t, "", changes[0].OldValue)
	assert.Equal(t, "2", changes[0].NewValue)
	assert.Equal(t, "default", changes[0].ChangeType)

	// Verify in-place mutation
	spec := obj["spec"].(map[string]interface{})
	assert.Equal(t, "2", spec["replicas"])
}

func TestApplyMutationRules_DefaultSkippedWhenPresent(t *testing.T) {
	h := &WebhookServer{}
	obj := map[string]interface{}{
		"metadata": map[string]interface{}{"name": "my-site"},
		"spec":     map[string]interface{}{"replicas": "5"},
	}
	cfg := &types.MutationConfig{
		Rules: []types.MutationRule{
			{Field: "spec.replicas", Default: "2"},
		},
	}

	changes, err := h.applyMutationRules(context.Background(), obj, cfg, "Website")

	require.NoError(t, err)
	assert.Empty(t, changes, "default must not overwrite an existing value")

	spec := obj["spec"].(map[string]interface{})
	assert.Equal(t, "5", spec["replicas"], "original value must be preserved")
}

func TestApplyMutationRules_DefaultSkippedWhenValueAlreadyMatches(t *testing.T) {
	h := &WebhookServer{}
	obj := map[string]interface{}{
		"metadata": map[string]interface{}{"name": "my-site"},
		"spec":     map[string]interface{}{"logLevel": "info"},
	}
	cfg := &types.MutationConfig{
		Rules: []types.MutationRule{
			{Field: "spec.logLevel", Default: "info"},
		},
	}

	changes, err := h.applyMutationRules(context.Background(), obj, cfg, "Website")

	require.NoError(t, err)
	assert.Empty(t, changes, "no change emitted when value already matches")
}

func TestApplyMutationRules_OverrideAlwaysApplies(t *testing.T) {
	h := &WebhookServer{}
	obj := map[string]interface{}{
		"metadata": map[string]interface{}{"name": "my-site"},
		"spec":     map[string]interface{}{},
	}
	cfg := &types.MutationConfig{
		Rules: []types.MutationRule{
			{Field: "metadata.labels.managed-by", Override: "inrun"},
		},
	}

	changes, err := h.applyMutationRules(context.Background(), obj, cfg, "Website")

	require.NoError(t, err)
	require.Len(t, changes, 1)
	assert.Equal(t, "metadata.labels.managed-by", changes[0].Field)
	assert.Equal(t, "inrun", changes[0].NewValue)
	assert.Equal(t, "override", changes[0].ChangeType)
}

func TestApplyMutationRules_OverrideReplacesExistingValue(t *testing.T) {
	h := &WebhookServer{}
	obj := map[string]interface{}{
		"metadata": map[string]interface{}{
			"name": "my-site",
			"labels": map[string]interface{}{
				"managed-by": "helm",
			},
		},
		"spec": map[string]interface{}{},
	}
	cfg := &types.MutationConfig{
		Rules: []types.MutationRule{
			{Field: "metadata.labels.managed-by", Override: "inrun"},
		},
	}

	changes, err := h.applyMutationRules(context.Background(), obj, cfg, "Website")

	require.NoError(t, err)
	require.Len(t, changes, 1)
	assert.Equal(t, "helm", changes[0].OldValue)
	assert.Equal(t, "inrun", changes[0].NewValue)
	assert.Equal(t, "override", changes[0].ChangeType)
}

func TestApplyMutationRules_MultipleRules(t *testing.T) {
	h := &WebhookServer{}
	obj := map[string]interface{}{
		"metadata": map[string]interface{}{"name": "my-site"},
		"spec":     map[string]interface{}{},
	}
	cfg := &types.MutationConfig{
		Rules: []types.MutationRule{
			{Field: "spec.replicas", Default: "2"},
			{Field: "spec.logLevel", Default: "info"},
			{Field: "metadata.labels.managed-by", Override: "inrun"},
		},
	}

	changes, err := h.applyMutationRules(context.Background(), obj, cfg, "Website")

	require.NoError(t, err)
	assert.Len(t, changes, 3)

	spec := obj["spec"].(map[string]interface{})
	assert.Equal(t, "2", spec["replicas"])
	assert.Equal(t, "info", spec["logLevel"])
}

func TestApplyMutationRules_NestedFieldCreated(t *testing.T) {
	h := &WebhookServer{}
	obj := map[string]interface{}{
		"metadata": map[string]interface{}{"name": "my-site"},
		"spec":     map[string]interface{}{},
	}
	cfg := &types.MutationConfig{
		Rules: []types.MutationRule{
			{Field: "spec.db.engine", Default: "postgres"},
		},
	}

	changes, err := h.applyMutationRules(context.Background(), obj, cfg, "Website")

	require.NoError(t, err)
	require.Len(t, changes, 1)

	spec := obj["spec"].(map[string]interface{})
	db, ok := spec["db"].(map[string]interface{})
	require.True(t, ok, "intermediate map should have been created")
	assert.Equal(t, "postgres", db["engine"])
}

// ── deepCopyMap ───────────────────────────────────────────────────────────────

func TestDeepCopyMap_IndependentFromOriginal(t *testing.T) {
	original := map[string]interface{}{
		"spec": map[string]interface{}{
			"image": "myorg/nginx:1.25",
		},
	}

	cp := deepCopyMap(original)

	// Mutate the copy — original must not change
	cp["spec"].(map[string]interface{})["image"] = "mutated"

	assert.Equal(t, "myorg/nginx:1.25",
		original["spec"].(map[string]interface{})["image"],
		"original must not be affected by copy mutation")
}

func TestDeepCopyMap_NilReturnsNil(t *testing.T) {
	assert.Nil(t, deepCopyMap(nil))
}

func TestDeepCopyMap_PreservesAllFields(t *testing.T) {
	original := map[string]interface{}{
		"a": "string",
		"b": float64(42),
		"c": true,
		"d": map[string]interface{}{"nested": "value"},
	}

	cp := deepCopyMap(original)

	assert.Equal(t, "string", cp["a"])
	assert.Equal(t, float64(42), cp["b"])
	assert.Equal(t, true, cp["c"])
	assert.Equal(t, "value", cp["d"].(map[string]interface{})["nested"])
}

// ── buildJSONPatch ────────────────────────────────────────────────────────────

func patchOps(t *testing.T, original, mutated map[string]interface{}) []map[string]interface{} {
	t.Helper()
	raw, err := buildJSONPatch(original, mutated)
	require.NoError(t, err)
	var ops []map[string]interface{}
	require.NoError(t, json.Unmarshal(raw, &ops))
	return ops
}

func TestBuildJSONPatch_AddForAbsentField(t *testing.T) {
	ops := patchOps(t,
		map[string]interface{}{"spec": map[string]interface{}{}},
		map[string]interface{}{"spec": map[string]interface{}{"replicas": "2"}})
	require.Len(t, ops, 1)
	assert.Equal(t, "add", ops[0]["op"])
	assert.Equal(t, "/spec/replicas", ops[0]["path"])
	assert.Equal(t, "2", ops[0]["value"])
}

func TestBuildJSONPatch_ReplaceForExistingField(t *testing.T) {
	labels := func(v string) map[string]interface{} {
		return map[string]interface{}{"metadata": map[string]interface{}{"labels": map[string]interface{}{"managed-by": v}}}
	}
	ops := patchOps(t, labels("helm"), labels("inrun"))
	require.Len(t, ops, 1)
	assert.Equal(t, "replace", ops[0]["op"])
	assert.Equal(t, "/metadata/labels/managed-by", ops[0]["path"])
}

func TestBuildJSONPatch_KeyWithDotsAndSlash(t *testing.T) {
	ops := patchOps(t,
		map[string]interface{}{"metadata": map[string]interface{}{"labels": map[string]interface{}{}}},
		map[string]interface{}{"metadata": map[string]interface{}{"labels": map[string]interface{}{"app.kubernetes.io/name": "web"}}})
	require.Len(t, ops, 1)
	assert.Equal(t, "/metadata/labels/app.kubernetes.io~1name", ops[0]["path"])
}

func TestBuildJSONPatch_AddsMissingParent(t *testing.T) {
	ops := patchOps(t,
		map[string]interface{}{"metadata": map[string]interface{}{"name": "a"}},
		map[string]interface{}{"metadata": map[string]interface{}{"name": "a", "labels": map[string]interface{}{"tier": "gold"}}})
	require.Len(t, ops, 1)
	assert.Equal(t, "add", ops[0]["op"])
	assert.Equal(t, "/metadata/labels", ops[0]["path"])
}

func TestBuildJSONPatch_NoChanges(t *testing.T) {
	obj := map[string]interface{}{"spec": map[string]interface{}{"replicas": "2"}}
	assert.Empty(t, patchOps(t, obj, obj))
}

// ── gvrToKey ──────────────────────────────────────────────────────────────────

func TestGVRToKey(t *testing.T) {
	tests := []struct {
		name string
		gvr  metav1.GroupVersionResource
		want string
	}{
		{
			name: "group resource",
			gvr:  metav1.GroupVersionResource{Group: "demo.inrun.dev", Version: "v1alpha1", Resource: "websites"},
			want: "demo.inrun.dev/v1alpha1/websites",
		},
		{
			name: "core group (empty group)",
			gvr:  metav1.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"},
			want: "v1/pods",
		},
		{
			name: "apps group",
			gvr:  metav1.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"},
			want: "apps/v1/deployments",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, gvrToKey(tc.gvr))
		})
	}
}

package intent

import (
	"testing"

	"github.com/intentd/inrun/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestNewCRSkeleton(t *testing.T) {
	crd := &types.CRDEntry{
		APITypes: types.APITypes{
			Group:   "platform.myorg.io",
			Version: "v1",
			Kind:    "AppRequest",
		},
		GroupVersionKind: schema.GroupVersionKind{
			Group: "platform.myorg.io", Version: "v1", Kind: "AppRequest",
		},
	}

	obj := newCRSkeleton(crd)

	expectedAPIVersion := crd.APITypes.Group + "/" + crd.APITypes.Version
	assert.Equal(t, expectedAPIVersion, obj.Object["apiVersion"])
	assert.Equal(t, crd.APITypes.Kind, obj.Object["kind"])

	// Check metadata exists
	meta, ok := obj.Object["metadata"].(map[string]interface{})
	assert.True(t, ok)
	assert.NotNil(t, meta["labels"])
	assert.NotNil(t, meta["annotations"])

	// Check spec exists
	spec, ok := obj.Object["spec"].(map[string]interface{})
	assert.True(t, ok)
	assert.NotNil(t, spec)
}

func TestRouteFields(t *testing.T) {
	crd := &types.CRDEntry{
		Serve: &types.ServeConfig{
			Fields: map[string]types.ServeFieldConfig{
				"repository": {},
				"image":      {},
				"replicas":   {},
			},
			Labels: map[string]types.ServeFieldConfig{
				"team":        {},
				"environment": {},
			},
			Annotations: map[string]types.ServeFieldConfig{
				"jira-ticket": {},
				"expose":      {},
			},
		},
	}

	raw := map[string]interface{}{
		"target":      "smartapp",
		"repository":  "myorg/payments-api",
		"image":       "ghcr.io/myorg/payments-api:v1.0.0",
		"replicas":    2,
		"team":        "team-payments",
		"environment": "staging",
		"jira-ticket": "PLAT-1234",
		"expose":      "true",
		"unknown":     "ignored",
	}

	obj := newCRSkeleton(crd)
	routeFields(raw, crd, types.NoteRegistry{}, obj)

	// Check spec fields
	spec := obj.Object["spec"].(map[string]interface{})
	assert.Equal(t, "myorg/payments-api", spec["repository"])
	assert.Equal(t, "ghcr.io/myorg/payments-api:v1.0.0", spec["image"])
	assert.Equal(t, 2, spec["replicas"])

	// Check labels
	meta := obj.Object["metadata"].(map[string]interface{})
	labels := meta["labels"].(map[string]interface{})
	assert.Equal(t, "team-payments", labels["team"])
	assert.Equal(t, "staging", labels["environment"])

	// Check annotations
	annotations := meta["annotations"].(map[string]interface{})
	assert.Equal(t, "PLAT-1234", annotations["jira-ticket"])
	assert.Equal(t, "true", annotations["expose"])

	// Check unknown field was ignored
	assert.NotContains(t, spec, "unknown")
	assert.NotContains(t, labels, "unknown")
	assert.NotContains(t, annotations, "unknown")
}

func TestRouteFields_MissingFields(t *testing.T) {
	crd := &types.CRDEntry{
		Serve: &types.ServeConfig{
			Fields: map[string]types.ServeFieldConfig{
				"repository": {},
				"image":      {},
			},
			Labels: map[string]types.ServeFieldConfig{
				"team": {},
			},
		},
	}

	raw := map[string]interface{}{
		"target": "smartapp",
		// Missing "repository" field
		"image": "ghcr.io/myorg/app:v1.0.0",
		"team":  "team-payments",
	}

	obj := newCRSkeleton(crd)
	routeFields(raw, crd, types.NoteRegistry{}, obj)

	spec := obj.Object["spec"].(map[string]interface{})
	assert.NotContains(t, spec, "repository") // Should be absent
	assert.Equal(t, "ghcr.io/myorg/app:v1.0.0", spec["image"])

	labels := obj.Object["metadata"].(map[string]interface{})["labels"].(map[string]interface{})
	assert.Equal(t, "team-payments", labels["team"])
}

func TestRouteFields_NonStringLabels(t *testing.T) {
	crd := &types.CRDEntry{
		Serve: &types.ServeConfig{
			Labels: map[string]types.ServeFieldConfig{
				"count": {},
			},
		},
	}

	raw := map[string]interface{}{
		"target": "smartapp",
		"count":  42, // Integer value
	}

	obj := newCRSkeleton(crd)
	routeFields(raw, crd, types.NoteRegistry{}, obj)

	labels := obj.Object["metadata"].(map[string]interface{})["labels"].(map[string]interface{})
	assert.Equal(t, "42", labels["count"]) // Should be converted to string
}

func TestResolveServeIdentity(t *testing.T) {
	// Use built-in notes (repoSlug is built-in)
	notes := types.NoteRegistry{}

	namespaced := true
	crd := &types.CRDEntry{
		Namespaced: &namespaced,
		APITypes: types.APITypes{
			Kind: "AppRequest",
		},
		Serve: &types.ServeConfig{
			Name:      "{{ repoSlug .repository }}",
			Namespace: "{{ .team }}-{{ .environment }}",
		},
	}

	raw := map[string]interface{}{
		"target":      "smartapp",
		"repository":  "myorg/payments-api",
		"team":        "team-payments",
		"environment": "staging",
	}

	obj := newCRSkeleton(crd)

	err := resolveServeIdentity(raw, crd, notes, obj)
	require.NoError(t, err)

	assert.Equal(t, "payments-api", obj.GetName())
	assert.Equal(t, "team-payments-staging", obj.GetNamespace())
}

func TestResolveServeIdentity_WithRepoSlugNote(t *testing.T) {
	// Use built-in notes (repoSlug is built-in)
	notes := types.NoteRegistry{}

	namespaced := true
	crd := &types.CRDEntry{
		Namespaced: &namespaced,
		APITypes: types.APITypes{
			Kind: "AppRequest",
		},
		Serve: &types.ServeConfig{
			Name:      "{{ repoSlug .repository }}",
			Namespace: "{{ .team }}-{{ .environment }}",
		},
	}

	raw := map[string]interface{}{
		"target":      "smartapp",
		"repository":  "myorg/payments-api",
		"team":        "team-payments",
		"environment": "staging",
	}

	obj := newCRSkeleton(crd)

	err := resolveServeIdentity(raw, crd, notes, obj)
	require.NoError(t, err)

	assert.Equal(t, "payments-api", obj.GetName())
	assert.Equal(t, "team-payments-staging", obj.GetNamespace())
}

func TestResolveServeIdentity_OnlyName(t *testing.T) {
	notes := types.NoteRegistry{}

	crd := &types.CRDEntry{
		APITypes: types.APITypes{
			Kind: "AppRequest",
		},
		Serve: &types.ServeConfig{
			Name: "{{ repoSlug .repository }}",
			// Namespace not set — should be empty
		},
	}

	raw := map[string]interface{}{
		"target":     "smartapp",
		"repository": "myorg/orders-api",
	}

	obj := newCRSkeleton(crd)

	err := resolveServeIdentity(raw, crd, notes, obj)
	require.NoError(t, err)

	assert.Equal(t, "orders-api", obj.GetName())
	assert.Empty(t, obj.GetNamespace())
}

func TestResolveServeIdentity_NoServeName_FallsBackToRawName(t *testing.T) {
	notes := types.NoteRegistry{}

	crd := &types.CRDEntry{
		APITypes: types.APITypes{
			Kind: "AppRequest",
		},
		Serve: &types.ServeConfig{
			// serve.name not declared.
		},
	}

	raw := map[string]interface{}{
		"target": "smartapp",
		"name":   "my-app",
	}

	obj := newCRSkeleton(crd)

	err := resolveServeIdentity(raw, crd, notes, obj)
	require.NoError(t, err)
	assert.Equal(t, "my-app", obj.GetName())
}

func TestResolveServeIdentity_NoServeName_NoRawName_StaysEmpty(t *testing.T) {
	notes := types.NoteRegistry{}

	crd := &types.CRDEntry{
		APITypes: types.APITypes{
			Kind: "AppRequest",
		},
		Serve: &types.ServeConfig{
			// serve.name not declared, caller sent no "name" either.
		},
	}

	raw := map[string]interface{}{
		"target": "smartapp",
	}

	obj := newCRSkeleton(crd)

	err := resolveServeIdentity(raw, crd, notes, obj)
	require.NoError(t, err)
	assert.Empty(t, obj.GetName())
}

func TestResolveServeIdentity_MissingName(t *testing.T) {
	notes := types.NoteRegistry{}

	namespaced := true
	crd := &types.CRDEntry{
		Namespaced: &namespaced,
		APITypes: types.APITypes{
			Kind: "AppRequest",
		},
		Serve: &types.ServeConfig{
			Name: "{{ repoSlug .repository }}",
		},
	}

	raw := map[string]interface{}{
		"target": "smartapp",
		// Missing "repository" field
		"image": "ghcr.io/myorg/app:v1.0.0",
	}

	obj := newCRSkeleton(crd)

	err := resolveServeIdentity(raw, crd, notes, obj)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "serve.name expression")
	assert.Contains(t, err.Error(), "could not be resolved")
}

func TestResolveServeIdentity_MissingNamespace(t *testing.T) {
	notes := types.NoteRegistry{}

	namespaced := true
	crd := &types.CRDEntry{
		Namespaced: &namespaced,
		APITypes: types.APITypes{
			Kind: "AppRequest",
		},
		Serve: &types.ServeConfig{
			Namespace: "{{ .team }}-{{ .environment }}",
		},
	}

	raw := map[string]interface{}{
		"target": "smartapp",
		// Missing "team" and "environment"
	}

	obj := newCRSkeleton(crd)

	err := resolveServeIdentity(raw, crd, notes, obj)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "serve.namespace expression")
	assert.Contains(t, err.Error(), "could not be resolved")
}

func TestResolveServeIdentity_EmptyName(t *testing.T) {
	notes := types.NoteRegistry{}

	crd := &types.CRDEntry{
		APITypes: types.APITypes{
			Kind: "AppRequest",
		},
		Serve: &types.ServeConfig{
			Name: "{{ .missingField }}",
		},
	}

	raw := map[string]interface{}{
		"target": "smartapp",
	}

	obj := newCRSkeleton(crd)

	err := resolveServeIdentity(raw, crd, notes, obj)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "serve.name expression")
	assert.Contains(t, err.Error(), "could not be resolved")
}

func TestNewBuild(t *testing.T) {
	// Use built-in notes (repoSlug is built-in)
	notes := types.NoteRegistry{}

	namespaced := true
	crd := &types.CRDEntry{
		Namespaced: &namespaced,
		APITypes: types.APITypes{
			Group:   "platform.myorg.io",
			Version: "v1",
			Kind:    "AppRequest",
		},
		GroupVersionKind: schema.GroupVersionKind{
			Group: "platform.myorg.io", Version: "v1", Kind: "AppRequest",
		},
		Serve: &types.ServeConfig{
			Name:      "{{ repoSlug .repository }}",
			Namespace: "{{ .team }}-{{ .environment }}",
			Fields: map[string]types.ServeFieldConfig{
				"repository": {},
				"image":      {},
				"replicas":   {},
			},
			Labels: map[string]types.ServeFieldConfig{
				"team":        {},
				"environment": {},
			},
			Annotations: map[string]types.ServeFieldConfig{
				"jira-ticket": {},
			},
		},
	}

	raw := map[string]interface{}{
		"target":      "smartapp",
		"repository":  "myorg/payments-api",
		"image":       "ghcr.io/myorg/payments-api:v1.0.0",
		"replicas":    2,
		"team":        "team-payments",
		"environment": "staging",
		"jira-ticket": "PLAT-1234",
	}

	obj, err := Build(raw, crd, notes)
	require.NoError(t, err)

	expectedAPIVersion := crd.APITypes.Group + "/" + crd.APITypes.Version
	assert.Equal(t, expectedAPIVersion, obj.Object["apiVersion"])
	assert.Equal(t, crd.APITypes.Kind, obj.Object["kind"])

	// Check spec
	spec := obj.Object["spec"].(map[string]interface{})
	assert.Equal(t, "myorg/payments-api", spec["repository"])
	assert.Equal(t, "ghcr.io/myorg/payments-api:v1.0.0", spec["image"])
	assert.Equal(t, 2, spec["replicas"])

	// Check labels
	meta := obj.Object["metadata"].(map[string]interface{})
	labels := meta["labels"].(map[string]interface{})
	assert.Equal(t, "team-payments", labels["team"])
	assert.Equal(t, "staging", labels["environment"])

	// Check annotations
	annotations := meta["annotations"].(map[string]interface{})
	assert.Equal(t, "PLAT-1234", annotations["jira-ticket"])

	// Check name and namespace
	assert.Equal(t, "payments-api", obj.GetName())
	assert.Equal(t, "team-payments-staging", obj.GetNamespace())
}

func TestIsRequest(t *testing.T) {
	assert.True(t, IsRequest(map[string]interface{}{
		"target": "app",
	}))
	// target wins even when apiVersion is also present (gradual migration path)
	assert.True(t, IsRequest(map[string]interface{}{
		"target":     "app",
		"apiVersion": "v1",
	}))
	assert.False(t, IsRequest(map[string]interface{}{
		"apiVersion": "platform.myorg.io/v1",
		"kind":       "App",
	}))
	assert.False(t, IsRequest(map[string]interface{}{}))
}

func TestBuild(t *testing.T) {
	appCRD := &types.CRDEntry{
		APITypes: types.APITypes{
			Group:   "platform.myorg.io",
			Version: "v1",
			Kind:    "App",
			Plural:  "apps",
		},
		GroupVersionKind: schema.GroupVersionKind{
			Group: "platform.myorg.io", Version: "v1", Kind: "App",
		},
		Serve: &types.ServeConfig{
			Target: types.ServeTargetValue{Entries: map[string]*types.ServeTargetConfig{
				"app": {Primary: true},
			}},
			Name:      `{{ .repository | repoSlug }}`,
			Namespace: `{{ .team }}-{{ .environment }}`,
			Fields: map[string]types.ServeFieldConfig{
				"repository":  {},
				"image":       {},
				"environment": {},
				"replicas":    {},
			},
			Labels: map[string]types.ServeFieldConfig{
				"team": {},
			},
			Annotations: map[string]types.ServeFieldConfig{
				"jira-ticket": {},
			},
		},
	}

	t.Run("spec fields routed correctly", func(t *testing.T) {
		raw := map[string]interface{}{
			"target":      "app",
			"repository":  "myorg/payments-api",
			"image":       "ghcr.io/myorg/payments-api:v1",
			"environment": "staging",
			"replicas":    float64(2),
			"team":        "payments",
			"jira-ticket": "PLAT-1234",
		}

		obj, err := Build(raw, appCRD, types.NoteRegistry{})
		require.NoError(t, err)

		spec := obj.Object["spec"].(map[string]interface{})
		assert.Equal(t, "myorg/payments-api", spec["repository"])
		assert.Equal(t, "ghcr.io/myorg/payments-api:v1", spec["image"])
		assert.Equal(t, "staging", spec["environment"])
		assert.Equal(t, float64(2), spec["replicas"])

		labels := obj.Object["metadata"].(map[string]interface{})["labels"].(map[string]interface{})
		assert.Equal(t, "payments", labels["team"])

		annotations := obj.Object["metadata"].(map[string]interface{})["annotations"].(map[string]interface{})
		assert.Equal(t, "PLAT-1234", annotations["jira-ticket"])

		// team and jira-ticket must NOT be in spec.
		assert.Nil(t, spec["team"])
		assert.Nil(t, spec["jira-ticket"])
	})

	t.Run("unknown fields ignored", func(t *testing.T) {
		raw := map[string]interface{}{
			"target":        "app",
			"repository":    "myorg/payments-api",
			"team":          "payments",
			"environment":   "staging",
			"unknown-field": "should be ignored",
		}
		obj, err := Build(raw, appCRD, types.NoteRegistry{})
		require.NoError(t, err)

		spec := obj.Object["spec"].(map[string]interface{})
		_, exists := spec["unknown-field"]
		assert.False(t, exists)
	})

	t.Run("apiVersion and kind set from CRD entry", func(t *testing.T) {
		raw := map[string]interface{}{
			"target":      "app",
			"repository":  "myorg/payments-api",
			"team":        "payments",
			"environment": "staging",
		}
		obj, err := Build(raw, appCRD, types.NoteRegistry{})
		require.NoError(t, err)
		assert.Equal(t, "platform.myorg.io/v1", obj.GetAPIVersion())
		assert.Equal(t, "App", obj.GetKind())
	})
}

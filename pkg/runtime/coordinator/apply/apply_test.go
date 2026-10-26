package apply

import (
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/intentd/inrun/pkg/kubeclient/kubefake/object"
	"github.com/intentd/inrun/pkg/labels"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func ownerObj(name, namespace string) *unstructured.Unstructured {
	return object.New("example.io/v1", "App", name, object.InNamespace(namespace), object.WithUID("test-uid"))
}

func TestExpand_Empty(t *testing.T) {
	got, err := Expand(nil, nil)
	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestExpand_FullForm_PassThrough(t *testing.T) {
	full := map[string]interface{}{
		"apiVersion": "apps/v1",
		"kind":       "Deployment",
		"metadata":   map[string]interface{}{"name": "my-app"},
	}
	got, err := Expand([]map[string]interface{}{full}, nil)
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "apps/v1", got[0]["apiVersion"])
}

func TestExpand_IntentForm_Deployment(t *testing.T) {
	owner := ownerObj("my-app", "default")
	intent := map[string]interface{}{
		"type": "deployment",
		"fields": map[string]interface{}{
			"name":  "my-app",
			"image": "nginx:1.25",
		},
	}
	got, err := Expand([]map[string]interface{}{intent}, owner)
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "apps/v1", got[0]["apiVersion"])
	assert.Equal(t, "Deployment", got[0]["kind"])
}

func TestExpand_IntentForm_UnknownType(t *testing.T) {
	owner := ownerObj("my-app", "default")
	intent := map[string]interface{}{
		"type":   "daemonset",
		"fields": map[string]interface{}{"name": "my-ds"},
	}
	_, err := Expand([]map[string]interface{}{intent}, owner)
	assert.ErrorContains(t, err, "unknown resource type")
}

func TestExpand_IntentForm_NilOwner_Errors(t *testing.T) {
	intent := map[string]interface{}{
		"type":   "deployment",
		"fields": map[string]interface{}{"name": "my-app", "image": "nginx:1.25"},
	}
	_, err := Expand([]map[string]interface{}{intent}, nil)
	assert.ErrorContains(t, err, "owner object")
}

func TestExpand_IntentForm_Custom(t *testing.T) {
	owner := ownerObj("my-app", "default")
	intent := map[string]interface{}{
		"type": "custom",
		"fields": map[string]interface{}{
			"apiVersion": "example.io/v1",
			"kind":       "WebApp",
			"name":       "my-app",
			"spec":       map[string]interface{}{"replicas": float64(3)},
		},
	}
	got, err := Expand([]map[string]interface{}{intent}, owner)
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "example.io/v1", got[0]["apiVersion"])
	assert.Equal(t, "WebApp", got[0]["kind"])
}

func TestExpand_IntentForm_Custom_MissingAPIVersion(t *testing.T) {
	owner := ownerObj("my-app", "default")
	intent := map[string]interface{}{
		"type": "custom",
		"fields": map[string]interface{}{
			"kind": "WebApp",
			"name": "my-app",
		},
	}
	_, err := Expand([]map[string]interface{}{intent}, owner)
	assert.ErrorContains(t, err, "apiVersion is required")
}

func TestExpand_MixedForms(t *testing.T) {
	owner := ownerObj("my-app", "default")
	raw := []map[string]interface{}{
		{"apiVersion": "v1", "kind": "ConfigMap", "metadata": map[string]interface{}{"name": "cfg"}},
		{"type": "serviceaccount", "fields": map[string]interface{}{"name": "my-sa"}},
	}
	got, err := Expand(raw, owner)
	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.Equal(t, "v1", got[0]["apiVersion"])
	assert.Equal(t, "ServiceAccount", got[1]["kind"])
}

func TestManagedSince(t *testing.T) {
	owner := ownerObj("my-app", "default")
	assert.Equal(t, "", managedSince(owner), "no annotation, no creation time")

	created := metav1.NewTime(time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC))
	owner.SetCreationTimestamp(created)
	assert.Equal(t, "2026-10-01T12:00:00Z", managedSince(owner))

	owner.SetAnnotations(map[string]string{labels.AnnotationManagedSince: "2026-09-01T00:00:00Z"})
	assert.Equal(t, "2026-09-01T00:00:00Z", managedSince(owner), "the owner's annotation wins")
}

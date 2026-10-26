package api

import (
	"testing"

	"github.com/intentd/inrun/pkg/types"
	"github.com/stretchr/testify/assert"
)

func TestApplyExclusions(t *testing.T) {
	t.Run("nil crd is a no-op", func(t *testing.T) {
		response := map[string]interface{}{"a": "b"}
		assert.NotPanics(t, func() {
			ApplyExclusions(response, nil, "", noopNotes())
		})
		assert.Equal(t, "b", response["a"])
	})

	t.Run("no response config is a no-op", func(t *testing.T) {
		crd := appCRD()
		crd.Serve.View = nil
		response := map[string]interface{}{"a": "b"}
		ApplyExclusions(response, crd, "", noopNotes())
		assert.Equal(t, "b", response["a"])
	})

	t.Run("no exclude declared is a no-op", func(t *testing.T) {
		crd := appCRD()
		crd.Serve.View = &types.ServeView{}
		response := map[string]interface{}{"a": "b"}
		ApplyExclusions(response, crd, "", noopNotes())
		assert.Equal(t, "b", response["a"])
	})

	t.Run("single static path removed", func(t *testing.T) {
		crd := appCRD()
		crd.Serve.View = &types.ServeView{
			Exclude: []string{"metadata.managedFields"},
		}
		response := map[string]interface{}{
			"metadata": map[string]interface{}{
				"name":          "my-app",
				"managedFields": []interface{}{"something"},
			},
		}
		ApplyExclusions(response, crd, "", noopNotes())
		meta := response["metadata"].(map[string]interface{})
		_, hasMF := meta["managedFields"]
		assert.False(t, hasMF)
		assert.Equal(t, "my-app", meta["name"])
	})

	t.Run("multiple entries each remove their own path", func(t *testing.T) {
		crd := appCRD()
		crd.Serve.View = &types.ServeView{
			Exclude: []string{"metadata.managedFields", "status.observedGeneration"},
		}
		response := map[string]interface{}{
			"metadata": map[string]interface{}{
				"managedFields": []interface{}{"something"},
			},
			"status": map[string]interface{}{
				"phase":              "Ready",
				"observedGeneration": 1,
			},
		}
		ApplyExclusions(response, crd, "", noopNotes())
		meta := response["metadata"].(map[string]interface{})
		_, hasMF := meta["managedFields"]
		assert.False(t, hasMF)
		status := response["status"].(map[string]interface{})
		_, hasOG := status["observedGeneration"]
		assert.False(t, hasOG)
		assert.Equal(t, "Ready", status["phase"])
	})

	t.Run("comma separated string in a single entry removes multiple paths", func(t *testing.T) {
		crd := appCRD()
		crd.Serve.View = &types.ServeView{
			Exclude: []string{"metadata.managedFields,status.observedGeneration"},
		}
		response := map[string]interface{}{
			"metadata": map[string]interface{}{
				"managedFields": []interface{}{"something"},
			},
			"status": map[string]interface{}{
				"observedGeneration": 1,
			},
		}
		ApplyExclusions(response, crd, "", noopNotes())
		meta := response["metadata"].(map[string]interface{})
		_, hasMF := meta["managedFields"]
		assert.False(t, hasMF)
		status := response["status"].(map[string]interface{})
		_, hasOG := status["observedGeneration"]
		assert.False(t, hasOG)
	})

	t.Run("dynamic list from annotation via toList", func(t *testing.T) {
		crd := appCRD()
		crd.Serve.View = &types.ServeView{
			Exclude: []string{`{{ toList (getAnnotation . "platform.myorg.io/exclude") }}`},
		}
		response := map[string]interface{}{
			"metadata": map[string]interface{}{
				"annotations": map[string]interface{}{
					"platform.myorg.io/exclude": "metadata.managedFields,status.observedGeneration",
				},
				"managedFields": []interface{}{"something"},
			},
			"status": map[string]interface{}{
				"observedGeneration": 1,
				"phase":              "Ready",
			},
		}
		ApplyExclusions(response, crd, "", noopNotes())
		meta := response["metadata"].(map[string]interface{})
		_, hasMF := meta["managedFields"]
		assert.False(t, hasMF)
		status := response["status"].(map[string]interface{})
		_, hasOG := status["observedGeneration"]
		assert.False(t, hasOG)
		assert.Equal(t, "Ready", status["phase"])
	})

	t.Run("path that does not exist is a no-op, not an error", func(t *testing.T) {
		crd := appCRD()
		crd.Serve.View = &types.ServeView{
			Exclude: []string{"spec.nonexistent.deeply.nested"},
		}
		response := map[string]interface{}{"a": "b"}
		assert.NotPanics(t, func() {
			ApplyExclusions(response, crd, "", noopNotes())
		})
		assert.Equal(t, "b", response["a"])
	})

	t.Run("empty entry is skipped", func(t *testing.T) {
		crd := appCRD()
		crd.Serve.View = &types.ServeView{
			Exclude: []string{""},
		}
		response := map[string]interface{}{"a": "b"}
		ApplyExclusions(response, crd, "", noopNotes())
		assert.Equal(t, "b", response["a"])
	})
}

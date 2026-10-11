package api

import (
	"testing"

	"github.com/intentd/inrun/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// EvaluatePayload
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

func TestEvaluatePayload(t *testing.T) {
	t.Run("nil config returns nil", func(t *testing.T) {
		crd := appCRD()
		crd.Serve.Config = nil
		result := EvaluatePayload(map[string]interface{}{}, crd, "", noopNotes())
		assert.Nil(t, result)
	})

	t.Run("no payload declared returns nil", func(t *testing.T) {
		crd := appCRD()
		crd.Serve.Config = &types.ServeConfigSettings{
			Response: &types.ServeResponseConfig{},
		}
		result := EvaluatePayload(map[string]interface{}{}, crd, "", noopNotes())
		assert.Nil(t, result)
	})

	t.Run("plain string passes through", func(t *testing.T) {
		crd := appCRD()
		crd.Serve.Config = &types.ServeConfigSettings{
			Response: &types.ServeResponseConfig{
				Payload: map[string]string{
					"supportChannel": "#platform",
				},
			},
		}
		result := EvaluatePayload(map[string]interface{}{}, crd, "", noopNotes())
		require.NotNil(t, result)
		assert.Equal(t, "#platform", result["supportChannel"])
	})

	t.Run("template expression resolved against CR", func(t *testing.T) {
		crd := appCRD()
		crd.Serve.Config = &types.ServeConfigSettings{
			Response: &types.ServeResponseConfig{
				Payload: map[string]string{
					"phase": `{{ .status.phase }}`,
				},
			},
		}
		obj := map[string]interface{}{
			"status": map[string]interface{}{"phase": "Ready"},
		}
		result := EvaluatePayload(obj, crd, "", noopNotes())
		require.NotNil(t, result)
		assert.Equal(t, "Ready", result["phase"])
	})

	t.Run("unresolvable expression returns empty string not error", func(t *testing.T) {
		crd := appCRD()
		crd.Serve.Config = &types.ServeConfigSettings{
			Response: &types.ServeResponseConfig{
				Payload: map[string]string{
					"phase": `{{ .status.phase }}`,
				},
			},
		}
		// status not yet present — at apply time
		result := EvaluatePayload(map[string]interface{}{}, crd, "", noopNotes())
		require.NotNil(t, result)
		assert.Equal(t, "", result["phase"])
	})

	t.Run("payload ignores default regardless of value", func(t *testing.T) {
		// Default controls whether the caller (resources.go/apply.go) merges
		// the payload into the full CR or returns it alone — EvaluatePayload
		// itself always returns only the declared payload fields.
		crd := appCRD()
		tr := true
		crd.Serve.Config = &types.ServeConfigSettings{
			Response: &types.ServeResponseConfig{
				Default: &tr,
				Payload: map[string]string{"extra": "value"},
			},
		}
		obj := map[string]interface{}{
			"spec": map[string]interface{}{"image": "myimage"},
		}
		result := EvaluatePayload(obj, crd, "", noopNotes())
		require.NotNil(t, result)
		assert.Equal(t, "value", result["extra"])
		_, hasSpec := result["spec"]
		assert.False(t, hasSpec, "EvaluatePayload never includes the full CR")
	})

	t.Run("payload ignores exclude entirely", func(t *testing.T) {
		// Exclude is applied by ApplyExclusions at the resource GET/list
		// level, before EvaluatePayload runs — not by EvaluatePayload itself.
		crd := appCRD()
		crd.Serve.Config = &types.ServeConfigSettings{
			Response: &types.ServeResponseConfig{
				Exclude: []string{"metadata.managedFields"},
				Payload: map[string]string{"name": `{{ .metadata.name }}`},
			},
		}
		obj := map[string]interface{}{
			"metadata": map[string]interface{}{
				"name":          "my-app",
				"managedFields": []interface{}{"something"},
			},
		}
		result := EvaluatePayload(obj, crd, "", noopNotes())
		require.NotNil(t, result)
		assert.Equal(t, "my-app", result["name"])
		_, hasManagedFields := result["managedFields"]
		assert.False(t, hasManagedFields, "payload only ever contains declared keys")
	})
}

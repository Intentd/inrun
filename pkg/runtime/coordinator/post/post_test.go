package post

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestSameHealth_IgnoresPerPassFields(t *testing.T) {
	a := `{"healthy":true,"state":"healthy","lastReconcile":"2026-10-12T10:00:00Z","uptime":"5s","totalReconciles":3}`
	b := `{"healthy":true,"state":"healthy","lastReconcile":"2026-10-12T10:00:07Z","uptime":"12s","totalReconciles":4}`
	assert.True(t, sameHealth(a, b), "only per-pass fields moved: no patch")
	c := `{"healthy":false,"state":"degraded","lastReconcile":"2026-10-12T10:00:07Z"}`
	assert.False(t, sameHealth(a, c), "health changed: patch")
	assert.False(t, sameHealth("", a), "first write")
}

func TestKeepTransitionTimes(t *testing.T) {
	obj := &unstructured.Unstructured{Object: map[string]interface{}{
		"status": map[string]interface{}{"conditions": []interface{}{
			map[string]interface{}{"type": "Ready", "status": "True", "reason": "ReconcileSucceeded", "message": "", "lastTransitionTime": "2026-10-01T00:00:00Z"},
			map[string]interface{}{"type": "ValidationFailed", "status": "True", "reason": "DenyRuleViolation", "message": "bad", "lastTransitionTime": "2026-10-01T00:00:00Z"},
		}},
	}}
	conditions := []interface{}{
		map[string]interface{}{"type": "Ready", "status": "True", "reason": "ReconcileSucceeded", "message": "", "lastTransitionTime": "2026-10-12T10:00:00Z"},
		map[string]interface{}{"type": "ValidationFailed", "status": "False", "reason": "ValidationPassed", "message": "", "lastTransitionTime": "2026-10-12T10:00:00Z"},
		map[string]interface{}{"type": "ValidationWarning", "status": "False", "reason": "NoWarnings", "message": "", "lastTransitionTime": "2026-10-12T10:00:00Z"},
	}
	keepTransitionTimes(obj, conditions)

	at := func(i int) string { return conditions[i].(map[string]interface{})["lastTransitionTime"].(string) }
	assert.Equal(t, "2026-10-01T00:00:00Z", at(0), "unchanged condition keeps its time")
	assert.Equal(t, "2026-10-12T10:00:00Z", at(1), "a status flip moves it")
	assert.Equal(t, "2026-10-12T10:00:00Z", at(2), "a new condition starts now")
}

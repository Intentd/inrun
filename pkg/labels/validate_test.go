package labels

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── ValidateLabelMap ─────────────────────────────────────────────────────────

func TestValidateLabelMap_Empty(t *testing.T) {
	assert.NoError(t, ValidateLabelMap(nil))
	assert.NoError(t, ValidateLabelMap(map[string]string{}))
}

func TestValidateLabelMap_Valid(t *testing.T) {
	assert.NoError(t, ValidateLabelMap(map[string]string{
		"app.kubernetes.io/name": "hello-intent",
		"env":                    "production",
		"tier":                   "gold",
	}))
}

func TestValidateLabelMap_EmptyValueAllowed(t *testing.T) {
	// A label key with an empty value is valid in Kubernetes.
	assert.NoError(t, ValidateLabelMap(map[string]string{"app": ""}))
}

func TestValidateLabelMap_InvalidKey_Rejected(t *testing.T) {
	err := ValidateLabelMap(map[string]string{"invalid key!": "value"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), `invalid label key "invalid key!"`)
}

func TestValidateLabelMap_InvalidValue_Rejected(t *testing.T) {
	err := ValidateLabelMap(map[string]string{"tier": "value/with/slashes"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), `invalid label value for key "tier"`)
}

func TestValidateLabelMap_TemplateValueRejected(t *testing.T) {
	// Template syntax is a label-value violation at this layer — expansion
	// happens before validation in the caller, not here.
	err := ValidateLabelMap(map[string]string{"tier": "{{ .metadata.name }}"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), `invalid label value for key "tier"`)
}

// ── ValidateAnnotationKeys ───────────────────────────────────────────────────

func TestValidateAnnotationKeys_Empty(t *testing.T) {
	assert.NoError(t, ValidateAnnotationKeys(nil))
	assert.NoError(t, ValidateAnnotationKeys(map[string]string{}))
}

func TestValidateAnnotationKeys_Valid(t *testing.T) {
	assert.NoError(t, ValidateAnnotationKeys(map[string]string{
		"example.io/owner":  "team-a",
		"example.io/region": "us-east-1",
	}))
}

func TestValidateAnnotationKeys_InvalidKey_Rejected(t *testing.T) {
	err := ValidateAnnotationKeys(map[string]string{"invalid annotation!": "value"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), `invalid annotation key "invalid annotation!"`)
}

func TestValidateAnnotationKeys_ValueNotChecked(t *testing.T) {
	// The whole point of this function vs. ValidateLabelMap: annotation
	// values are unconstrained. This must pass.
	assert.NoError(t, ValidateAnnotationKeys(map[string]string{
		"example.io/desc": "a value with / slashes and spaces, and even {{ template }} syntax",
	}))
}

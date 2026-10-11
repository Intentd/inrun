package validate

import (
	"testing"

	"github.com/inrundev/inrun/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func catalogWithLabels(crdName string, labels types.Labels) *executor {
	return newCatalogExec(map[string]types.CRDEntry{
		crdName: {Labels: labels},
	})
}

func catalogWithAnnotations(crdName string, annotations types.Labels) *executor {
	return newCatalogExec(map[string]types.CRDEntry{
		crdName: {Annotations: annotations},
	})
}

// ── Labels ────────────────────────────────────────────────────────────────────

func TestValidateCRDEntryLabels_NoLabels(t *testing.T) {
	k := catalogWithLabels("myapp", nil)
	assert.NoError(t, k.validateCRDEntryLabels())
}

func TestValidateCRDEntryLabels_ValidStaticKey(t *testing.T) {
	k := catalogWithLabels("myapp", types.Labels{
		"app.kubernetes.io/name": "{{ .metadata.name }}",
	})
	assert.NoError(t, k.validateCRDEntryLabels())
}

func TestValidateCRDEntryLabels_ValidPlainValue(t *testing.T) {
	k := catalogWithLabels("myapp", types.Labels{
		"env": "production",
	})
	assert.NoError(t, k.validateCRDEntryLabels())
}

func TestValidateCRDEntryLabels_TemplateKey_Rejected(t *testing.T) {
	k := catalogWithLabels("myapp", types.Labels{
		"{{ .metadata.name }}": "value",
	})
	err := k.validateCRDEntryLabels()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must be a static label key")
	assert.Contains(t, err.Error(), "myapp")
}

func TestValidateCRDEntryLabels_InvalidKey_Rejected(t *testing.T) {
	k := catalogWithLabels("myapp", types.Labels{
		"invalid key!": "value",
	})
	err := k.validateCRDEntryLabels()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not a valid Kubernetes label key")
	assert.Contains(t, err.Error(), "myapp")
}

func TestValidateCRDEntryLabels_InvalidValueTemplate_Rejected(t *testing.T) {
	k := catalogWithLabels("myapp", types.Labels{
		"tier": "{{ .metadata.name",
	})
	err := k.validateCRDEntryLabels()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid template")
}

func TestValidateCRDEntryLabels_InvalidValue_Rejected(t *testing.T) {
	k := catalogWithLabels("myapp", types.Labels{
		"tier": "value/with/slashes",
	})
	err := k.validateCRDEntryLabels()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "is not a valid Kubernetes label value")
}

func TestValidateCRDEntryLabels_MultipleKeys_OneInvalid(t *testing.T) {
	k := catalogWithLabels("myapp", types.Labels{
		"app":          "{{ .metadata.name }}",
		"invalid key!": "value",
	})
	err := k.validateCRDEntryLabels()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not a valid Kubernetes label key")
}

func TestValidateCRDEntryLabels_ReservedDomain_Rejected(t *testing.T) {
	k := catalogWithLabels("myapp", types.Labels{
		"inrun.dev/managed": "true",
	})
	err := k.validateCRDEntryLabels()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "inrun.dev domain is reserved")
}

// ── Annotations ───────────────────────────────────────────────────────────────

func TestValidateCRDEntryAnnotations_NoAnnotations(t *testing.T) {
	k := catalogWithAnnotations("myapp", nil)
	assert.NoError(t, k.validateCRDEntryAnnotations())
}

func TestValidateCRDEntryAnnotations_ValidStaticKey(t *testing.T) {
	k := catalogWithAnnotations("myapp", types.Labels{
		"example.io/owner": "{{ .spec.owner }}",
	})
	assert.NoError(t, k.validateCRDEntryAnnotations())
}

func TestValidateCRDEntryAnnotations_ValidPlainValue(t *testing.T) {
	k := catalogWithAnnotations("myapp", types.Labels{
		"example.io/region": "us-east-1",
	})
	assert.NoError(t, k.validateCRDEntryAnnotations())
}

func TestValidateCRDEntryAnnotations_TemplateKey_Rejected(t *testing.T) {
	k := catalogWithAnnotations("myapp", types.Labels{
		"{{ .metadata.name }}": "value",
	})
	err := k.validateCRDEntryAnnotations()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must be a static annotation key")
	assert.Contains(t, err.Error(), "myapp")
}

func TestValidateCRDEntryAnnotations_InvalidKey_Rejected(t *testing.T) {
	k := catalogWithAnnotations("myapp", types.Labels{
		"invalid annotation!": "value",
	})
	err := k.validateCRDEntryAnnotations()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not a valid Kubernetes annotation key")
	assert.Contains(t, err.Error(), "myapp")
}

func TestValidateCRDEntryAnnotations_InvalidValueTemplate_Rejected(t *testing.T) {
	k := catalogWithAnnotations("myapp", types.Labels{
		"example.io/desc": "{{ .spec.name",
	})
	err := k.validateCRDEntryAnnotations()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid template")
}

func TestValidateCRDEntryAnnotations_ReservedDomain_Rejected(t *testing.T) {
	k := catalogWithAnnotations("myapp", types.Labels{
		"annotation.inrun.dev/mine": "myapp",
	})
	err := k.validateCRDEntryAnnotations()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "inrun.dev domain is reserved")
}

// ── Combined (validateCRDEntryMetadata) ──────────────────────────────────────

func TestValidateCRDEntryMetadata_BothValid(t *testing.T) {
	k := newCatalogExec(map[string]types.CRDEntry{
		"myapp": {
			Labels:      types.Labels{"app": "{{ .metadata.name }}"},
			Annotations: types.Labels{"example.io/owner": "team-a"},
		},
	})
	assert.NoError(t, k.validateCRDEntryMetadata())
}

func TestValidateCRDEntryMetadata_InvalidLabel_Short_Circuits(t *testing.T) {
	k := newCatalogExec(map[string]types.CRDEntry{
		"myapp": {
			Labels:      types.Labels{"invalid key!": "value"},
			Annotations: types.Labels{"example.io/ok": "value"},
		},
	})
	err := k.validateCRDEntryMetadata()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not a valid Kubernetes label key")
}

func TestValidateCRDEntryMetadata_InvalidAnnotation_Caught(t *testing.T) {
	k := newCatalogExec(map[string]types.CRDEntry{
		"myapp": {
			Labels:      types.Labels{"app": "myapp"},
			Annotations: types.Labels{"bad annotation!": "value"},
		},
	})
	err := k.validateCRDEntryMetadata()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not a valid Kubernetes annotation key")
}

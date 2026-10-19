package presets

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/intentd/inrun/pkg/types"
)

func blueprintWithResourceQuota(crdName string, rqs ...types.ResourceQuotaTemplateSource) *checker {
	return newBlueprintExec(map[string]types.CRDEntry{
		crdName: {
			OperatorBox: &types.OperatorBoxConfig{
				Reconcile: &types.ReconcileConfig{
					OnCreate: &types.HookTemplates{
						ResourceQuotas: rqs,
					},
				},
			},
		},
	})
}

func TestValidateResourceQuotaPresets_NoCRDs(t *testing.T) {
	e := newBlueprintExec(map[string]types.CRDEntry{})
	assert.NoError(t, e.validateResourceQuotaPresets())
}

func TestValidateResourceQuotaPresets_NoPreset(t *testing.T) {
	e := blueprintWithResourceQuota("app", types.ResourceQuotaTemplateSource{Name: "quota"})
	assert.NoError(t, e.validateResourceQuotaPresets())
}

func TestValidateResourceQuotaPresets_ValidPreset_Small(t *testing.T) {
	e := blueprintWithResourceQuota("app", types.ResourceQuotaTemplateSource{
		Name:   "quota",
		Preset: "small",
	})
	assert.NoError(t, e.validateResourceQuotaPresets())
}

func TestValidateResourceQuotaPresets_ValidPreset_Medium(t *testing.T) {
	e := blueprintWithResourceQuota("app", types.ResourceQuotaTemplateSource{
		Preset: "medium",
	})
	assert.NoError(t, e.validateResourceQuotaPresets())
}

func TestValidateResourceQuotaPresets_ValidPreset_Large(t *testing.T) {
	e := blueprintWithResourceQuota("app", types.ResourceQuotaTemplateSource{
		Preset: "large",
	})
	assert.NoError(t, e.validateResourceQuotaPresets())
}

func TestValidateResourceQuotaPresets_ValidPreset_XLarge(t *testing.T) {
	e := blueprintWithResourceQuota("app", types.ResourceQuotaTemplateSource{
		Preset: "xlarge",
	})
	assert.NoError(t, e.validateResourceQuotaPresets())
}

func TestValidateResourceQuotaPresets_UnknownPreset(t *testing.T) {
	e := blueprintWithResourceQuota("app", types.ResourceQuotaTemplateSource{
		Name:   "quota",
		Preset: "massive",
	})
	err := e.validateResourceQuotaPresets()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown preset")
	assert.Contains(t, err.Error(), "massive")
	assert.Contains(t, err.Error(), "small, medium, large, xlarge")
}

func TestValidateResourceQuotaPresets_MixedWithHard(t *testing.T) {
	e := blueprintWithResourceQuota("app", types.ResourceQuotaTemplateSource{
		Name:   "quota",
		Preset: "small",
		Hard:   map[string]string{"pods": "5"},
	})
	err := e.validateResourceQuotaPresets()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "explicit hard limits")
}

func TestValidateResourceQuotaPresets_TemplateExprSkipped(t *testing.T) {
	e := blueprintWithResourceQuota("app", types.ResourceQuotaTemplateSource{
		Preset: "{{ .Spec.QuotaPreset }}",
	})
	assert.NoError(t, e.validateResourceQuotaPresets())
}

func TestValidateResourceQuotaPresets_MultipleEntries_OneInvalid(t *testing.T) {
	e := blueprintWithResourceQuota("app",
		types.ResourceQuotaTemplateSource{Name: "q1", Preset: "small"},
		types.ResourceQuotaTemplateSource{Name: "q2", Preset: "unknown-tier"},
	)
	err := e.validateResourceQuotaPresets()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown-tier")
}

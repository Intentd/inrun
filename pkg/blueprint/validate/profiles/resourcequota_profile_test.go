package profiles

import (
	"testing"

	"github.com/intentd/inrun/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestValidateResourceQuotaProfiles_NoCRDs(t *testing.T) {
	e := newBlueprintExec(map[string]types.CRDEntry{})
	assert.NoError(t, e.validateResourceQuotaProfiles())
}

func TestValidateResourceQuotaProfiles_NoProfile(t *testing.T) {
	e := blueprintWithResourceQuota("app", types.ResourceQuotaTemplateSource{Name: "quota"})
	assert.NoError(t, e.validateResourceQuotaProfiles())
}

func TestValidateResourceQuotaProfiles_ValidProfile_Small(t *testing.T) {
	e := blueprintWithResourceQuota("app", types.ResourceQuotaTemplateSource{
		Name:    "quota",
		Profile: "small",
	})
	assert.NoError(t, e.validateResourceQuotaProfiles())
}

func TestValidateResourceQuotaProfiles_ValidProfile_Medium(t *testing.T) {
	e := blueprintWithResourceQuota("app", types.ResourceQuotaTemplateSource{
		Profile: "medium",
	})
	assert.NoError(t, e.validateResourceQuotaProfiles())
}

func TestValidateResourceQuotaProfiles_ValidProfile_Large(t *testing.T) {
	e := blueprintWithResourceQuota("app", types.ResourceQuotaTemplateSource{
		Profile: "large",
	})
	assert.NoError(t, e.validateResourceQuotaProfiles())
}

func TestValidateResourceQuotaProfiles_ValidProfile_XLarge(t *testing.T) {
	e := blueprintWithResourceQuota("app", types.ResourceQuotaTemplateSource{
		Profile: "xlarge",
	})
	assert.NoError(t, e.validateResourceQuotaProfiles())
}

func TestValidateResourceQuotaProfiles_UnknownProfile(t *testing.T) {
	e := blueprintWithResourceQuota("app", types.ResourceQuotaTemplateSource{
		Name:    "quota",
		Profile: "massive",
	})
	err := e.validateResourceQuotaProfiles()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown profile")
	assert.Contains(t, err.Error(), "massive")
	assert.Contains(t, err.Error(), "small, medium, large, xlarge")
}

func TestValidateResourceQuotaProfiles_MixedWithHard(t *testing.T) {
	e := blueprintWithResourceQuota("app", types.ResourceQuotaTemplateSource{
		Name:    "quota",
		Profile: "small",
		Hard:    map[string]string{"pods": "5"},
	})
	err := e.validateResourceQuotaProfiles()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "explicit hard limits")
}

func TestValidateResourceQuotaProfiles_TemplateExprSkipped(t *testing.T) {
	e := blueprintWithResourceQuota("app", types.ResourceQuotaTemplateSource{
		Profile: "{{ .Spec.QuotaProfile }}",
	})
	assert.NoError(t, e.validateResourceQuotaProfiles())
}

func TestValidateResourceQuotaProfiles_MultipleEntries_OneInvalid(t *testing.T) {
	e := blueprintWithResourceQuota("app",
		types.ResourceQuotaTemplateSource{Name: "q1", Profile: "small"},
		types.ResourceQuotaTemplateSource{Name: "q2", Profile: "unknown-tier"},
	)
	err := e.validateResourceQuotaProfiles()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown-tier")
}

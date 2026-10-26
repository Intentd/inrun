package simulate

import (
	"testing"

	"github.com/intentd/inrun/pkg/types"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestRandomSecretKeys(t *testing.T) {
	crd := types.CRDEntry{OperatorBox: &types.OperatorBoxConfig{Reconcile: &types.ReconcileConfig{
		OnCreate: &types.HookTemplates{Secrets: []types.SecretTemplateSource{{
			Name: "{{ .metadata.name }}-creds",
			Data: map[string]string{
				"token":   "{{ randomAlphanumeric 32 }}",
				"api-key": "{{ myKey }}",
				"user":    "{{ .spec.user }}",
				"static":  "admin",
			},
		}}},
	}}}
	functions := types.KELFunctionRegistry{Definitions: types.UserDefinedFunctions{
		{Name: "myKey", Expression: "{{ randomHex 8 }}"},
	}}

	assert.Equal(t, []string{"api-key", "token"}, randomSecretKeys(crd.Box(), functions),
		"a random function called directly or through a Blueprint function")
	assert.Equal(t, []string{"token"}, randomSecretKeys(crd.Box(), types.KELFunctionRegistry{}),
		"without the definition, myKey is not known to be random")
}

func secretWith(data map[string]any) *unstructured.Unstructured {
	u := &unstructured.Unstructured{Object: map[string]any{"data": data}}
	u.SetAPIVersion("v1")
	u.SetKind("Secret")
	u.SetName("probe-creds")
	u.SetNamespace("probe-ns")
	return u
}

// TestCompare_RandomSecretValuesVary: a random key's value differs between
// runs by design and is reported as varying; any other difference — another
// key, or a random key only one run holds — still counts.
func TestCompare_RandomSecretValuesVary(t *testing.T) {
	mem := &Result{RandomSecretKeys: []string{"api-key"}, Final: []*unstructured.Unstructured{
		secretWith(map[string]any{"api-key": "bWVt", "user": "YQ==", "only-mem": "eA=="}),
	}}
	env := &Result{RandomSecretKeys: []string{"api-key"}, Final: []*unstructured.Unstructured{
		secretWith(map[string]any{"api-key": "ZW52", "user": "Yg=="}),
	}}

	c := Compare("probe", mem, env, nil, nil)
	o := c.Objects[0]
	assert.Equal(t, []FieldDiff{{Path: "data.api-key", Kind: FieldVaries, Memory: "bWVt", Envtest: "ZW52"}}, o.Varies)
	var paths []string
	for _, d := range o.Differences {
		paths = append(paths, d.Path)
	}
	assert.Equal(t, []string{"data.only-mem", "data.user"}, paths, "other differences still count")

	same := Compare("probe", &Result{RandomSecretKeys: []string{"api-key"}, Final: []*unstructured.Unstructured{secretWith(map[string]any{"api-key": "bWVt"})}},
		&Result{RandomSecretKeys: []string{"api-key"}, Final: []*unstructured.Unstructured{secretWith(map[string]any{"api-key": "ZW52"})}}, nil, nil)
	assert.True(t, same.Equivalent(), "a varying field alone is not a difference")
}

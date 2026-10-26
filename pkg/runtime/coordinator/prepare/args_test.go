package prepare

import (
	"testing"

	"github.com/intentd/inrun/domain"
	"github.com/intentd/inrun/pkg/template"
	"github.com/intentd/inrun/pkg/types"
	"github.com/stretchr/testify/assert"
)

func preparedWithArgs(args map[string]interface{}, data map[string]interface{}) *domain.PreparedRequest {
	return &domain.PreparedRequest{
		Box:     types.OperatorBoxConfig{Reconcile: &types.ReconcileConfig{Args: args}},
		Context: template.NewResolverFromMap(data),
	}
}

func TestArgs_NoArgs(t *testing.T) {
	assert.Nil(t, Args(nil))
	assert.Nil(t, Args(preparedWithArgs(nil, nil)))
}

func TestArgs_StaticPassThrough(t *testing.T) {
	got := Args(preparedWithArgs(map[string]interface{}{"env": "prod", "replicas": 3}, nil))
	assert.Equal(t, "prod", got["env"])
	assert.Equal(t, 3, got["replicas"])
}

func TestArgs_TemplatesRenderedAgainstTheCR(t *testing.T) {
	got := Args(preparedWithArgs(
		map[string]interface{}{
			"replicas": `{{ index . "replicas" }}`,
			"debug":    `{{ index . "debug" }}`,
		},
		map[string]interface{}{"replicas": "5", "debug": "true"},
	))
	assert.Equal(t, float64(5), got["replicas"])
	assert.Equal(t, true, got["debug"])
}

func TestRenderArgs_ArgsReachTheResolver(t *testing.T) {
	prepared := preparedWithArgs(
		map[string]interface{}{"image": `{{ index . "image" }}`},
		map[string]interface{}{"image": "nginx:1.27"},
	)
	RenderArgs(prepared)

	assert.Equal(t, "nginx:1.27", prepared.Args["image"], "the reconciler still gets them")
	resolver := prepared.Context.(*template.Resolver)
	got, ok := resolver.RenderString(`{{ .args.image }}`)
	assert.True(t, ok)
	assert.Equal(t, "nginx:1.27", got, "templates rendered afterwards see .args")
}

func TestRenderArgs_NoArgsLeavesTheResolver(t *testing.T) {
	prepared := preparedWithArgs(nil, map[string]interface{}{"image": "nginx"})
	before := prepared.Context
	RenderArgs(prepared)
	assert.Same(t, before, prepared.Context)
}

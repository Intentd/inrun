package expect

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/intentd/inrun/pkg/template"
	"github.com/intentd/inrun/pkg/types"
)

func healthServer(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/down" {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		_, _ = w.Write([]byte(`{"state":"healthy"}`))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func call(name, url string) types.E2EExternalCheck {
	return Call(name, url)
}

func checkResolver() *template.Resolver {
	return template.NewResolverFromMap(map[string]interface{}{}).WithInrunContext(template.InrunContext{})
}

func TestCheckExternal(t *testing.T) {
	srv := healthServer(t)
	ctx := context.Background()

	ok := call("health", srv.URL+"/health")
	assert.NoError(t, checkExternal(ctx, ok, "", checkResolver(), nil), "a call that succeeds passes with no assertions")

	body := call("health", srv.URL+"/health")
	body.Contains = `"healthy"`
	assert.NoError(t, checkExternal(ctx, body, "", checkResolver(), nil), "without jq the assertions read the body")

	wrong := call("health", srv.URL+"/health")
	wrong.Contains = "degraded"
	assert.ErrorContains(t, checkExternal(ctx, wrong, "", checkResolver(), nil), `external "health"`)

	down := call("down", srv.URL+"/down")
	assert.Error(t, checkExternal(ctx, down, "", checkResolver(), nil), "a failed call fails the check")

	skipped := call("gated", srv.URL+"/down")
	skipped.When = []types.Condition{{Field: "{{ .inrun.inPod }}", Equals: "true"}}
	assert.NoError(t, checkExternal(ctx, skipped, "", checkResolver(), nil), "a call whose when: fails is skipped")
}

func TestCheckExternal_JQ(t *testing.T) {
	if _, err := exec.LookPath("jq"); err != nil {
		t.Skip("jq not installed")
	}
	srv := healthServer(t)
	c := call("health", srv.URL+"/health")
	c.JQ = ".json.state"
	c.Equals = "healthy"
	assert.NoError(t, checkExternal(context.Background(), c, "", checkResolver(), nil))

	c.Equals = "degraded"
	assert.Error(t, checkExternal(context.Background(), c, "", checkResolver(), nil))
}

func TestCheckExternal_UserFunctionInURL(t *testing.T) {
	srv := healthServer(t)
	resolver := checkResolver().WithUserFunctions(types.KELFunctionRegistry{Definitions: []types.UserDefinedFunction{{
		Name:       "healthURL",
		Expression: `{{ if .inrun.inPod }}http://in-cluster.invalid/health{{ else }}` + srv.URL + `/health{{ end }}`,
	}}})
	assert.NoError(t, checkExternal(context.Background(), call("health", "{{ healthURL }}"), "", resolver, nil),
		"e2e calls run from this machine, so .inrun.inPod is false")
}

func TestExpandIncludes_ExternalIncludes(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "calls.yaml"), []byte(`calls:
  - name: a
    url: http://a.invalid
  - name: b
    url: http://b.invalid
`), 0o644))

	inc := types.E2EExternalCheck{}
	inc.Include = "calls.yaml"
	own := call("own", "http://own.invalid")
	got, err := ExpandIncludes([]types.E2EExpectation{{Name: "s", External: []types.E2EExternalCheck{inc, own}}}, dir)
	require.NoError(t, err)
	var names []string
	for _, c := range got[0].External {
		names = append(names, c.Name)
	}
	assert.Equal(t, []string{"a", "b", "own"}, names)

	// An e2e calls file can carry each call's own checks.
	require.NoError(t, os.WriteFile(filepath.Join(dir, "checks.yaml"), []byte(`calls:
  - name: health
    url: http://health.invalid
    jq: .json.state
    equals: healthy
`), 0o644))
	inc.Include = "checks.yaml"
	got, err = ExpandIncludes([]types.E2EExpectation{{Name: "s", External: []types.E2EExternalCheck{inc}}}, dir)
	require.NoError(t, err)
	require.Len(t, got[0].External, 1)
	assert.Equal(t, ".json.state", got[0].External[0].JQ)
	assert.Equal(t, "healthy", got[0].External[0].Equals)
}

func TestValidateExternal(t *testing.T) {
	noNamespace := call("db", "http://db.invalid")
	noNamespace.Auth = &types.ExternalAuth{SecretRef: &types.APISecretRef{Name: "creds", Key: "token"}}

	errs := ValidateExternal([]types.E2EExpectation{{Name: "s", External: []types.E2EExternalCheck{
		call("", "http://x.invalid"),
		call("dup", "http://x.invalid"),
		call("dup", "http://x.invalid"),
		call("nourl", ""),
		noNamespace,
	}}})
	var got []string
	for _, e := range errs {
		got = append(got, e.Error())
	}
	joined := strings.Join(got, "\n")
	assert.Contains(t, joined, "name is required")
	assert.Contains(t, joined, `duplicate call name "dup"`)
	assert.Contains(t, joined, "external[3]")
	assert.Contains(t, joined, "auth.secretRef.namespace is required")

	assert.Empty(t, ValidateExternal([]types.E2EExpectation{{Name: "s", External: []types.E2EExternalCheck{call("ok", "http://x.invalid")}}}))
}

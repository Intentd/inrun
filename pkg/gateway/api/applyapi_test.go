package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/intentd/inrun/pkg/util/httpapi"

	"github.com/intentd/inrun/pkg/blueprint"
	"github.com/intentd/inrun/pkg/registry/simulate"
	"github.com/intentd/inrun/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// ── helpers ───────────────────────────────────────────────────────────────────

// newKat builds a minimal *blueprint.Blueprint with one serve-enabled CRD
// so handler tests have a real lookup target without a cluster.
func newKat(entries map[string]*types.CRDEntry) *blueprint.Blueprint {
	return blueprint.NewFromEntryPointers(entries)
}

// appCRD is a reusable serve-enabled CRD entry for tests.
func appCRD() *types.CRDEntry {
	return &types.CRDEntry{
		APITypes: types.APITypes{
			Group:   "platform.myorg.io",
			Version: "v1",
			Kind:    "App",
			Plural:  "apps",
		},
		Serve: &types.ServeConfig{
			Enabled: true,
			Target: types.ServeTargetValue{Entries: map[string]*types.ServeTargetConfig{
				"app": {Primary: true},
			}},
			Title:       "Application",
			Description: "Deploy an application",
			Name:        `{{ .name }}`,
			Namespace:   `{{ .team }}-{{ .environment }}`,
			Fields: map[string]types.ServeFieldConfig{
				"name":        {Label: "Name", Type: "string", Required: true, Order: 1},
				"image":       {Label: "Image", Type: "string", Required: true, Order: 2},
				"environment": {Label: "Environment", Type: "string", Required: true, Order: 3},
				"replicas":    {Label: "Replicas", Type: "integer", Order: 4},
			},
			Labels: map[string]types.ServeFieldConfig{
				"team": {Label: "Team", Type: "string", Required: true, Order: 0},
			},
			Annotations: map[string]types.ServeFieldConfig{
				"jira-ticket": {Label: "Jira Ticket", Type: "string", Order: 5},
			},
		},
	}
}

// restrictedCRD returns a CRD with tokens configured.
func restrictedCRD() *types.CRDEntry {
	crd := appCRD()
	crd.Serve.Tokens = map[string]types.ServeTokenPermissions{
		"ci-pipeline": {
			Permissions: types.ServePermissionSet{
				Resources: []string{"create", "update", "get", "list"},
				Schema:    []string{"get"},
			},
		},
		"read-only": {
			Permissions: types.ServePermissionSet{
				Global: []string{"get", "list"},
			},
		},
	}
	return crd
}

// requestWithToken injects a token name into the request context, simulating
// what the auth middleware does. Tests call this instead of setting an
// Authorization header so they do not depend on the token resolution logic.
func requestWithToken(r *http.Request, token string) *http.Request {
	return r.WithContext(WithTokenName(r.Context(), token))
}

// postJSON builds a POST request with a JSON body.
func postJSON(t *testing.T, path string, body interface{}) *http.Request {
	t.Helper()
	b, err := json.Marshal(body)
	require.NoError(t, err)
	r := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(b))
	r.Header.Set("Content-Type", "application/json")
	return r
}

// noopNotes
func noopNotes() types.KELFunctionRegistry { return types.KELFunctionRegistry{} }

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// schemaHandler
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

func TestSchemaHandler(t *testing.T) {
	bp := newKat(map[string]*types.CRDEntry{"app": appCRD()})
	handler := ExportedSchemaHandler(bp)

	t.Run("method not allowed", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodPost, "/api/v1/schema/", nil)
		r = requestWithToken(r, "ci-pipeline")
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, r)
		assert.Equal(t, http.StatusMethodNotAllowed, rr.Code)
	})

	t.Run("service list returned when no target", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/api/v1/schema/", nil)
		r = requestWithToken(r, "ci-pipeline")
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, r)
		assert.Equal(t, http.StatusOK, rr.Code)

		var resp httpapi.PaginatedResponse[ServiceEntry]
		require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
		assert.NotNil(t, resp.Items)
	})

	t.Run("schema returned for valid target", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/api/v1/schema/?target=app", nil)
		r = requestWithToken(r, "ci-pipeline")
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, r)
		assert.Equal(t, http.StatusOK, rr.Code)

		var resp SchemaResponse
		require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
		assert.Equal(t, "app", resp.Target)
		assert.Equal(t, "Application", resp.Title)
		assert.NotEmpty(t, resp.Fields)
		// Flat fields include spec + label + annotation fields merged
		_, hasTeam := resp.Fields["team"]
		assert.True(t, hasTeam, "label field 'team' must appear in flat fields")
		_, hasJira := resp.Fields["jira-ticket"]
		assert.True(t, hasJira, "annotation field 'jira-ticket' must appear in flat fields")
	})

	t.Run("unknown target returns 404", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/api/v1/schema/?target=unknown", nil)
		r = requestWithToken(r, "ci-pipeline")
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, r)
		assert.Equal(t, http.StatusNotFound, rr.Code)
	})

	t.Run("permission denied on restricted CRD", func(t *testing.T) {
		cat2 := newKat(map[string]*types.CRDEntry{"app": restrictedCRD()})
		h2 := ExportedSchemaHandler(cat2)

		// ci-pipeline has schema:get — allowed
		r := httptest.NewRequest(http.MethodGet, "/api/v1/schema/?target=app", nil)
		r = requestWithToken(r, "ci-pipeline")
		rr := httptest.NewRecorder()
		h2.ServeHTTP(rr, r)
		assert.Equal(t, http.StatusOK, rr.Code)

		// unknown token — denied
		r2 := httptest.NewRequest(http.MethodGet, "/api/v1/schema/?target=app", nil)
		r2 = requestWithToken(r2, "rogue-token")
		rr2 := httptest.NewRecorder()
		h2.ServeHTTP(rr2, r2)
		assert.Equal(t, http.StatusForbidden, rr2.Code)
	})

	t.Run("pagination params accepted on service list", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/api/v1/schema/?limit=1&offset=0", nil)
		r = requestWithToken(r, "ci-pipeline")
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, r)
		assert.Equal(t, http.StatusOK, rr.Code)
	})
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// resourcesHandler — routing and guards (no cluster needed)
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

func TestResourcesHandler_Routing(t *testing.T) {
	bp := newKat(map[string]*types.CRDEntry{"app": appCRD()})

	t.Run("method not allowed", func(t *testing.T) {
		h := ExportedResourcesHandler(nil, bp, types.KELFunctionRegistry{})
		r := httptest.NewRequest(http.MethodPut, "/api/v1/resources/app/default/x", nil)
		r = requestWithToken(r, "ci-pipeline")
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, r)
		assert.Equal(t, http.StatusMethodNotAllowed, rr.Code)
	})

	t.Run("bad path — only one segment", func(t *testing.T) {
		h := ExportedResourcesHandler(nil, bp, types.KELFunctionRegistry{})
		r := httptest.NewRequest(http.MethodGet, "/api/v1/resources/onlyone", nil)
		r = requestWithToken(r, "ci-pipeline")
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, r)
		assert.Equal(t, http.StatusBadRequest, rr.Code)
	})

	t.Run("unknown kind returns 404", func(t *testing.T) {
		h := ExportedResourcesHandler(nil, bp, types.KELFunctionRegistry{})
		r := httptest.NewRequest(http.MethodGet, "/api/v1/resources/unknown/default", nil)
		r = requestWithToken(r, "ci-pipeline")
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, r)
		assert.Equal(t, http.StatusNotFound, rr.Code)
	})

	t.Run("DELETE without name returns 400", func(t *testing.T) {
		h := ExportedResourcesHandler(nil, bp, types.KELFunctionRegistry{})
		r := httptest.NewRequest(http.MethodDelete, "/api/v1/resources/app/default", nil)
		r = requestWithToken(r, "ci-pipeline")
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, r)
		assert.Equal(t, http.StatusBadRequest, rr.Code)
	})
}

func TestResourcesHandler_Permissions(t *testing.T) {
	bp := newKat(map[string]*types.CRDEntry{"app": restrictedCRD()})

	// The fake dynamic client needs its List kind registered for any GVR it
	// will List() — "coding error" panic otherwise, not a graceful error.
	scheme := runtime.NewScheme()
	appGVK := schema.GroupVersionKind{Group: "platform.myorg.io", Version: "v1", Kind: "App"}
	scheme.AddKnownTypeWithName(appGVK, &unstructured.Unstructured{})
	scheme.AddKnownTypeWithName(appGVK.GroupVersion().WithKind("AppList"), &unstructured.UnstructuredList{})

	kube := simulate.NewFakeKubeclient(scheme)
	h := ExportedResourcesHandler(kube, bp, types.KELFunctionRegistry{})

	cases := []struct {
		name     string
		token    string
		method   string
		path     string
		wantCode int
	}{
		// ci-pipeline has resources: [create update get list]
		{"ci-pipeline can list", "ci-pipeline", http.MethodGet, "/api/v1/resources/app/default", http.StatusOK},
		{"ci-pipeline can get", "ci-pipeline", http.MethodGet, "/api/v1/resources/app/default/my-app", http.StatusOK},
		// ci-pipeline does NOT have delete
		{"ci-pipeline cannot delete", "ci-pipeline", http.MethodDelete, "/api/v1/resources/app/default/my-app", http.StatusForbidden},
		// read-only has global: [get list]
		{"read-only can get", "read-only", http.MethodGet, "/api/v1/resources/app/default/my-app", http.StatusOK},
		{"read-only can list", "read-only", http.MethodGet, "/api/v1/resources/app/default", http.StatusOK},
		// unknown token
		{"unknown token denied", "rogue", http.MethodGet, "/api/v1/resources/app/default", http.StatusForbidden},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest(tc.method, tc.path, nil)
			r = requestWithToken(r, tc.token)
			rr := httptest.NewRecorder()
			h.ServeHTTP(rr, r)
			// Only the permission layer is asserted here — the fake dynamic
			// client has no seeded objects, so a 200-permitted "get" against a
			// name that doesn't exist still returns 404, not 200.
			if tc.wantCode == http.StatusForbidden {
				assert.Equal(t, http.StatusForbidden, rr.Code)
			} else {
				assert.NotEqual(t, http.StatusForbidden, rr.Code,
					"permission check should not deny token %q", tc.token)
			}
		})
	}
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// applyHandler — format detection and validation (no cluster needed)
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

func TestApplyHandler_Format(t *testing.T) {
	bp := newKat(map[string]*types.CRDEntry{"app": appCRD()})
	h := ExportedApplyHandler(nil, bp, types.KELFunctionRegistry{})

	t.Run("method not allowed", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/api/v1/apply", nil)
		r = requestWithToken(r, "ci-pipeline")
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, r)
		assert.Equal(t, http.StatusMethodNotAllowed, rr.Code)
	})

	t.Run("invalid body returns 400", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodPost, "/api/v1/apply",
			bytes.NewBufferString("{not json"))
		r = requestWithToken(r, "ci-pipeline")
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, r)
		assert.Equal(t, http.StatusBadRequest, rr.Code)
		var resp ApplyResponse
		require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
		assert.Contains(t, resp.Message, "invalid JSON or YAML")
	})

	// A YAML body is read like JSON: these reach the target checks.
	t.Run("YAML body with unknown target returns 400 with available targets", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodPost, "/api/v1/apply",
			bytes.NewBufferString("target: unknown-target\nname: payments-api\n"))
		r = requestWithToken(r, "ci-pipeline")
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, r)
		assert.Equal(t, http.StatusBadRequest, rr.Code)
		var resp ApplyResponse
		require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
		assert.Contains(t, resp.Message, "unknown target")
		assert.Contains(t, resp.Message, "app")
	})

	t.Run("YAML body without target returns 400", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodPost, "/api/v1/apply",
			bytes.NewBufferString("name: payments-api\n"))
		r = requestWithToken(r, "ci-pipeline")
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, r)
		assert.Equal(t, http.StatusBadRequest, rr.Code)
		var resp ApplyResponse
		require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
		assert.Contains(t, resp.Message, "target")
	})

	t.Run("no target or apiVersion returns 400", func(t *testing.T) {
		r := postJSON(t, "/api/v1/apply", map[string]interface{}{
			"name": "payments-api",
		})
		r = requestWithToken(r, "ci-pipeline")
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, r)
		assert.Equal(t, http.StatusBadRequest, rr.Code)
		var resp ApplyResponse
		require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
		assert.Contains(t, resp.Message, "target")
	})

	t.Run("unknown target returns 400 with available targets", func(t *testing.T) {
		r := postJSON(t, "/api/v1/apply", map[string]interface{}{
			"target": "unknown-target",
		})
		r = requestWithToken(r, "ci-pipeline")
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, r)
		assert.Equal(t, http.StatusBadRequest, rr.Code)
		var resp ApplyResponse
		require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
		assert.Contains(t, resp.Message, "unknown target")
		assert.Contains(t, resp.Message, "app") // available targets listed
	})

	t.Run("empty target string returns 400", func(t *testing.T) {
		r := postJSON(t, "/api/v1/apply", map[string]interface{}{
			"target": "",
		})
		r = requestWithToken(r, "ci-pipeline")
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, r)
		assert.Equal(t, http.StatusBadRequest, rr.Code)
	})

	t.Run("unknown kind in full CR mode returns 400", func(t *testing.T) {
		r := postJSON(t, "/api/v1/apply", map[string]interface{}{
			"apiVersion": "platform.myorg.io/v1",
			"kind":       "UnknownKind",
			"metadata":   map[string]interface{}{"name": "x", "namespace": "default"},
			"spec":       map[string]interface{}{},
		})
		r = requestWithToken(r, "ci-pipeline")
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, r)
		assert.Equal(t, http.StatusBadRequest, rr.Code)
		var resp ApplyResponse
		require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
		assert.Contains(t, resp.Message, "UnknownKind")
	})
}

func TestApplyHandler_Permissions(t *testing.T) {
	bp := newKat(map[string]*types.CRDEntry{"app": restrictedCRD()})
	// Real fake client, not nil — the permission check probes
	// kube.DynamicClient() to distinguish create vs update before deciding
	// whether the token is allowed, even for a token that will be denied
	// outright regardless of operation.
	kube := simulate.NewFakeKubeclient(runtime.NewScheme())
	h := ExportedApplyHandler(kube, bp, types.KELFunctionRegistry{})

	t.Run("unknown token denied before SSA", func(t *testing.T) {
		r := postJSON(t, "/api/v1/apply", map[string]interface{}{
			"target":      "app",
			"name":        "my-app",
			"team":        "payments",
			"environment": "staging",
		})
		r = requestWithToken(r, "rogue-token")
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, r)
		assert.Equal(t, http.StatusForbidden, rr.Code)
	})
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// checkServePermission
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

func TestCheckServePermission(t *testing.T) {
	crd := restrictedCRD()

	check := func(token, op, ns string, class types.ServeEndpointClass) int {
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r = requestWithToken(r, token)
		rr := httptest.NewRecorder()
		ok := checkServePermission(rr, r, crd, class, op, ns, "")
		if ok {
			return http.StatusOK
		}
		return rr.Code
	}

	// ci-pipeline: resources=[create update get list], schema=[get]
	assert.Equal(t, http.StatusOK, check("ci-pipeline", "get", "default", types.ServeClassResources))
	assert.Equal(t, http.StatusOK, check("ci-pipeline", "list", "default", types.ServeClassResources))
	assert.Equal(t, http.StatusOK, check("ci-pipeline", "get", "default", types.ServeClassSchema))
	assert.Equal(t, http.StatusForbidden, check("ci-pipeline", "delete", "default", types.ServeClassResources))

	// read-only: global=[get list] — applies to all classes
	assert.Equal(t, http.StatusOK, check("read-only", "get", "default", types.ServeClassResources))
	assert.Equal(t, http.StatusOK, check("read-only", "get", "default", types.ServeClassSchema))
	assert.Equal(t, http.StatusForbidden, check("read-only", "delete", "default", types.ServeClassResources))

	// unknown token
	assert.Equal(t, http.StatusForbidden, check("rogue", "get", "default", types.ServeClassResources))

	// no restrictions — nil serve
	openCRD := appCRD()
	openCRD.Serve.Tokens = nil
	rOpen := httptest.NewRequest(http.MethodGet, "/", nil)
	rOpen = requestWithToken(rOpen, "anyone")
	rrOpen := httptest.NewRecorder()
	assert.True(t, checkServePermission(rrOpen, rOpen, openCRD,
		types.ServeClassResources, "delete", "default", ""))
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// writeKubeError
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

func TestWriteKubeError(t *testing.T) {
	cases := []struct {
		msg      string
		wantCode int
	}{
		{"resource not found", http.StatusNotFound},
		{"NotFound: no such resource", http.StatusNotFound},
		{"forbidden: access denied", http.StatusForbidden},
		{"internal server error", http.StatusInternalServerError},
	}
	for _, tc := range cases {
		t.Run(tc.msg, func(t *testing.T) {
			rr := httptest.NewRecorder()
			ExportedWriteKubeError(rr, fmt.Errorf("%s", tc.msg))
			assert.Equal(t, tc.wantCode, rr.Code)
		})
	}
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// pagination
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

func TestParsePagination(t *testing.T) {
	parse := func(query string) httpapi.PaginationParams {
		r := httptest.NewRequest(http.MethodGet, "/?"+query, nil)
		return httpapi.ParsePagination(r)
	}

	t.Run("defaults", func(t *testing.T) {
		p := parse("")
		assert.Equal(t, 100, p.Limit)
		assert.Equal(t, 0, p.Offset)
		assert.Equal(t, "", p.Continue)
	})

	t.Run("custom limit and offset", func(t *testing.T) {
		p := parse("limit=25&offset=50")
		assert.Equal(t, 25, p.Limit)
		assert.Equal(t, 50, p.Offset)
	})

	t.Run("limit capped at 1000", func(t *testing.T) {
		p := parse("limit=9999")
		assert.Equal(t, 1000, p.Limit)
	})

	t.Run("invalid limit ignored — falls back to default", func(t *testing.T) {
		p := parse("limit=abc")
		assert.Equal(t, 100, p.Limit)
	})

	t.Run("negative offset ignored", func(t *testing.T) {
		p := parse("offset=-5")
		assert.Equal(t, 0, p.Offset)
	})

	t.Run("continue token", func(t *testing.T) {
		p := parse("continue=abc123")
		assert.Equal(t, "abc123", p.Continue)
	})
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// ServeTokenPermissions.TokenAllowed — class-aware permission model
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

func TestTokenAllowed(t *testing.T) {
	cfg := func(tokens map[string]types.ServeTokenPermissions) *types.ServeConfig {
		return &types.ServeConfig{
			Enabled: true,
			Tokens:  tokens,
		}
	}

	cases := []struct {
		name       string
		serve      *types.ServeConfig
		token      string
		op         string
		ns         string
		class      types.ServeEndpointClass
		wantOK     bool
		wantReason types.ServeDenyReason
	}{
		{
			name:  "no restrictions — any token allowed",
			serve: cfg(nil),
			token: "anyone", op: "delete", ns: "prod",
			class:  types.ServeClassResources,
			wantOK: true,
		},
		{
			name: "global wildcard allows all",
			serve: cfg(map[string]types.ServeTokenPermissions{
				"cc": {Permissions: types.ServePermissionSet{Global: []string{"*"}}},
			}),
			token: "cc", op: "delete", ns: "prod",
			class:  types.ServeClassResources,
			wantOK: true,
		},
		{
			name: "class-specific list takes precedence over global",
			serve: cfg(map[string]types.ServeTokenPermissions{
				"ci": {Permissions: types.ServePermissionSet{
					Global:    []string{"get", "list"},
					Resources: []string{"create", "update"},
				}},
			}),
			token: "ci", op: "create", ns: "staging",
			class:  types.ServeClassResources,
			wantOK: true,
		},
		{
			name: "global not used when class-specific is set",
			serve: cfg(map[string]types.ServeTokenPermissions{
				"ci": {Permissions: types.ServePermissionSet{
					Global:    []string{"get", "list"},
					Resources: []string{"create", "update"},
				}},
			}),
			// get IS in global but resources list is [create update] — get not present
			token: "ci", op: "get", ns: "staging",
			class:      types.ServeClassResources,
			wantOK:     false,
			wantReason: types.ServeDenyReasonOperation,
		},
		{
			name: "falls back to global when class list empty",
			serve: cfg(map[string]types.ServeTokenPermissions{
				"audit": {Permissions: types.ServePermissionSet{
					Global: []string{"get", "list"},
				}},
			}),
			token: "audit", op: "get", ns: "prod",
			class:  types.ServeClassSchema,
			wantOK: true,
		},
		{
			name: "unknown token denied",
			serve: cfg(map[string]types.ServeTokenPermissions{
				"ci": {Permissions: types.ServePermissionSet{Global: []string{"*"}}},
			}),
			token: "rogue", op: "get", ns: "default",
			class:      types.ServeClassResources,
			wantOK:     false,
			wantReason: types.ServeDenyReasonUnknownToken,
		},
		{
			name: "namespace restriction denied",
			serve: cfg(map[string]types.ServeTokenPermissions{
				"ci": {
					Namespaces:  []string{"staging"},
					Permissions: types.ServePermissionSet{Global: []string{"*"}},
				},
			}),
			token: "ci", op: "create", ns: "production",
			class:      types.ServeClassResources,
			wantOK:     false,
			wantReason: types.ServeDenyReasonNamespace,
		},
		{
			name: "empty permissions denies",
			serve: cfg(map[string]types.ServeTokenPermissions{
				"empty": {Permissions: types.ServePermissionSet{}},
			}),
			token: "empty", op: "get", ns: "default",
			class:      types.ServeClassResources,
			wantOK:     false,
			wantReason: types.ServeDenyReasonOperation,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ok, reason := tc.serve.TokenAllowed(tc.token, tc.op, tc.ns, tc.class)
			assert.Equal(t, tc.wantOK, ok)
			if !tc.wantOK {
				assert.Equal(t, tc.wantReason, reason)
				msg := reason.Message(tc.token, tc.op, "MyKind", tc.ns)
				assert.NotEmpty(t, msg)
				assert.Contains(t, msg, tc.token)
			}
		})
	}
}

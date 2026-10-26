package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/intentd/inrun/pkg/blueprint"
	"github.com/intentd/inrun/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// registeredBlueprint builds a *blueprint.Blueprint with one serve-enabled CRD whose
// Kind matches the given kind string, for tests that need kind lookup to
// succeed so execution reaches the method/name checks after it.
func registeredBlueprint(kind string) *blueprint.Blueprint {
	return blueprint.NewFromEntryPointers(map[string]*types.CRDEntry{
		kind: {
			APITypes: types.APITypes{
				Group:   "platform.myorg.io",
				Version: "v1",
				Kind:    kind,
				Plural:  kind + "s",
			},
			Serve: &types.ServeConfig{Enabled: true},
		},
	})
}

func TestParsePath(t *testing.T) {
	cases := []struct {
		path      string
		wantKind  string
		wantNS    string
		wantName  string
		wantError bool
	}{
		// list paths
		{"/api/v1/resources/platformresource/team-payments", "platformresource", "team-payments", "", false},
		// get/delete paths
		{"/api/v1/resources/platformresource/team-payments/my-app", "platformresource", "team-payments", "my-app", false},
		{"/api/v1/resources/platform/default/app-1", "platform", "default", "app-1", false},
		// too few segments
		{"/api/v1/resources/kind", "", "", "", true},
		{"/api/v1/resources/", "", "", "", true},
	}
	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			kind, ns, name, err := ParsePath(tc.path)
			if tc.wantError {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.wantKind, kind)
			assert.Equal(t, tc.wantNS, ns)
			assert.Equal(t, tc.wantName, name)
		})
	}
}

func TestResourcesHandler_UnknownKind(t *testing.T) {
	h := resourcesHandler(nil, &ClusterRegistry{}, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/resources/unknown/default", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rr.Code)
	}
}

func TestResourcesHandler_MethodNotAllowed(t *testing.T) {
	h := resourcesHandler(nil, &ClusterRegistry{}, registeredBlueprint("platform"))
	req := httptest.NewRequest(http.MethodPut, "/api/v1/resources/platform/default/x", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want 405", rr.Code)
	}
}

func TestResourcesHandler_BadPath(t *testing.T) {
	h := resourcesHandler(nil, &ClusterRegistry{}, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/resources/onlyone", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rr.Code)
	}
}

func TestResourcesHandler_DeleteRequiresName(t *testing.T) {
	h := resourcesHandler(nil, &ClusterRegistry{}, registeredBlueprint("thing"))
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/resources/thing/default", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rr.Code)
	}
}

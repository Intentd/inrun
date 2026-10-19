package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/intentd/inrun/pkg/blueprint"
	"github.com/intentd/inrun/pkg/types"
)

// serverKat returns a *blueprint.Blueprint with the Gateway API enabled and one
// serve-enabled CRD, so Register() actually registers routes.
func serverKat() *blueprint.Blueprint {
	bp := newKat(map[string]*types.CRDEntry{"app": appCRD()})
	bp.Gateway = &types.GatewayConfig{
		API: &types.GatewayAPIConfig{Enabled: true},
	}
	return bp
}

type fakeRegistrar struct {
	routes map[string]http.HandlerFunc
}

func newFakeRegistrar() *fakeRegistrar {
	return &fakeRegistrar{routes: make(map[string]http.HandlerFunc)}
}

func (f *fakeRegistrar) Register(path string, h http.HandlerFunc) {
	f.routes[path] = h
}

func TestAPIServer_Register_RoutesPresent(t *testing.T) {
	ts := &TokenSet{entries: []resolvedToken{{name: "ci", value: "tok"}}}
	s := &APIServer{tokens: ts, bp: serverKat()}

	reg := newFakeRegistrar()
	s.Register(reg)

	required := []string{"/api/v1/apply", "/api/v1/resources/", "/api/v1/schema"}
	for _, r := range required {
		if _, ok := reg.routes[r]; !ok {
			t.Errorf("route %q not registered", r)
		}
	}
}

func TestAPIServer_AuthWrapsAllRoutes(t *testing.T) {
	ts := &TokenSet{entries: []resolvedToken{{name: "ci", value: "valid-token"}}}
	s := &APIServer{tokens: ts, bp: serverKat()}

	reg := newFakeRegistrar()
	s.Register(reg)

	for path, h := range reg.routes {
		t.Run(path, func(t *testing.T) {
			// Request without token → 401
			req := httptest.NewRequest(http.MethodGet, path, nil)
			rr := httptest.NewRecorder()
			h(rr, req)
			if rr.Code != http.StatusUnauthorized {
				t.Errorf("%s without token: status = %d, want 401", path, rr.Code)
			}
		})
	}
}

package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/intentd/inrun/pkg/blueprint"
	"github.com/intentd/inrun/pkg/registry/simulate"
	"github.com/intentd/inrun/pkg/types"
	"k8s.io/apimachinery/pkg/runtime"
)

// appRequestBody is a minimal, valid AppRequest apply body. name is omitted
// when empty, matching what a client that never set metadata.name sends.
func appRequestBody(name string) []byte {
	metadata := map[string]any{}
	if name != "" {
		metadata["name"] = name
	}
	body, _ := json.Marshal(map[string]any{
		"apiVersion": "platform.myorg.io/v1",
		"kind":       "AppRequest",
		"metadata":   metadata,
		"spec":       map[string]any{"image": "nginx"},
	})
	return body
}

// appRequestBlueprint builds a real, lookup-ready *blueprint.Blueprint with one
// serve-enabled "AppRequest" CRD.
func appRequestBlueprint(serveName string) *blueprint.Blueprint {
	return blueprint.NewFromEntryPointers(map[string]*types.CRDEntry{
		"apprequest": {
			APITypes: types.APITypes{
				Group:   "platform.myorg.io",
				Version: "v1",
				Kind:    "AppRequest",
				Plural:  "apprequests",
			},
			Serve: &types.ServeConfig{Enabled: true, Name: serveName},
		},
	})
}

func TestApplyHandler_MissingName_Rejected(t *testing.T) {
	kube := simulate.NewFakeKubeclient(runtime.NewScheme())
	h := applyHandler(kube, &ClusterRegistry{}, appRequestBlueprint(""))
	req := httptest.NewRequest(http.MethodPost, "/api/v1/apply", bytes.NewReader(appRequestBody("")))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422", rr.Code)
	}
	var resp ApplyResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Accepted {
		t.Error("Accepted should be false when name is missing and serve.name is not declared")
	}
	if resp.Message != "name is required" {
		t.Errorf("Message = %q, want %q", resp.Message, "name is required")
	}
	if len(resp.Violations) != 1 || resp.Violations[0].Field != "metadata.name" {
		t.Errorf("Violations = %+v, want one violation on metadata.name", resp.Violations)
	}
}

func TestApplyHandler_NameSupplied_NotRejected(t *testing.T) {
	kube := simulate.NewFakeKubeclient(runtime.NewScheme())
	h := applyHandler(kube, &ClusterRegistry{}, appRequestBlueprint(""))
	req := httptest.NewRequest(http.MethodPost, "/api/v1/apply", bytes.NewReader(appRequestBody("payments-api")))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	// Whatever happens downstream against the fake dynamic client, it must
	// not be the "name is required" rejection — a name was supplied.
	if rr.Code == http.StatusUnprocessableEntity {
		var resp ApplyResponse
		if err := json.NewDecoder(rr.Body).Decode(&resp); err == nil && resp.Message == "name is required" {
			t.Errorf("got the missing-name rejection even though a name was supplied: %+v", resp)
		}
	}
}

func TestApplyHandler_SerbeName_ResolvesWithoutClientName(t *testing.T) {
	kube := simulate.NewFakeKubeclient(runtime.NewScheme())
	h := applyHandler(kube, &ClusterRegistry{}, appRequestBlueprint("resolved-name"))
	req := httptest.NewRequest(http.MethodPost, "/api/v1/apply", bytes.NewReader(appRequestBody("")))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	// serve.name is declared, so the client never having sent a name must not
	// produce the missing-name rejection — it resolves to "resolved-name" instead.
	if rr.Code == http.StatusUnprocessableEntity {
		var resp ApplyResponse
		if err := json.NewDecoder(rr.Body).Decode(&resp); err == nil && resp.Message == "name is required" {
			t.Errorf("got the missing-name rejection even though serve.name is declared: %+v", resp)
		}
	}
}

func TestApplyHandler_MethodNotAllowed(t *testing.T) {
	h := applyHandler(nil, &ClusterRegistry{}, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/apply", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want 405", rr.Code)
	}
}

func TestApplyHandler_InvalidJSON(t *testing.T) {
	h := applyHandler(nil, &ClusterRegistry{}, nil)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/apply", strings.NewReader("not-json"))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rr.Code)
	}
	var resp ApplyResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Accepted {
		t.Error("Accepted should be false for invalid JSON")
	}
	if !strings.Contains(resp.Message, "invalid JSON") {
		t.Errorf("message = %q, want to contain 'invalid JSON'", resp.Message)
	}
}

func TestApplyHandler_EmptyBody(t *testing.T) {
	h := applyHandler(nil, &ClusterRegistry{}, nil)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/apply", bytes.NewReader([]byte{}))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rr.Code)
	}
}

func TestApplyResponse_JSON(t *testing.T) {
	resp := ApplyResponse{
		Accepted:   true,
		Name:       "my-app",
		Namespace:  "team-payments",
		Kind:       "PlatformResource",
		APIVersion: "platform.inrun.dev/v1alpha1",
		PollURL:    "/api/v1/resources/PlatformResource/team-payments/my-app",
	}
	b, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got ApplyResponse
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.Name != "my-app" || got.Namespace != "team-payments" || !got.Accepted {
		t.Errorf("round-trip mismatch: %+v", got)
	}
}

func TestApplyResponse_DryRunAndViolations(t *testing.T) {
	resp := ApplyResponse{
		Accepted: false,
		DryRun:   true,
		Message:  "admission webhook denied the request",
		Violations: []ApplyViolation{
			{Field: "spec.environment", Message: "must be staging or production", Severity: "error"},
			{Field: "spec.team", Message: "required field", Severity: "error"},
		},
	}
	b, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got ApplyResponse
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.Accepted {
		t.Error("Accepted should be false")
	}
	if !got.DryRun {
		t.Error("DryRun should be true")
	}
	if len(got.Violations) != 2 {
		t.Fatalf("Violations len = %d, want 2", len(got.Violations))
	}
	if got.Violations[0].Field != "spec.environment" {
		t.Errorf("Violations[0].Field = %q", got.Violations[0].Field)
	}
	if got.Violations[0].Severity != "error" {
		t.Errorf("Violations[0].Severity = %q", got.Violations[0].Severity)
	}
}

func TestExtractViolations_NoStatusError(t *testing.T) {
	// A plain error produces no violations.
	vs := extractViolations(fmt.Errorf("some error"))
	if len(vs) != 0 {
		t.Errorf("expected no violations for plain error, got %v", vs)
	}
}

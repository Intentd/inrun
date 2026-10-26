package runtimeapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// fakeRuntime serves the runtime routes the admission callers read.
func fakeRuntime(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc(CRListPath("website"), func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("field") != "spec.domain" {
			http.Error(w, "missing field", http.StatusBadRequest)
			return
		}
		_ = json.NewEncoder(w).Encode(CRListResponse{Items: []CRSummary{
			{Namespace: "default", Name: "site-a", Value: "a.example.com"},
		}})
	})
	mux.HandleFunc(HealthPath("website"), func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"healthy": true})
	})
	mux.HandleFunc(CRDPath("website"), func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"metrics": map[string]any{"workers": 3}})
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

// The callers see what the runtime answered: a value another CR holds is not
// unique, and health and metrics come back filled in.
func TestCallersDecodeTheRuntimesAnswer(t *testing.T) {
	q := New(context.Background(), fakeRuntime(t).URL, "website")

	unique, err := q.IsUnique("spec.domain", "a.example.com", "default", "site-b")
	if err != nil {
		t.Fatalf("IsUnique: %v", err)
	}
	if unique {
		t.Error("a.example.com is held by site-a, but IsUnique said unique")
	}

	unique, err = q.IsUnique("spec.domain", "a.example.com", "default", "site-a")
	if err != nil || !unique {
		t.Errorf("site-a's own value: unique=%v err=%v, want unique", unique, err)
	}

	if got := q.Health()["healthy"]; got != true {
		t.Errorf("Health()[healthy] = %v, want true", got)
	}
	if got := q.Metrics()["workers"]; got != float64(3) {
		t.Errorf("Metrics()[workers] = %v, want 3", got)
	}
}

// A runtime that answers with an error is an error, never a pass.
func TestIsUnique_RuntimeErrorIsAnError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "down", http.StatusServiceUnavailable)
	}))
	t.Cleanup(srv.Close)

	if _, err := New(context.Background(), srv.URL, "website").IsUnique("spec.domain", "x", "default", "a"); err == nil {
		t.Error("a 503 from the runtime was not reported")
	}
}

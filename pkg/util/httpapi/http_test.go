package httpapi

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
)

func TestWriteJSON_BasicPayload(t *testing.T) {
	w := httptest.NewRecorder()
	WriteJSON(w, 200, map[string]string{"status": "ok"})

	if w.Code != 200 {
		t.Errorf("expected 200, got %d", w.Code)
	}
	var got map[string]string
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("response is not valid JSON: %v", err)
	}
	if got["status"] != "ok" {
		t.Errorf("expected status=ok, got %q", got["status"])
	}
}

func TestWriteJSON_ContentTypeHeader(t *testing.T) {
	w := httptest.NewRecorder()
	WriteJSON(w, 201, map[string]string{"id": "1"})
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("expected application/json, got %q", ct)
	}
}

func TestWriteJSON_NilData(t *testing.T) {
	w := httptest.NewRecorder()
	WriteJSON(w, 204, nil)
	if w.Code != 204 {
		t.Errorf("expected 204, got %d", w.Code)
	}
	if w.Body.Len() != 0 {
		t.Error("nil data must produce empty body")
	}
}

func TestWriteJSON_ErrorStatus(t *testing.T) {
	w := httptest.NewRecorder()
	WriteJSON(w, 400, map[string]string{"error": "bad request"})
	if w.Code != 400 {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

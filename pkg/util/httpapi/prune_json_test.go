package httpapi

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
)

// ── pruneJSONValue ────────────────────────────────────────────────────────────

func TestPruneJSONValue_NilValue(t *testing.T) {
	if pruneJSONValue(nil) != nil {
		t.Error("nil must prune to nil")
	}
}

func TestPruneJSONValue_EmptyString(t *testing.T) {
	if pruneJSONValue("") != nil {
		t.Error("empty string must prune to nil")
	}
}

func TestPruneJSONValue_ZeroNumber(t *testing.T) {
	if pruneJSONValue(float64(0)) != nil {
		t.Error("zero float64 must prune to nil")
	}
}

func TestPruneJSONValue_FalseBoolean(t *testing.T) {
	if pruneJSONValue(false) != nil {
		t.Error("false bool must prune to nil")
	}
}

func TestPruneJSONValue_TrueBoolean(t *testing.T) {
	if pruneJSONValue(true) == nil {
		t.Error("true bool must survive pruning")
	}
}

func TestPruneJSONValue_NonZeroNumber(t *testing.T) {
	if pruneJSONValue(float64(42)) == nil {
		t.Error("non-zero float64 must survive pruning")
	}
}

func TestPruneJSONValue_NonEmptyString(t *testing.T) {
	if pruneJSONValue("hello") == nil {
		t.Error("non-empty string must survive pruning")
	}
}

func TestPruneJSONValue_MapWithNilValues(t *testing.T) {
	input := map[string]interface{}{
		"keep":   "value",
		"remove": nil,
	}
	result := pruneJSONValue(input)
	m, ok := result.(map[string]interface{})
	if !ok {
		t.Fatal("result must be a map")
	}
	if _, hasRemove := m["remove"]; hasRemove {
		t.Error("nil value must be pruned from map")
	}
	if m["keep"] != "value" {
		t.Error("non-nil value must be kept")
	}
}

func TestPruneJSONValue_MapAllNil(t *testing.T) {
	input := map[string]interface{}{
		"a": nil,
		"b": nil,
	}
	result := pruneJSONValue(input)
	if result != nil {
		t.Error("fully-nil map must prune to nil")
	}
}

func TestPruneJSONValue_MapEmptyNestedMap(t *testing.T) {
	input := map[string]interface{}{
		"nested": map[string]interface{}{},
	}
	result := pruneJSONValue(input)
	if result != nil {
		t.Error("map containing only empty nested map must prune to nil")
	}
}

func TestPruneJSONValue_SliceWithNils(t *testing.T) {
	input := []interface{}{nil, "keep", nil}
	result := pruneJSONValue(input)
	s, ok := result.([]interface{})
	if !ok {
		t.Fatal("result must be a slice")
	}
	if len(s) != 1 || s[0] != "keep" {
		t.Errorf("expected [keep], got %v", s)
	}
}

func TestPruneJSONValue_EmptySlice(t *testing.T) {
	input := []interface{}{}
	result := pruneJSONValue(input)
	if result != nil {
		t.Error("empty slice must prune to nil")
	}
}

func TestPruneJSONValue_SliceAllNil(t *testing.T) {
	input := []interface{}{nil, nil}
	result := pruneJSONValue(input)
	if result != nil {
		t.Error("all-nil slice must prune to nil")
	}
}

// ── WriteJSONPruned ───────────────────────────────────────────────────────────

func TestWriteJSONPruned_BasicResponse(t *testing.T) {
	w := httptest.NewRecorder()
	payload := map[string]interface{}{
		"name":    "inrun",
		"version": nil,
	}
	WriteJSONPruned(w, 200, payload)

	if w.Code != 200 {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var got map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("response is not valid JSON: %v", err)
	}
	if got["name"] != "inrun" {
		t.Errorf("expected name=inrun, got %v", got["name"])
	}
	if _, hasVersion := got["version"]; hasVersion {
		t.Error("nil version must be pruned from response")
	}
}

func TestWriteJSONPruned_ContentTypeHeader(t *testing.T) {
	w := httptest.NewRecorder()
	WriteJSONPruned(w, 200, map[string]interface{}{"ok": true})
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("expected application/json, got %q", ct)
	}
}

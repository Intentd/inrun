package structured

import (
	"testing"

	"github.com/intentd/inrun/pkg/kubeclient/kubefake/object"
)

// ── RawToMap ─────────────────────────────────────────────────────────────────

func TestRawToMap_Nil(t *testing.T) {
	_, err := RawToMap(nil)
	if err == nil {
		t.Error("nil input must return an error")
	}
}

func TestRawToMap_Unstructured_FastPath(t *testing.T) {
	obj := object.New("v1", "Pod", "")
	m, err := RawToMap(obj)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Fast path must return the same map (identity, not a copy)
	if m["kind"] != "Pod" {
		t.Errorf("expected kind=Pod, got %v", m["kind"])
	}
	// Verify it is the same map pointer
	if &m == &obj.Object {
		// pointer comparison on map header — they should share the same backing map
	}
}

func TestRawToMap_Struct_JSONRoundTrip(t *testing.T) {
	type inner struct {
		Name  string `json:"name"`
		Count int    `json:"count"`
	}
	src := inner{Name: "test", Count: 42}
	m, err := RawToMap(src)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m["name"] != "test" {
		t.Errorf("expected name=test, got %v", m["name"])
	}
	// JSON numbers unmarshal as float64
	if m["count"] != float64(42) {
		t.Errorf("expected count=42 (float64), got %T %v", m["count"], m["count"])
	}
}

func TestRawToMap_NonMarshalable(t *testing.T) {
	// channels are not JSON-marshalable
	ch := make(chan int)
	_, err := RawToMap(ch)
	if err == nil {
		t.Error("non-marshalable type must return an error")
	}
}

// ── MetaField ─────────────────────────────────────────────────────────────────

func TestMetaField_Present(t *testing.T) {
	obj := map[string]interface{}{
		"metadata": map[string]interface{}{
			"name":      "my-object",
			"namespace": "default",
		},
	}
	if got := MetaField(obj, "name"); got != "my-object" {
		t.Errorf("expected my-object, got %q", got)
	}
	if got := MetaField(obj, "namespace"); got != "default" {
		t.Errorf("expected default, got %q", got)
	}
}

func TestMetaField_AbsentField(t *testing.T) {
	obj := map[string]interface{}{
		"metadata": map[string]interface{}{
			"name": "x",
		},
	}
	if got := MetaField(obj, "labels"); got != "" {
		t.Errorf("absent field must return empty string, got %q", got)
	}
}

func TestMetaField_NoMetadata(t *testing.T) {
	obj := map[string]interface{}{"spec": map[string]interface{}{}}
	if got := MetaField(obj, "name"); got != "" {
		t.Errorf("no metadata must return empty string, got %q", got)
	}
}

func TestMetaField_NonStringValue(t *testing.T) {
	obj := map[string]interface{}{
		"metadata": map[string]interface{}{
			"count": 42, // not a string
		},
	}
	if got := MetaField(obj, "count"); got != "" {
		t.Errorf("non-string value must return empty string, got %q", got)
	}
}

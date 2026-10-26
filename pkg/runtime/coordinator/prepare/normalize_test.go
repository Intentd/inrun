package prepare

import (
	"testing"
)

// ── parseNormalizedValue ──────────────────────────────────────────────────────

func TestParseNormalizedValue_EmptyString(t *testing.T) {
	got := parseNormalizedValue("")
	if got != "" {
		t.Errorf("empty input must return empty string, got %v", got)
	}
}

func TestParseNormalizedValue_Integer(t *testing.T) {
	got := parseNormalizedValue("3")
	if got != int64(3) {
		t.Errorf("expected int64(3), got %T %v", got, got)
	}
}

func TestParseNormalizedValue_NegativeInteger(t *testing.T) {
	got := parseNormalizedValue("-1")
	if got != int64(-1) {
		t.Errorf("expected int64(-1), got %T %v", got, got)
	}
}

func TestParseNormalizedValue_Float(t *testing.T) {
	got := parseNormalizedValue("3.14")
	if got != float64(3.14) {
		t.Errorf("expected 3.14, got %T %v", got, got)
	}
}

func TestParseNormalizedValue_True(t *testing.T) {
	got := parseNormalizedValue("true")
	if got != true {
		t.Errorf("expected true, got %T %v", got, got)
	}
}

func TestParseNormalizedValue_False(t *testing.T) {
	got := parseNormalizedValue("false")
	if got != false {
		t.Errorf("expected false, got %T %v", got, got)
	}
}

func TestParseNormalizedValue_Null_ReturnsString(t *testing.T) {
	// NOTE(logic): The case nil branch in parseNormalizedValue is dead code —
	// the outer `v != nil` guard prevents it from ever executing. "null" falls
	// through to return s. Tracked separately; test documents actual behaviour.
	got := parseNormalizedValue("null")
	if got != "null" {
		t.Errorf("expected string 'null' (actual behaviour), got %T %v", got, got)
	}
}

func TestParseNormalizedValue_PlainString(t *testing.T) {
	got := parseNormalizedValue("production")
	if got != "production" {
		t.Errorf("expected string production, got %T %v", got, got)
	}
}

func TestParseNormalizedValue_CronString(t *testing.T) {
	// Cron strings contain * and / which YAML will not misparse as a number
	got := parseNormalizedValue("*/5 * * * *")
	if got != "*/5 * * * *" {
		t.Errorf("cron string must be kept as-is, got %T %v", got, got)
	}
}

func TestParseNormalizedValue_StringWithSpaces(t *testing.T) {
	got := parseNormalizedValue("hello world")
	if got != "hello world" {
		t.Errorf("expected 'hello world', got %T %v", got, got)
	}
}

// ── coerceNormalizedValue ─────────────────────────────────────────────────────

func TestCoerceNormalizedValue_EmptyStringReturnsNil(t *testing.T) {
	for _, typ := range []string{"int", "bool", "float", "string"} {
		got, err := coerceNormalizedValue("", typ)
		if err != nil {
			t.Errorf("type %q: unexpected error: %v", typ, err)
		}
		if got != nil {
			t.Errorf("type %q: empty string must return nil, got %v", typ, got)
		}
	}
}

func TestCoerceNormalizedValue_Int(t *testing.T) {
	got, err := coerceNormalizedValue("3", "int")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != int64(3) {
		t.Errorf("expected int64(3), got %T %v", got, got)
	}
}

func TestCoerceNormalizedValue_IntegerAlias(t *testing.T) {
	got, err := coerceNormalizedValue("42", "integer")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != int64(42) {
		t.Errorf("expected int64(42), got %T %v", got, got)
	}
}

func TestCoerceNormalizedValue_BoolTrue(t *testing.T) {
	got, err := coerceNormalizedValue("true", "bool")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != true {
		t.Errorf("expected true, got %T %v", got, got)
	}
}

func TestCoerceNormalizedValue_BoolFalse(t *testing.T) {
	got, err := coerceNormalizedValue("false", "bool")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != false {
		t.Errorf("expected false, got %T %v", got, got)
	}
}

// Key difference from parseNormalizedValue: "0" declared as bool → false, not int64(0).
func TestCoerceNormalizedValue_ZeroAsBool(t *testing.T) {
	got, err := coerceNormalizedValue("0", "bool")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != false {
		t.Errorf("expected false, got %T %v", got, got)
	}
}

func TestCoerceNormalizedValue_Float(t *testing.T) {
	got, err := coerceNormalizedValue("0.75", "float")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != float64(0.75) {
		t.Errorf("expected 0.75, got %T %v", got, got)
	}
}

func TestCoerceNormalizedValue_String(t *testing.T) {
	got, err := coerceNormalizedValue("production", "string")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "production" {
		t.Errorf("expected production, got %T %v", got, got)
	}
}

func TestCoerceNormalizedValue_UnknownTypeReturnsError(t *testing.T) {
	_, err := coerceNormalizedValue("value", "uuid")
	if err == nil {
		t.Error("expected error for unknown type")
	}
}

func TestCoerceNormalizedValue_BadIntReturnsError(t *testing.T) {
	_, err := coerceNormalizedValue("not-a-number", "int")
	if err == nil {
		t.Error("expected error for non-integer value with int type")
	}
}

// ── splitDotPath ──────────────────────────────────────────────────────────────

func TestSplitDotPath_SingleSegment(t *testing.T) {
	parts := splitDotPath("replicas")
	if len(parts) != 1 || parts[0] != "replicas" {
		t.Errorf("unexpected: %v", parts)
	}
}

func TestSplitDotPath_TwoSegments(t *testing.T) {
	parts := splitDotPath("spec.replicas")
	if len(parts) != 2 || parts[0] != "spec" || parts[1] != "replicas" {
		t.Errorf("unexpected: %v", parts)
	}
}

func TestSplitDotPath_DeepPath(t *testing.T) {
	parts := splitDotPath("spec.resources.limits.cpu")
	want := []string{"spec", "resources", "limits", "cpu"}
	if len(parts) != len(want) {
		t.Fatalf("expected %d parts, got %d: %v", len(want), len(parts), parts)
	}
	for i, w := range want {
		if parts[i] != w {
			t.Errorf("part[%d]: expected %q, got %q", i, w, parts[i])
		}
	}
}

func TestSplitDotPath_Empty(t *testing.T) {
	parts := splitDotPath("")
	// An empty string produces one empty-string segment
	if len(parts) != 1 {
		t.Errorf("empty string should produce 1 part, got %v", parts)
	}
}

// ── setNestedNormalized ───────────────────────────────────────────────────────

func TestSetNestedNormalized_SingleKey(t *testing.T) {
	obj := map[string]interface{}{}
	if err := setNestedNormalized(obj, "key", "value"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if obj["key"] != "value" {
		t.Errorf("expected key=value, got %v", obj["key"])
	}
}

func TestSetNestedNormalized_TwoLevels(t *testing.T) {
	obj := map[string]interface{}{}
	if err := setNestedNormalized(obj, "spec.replicas", int64(3)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	spec, ok := obj["spec"].(map[string]interface{})
	if !ok {
		t.Fatal("expected spec to be a map")
	}
	if spec["replicas"] != int64(3) {
		t.Errorf("expected 3, got %v", spec["replicas"])
	}
}

func TestSetNestedNormalized_ThreeLevels(t *testing.T) {
	obj := map[string]interface{}{}
	if err := setNestedNormalized(obj, "spec.resources.limits", "cpu=500m"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	spec := obj["spec"].(map[string]interface{})
	resources := spec["resources"].(map[string]interface{})
	if resources["limits"] != "cpu=500m" {
		t.Errorf("unexpected: %v", resources["limits"])
	}
}

func TestSetNestedNormalized_CreatesIntermediateMaps(t *testing.T) {
	obj := map[string]interface{}{}
	// No "spec" key exists yet — must be created
	if err := setNestedNormalized(obj, "spec.schedule", "*/5 * * * *"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := obj["spec"]; !ok {
		t.Error("intermediate spec map must be created")
	}
}

func TestSetNestedNormalized_OverwritesExistingValue(t *testing.T) {
	obj := map[string]interface{}{
		"spec": map[string]interface{}{
			"replicas": int64(1),
		},
	}
	if err := setNestedNormalized(obj, "spec.replicas", int64(5)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	spec := obj["spec"].(map[string]interface{})
	if spec["replicas"] != int64(5) {
		t.Errorf("expected 5, got %v", spec["replicas"])
	}
}

func TestSetNestedNormalized_ErrorWhenIntermediateNotMap(t *testing.T) {
	obj := map[string]interface{}{
		"spec": "this-is-a-string", // not a map
	}
	err := setNestedNormalized(obj, "spec.replicas", int64(3))
	if err == nil {
		t.Error("expected error when intermediate segment is not a map")
	}
}

func TestSetNestedNormalized_EmptyPath_SilentlyWritesEmptyKey(t *testing.T) {
	// NOTE(logic): splitDotPath("") returns [""] — one empty segment — so the
	// len(parts)==0 guard in setNestedNormalized never triggers. Empty path
	// silently writes obj[""] = value. Tracked as a logic issue separately.
	obj := map[string]interface{}{}
	err := setNestedNormalized(obj, "", "value")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if obj[""] != "value" {
		t.Errorf("expected obj[\"\"]=\"value\", got %v", obj[""])
	}
}

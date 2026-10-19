// pkg/merger/parse_test.go
package merger

import (
	"testing"

	"github.com/intentd/inrun/pkg/config"
)

// ── containsValidKind ─────────────────────────────────────────────────────────

func TestContainsValidKind_Blueprint(t *testing.T) {
	doc := []byte("kind: " + config.BlueprintKind() + "\napiVersion: inrun.dev/v1\n")
	if !containsValidKind(doc) {
		t.Error("document with Blueprint kind must return true")
	}
}

func TestContainsValidKind_Stack(t *testing.T) {
	doc := []byte("kind: " + config.StackKind() + "\napiVersion: inrun.dev/v1\n")
	if !containsValidKind(doc) {
		t.Error("document with Stack kind must return true")
	}
}

func TestContainsValidKind_OtherKind(t *testing.T) {
	doc := []byte("kind: Deployment\napiVersion: apps/v1\n")
	if containsValidKind(doc) {
		t.Error("Deployment kind must return false")
	}
}

func TestContainsValidKind_Empty(t *testing.T) {
	if containsValidKind([]byte{}) {
		t.Error("empty document must return false")
	}
}

func TestContainsValidKind_KindInComment(t *testing.T) {
	// The word "Blueprint" appears in a comment — fast check will match.
	// This is known behaviour: the full parse handles it, not containsValidKind.
	doc := []byte("# kind: Blueprint\napiVersion: apps/v1\nkind: Deployment\n")
	// We don't assert here — the fast check can produce false positives.
	// The test documents the known limitation.
	_ = containsValidKind(doc)
}

// ── parseBlueprintDoc ───────────────────────────────────────────────────────────

func validBlueprintDoc() []byte {
	return []byte(`apiVersion: inrun.dev/v1
kind: Blueprint
metadata:
  name: test-blueprint
`)
}

func TestParseBlueprintDoc_Valid(t *testing.T) {
	kf, err := parseBlueprintDoc(validBlueprintDoc(), "test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if kf == nil {
		t.Fatal("expected non-nil BlueprintFile")
	}
	if kf.Metadata.Name != "test-blueprint" {
		t.Errorf("expected name=test-blueprint, got %q", kf.Metadata.Name)
	}
	if kf.Kind != config.BlueprintKind() {
		t.Errorf("expected kind=%s, got %q", config.BlueprintKind(), kf.Kind)
	}
}

func TestParseBlueprintDoc_NonBlueprintReturnsNil(t *testing.T) {
	doc := []byte("apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: foo\n")
	kf, err := parseBlueprintDoc(doc, "test")
	if err != nil {
		t.Fatalf("non-blueprint doc must not error: %v", err)
	}
	if kf != nil {
		t.Error("non-blueprint doc must return nil BlueprintFile")
	}
}

func TestParseBlueprintDoc_MissingName_Error(t *testing.T) {
	doc := []byte("apiVersion: inrun.dev/v1\nkind: Blueprint\nmetadata:\n  name: \"\"\n")
	_, err := parseBlueprintDoc(doc, "test")
	if err == nil {
		t.Error("missing metadata.name must return error")
	}
}

func TestParseBlueprintDoc_UnsupportedApiVersion_Error(t *testing.T) {
	doc := []byte("apiVersion: example.io/v999\nkind: Blueprint\nmetadata:\n  name: foo\n")
	_, err := parseBlueprintDoc(doc, "test")
	if err == nil {
		t.Error("unsupported apiVersion must return error")
	}
}

func TestParseBlueprintDoc_ValidStack(t *testing.T) {
	doc := []byte(`apiVersion: inrun.dev/v1
kind: Stack
metadata:
  name: my-stack
`)
	kf, err := parseBlueprintDoc(doc, "test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if kf == nil || kf.Kind != config.StackKind() {
		t.Errorf("expected Stack kind, got %v", kf)
	}
}

func TestParseBlueprintDoc_EmptyDoc(t *testing.T) {
	kf, err := parseBlueprintDoc([]byte{}, "empty")
	if err != nil {
		t.Fatalf("empty doc must not error: %v", err)
	}
	if kf != nil {
		t.Error("empty doc must return nil")
	}
}

func TestParseBlueprintDoc_UnknownField_Error(t *testing.T) {
	doc := []byte(`apiVersion: inrun.dev/v1
kind: Blueprint
metadata:
  name: test
unknownTopLevelField: boom
`)
	_, err := parseBlueprintDoc(doc, "test")
	if err == nil {
		t.Error("unknown field in strict parse must return error")
	}
}

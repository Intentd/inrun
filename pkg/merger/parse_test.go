// pkg/merger/parse_test.go
package merger

import (
	"testing"

	"github.com/intentd/inrun/pkg/config"
)

// ── containsValidKind ─────────────────────────────────────────────────────────

func TestContainsValidKind_Catalog(t *testing.T) {
	doc := []byte("kind: " + config.CatalogKind() + "\napiVersion: inrun.dev/v1\n")
	if !containsValidKind(doc) {
		t.Error("document with Catalog kind must return true")
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
	// The word "Catalog" appears in a comment — fast check will match.
	// This is known behaviour: the full parse handles it, not containsValidKind.
	doc := []byte("# kind: Catalog\napiVersion: apps/v1\nkind: Deployment\n")
	// We don't assert here — the fast check can produce false positives.
	// The test documents the known limitation.
	_ = containsValidKind(doc)
}

// ── parseCatalogDoc ───────────────────────────────────────────────────────────

func validCatalogDoc() []byte {
	return []byte(`apiVersion: inrun.dev/v1
kind: Catalog
metadata:
  name: test-catalog
`)
}

func TestParseCatalogDoc_Valid(t *testing.T) {
	kf, err := parseCatalogDoc(validCatalogDoc(), "test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if kf == nil {
		t.Fatal("expected non-nil CatalogFile")
	}
	if kf.Metadata.Name != "test-catalog" {
		t.Errorf("expected name=test-catalog, got %q", kf.Metadata.Name)
	}
	if kf.Kind != config.CatalogKind() {
		t.Errorf("expected kind=%s, got %q", config.CatalogKind(), kf.Kind)
	}
}

func TestParseCatalogDoc_NonCatalogReturnsNil(t *testing.T) {
	doc := []byte("apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: foo\n")
	kf, err := parseCatalogDoc(doc, "test")
	if err != nil {
		t.Fatalf("non-catalog doc must not error: %v", err)
	}
	if kf != nil {
		t.Error("non-catalog doc must return nil CatalogFile")
	}
}

func TestParseCatalogDoc_MissingName_Error(t *testing.T) {
	doc := []byte("apiVersion: inrun.dev/v1\nkind: Catalog\nmetadata:\n  name: \"\"\n")
	_, err := parseCatalogDoc(doc, "test")
	if err == nil {
		t.Error("missing metadata.name must return error")
	}
}

func TestParseCatalogDoc_UnsupportedApiVersion_Error(t *testing.T) {
	doc := []byte("apiVersion: example.io/v999\nkind: Catalog\nmetadata:\n  name: foo\n")
	_, err := parseCatalogDoc(doc, "test")
	if err == nil {
		t.Error("unsupported apiVersion must return error")
	}
}

func TestParseCatalogDoc_ValidStack(t *testing.T) {
	doc := []byte(`apiVersion: inrun.dev/v1
kind: Stack
metadata:
  name: my-stack
`)
	kf, err := parseCatalogDoc(doc, "test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if kf == nil || kf.Kind != config.StackKind() {
		t.Errorf("expected Stack kind, got %v", kf)
	}
}

func TestParseCatalogDoc_EmptyDoc(t *testing.T) {
	kf, err := parseCatalogDoc([]byte{}, "empty")
	if err != nil {
		t.Fatalf("empty doc must not error: %v", err)
	}
	if kf != nil {
		t.Error("empty doc must return nil")
	}
}

func TestParseCatalogDoc_UnknownField_Error(t *testing.T) {
	doc := []byte(`apiVersion: inrun.dev/v1
kind: Catalog
metadata:
  name: test
unknownTopLevelField: boom
`)
	_, err := parseCatalogDoc(doc, "test")
	if err == nil {
		t.Error("unknown field in strict parse must return error")
	}
}

package registry

import (
	"os"
	"path/filepath"
	"testing"
)

// ── annotation round-trip ─────────────────────────────────────────────────────

func TestAnnotationRoundTrip_Typed(t *testing.T) {
	meta := &PatternMeta{
		Kind:    BlueprintKind,
		Name:    "postgres",
		Version: "v1",
		Typed: &PatternTyped{
			HasHooks:       true,
			HasConstructor: false,
		},
	}

	ann := artifactMetaToAnnotations(meta)

	if ann["io.inrun.blueprint.typed"] != "true" {
		t.Errorf("typed annotation not set")
	}
	if ann["io.inrun.blueprint.has_hooks"] != "true" {
		t.Errorf("has_hooks annotation not set")
	}
	if ann["io.inrun.blueprint.has_constructor"] == "true" {
		t.Errorf("has_constructor should not be set when HasConstructor=false")
	}

	got := annotationsToMeta(ann)
	if got.Typed == nil {
		t.Fatal("Typed is nil after round-trip")
	}
	if !got.Typed.HasHooks {
		t.Errorf("HasHooks lost in round-trip")
	}
	if got.Typed.HasConstructor {
		t.Errorf("HasConstructor should be false after round-trip")
	}
}

func TestAnnotationRoundTrip_TypedBoth(t *testing.T) {
	meta := &PatternMeta{
		Kind:    BlueprintKind,
		Name:    "redis",
		Version: "v2",
		Typed: &PatternTyped{
			HasHooks:       true,
			HasConstructor: true,
		},
	}

	ann := artifactMetaToAnnotations(meta)
	got := annotationsToMeta(ann)

	if got.Typed == nil {
		t.Fatal("Typed is nil after round-trip")
	}
	if !got.Typed.HasHooks || !got.Typed.HasConstructor {
		t.Errorf("Typed fields lost: hooks=%v constructor=%v", got.Typed.HasHooks, got.Typed.HasConstructor)
	}
}

func TestAnnotationRoundTrip_Deprecated(t *testing.T) {
	meta := &PatternMeta{
		Kind:    BlueprintKind,
		Name:    "old-postgres",
		Version: "v1",
		Deprecated: &PatternDeprecated{
			MigratedTo: "ghcr.io/test/patterns/postgres:v14",
			Message:    "Use postgres:v14 — supports logical replication",
		},
	}

	ann := artifactMetaToAnnotations(meta)

	if ann["io.inrun.blueprint.deprecated"] != "true" {
		t.Errorf("deprecated annotation not set")
	}
	if ann["io.inrun.blueprint.deprecated.migrated_to"] != meta.Deprecated.MigratedTo {
		t.Errorf("migrated_to = %q; want %q", ann["io.inrun.blueprint.deprecated.migrated_to"], meta.Deprecated.MigratedTo)
	}
	if ann["io.inrun.blueprint.deprecated.message"] != meta.Deprecated.Message {
		t.Errorf("message = %q; want %q", ann["io.inrun.blueprint.deprecated.message"], meta.Deprecated.Message)
	}

	got := annotationsToMeta(ann)
	if got.Deprecated == nil {
		t.Fatal("Deprecated is nil after round-trip")
	}
	if got.Deprecated.MigratedTo != meta.Deprecated.MigratedTo {
		t.Errorf("MigratedTo lost: got %q", got.Deprecated.MigratedTo)
	}
	if got.Deprecated.Message != meta.Deprecated.Message {
		t.Errorf("Message lost: got %q", got.Deprecated.Message)
	}
}

func TestAnnotationRoundTrip_NoTypedNoDeprecated(t *testing.T) {
	meta := &PatternMeta{
		Kind:    BlueprintKind,
		Name:    "plain",
		Version: "v1",
	}

	ann := artifactMetaToAnnotations(meta)
	got := annotationsToMeta(ann)

	if got.Typed != nil {
		t.Errorf("Typed should be nil for plain blueprint")
	}
	if got.Deprecated != nil {
		t.Errorf("Deprecated should be nil for plain blueprint")
	}
}

// ── fixture: testdata/blueprint.yaml ───────────────────────────────────────────

// TestLoadPatternMeta_Fixture reads the testdata blueprint and asserts that
// metadata.deprecation and metadata.name are parsed correctly.
func TestLoadPatternMeta_Fixture(t *testing.T) {
	dir := "testdata"
	spec := &PatternSpec{Kind: BlueprintKind, PrimaryFile: FileBlueprint}

	meta, err := LoadPatternMeta(dir, spec)
	if err != nil {
		t.Fatalf("LoadPatternMeta: %v", err)
	}

	if meta.Name != "registry-pack-probe" {
		t.Errorf("Name = %q; want registry-pack-probe", meta.Name)
	}
	if meta.Deprecated == nil {
		t.Fatal("Deprecated is nil — fixture has lifecycle.deprecation")
	}
	if meta.Deprecated.MigratedTo == "" {
		t.Errorf("MigratedTo is empty")
	}
	if meta.Deprecated.Message == "" {
		t.Errorf("Message is empty")
	}
}

// ── LoadPatternMeta reads deprecation from YAML ───────────────────────────────

func TestLoadPatternMeta_Deprecation(t *testing.T) {
	dir := t.TempDir()
	blueprintYAML := `
kind: Blueprint
metadata:
  name: old-redis
  version: v6
  description: "Legacy Redis operator"
lifecycle:
  deprecation:
    migratedTo: ghcr.io/test/patterns/redis:v7
    message: "Upgrade to v7 — adds TLS support"
spec:
  crds: {}
`
	if err := os.WriteFile(filepath.Join(dir, FileBlueprint), []byte(blueprintYAML), 0o644); err != nil {
		t.Fatalf("writing test blueprint: %v", err)
	}

	spec := &PatternSpec{Kind: BlueprintKind, PrimaryFile: FileBlueprint}
	meta, err := LoadPatternMeta(dir, spec)
	if err != nil {
		t.Fatalf("LoadPatternMeta: %v", err)
	}

	if meta.Deprecated == nil {
		t.Fatal("Deprecated is nil — deprecation: block not read")
	}
	if meta.Deprecated.MigratedTo != "ghcr.io/test/patterns/redis:v7" {
		t.Errorf("MigratedTo = %q; want ghcr.io/test/patterns/redis:v7", meta.Deprecated.MigratedTo)
	}
	if meta.Deprecated.Message != "Upgrade to v7 — adds TLS support" {
		t.Errorf("Message = %q", meta.Deprecated.Message)
	}
}

func TestLoadPatternMeta_NoDeprecation(t *testing.T) {
	dir := t.TempDir()
	blueprintYAML := `
kind: Blueprint
metadata:
  name: postgres
  version: v14
spec:
  crds: {}
`
	if err := os.WriteFile(filepath.Join(dir, FileBlueprint), []byte(blueprintYAML), 0o644); err != nil {
		t.Fatalf("writing test blueprint: %v", err)
	}

	spec := &PatternSpec{Kind: BlueprintKind, PrimaryFile: FileBlueprint}
	meta, err := LoadPatternMeta(dir, spec)
	if err != nil {
		t.Fatalf("LoadPatternMeta: %v", err)
	}

	if meta.Deprecated != nil {
		t.Errorf("Deprecated should be nil for a non-deprecated pattern")
	}
}

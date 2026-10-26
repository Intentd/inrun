// pkg/generate/pure_funcs_test.go
package generate

import (
	"testing"
)

// ── inferTypeFromValue ────────────────────────────────────────────────────────

// ── trimSpecPrefix ────────────────────────────────────────────────────────────

// ── placeholderFor ────────────────────────────────────────────────────────────

// ── toTitle ───────────────────────────────────────────────────────────────────

// ── extractSpecPaths ──────────────────────────────────────────────────────────

// ── resolveAlias ──────────────────────────────────────────────────────────────

func TestResolveAlias_ExplicitAlias_ReturnsIt(t *testing.T) {
	got := resolveAlias("myalias", "prefix", "github.com/org/pkg/v1")
	if got != "myalias" {
		t.Errorf("expected explicit alias, got %q", got)
	}
}

func TestResolveAlias_NoExplicit_DerivesTwoSegments(t *testing.T) {
	got := resolveAlias("", "", "github.com/myorg/apis/project/v1alpha1")
	// last two parts: project + v1alpha1 → "projectv1alpha1"
	if got != "projectv1alpha1" {
		t.Errorf("expected projectv1alpha1, got %q", got)
	}
}

func TestResolveAlias_WithPrefix(t *testing.T) {
	got := resolveAlias("", "gen", "github.com/myorg/hooks")
	// last two parts: myorg + hooks → "myorghooks", with prefix "gen"
	if got != "genmyorghooks" {
		t.Errorf("expected genmyorghooks, got %q", got)
	}
}

func TestResolveAlias_DotInPath_Sanitized(t *testing.T) {
	got := resolveAlias("", "", "github.com/myorg/apis/v1.2.3")
	// dots removed in sanitization
	if got == "" {
		t.Error("expected non-empty alias")
	}
	// Should not contain dots
	for _, c := range got {
		if c == '.' {
			t.Errorf("alias must not contain dots, got %q", got)
		}
	}
}

// ── toRawExtension ────────────────────────────────────────────────────────────

// ── dedupeImport ─────────────────────────────────────────────────────────────

func TestDedupeImport_NewAlias_NoError(t *testing.T) {
	seen := map[string]string{}
	if err := dedupeImport(seen, "myalias", "github.com/org/pkg", "website"); err != nil {
		t.Errorf("first occurrence must not error: %v", err)
	}
}

func TestDedupeImport_SameAliasAndLocation_NoError(t *testing.T) {
	seen := map[string]string{"myalias": "github.com/org/pkg"}
	if err := dedupeImport(seen, "myalias", "github.com/org/pkg", "website"); err != nil {
		t.Errorf("same alias+location must not error: %v", err)
	}
}

func TestDedupeImport_SameAliasDifferentLocation_Error(t *testing.T) {
	seen := map[string]string{"myalias": "github.com/org/pkg-a"}
	if err := dedupeImport(seen, "myalias", "github.com/org/pkg-b", "website"); err == nil {
		t.Error("conflicting alias must return error")
	}
}

package catalog_test

import (
	"testing"
)

// ── serve.include ───────────────────────────────────────────────────────────────

func TestServeInclude_ExpandsFields(t *testing.T) {
	cat := mustParseTestdata(t, "include/serve.yaml")
	crd, _ := cat.Get("platform")
	if crd == nil {
		t.Fatal("platform CRD not found")
	}
	if crd.Serve == nil {
		t.Fatal("Serve is nil")
	}
	// Included fields
	if _, ok := crd.Serve.Fields["team"]; !ok {
		t.Error("included field 'team' missing")
	}
	if _, ok := crd.Serve.Fields["environment"]; !ok {
		t.Error("included field 'environment' missing")
	}
	// Inline field merged on top
	if _, ok := crd.Serve.Fields["image"]; !ok {
		t.Error("inline field 'image' missing")
	}
}

func TestServeInclude_InlineOverridesIncluded(t *testing.T) {
	cat := mustParseTestdata(t, "include/serve.yaml")
	crd, _ := cat.Get("platform")
	if crd == nil || crd.Serve == nil {
		t.Fatal("platform CRD or Serve is nil")
	}
	// 'team' from include has order 1; if an inline field with the same name
	// were present it would win. Here 'image' is inline-only with order 3.
	if f, ok := crd.Serve.Fields["image"]; !ok || f.Order != 3 {
		t.Errorf("inline 'image' field: got order %d, want 3", crd.Serve.Fields["image"].Order)
	}
}

func TestServeInclude_ClearedAfterExpansion(t *testing.T) {
	cat := mustParseTestdata(t, "include/serve.yaml")
	crd, _ := cat.Get("platform")
	if crd == nil || crd.Serve == nil {
		t.Fatal("platform CRD or Serve is nil")
	}
	if crd.Serve.Include != "" {
		t.Errorf("Serve.Include not cleared after expansion, got %q", crd.Serve.Include)
	}
}

// ── validation.include ────────────────────────────────────────────────────────

func TestValidationInclude_ExpandsRules(t *testing.T) {
	cat := mustParseTestdata(t, "include/validation.yaml")
	crd, _ := cat.Get("platform")
	if crd == nil || crd.EffectiveValidation() == nil {
		t.Fatal("platform CRD or Validation is nil")
	}
	// 2 included + 1 inline = 3 total
	if len(crd.EffectiveValidation().Rules) != 3 {
		t.Errorf("len(Rules) = %d, want 3", len(crd.EffectiveValidation().Rules))
	}
}

func TestValidationInclude_IncludedRulesFirst(t *testing.T) {
	cat := mustParseTestdata(t, "include/validation.yaml")
	crd, _ := cat.Get("platform")
	if crd == nil || crd.EffectiveValidation() == nil {
		t.Fatal("platform CRD or Validation is nil")
	}
	// First rule must be the included one (spec.team exists)
	if got := crd.EffectiveValidation().Rules[0].Field; got != "spec.team" {
		t.Errorf("Rules[0].Field = %q, want %q", got, "spec.team")
	}
	// Last rule is the inline one (spec.replicas lte)
	last := crd.EffectiveValidation().Rules[len(crd.EffectiveValidation().Rules)-1]
	if last.Field != "spec.replicas" {
		t.Errorf("last rule Field = %q, want %q", last.Field, "spec.replicas")
	}
}

func TestValidationInclude_ClearedAfterExpansion(t *testing.T) {
	cat := mustParseTestdata(t, "include/validation.yaml")
	crd, _ := cat.Get("platform")
	if crd == nil || crd.EffectiveValidation() == nil {
		t.Fatal("platform CRD or Validation is nil")
	}
	if crd.EffectiveValidation().Include != "" {
		t.Errorf("Validation.Include not cleared after expansion, got %q", crd.EffectiveValidation().Include)
	}
}

// ── mutation.include ──────────────────────────────────────────────────────────

func TestMutationInclude_ExpandsRules(t *testing.T) {
	cat := mustParseTestdata(t, "include/mutation.yaml")
	crd, _ := cat.Get("platform")
	if crd == nil || crd.EffectiveMutation() == nil {
		t.Fatal("platform CRD or Mutation is nil")
	}
	// 2 included + 1 inline = 3 total
	if len(crd.EffectiveMutation().Rules) != 3 {
		t.Errorf("len(Rules) = %d, want 3", len(crd.EffectiveMutation().Rules))
	}
}

func TestMutationInclude_IncludedRulesFirst(t *testing.T) {
	cat := mustParseTestdata(t, "include/mutation.yaml")
	crd, _ := cat.Get("platform")
	if crd == nil || crd.EffectiveMutation() == nil {
		t.Fatal("platform CRD or Mutation is nil")
	}
	// First rule must be the included one (spec.replicas default)
	if got := crd.EffectiveMutation().Rules[0].Field; got != "spec.replicas" {
		t.Errorf("Rules[0].Field = %q, want %q", got, "spec.replicas")
	}
	// Last rule is the inline one (spec.logLevel default)
	last := crd.EffectiveMutation().Rules[len(crd.EffectiveMutation().Rules)-1]
	if last.Field != "spec.logLevel" {
		t.Errorf("last rule Field = %q, want %q", last.Field, "spec.logLevel")
	}
}

func TestMutationInclude_ClearedAfterExpansion(t *testing.T) {
	cat := mustParseTestdata(t, "include/mutation.yaml")
	crd, _ := cat.Get("platform")
	if crd == nil || crd.EffectiveMutation() == nil {
		t.Fatal("platform CRD or Mutation is nil")
	}
	if crd.EffectiveMutation().Include != "" {
		t.Errorf("Mutation.Include not cleared after expansion, got %q", crd.EffectiveMutation().Include)
	}
}

// ── conversion.include ────────────────────────────────────────────────────────

func TestConversionInclude_ExpandsPaths(t *testing.T) {
	cat := mustParseTestdata(t, "include/conversion.yaml")
	crd, _ := cat.Get("platform")
	if crd == nil || crd.EffectiveConversion() == nil {
		t.Fatal("platform CRD or Conversion is nil")
	}
	// 2 included + 1 inline = 3 total
	if len(crd.EffectiveConversion().Paths) != 3 {
		t.Errorf("len(Paths) = %d, want 3", len(crd.EffectiveConversion().Paths))
	}
}

func TestConversionInclude_IncludedPathsFirst(t *testing.T) {
	cat := mustParseTestdata(t, "include/conversion.yaml")
	crd, _ := cat.Get("platform")
	if crd == nil || crd.EffectiveConversion() == nil {
		t.Fatal("platform CRD or Conversion is nil")
	}
	// First path is from the include file (v1alpha1 → v1)
	if got := crd.EffectiveConversion().Paths[0].From; got != "v1alpha1" {
		t.Errorf("Paths[0].From = %q, want %q", got, "v1alpha1")
	}
	// Last path is the inline one (v1 → v1alpha1)
	last := crd.EffectiveConversion().Paths[len(crd.EffectiveConversion().Paths)-1]
	if last.From != "v1" {
		t.Errorf("last path From = %q, want %q", last.From, "v1")
	}
}

func TestConversionInclude_ClearedAfterExpansion(t *testing.T) {
	cat := mustParseTestdata(t, "include/conversion.yaml")
	crd, _ := cat.Get("platform")
	if crd == nil || crd.EffectiveConversion() == nil {
		t.Fatal("platform CRD or Conversion is nil")
	}
	if crd.EffectiveConversion().Include != "" {
		t.Errorf("Conversion.Include not cleared after expansion, got %q", crd.EffectiveConversion().Include)
	}
}

// ── status.include ────────────────────────────────────────────────────────────

func TestStatusInclude_ExpandsFields(t *testing.T) {
	cat := mustParseTestdata(t, "include/status.yaml")
	crd, _ := cat.Get("platform")
	if crd == nil || crd.OperatorBox.EffectiveStatus() == nil {
		t.Fatal("platform CRD or Status is nil")
	}
	// 2 included + 1 inline = 3 total
	if len(crd.OperatorBox.EffectiveStatus().Fields) != 3 {
		t.Errorf("len(Fields) = %d, want 3", len(crd.OperatorBox.EffectiveStatus().Fields))
	}
}

func TestStatusInclude_IncludedFieldsFirst(t *testing.T) {
	cat := mustParseTestdata(t, "include/status.yaml")
	crd, _ := cat.Get("platform")
	if crd == nil || crd.OperatorBox.EffectiveStatus() == nil {
		t.Fatal("platform CRD or Status is nil")
	}
	// First field is the included one (phase)
	if got := crd.OperatorBox.EffectiveStatus().Fields[0].Path; got != "phase" {
		t.Errorf("Fields[0].Path = %q, want %q", got, "phase")
	}
	// Last field is the inline one (environment)
	last := crd.OperatorBox.EffectiveStatus().Fields[len(crd.OperatorBox.EffectiveStatus().Fields)-1]
	if last.Path != "environment" {
		t.Errorf("last field Path = %q, want %q", last.Path, "environment")
	}
}

func TestStatusInclude_ClearedAfterExpansion(t *testing.T) {
	cat := mustParseTestdata(t, "include/status.yaml")
	crd, _ := cat.Get("platform")
	if crd == nil || crd.OperatorBox.EffectiveStatus() == nil {
		t.Fatal("platform CRD or Status is nil")
	}
	if crd.OperatorBox.EffectiveStatus().Include != "" {
		t.Errorf("Status.Include not cleared after expansion, got %q", crd.OperatorBox.EffectiveStatus().Include)
	}
}

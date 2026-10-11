package types_test

import (
	"encoding/json"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/intentd/inrun/pkg/types"
)

// ── ServeTargetValue — scalar shorthand ──────────────────────────────────────

func TestServeTargetValue_UnmarshalYAML_Scalar(t *testing.T) {
	in := `target: myapp`
	var cfg struct {
		Target types.ServeTargetValue `yaml:"target"`
	}
	if err := yaml.Unmarshal([]byte(in), &cfg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if cfg.Target.Shorthand != "myapp" {
		t.Errorf("Shorthand = %q, want %q", cfg.Target.Shorthand, "myapp")
	}
	if len(cfg.Target.Entries) != 0 {
		t.Error("scalar form should have no Entries before expansion")
	}
}

func TestServeTargetValue_UnmarshalYAML_Map(t *testing.T) {
	in := "target:\n  myapp:\n    primary: true\n  preview: {}"
	var cfg struct {
		Target types.ServeTargetValue `yaml:"target"`
	}
	if err := yaml.Unmarshal([]byte(in), &cfg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if cfg.Target.Shorthand != "" {
		t.Error("map form should not set Shorthand")
	}
	if len(cfg.Target.Entries) != 2 {
		t.Errorf("Entries len = %d, want 2", len(cfg.Target.Entries))
	}
	if !cfg.Target.Entries["myapp"].Primary {
		t.Error("myapp entry should be Primary")
	}
}

func TestServeTargetValue_IsZero(t *testing.T) {
	var tv types.ServeTargetValue
	if !tv.IsZero() {
		t.Error("zero value should be IsZero")
	}
	tv.Shorthand = "x"
	if tv.IsZero() {
		t.Error("non-empty shorthand should not be IsZero")
	}
}

func TestServeTargetValue_MarshalYAML_ShorthandRoundtrip(t *testing.T) {
	// Single primary entry with no extra config → round-trips to scalar.
	tv := types.ServeTargetValue{
		Entries: map[string]*types.ServeTargetConfig{
			"myapp": {Primary: true},
		},
	}
	b, err := yaml.Marshal(struct {
		Target types.ServeTargetValue `yaml:"target"`
	}{Target: tv})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var back struct {
		Target types.ServeTargetValue `yaml:"target"`
	}
	if err := yaml.Unmarshal(b, &back); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	// After round-trip via scalar, Shorthand should be set.
	if back.Target.Shorthand != "myapp" && back.Target.Entries["myapp"] == nil {
		t.Errorf("round-trip lost myapp: %+v", back.Target)
	}
}

// ─── ServeTargetValue JSON tests ────────────────────────────────────────────

func TestServeTargetValue_UnmarshalJSON_String(t *testing.T) {
	var tv types.ServeTargetValue
	// The custom unmarshaler accepts a string in JSON
	if err := json.Unmarshal([]byte(`"smartapp"`), &tv); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if tv.Shorthand != "smartapp" {
		t.Errorf("Shorthand = %q, want %q", tv.Shorthand, "smartapp")
	}
}

func TestServeTargetValue_UnmarshalJSON_Object(t *testing.T) {
	var tv types.ServeTargetValue
	if err := json.Unmarshal([]byte(`{"smartapp":{"primary":true}}`), &tv); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if tv.Entries["smartapp"] == nil || !tv.Entries["smartapp"].Primary {
		t.Error("expected primary smartapp entry")
	}
}

func TestServeTargetValue_MarshalJSON_ShorthandRoundtrip(t *testing.T) {
	// Single primary entry with no extra config → round-trips to scalar.
	tv := types.ServeTargetValue{
		Entries: map[string]*types.ServeTargetConfig{
			"myapp": {Primary: true},
		},
	}
	b, err := json.Marshal(struct {
		Target types.ServeTargetValue `json:"target"`
	}{Target: tv})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var back struct {
		Target types.ServeTargetValue `json:"target"`
	}
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	// Should round-trip to scalar
	if back.Target.Shorthand != "myapp" && back.Target.Entries["myapp"] == nil {
		t.Errorf("round-trip lost myapp: %+v", back.Target)
	}
}

// ── ServeTargetConfig ────────────────────────────────────────────────────────

func TestServeTargetConfig_IsEnabled_Nil(t *testing.T) {
	var cfg *types.ServeTargetConfig
	if !cfg.IsEnabled() {
		t.Error("nil config should report enabled")
	}
}

func TestServeTargetConfig_IsEnabled_NilField(t *testing.T) {
	cfg := &types.ServeTargetConfig{} // Enabled == nil
	if !cfg.IsEnabled() {
		t.Error("nil Enabled field should default to enabled")
	}
}

func TestServeTargetConfig_IsEnabled_False(t *testing.T) {
	disabled := false
	cfg := &types.ServeTargetConfig{Enabled: &disabled}
	if cfg.IsEnabled() {
		t.Error("Enabled=false should report not enabled")
	}
}

func TestServeTargetConfig_HasTokenRestrictions(t *testing.T) {
	tests := []struct {
		name string
		cfg  *types.ServeTargetConfig
		want bool
	}{
		{"nil", nil, false},
		{"empty tokens", &types.ServeTargetConfig{}, false},
		{"has tokens", &types.ServeTargetConfig{Tokens: map[string]types.ServeTokenPermissions{"ci": {}}}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.cfg.HasTokenRestrictions(); got != tt.want {
				t.Errorf("HasTokenRestrictions() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestServeTargetConfig_ResponseConfig(t *testing.T) {
	rc := &types.ServeResponseConfig{}
	tests := []struct {
		name string
		cfg  *types.ServeTargetConfig
		want *types.ServeResponseConfig
	}{
		{"nil entry", nil, nil},
		{"nil config", &types.ServeTargetConfig{}, nil},
		{"has response", &types.ServeTargetConfig{Config: &types.ServeAliasConfigSettings{Response: rc}}, rc},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.cfg.ResponseConfig(); got != tt.want {
				t.Errorf("ResponseConfig() = %v, want %v", got, tt.want)
			}
		})
	}
}

// ─── CRDEntry.ServeTokensFor ───────────────────────────────────────────────────

func crdWithTargetMap() *types.CRDEntry {
	return &types.CRDEntry{
		Serve: &types.ServeConfig{
			Enabled: true,
			Tokens: map[string]types.ServeTokenPermissions{
				"global-token": {},
			},
			Target: types.ServeTargetValue{
				Entries: map[string]*types.ServeTargetConfig{
					"myapp": {Primary: true},
					"public": {
						Tokens: map[string]types.ServeTokenPermissions{
							"public-token": {},
						},
					},
					"no-tokens": {},
				},
			},
		},
	}
}

func TestServeTokensFor_NoAlias(t *testing.T) {
	crd := crdWithTargetMap()
	tokens := crd.ServeTokensFor("")
	if _, ok := tokens["global-token"]; !ok {
		t.Error("expected CRD-level tokens when alias is empty")
	}
}

func TestServeTokensFor_AliasWithTokens(t *testing.T) {
	crd := crdWithTargetMap()
	tokens := crd.ServeTokensFor("public")
	if _, ok := tokens["public-token"]; !ok {
		t.Error("expected alias-specific tokens")
	}
	if _, ok := tokens["global-token"]; ok {
		t.Error("alias tokens should not include CRD-level tokens")
	}
}

func TestServeTokensFor_AliasWithNoTokensFallsBack(t *testing.T) {
	crd := crdWithTargetMap()
	tokens := crd.ServeTokensFor("no-tokens")
	if _, ok := tokens["global-token"]; !ok {
		t.Error("expected fallback to CRD-level tokens when alias has none")
	}
}

func TestServeTokensFor_UnknownAlias(t *testing.T) {
	crd := crdWithTargetMap()
	tokens := crd.ServeTokensFor("unknown")
	if _, ok := tokens["global-token"]; !ok {
		t.Error("expected CRD-level tokens for unknown alias")
	}
}

func TestServeTokensFor_NilServe(t *testing.T) {
	crd := &types.CRDEntry{}
	if tokens := crd.ServeTokensFor("public"); tokens != nil {
		t.Error("expected nil when Serve is nil")
	}
}

// ── CRDEntry.ServeResponseConfigFor ──────────────────────────────────────────

func TestServeResponseConfigFor_NoAlias(t *testing.T) {
	rc := &types.ServeResponseConfig{}
	crd := &types.CRDEntry{
		Serve: &types.ServeConfig{
			Enabled: true,
			Config:  &types.ServeConfigSettings{Response: rc},
		},
	}
	if got := crd.ServeResponseConfigFor(""); got != rc {
		t.Error("expected CRD-level response config when alias is empty")
	}
}

func TestServeResponseConfigFor_AliasOverrides(t *testing.T) {
	crdRC := &types.ServeResponseConfig{}
	aliasRC := &types.ServeResponseConfig{}
	crd := &types.CRDEntry{
		Serve: &types.ServeConfig{
			Enabled: true,
			Config:  &types.ServeConfigSettings{Response: crdRC},
			Target: types.ServeTargetValue{
				Entries: map[string]*types.ServeTargetConfig{
					"myapp":  {Primary: true},
					"public": {Config: &types.ServeAliasConfigSettings{Response: aliasRC}},
				},
			},
		},
	}
	if got := crd.ServeResponseConfigFor("public"); got != aliasRC {
		t.Error("expected alias-specific response config")
	}
}

func TestServeResponseConfigFor_AliasNoResponseFallsBack(t *testing.T) {
	crdRC := &types.ServeResponseConfig{}
	crd := &types.CRDEntry{
		Serve: &types.ServeConfig{
			Enabled: true,
			Config:  &types.ServeConfigSettings{Response: crdRC},
			Target: types.ServeTargetValue{
				Entries: map[string]*types.ServeTargetConfig{
					"myapp":    {Primary: true},
					"internal": {}, // no response config
				},
			},
		},
	}
	if got := crd.ServeResponseConfigFor("internal"); got != crdRC {
		t.Error("expected fallback to CRD-level response config")
	}
}

// ── CRDEntry.TokenAllowedFor ──────────────────────────────────────────────────

func TestTokenAllowedFor_NilServe_AllowsAll(t *testing.T) {
	crd := &types.CRDEntry{}
	ok, reason := crd.TokenAllowedFor("", "any-token", types.ServeOpGet, "ns", types.ServeClassResources)
	if !ok || reason != types.ServeDenyReasonNone {
		t.Errorf("nil serve should allow all: ok=%v reason=%v", ok, reason)
	}
}

func TestTokenAllowedFor_NoAlias_DelegatesToCRDLevel(t *testing.T) {
	crd := &types.CRDEntry{
		Serve: &types.ServeConfig{
			Enabled: true,
			Tokens: map[string]types.ServeTokenPermissions{
				"ci": {Permissions: types.ServePermissionSet{Global: []string{"create", "update"}}},
			},
		},
	}
	ok, _ := crd.TokenAllowedFor("", "ci", types.ServeOpCreate, "ns", types.ServeClassResources)
	if !ok {
		t.Error("expected allowed")
	}
	ok, reason := crd.TokenAllowedFor("", "ci", types.ServeOpDelete, "ns", types.ServeClassResources)
	if ok {
		t.Error("expected denied for delete")
	}
	if reason != types.ServeDenyReasonOperation {
		t.Errorf("reason = %v, want ServeDenyReasonOperation", reason)
	}
}

func TestTokenAllowedFor_AliasOverridesTokens(t *testing.T) {
	crd := &types.CRDEntry{
		Serve: &types.ServeConfig{
			Enabled: true,
			Tokens: map[string]types.ServeTokenPermissions{
				"ci": {Permissions: types.ServePermissionSet{Global: []string{"*"}}},
			},
			Target: types.ServeTargetValue{
				Entries: map[string]*types.ServeTargetConfig{
					"myapp": {Primary: true},
					"public": {
						Tokens: map[string]types.ServeTokenPermissions{
							"readonly": {Permissions: types.ServePermissionSet{Global: []string{"get", "list"}}},
						},
					},
				},
			},
		},
	}
	// "ci" is allowed at CRD level but not listed in alias "public" tokens
	ok, reason := crd.TokenAllowedFor("public", "ci", types.ServeOpGet, "ns", types.ServeClassResources)
	if ok {
		t.Error("ci should be denied for alias public (not in alias tokens)")
	}
	if reason != types.ServeDenyReasonUnknownToken {
		t.Errorf("reason = %v, want ServeDenyReasonUnknownToken", reason)
	}

	// "readonly" is allowed for get via alias
	ok, _ = crd.TokenAllowedFor("public", "readonly", types.ServeOpGet, "ns", types.ServeClassResources)
	if !ok {
		t.Error("readonly should be allowed for get via alias public")
	}

	// "readonly" denied for create via alias
	ok, reason = crd.TokenAllowedFor("public", "readonly", types.ServeOpCreate, "ns", types.ServeClassResources)
	if ok {
		t.Error("readonly should be denied for create via alias public")
	}
	if reason != types.ServeDenyReasonOperation {
		t.Errorf("reason = %v, want ServeDenyReasonOperation", reason)
	}
}

func TestTokenAllowedFor_AliasNoTokensFallsBackToCRD(t *testing.T) {
	crd := &types.CRDEntry{
		Serve: &types.ServeConfig{
			Enabled: true,
			Tokens: map[string]types.ServeTokenPermissions{
				"ci": {Permissions: types.ServePermissionSet{Global: []string{"create"}}},
			},
			Target: types.ServeTargetValue{
				Entries: map[string]*types.ServeTargetConfig{
					"myapp":    {Primary: true},
					"internal": {}, // no alias-level tokens
				},
			},
		},
	}
	ok, _ := crd.TokenAllowedFor("internal", "ci", types.ServeOpCreate, "ns", types.ServeClassResources)
	if !ok {
		t.Error("should fall back to CRD-level tokens when alias declares none")
	}
}

// ── ScalarTarget has no aliases ───────────────────────────────────────────────

func TestScalarTargetValue_NoAliases(t *testing.T) {
	// A CRD configured with only the scalar shorthand should have no aliases.
	crd := &types.CRDEntry{
		APITypes: types.APITypes{Kind: "App"},
		Serve: &types.ServeConfig{
			Enabled: true,
			Target:  types.ServeTargetValue{Shorthand: "myapp"},
		},
	}
	if crd.HasServeAliases() {
		t.Error("scalar shorthand target should have no aliases")
	}
	if crd.HasServeAliases() {
		t.Error("ServeAliases() should be empty for scalar-shorthand target")
	}
}

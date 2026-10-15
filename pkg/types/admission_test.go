// Tests for the validation and mutation type definitions in admission.go.
//
// These types form the public contract between the Catalog schema and the
// admission/reconcile enforcement engine. Tests verify that default behaviours
// and policy helper methods match the documented semantics.
package types_test

import (
	"testing"

	"github.com/intentd/inrun/pkg/types"
	"github.com/stretchr/testify/assert"
)

// ── EffectiveAction ───────────────────────────────────────────────────────────

func TestEffectiveAction_EmptyDefaultsToDeny(t *testing.T) {
	// Omitting action in a Catalog rule must default to deny — new rules
	// should block by default so that adding a rule is always safe.
	assert.Equal(t, types.ValidationActionDeny, types.EffectiveAction(""))
}

func TestEffectiveAction_ExplicitDenyPassthrough(t *testing.T) {
	assert.Equal(t, types.ValidationActionDeny, types.EffectiveAction(types.ValidationActionDeny))
}

func TestEffectiveAction_ExplicitWarnPassthrough(t *testing.T) {
	assert.Equal(t, types.ValidationActionWarn, types.EffectiveAction(types.ValidationActionWarn))
}

// ── ValidationAction.IsDeny / IsWarn ─────────────────────────────────────────

func TestValidationAction_IsDeny(t *testing.T) {
	assert.True(t, types.ValidationActionDeny.IsDeny())
	assert.False(t, types.ValidationActionDeny.IsWarn())
}

func TestValidationAction_IsWarn(t *testing.T) {
	assert.True(t, types.ValidationActionWarn.IsWarn())
	assert.False(t, types.ValidationActionWarn.IsDeny())
}

func TestValidationAction_EmptyIsDenyByDefault(t *testing.T) {
	var a types.ValidationAction
	assert.True(t, a.IsDeny(), "empty action must be treated as deny")
	assert.False(t, a.IsWarn())
}

// ── ValidationConfig.HasDenyRules ────────────────────────────────────────────

func TestValidationConfig_HasDenyRules(t *testing.T) {
	t.Run("nil config returns false", func(t *testing.T) {
		var cfg *types.ValidationConfig
		assert.False(t, cfg.HasDenyRules())
	})

	t.Run("empty rules slice returns false", func(t *testing.T) {
		cfg := &types.ValidationConfig{}
		assert.False(t, cfg.HasDenyRules())
	})

	t.Run("explicit deny rule", func(t *testing.T) {
		cfg := &types.ValidationConfig{
			Rules: []types.ValidationRule{
				{Field: "spec.image", Action: types.ValidationActionDeny},
			},
		}
		assert.True(t, cfg.HasDenyRules())
	})

	t.Run("omitted action defaults to deny", func(t *testing.T) {
		// A rule without an action field is deny — this is the
		// common authoring pattern and must be correctly detected.
		cfg := &types.ValidationConfig{
			Rules: []types.ValidationRule{
				{Field: "spec.image", Prefix: "myorg/"},
			},
		}
		assert.True(t, cfg.HasDenyRules())
	})

	t.Run("only warn rules returns false", func(t *testing.T) {
		cfg := &types.ValidationConfig{
			Rules: []types.ValidationRule{
				{Field: "metadata.labels.team", Action: types.ValidationActionWarn},
			},
		}
		assert.False(t, cfg.HasDenyRules())
	})

	t.Run("mixed deny and warn — returns true", func(t *testing.T) {
		cfg := &types.ValidationConfig{
			Rules: []types.ValidationRule{
				{Field: "spec.image", Action: types.ValidationActionDeny},
				{Field: "metadata.labels.team", Action: types.ValidationActionWarn},
			},
		}
		assert.True(t, cfg.HasDenyRules())
	})
}

// ── ValidationConfig.HasWarnRules ────────────────────────────────────────────

func TestValidationConfig_HasWarnRules(t *testing.T) {
	t.Run("nil config returns false", func(t *testing.T) {
		var cfg *types.ValidationConfig
		assert.False(t, cfg.HasWarnRules())
	})

	t.Run("empty rules returns false", func(t *testing.T) {
		cfg := &types.ValidationConfig{}
		assert.False(t, cfg.HasWarnRules())
	})

	t.Run("explicit warn rule", func(t *testing.T) {
		cfg := &types.ValidationConfig{
			Rules: []types.ValidationRule{
				{Field: "metadata.labels.team", Action: types.ValidationActionWarn},
			},
		}
		assert.True(t, cfg.HasWarnRules())
	})

	t.Run("deny-only rules return false", func(t *testing.T) {
		cfg := &types.ValidationConfig{
			Rules: []types.ValidationRule{
				{Field: "spec.image", Action: types.ValidationActionDeny},
				{Field: "spec.replicas"},
			},
		}
		assert.False(t, cfg.HasWarnRules())
	})

	t.Run("mixed deny and warn — returns true", func(t *testing.T) {
		cfg := &types.ValidationConfig{
			Rules: []types.ValidationRule{
				{Field: "spec.image", Action: types.ValidationActionDeny},
				{Field: "metadata.labels.team", Action: types.ValidationActionWarn},
			},
		}
		assert.True(t, cfg.HasWarnRules())
		assert.True(t, cfg.HasDenyRules())
	})
}

// ── AdmissionWebhookConfig.WebhookValidationEnabled ──────────────────────────

func boolPtr(v bool) *bool { return &v }

func TestWebhookValidationEnabled(t *testing.T) {
	tests := []struct {
		name string
		cfg  *types.AdmissionWebhookConfig
		want bool
	}{
		{
			name: "nil config — enabled by default",
			cfg:  nil,
			want: true,
		},
		{
			name: "config with nil Validation — enabled by default",
			cfg:  &types.AdmissionWebhookConfig{},
			want: true,
		},
		{
			name: "Validation = true — enabled",
			cfg:  &types.AdmissionWebhookConfig{Validation: boolPtr(true)},
			want: true,
		},
		{
			name: "Validation = false — disabled",
			cfg:  &types.AdmissionWebhookConfig{Validation: boolPtr(false)},
			want: false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, tc.cfg.WebhookValidationEnabled())
		})
	}
}

// ── AdmissionWebhookConfig.WebhookMutationEnabled ────────────────────────────

func TestWebhookMutationEnabled(t *testing.T) {
	tests := []struct {
		name string
		cfg  *types.AdmissionWebhookConfig
		want bool
	}{
		{
			name: "nil config — enabled by default",
			cfg:  nil,
			want: true,
		},
		{
			name: "config with nil Mutation — enabled by default",
			cfg:  &types.AdmissionWebhookConfig{},
			want: true,
		},
		{
			name: "Mutation = true — enabled",
			cfg:  &types.AdmissionWebhookConfig{Mutation: boolPtr(true)},
			want: true,
		},
		{
			name: "Mutation = false — disabled",
			cfg:  &types.AdmissionWebhookConfig{Mutation: boolPtr(false)},
			want: false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, tc.cfg.WebhookMutationEnabled())
		})
	}
}

// ── AdmissionWebhookConfig.EffectiveOperations ───────────────────────────────

func TestEffectiveOperations(t *testing.T) {
	t.Run("nil config returns CREATE and UPDATE", func(t *testing.T) {
		var cfg *types.AdmissionWebhookConfig
		assert.Equal(t, []string{"CREATE", "UPDATE"}, cfg.EffectiveOperations())
	})

	t.Run("empty operations returns CREATE and UPDATE", func(t *testing.T) {
		cfg := &types.AdmissionWebhookConfig{}
		assert.Equal(t, []string{"CREATE", "UPDATE"}, cfg.EffectiveOperations())
	})

	t.Run("custom operations returned as declared", func(t *testing.T) {
		cfg := &types.AdmissionWebhookConfig{
			Operations: []string{"CREATE", "UPDATE", "DELETE"},
		}
		assert.Equal(t, []string{"CREATE", "UPDATE", "DELETE"}, cfg.EffectiveOperations())
	})

	t.Run("single operation override", func(t *testing.T) {
		cfg := &types.AdmissionWebhookConfig{
			Operations: []string{"CREATE"},
		}
		assert.Equal(t, []string{"CREATE"}, cfg.EffectiveOperations())
	})
}

// ── MutationConfig.MutateFirst ────────────────────────────────────────────────

func TestMutationConfig_MutateFirstDefault(t *testing.T) {
	// MutateFirst defaults to false — validate before mutate at reconcile time.
	// Admission time always mutates first regardless of this setting.
	cfg := &types.MutationConfig{
		Rules: []types.MutationRule{
			{Field: "spec.replicas", Default: "2"},
		},
	}
	assert.False(t, cfg.MutateFirst)
}

// ── ValidationConfig.References ──────────────────────────────────────────────
//
// Both plain dot-path forms and Go template expression forms are detected.
// When a rule's field is a user-defined note reference ({{ noteName }}), the
// note's expression body is scanned for .health.* / .metrics.* so the webhook
// correctly fetches runtime data even when the reference is indirect.

func noteReg(fns ...types.UserDefinedNote) types.NoteRegistry {
	return types.NoteRegistry{Functions: fns}
}

func note(name, expr string) types.UserDefinedNote {
	return types.UserDefinedNote{Name: name, Expression: expr}
}

func TestValidationConfig_ReferencesHealth(t *testing.T) {
	empty := types.NoteRegistry{}

	t.Run("nil config returns false", func(t *testing.T) {
		var cfg *types.ValidationConfig
		assert.False(t, cfg.References(empty, "health"))
	})

	t.Run("empty rules returns false", func(t *testing.T) {
		cfg := &types.ValidationConfig{}
		assert.False(t, cfg.References(empty, "health"))
	})

	t.Run("plain dot-path .health.status", func(t *testing.T) {
		cfg := &types.ValidationConfig{
			Rules: []types.ValidationRule{{Field: ".health.status", Prefix: "healthy"}},
		}
		assert.True(t, cfg.References(empty, "health"))
	})

	t.Run("template form {{ .health.status }}", func(t *testing.T) {
		cfg := &types.ValidationConfig{
			Rules: []types.ValidationRule{{Field: "{{ .health.status }}", Prefix: "healthy"}},
		}
		assert.True(t, cfg.References(empty, "health"))
	})

	t.Run("template form without trailing space", func(t *testing.T) {
		cfg := &types.ValidationConfig{
			Rules: []types.ValidationRule{{Field: "{{.health.readyCount}}"}},
		}
		assert.True(t, cfg.References(empty, "health"))
	})

	t.Run("spec field with 'health' in name is not a false positive", func(t *testing.T) {
		cfg := &types.ValidationConfig{
			Rules: []types.ValidationRule{{Field: "spec.healthCheck"}},
		}
		assert.False(t, cfg.References(empty, "health"))
	})

	t.Run("note whose body references .health triggers fetch", func(t *testing.T) {
		cfg := &types.ValidationConfig{
			Rules: []types.ValidationRule{{Field: "{{ inBusinessHours }}", Equals: "true"}},
		}
		nr := noteReg(note("inBusinessHours", `{{ eq .health.status "healthy" }}`))
		assert.True(t, cfg.References(nr, "health"))
	})

	t.Run("transitive — note calls another note that references .health", func(t *testing.T) {
		cfg := &types.ValidationConfig{
			Rules: []types.ValidationRule{{Field: "{{ canAccept }}", Equals: "true"}},
		}
		nr := noteReg(
			note("isHealthy", `{{ eq .health.status "healthy" }}`),
			note("canAccept", `{{ and isHealthy (lt .metrics.queueDepth 500) }}`),
		)
		assert.True(t, cfg.References(nr, "health"))
		assert.True(t, cfg.References(nr, "metrics"))
	})

	t.Run("note whose body does not reference .health returns false", func(t *testing.T) {
		cfg := &types.ValidationConfig{
			Rules: []types.ValidationRule{{Field: "{{ inBusinessHours }}", Equals: "true"}},
		}
		nr := noteReg(note("inBusinessHours", `{{ isWithinHours 9 17 }}`))
		assert.False(t, cfg.References(nr, "health"))
	})

	t.Run("sentinel in field is not a note ref", func(t *testing.T) {
		cfg := &types.ValidationConfig{
			Rules: []types.ValidationRule{{Field: "{{ generationChanged }}", Equals: "true"}},
		}
		nr := noteReg(note("generationChanged", `{{ eq .health.status "healthy" }}`))
		assert.False(t, cfg.References(nr, "health"))
	})

	t.Run("note not in registry returns false", func(t *testing.T) {
		cfg := &types.ValidationConfig{
			Rules: []types.ValidationRule{{Field: "{{ unknownNote }}", Equals: "true"}},
		}
		assert.False(t, cfg.References(empty, "health"))
	})

	t.Run("only one rule needs to match", func(t *testing.T) {
		cfg := &types.ValidationConfig{
			Rules: []types.ValidationRule{
				{Field: "spec.image"},
				{Field: "{{ .health.phase }}"},
			},
		}
		assert.True(t, cfg.References(empty, "health"))
	})
}

func TestValidationConfig_ReferencesMetrics(t *testing.T) {
	empty := types.NoteRegistry{}

	t.Run("nil config returns false", func(t *testing.T) {
		var cfg *types.ValidationConfig
		assert.False(t, cfg.References(empty, "metrics"))
	})

	t.Run("plain dot-path .metrics.queueDepth", func(t *testing.T) {
		cfg := &types.ValidationConfig{
			Rules: []types.ValidationRule{{Field: ".metrics.queueDepth"}},
		}
		assert.True(t, cfg.References(empty, "metrics"))
	})

	t.Run("template form {{ .metrics.workersBusyPercent }}", func(t *testing.T) {
		cfg := &types.ValidationConfig{
			Rules: []types.ValidationRule{{Field: "{{ .metrics.workersBusyPercent }}"}},
		}
		assert.True(t, cfg.References(empty, "metrics"))
	})

	t.Run("spec field with 'metrics' in name is not a false positive", func(t *testing.T) {
		cfg := &types.ValidationConfig{
			Rules: []types.ValidationRule{{Field: "spec.metricsPort"}},
		}
		assert.False(t, cfg.References(empty, "metrics"))
	})

	t.Run("note whose body references .metrics triggers fetch", func(t *testing.T) {
		cfg := &types.ValidationConfig{
			Rules: []types.ValidationRule{{Field: "{{ inBusinessHours }}", Equals: "false"}},
		}
		nr := noteReg(note("inBusinessHours", `{{ gt .metrics.workersBusyPercent 80 }}`))
		assert.True(t, cfg.References(nr, "metrics"))
	})

	t.Run("note composing both .health and .metrics — both detected", func(t *testing.T) {
		cfg := &types.ValidationConfig{
			Rules: []types.ValidationRule{{Field: "{{ combinedCheck }}", Equals: "true"}},
		}
		nr := noteReg(note("combinedCheck", `{{ and (eq .health.status "healthy") (lt .metrics.queueDepth 100) }}`))
		assert.True(t, cfg.References(nr, "health"))
		assert.True(t, cfg.References(nr, "metrics"))
	})
}

// ── MutationConfig.References ────────────────────────────────────────────────
//
// Mutation rules also check Default and Override expressions in addition to
// the Field path, because the live value may be used on the right-hand side.

func TestMutationConfig_ReferencesHealth(t *testing.T) {
	empty := types.NoteRegistry{}

	t.Run("nil config returns false", func(t *testing.T) {
		var cfg *types.MutationConfig
		assert.False(t, cfg.References(empty, "health"))
	})

	t.Run("field expression references .health", func(t *testing.T) {
		cfg := &types.MutationConfig{
			Rules: []types.MutationRule{{Field: "{{ .health.status }}"}},
		}
		assert.True(t, cfg.References(empty, "health"))
	})

	t.Run("field is note ref whose body references .health", func(t *testing.T) {
		cfg := &types.MutationConfig{
			Rules: []types.MutationRule{{Field: "{{ isHealthy }}", Default: "false"}},
		}
		nr := noteReg(note("isHealthy", `{{ eq .health.status "healthy" }}`))
		assert.True(t, cfg.References(nr, "health"))
	})

	t.Run("default expression references .health", func(t *testing.T) {
		cfg := &types.MutationConfig{
			Rules: []types.MutationRule{
				{Field: "spec.statusCopy", Default: "{{ .health.status }}"},
			},
		}
		assert.True(t, cfg.References(empty, "health"))
	})

	t.Run("override expression references .health", func(t *testing.T) {
		cfg := &types.MutationConfig{
			Rules: []types.MutationRule{
				{Field: "spec.phase", Override: ".health.phase"},
			},
		}
		assert.True(t, cfg.References(empty, "health"))
	})

	t.Run("unrelated rules return false", func(t *testing.T) {
		cfg := &types.MutationConfig{
			Rules: []types.MutationRule{
				{Field: "spec.replicas", Default: "2"},
			},
		}
		assert.False(t, cfg.References(empty, "health"))
	})
}

func TestMutationConfig_ReferencesMetrics(t *testing.T) {
	empty := types.NoteRegistry{}

	t.Run("nil config returns false", func(t *testing.T) {
		var cfg *types.MutationConfig
		assert.False(t, cfg.References(empty, "metrics"))
	})

	t.Run("field expression references .metrics", func(t *testing.T) {
		cfg := &types.MutationConfig{
			Rules: []types.MutationRule{{Field: ".metrics.queueDepth"}},
		}
		assert.True(t, cfg.References(empty, "metrics"))
	})

	t.Run("field is note ref whose body references .metrics", func(t *testing.T) {
		cfg := &types.MutationConfig{
			Rules: []types.MutationRule{{Field: "{{ isBusy }}", Default: "false"}},
		}
		nr := noteReg(note("isBusy", `{{ gt .metrics.workersBusyPercent 80 }}`))
		assert.True(t, cfg.References(nr, "metrics"))
	})

	t.Run("default expression references .metrics", func(t *testing.T) {
		cfg := &types.MutationConfig{
			Rules: []types.MutationRule{
				{Field: "spec.depth", Default: "{{ .metrics.queueDepth }}"},
			},
		}
		assert.True(t, cfg.References(empty, "metrics"))
	})

	t.Run("note not in registry returns false", func(t *testing.T) {
		cfg := &types.MutationConfig{
			Rules: []types.MutationRule{
				{Field: "{{ unknownNote }}", Default: "false"},
			},
		}
		assert.False(t, cfg.References(empty, "metrics"))
	})
}

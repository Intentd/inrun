package types

// PublishConfig declares the publishing and consumer policy for a pattern.
// Distinct from security: (which covers admission, RBAC, and namespace protection
// at runtime) — publish: is about which quality gates a pattern must pass
// before it is pushed.
//
//	publish:
//	  tests:
//	    e2e: true
//	    simulate: true
//	    intent: false
type PublishConfig struct {
	// Tests controls which quality gates run at push time and are required by
	// consumers at pull time. Absent fields use the defaults documented on each field.
	Tests *PublishTestsConfig `yaml:"tests,omitempty" json:"tests,omitempty"`
}

// PublishTestsConfig controls which quality gates run at push time and are
// required by consumers at pull time.
type PublishTestsConfig struct {
	// E2E controls whether the e2e gate runs at push time.
	// Default: true — matches current behaviour.
	// Setting false is equivalent to passing --no-e2e at push.
	E2E *bool `yaml:"e2e,omitempty" json:"e2e,omitempty"`

	// Simulate controls whether the simulate gate runs at push time.
	// Default: true — matches current behaviour.
	// Setting false is equivalent to passing --no-simulate at push.
	Simulate *bool `yaml:"simulate,omitempty" json:"simulate,omitempty"`

	// Intent controls whether inrun serve play runs at push time.
	// Default: false — opt-in. Equivalent to passing --add-intent at push.
	// When true, intent.yaml or intent.json must be present in the pattern directory.
	Intent *bool `yaml:"intent,omitempty" json:"intent,omitempty"`
}

// E2EEnabled reports whether the e2e gate is enabled.
// Defaults to true when the field is absent.
func (t *PublishTestsConfig) E2EEnabled() bool {
	if t == nil || t.E2E == nil {
		return true
	}
	return *t.E2E
}

// SimulateEnabled reports whether the simulate gate is enabled.
// Defaults to true when the field is absent.
func (t *PublishTestsConfig) SimulateEnabled() bool {
	if t == nil || t.Simulate == nil {
		return true
	}
	return *t.Simulate
}

// IntentEnabled reports whether the intent play gate is enabled.
// Defaults to false when the field is absent.
func (t *PublishTestsConfig) IntentEnabled() bool {
	if t == nil || t.Intent == nil {
		return false
	}
	return *t.Intent
}

// TestsConfig returns the test gate config, never nil.
func (p *PublishConfig) TestsConfig() *PublishTestsConfig {
	if p == nil || p.Tests == nil {
		return &PublishTestsConfig{}
	}
	return p.Tests
}

// HasTestsConfig reports whether a tests: block is explicitly declared.
func (p *PublishConfig) HasTestsConfig() bool {
	return p != nil && p.Tests != nil
}

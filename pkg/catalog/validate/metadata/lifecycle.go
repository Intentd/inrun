package metadata

import (
	"fmt"
	"time"

	"github.com/intentd/kel/core"

	"github.com/intentd/inrun/pkg/config"
	"github.com/intentd/inrun/pkg/types"
)

// validateLifecycle checks the lifecycle: block when present.
//
// Rules:
//  1. maturity must be one of: alpha, beta, stable, deprecated.
//  2. maturity: alpha or beta adds a non-fatal warning.
//  3. maturity: deprecated without lifecycle.deprecation adds a non-fatal warning
//     (the deprecation block is the primary signal; maturity is advisory).
//  4. lifecycle.deprecation present without maturity is valid (block implies deprecated).
//  5. compatibility.kubernetes and compatibility.inrun must be valid semver ranges when set.
//  6. lifecycle.accept is only valid on a Stack — error on a Catalog.
//  7. lifecycle.accept.patterns[].version must be a valid semver range when set.
func (c *checker) validateLifecycle() error {
	lc := c.Cat.Lifecycle()
	if lc == nil {
		return nil
	}

	if err := c.validateLifecycleMaturity(lc); err != nil {
		return err
	}

	if err := validateLifecycleCompatibility(lc); err != nil {
		return err
	}

	if err := c.validateLifecycleDeprecation(); err != nil {
		return err
	}

	if err := c.validateLifecycleAcceptBoundary(lc); err != nil {
		return err
	}

	if err := validateLifecycleAcceptPatterns(lc); err != nil {
		return err
	}

	return nil
}

func (c *checker) validateLifecycleMaturity(lc *types.CatalogLifecycle) error {
	if lc.Maturity == "" {
		return nil
	}
	switch lc.Maturity {
	case types.MaturityAlpha:
		c.Cat.Warnings.AddWarning("lifecycle.maturity is alpha — this pattern is experimental and not recommended for production")
	case types.MaturityBeta:
		c.Cat.Warnings.AddWarning("lifecycle.maturity is beta — this pattern is stabilising; API may still change")
	case types.MaturityStable:
		// no warning
	case types.MaturityDeprecated:
		if lc.Deprecation == nil {
			// advisory — the deprecation block is the source of truth
			c.Cat.Warnings.AddWarning("lifecycle.maturity is deprecated but lifecycle.deprecation is not set — add the deprecation block or remove the maturity")
		}
	default:
		return fmt.Errorf(
			"%s lifecycle.maturity: %q is not valid — must be one of: alpha, beta, stable, deprecated",
			failureMark(), lc.Maturity,
		)
	}
	return nil
}

func validateLifecycleCompatibility(lc *types.CatalogLifecycle) error {
	if lc.Compatibility == nil {
		return nil
	}
	if k8s := lc.Compatibility.Kubernetes; k8s != "" {
		if _, err := core.SemverConstraint("1.0.0", k8s); err != nil {
			return fmt.Errorf(
				"%s lifecycle.compatibility.kubernetes: %q is not a valid semver range",
				failureMark(), k8s,
			)
		}
	}
	if inrun := lc.Compatibility.Inrun; inrun != "" {
		if _, err := core.SemverConstraint("1.0.0", inrun); err != nil {
			return fmt.Errorf(
				"%s lifecycle.compatibility.inrun: %q is not a valid semver range",
				failureMark(), inrun,
			)
		}
	}
	return nil
}

// validateLifecycleDeprecation checks the lifecycle.deprecation block when present.
//
// Rules:
//  1. message is required when deprecation: is declared.
//  2. timeline.from and timeline.to must parse as YYYY-MM-DD when set.
//  3. timeline.from must be before timeline.to when both are set.
func (c *checker) validateLifecycleDeprecation() error {
	d := c.Cat.Deprecation()
	if d == nil {
		return nil
	}

	if d.Message == "" {
		return fmt.Errorf(
			"%s lifecycle.deprecation: message is required when deprecation is declared",
			failureMark(),
		)
	}

	const layout = "2006-01-02"

	var from, to time.Time
	var hasFrom, hasTo bool

	if f := d.TimelineFrom(); f != "" {
		t, err := time.Parse(layout, f)
		if err != nil {
			return fmt.Errorf(
				"%s lifecycle.deprecation.timeline.from: %q is not a valid date (expected YYYY-MM-DD)",
				failureMark(), f,
			)
		}
		from = t
		hasFrom = true
	}

	if s := d.TimelineTo(); s != "" {
		t, err := time.Parse(layout, s)
		if err != nil {
			return fmt.Errorf(
				"%s lifecycle.deprecation.timeline.to: %q is not a valid date (expected YYYY-MM-DD)",
				failureMark(), s,
			)
		}
		to = t
		hasTo = true
	}

	if hasFrom && hasTo && !from.Before(to) {
		return fmt.Errorf(
			"%s lifecycle.deprecation.timeline: from (%s) must be before to (%s)",
			failureMark(), d.TimelineFrom(), d.TimelineTo(),
		)
	}

	return nil
}

// validateLifecycleAcceptBoundary checks the lifecycle.accept block when present.
func (c *checker) validateLifecycleAcceptBoundary(lc *types.CatalogLifecycle) error {
	// lifecycle.accept is Stack-only
	if lc.Accept != nil && !config.IsStackKind(c.Cat.Kind) {
		return fmt.Errorf(
			"%s lifecycle.accept is only valid in a Stack — move pattern acknowledgements to a stack.yaml",
			failureMark(),
		)
	}
	return nil
}

// validateLifecycleAcceptPatterns validates the version field on each accept.patterns
// entry. When set, version must be a valid semver range. Stale-acceptance detection
// (warning when the imported version no longer matches) is deferred pending import resolution.
func validateLifecycleAcceptPatterns(lc *types.CatalogLifecycle) error {
	if lc.Accept == nil {
		return nil
	}
	for _, e := range lc.Accept.Patterns {
		if e.Version == "" {
			continue
		}
		if _, err := core.SemverConstraint("1.0.0", e.Version); err != nil {
			return fmt.Errorf(
				"%s lifecycle.accept.patterns[%q].version: %q is not a valid semver range",
				failureMark(), e.Name, e.Version,
			)
		}
	}
	return nil
}

// validatePolicy checks the policy: block when present.
//
// Rules:
//  1. policy.lifecycle.minMaturity must be one of: alpha, beta, stable.
//     (deprecated is not a valid floor — deprecated imports require explicit accept regardless.)
func (c *checker) validatePolicy() error {
	p := c.Cat.Policy()
	if p == nil || p.Lifecycle == nil {
		return nil
	}
	lc := p.Lifecycle
	if lc.MinMaturity == "" {
		return nil
	}
	switch lc.MinMaturity {
	case types.MaturityAlpha, types.MaturityBeta, types.MaturityStable:
		// valid floors
	default:
		return fmt.Errorf(
			"%s policy.lifecycle.minMaturity: %q is not valid — must be one of: alpha, beta, stable",
			failureMark(), lc.MinMaturity,
		)
	}
	return nil
}

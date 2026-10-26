//go:build !runtime && !gateway

package cmdutil

// E2E status strings for registry info / list output.

func E2eVerified(suffix string) string {
	s := Green("✓ Verified")
	if suffix != "" {
		s += " · " + suffix
	}
	return s
}

func E2eFailed(suffix string) string {
	s := Red("✗ Failed")
	if suffix != "" {
		s += " · " + suffix
	}
	return s
}

func E2eSkipped() string {
	return Yellow("⊘ Skipped") + " (pushed with --no-e2e)"
}

func E2ENoAssertion() string {
	return Yellow("⚠ No assertions") + " (add expect: to simulate.yaml to enforce behavior)"
}

func SkippedShort() string {
	return Yellow("⊘ Skipped")
}

func E2eNotVerified() string {
	return Gray("- Not verified")
}

func SimulateVerified(suffix string) string {
	s := Green("✓ Verified")
	if suffix != "" {
		s += " · " + suffix
	}
	return s
}

func SimulateFailed(suffix string) string {
	s := Red("✓ Failed")
	if suffix != "" {
		s += " · " + suffix
	}
	return s
}

func SimulateSkipped() string {
	return Yellow("⊘ Skipped") + " (pushed with --no-simulate)"
}

func SimulateNoAssertion() string {
	return Yellow("⚠ No assertions") + " (add expect: to simulate.yaml to enforce behavior)"
}

func NoAssertion() string {
	return Yellow("⚠ No assertions")
}

// Diff change icons used in simulate output.

func IconAdded() string   { return Green("+") }
func IconRemoved() string { return Red("-") }
func IconChanged() string { return Yellow("~") }

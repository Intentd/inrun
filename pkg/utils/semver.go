package utils

import "github.com/intentd/kel/core"

// SemverValid reports whether v is a valid semver string.
func SemverValid(v string) bool {
	return core.SemverValid(v)
}

// SemverMajor returns the major component of v.
func SemverMajor(v string) (uint64, error) {
	return core.SemverMajor(v)
}

// SemverMinor returns the minor component of v.
func SemverMinor(v string) (uint64, error) {
	return core.SemverMinor(v)
}

// SemverPatch returns the patch component of v.
func SemverPatch(v string) (uint64, error) {
	return core.SemverPatch(v)
}

// SemverCompare compares two semver strings.
// Returns -1 when a < b, 0 when a == b, 1 when a > b.
func SemverCompare(a, b string) (int, error) {
	return core.SemverCompare(a, b)
}

// SemverCheck reports whether version v satisfies the constraint expression.
// Uses Masterminds semver constraint syntax: ">=1.0.0", "^2.0", "~1.2", "1.x".
func SemverCheck(v, constraint string) (bool, error) {
	return core.SemverConstraint(v, constraint)
}

// SemverBump increments the given component ("major", "minor", or "patch") of v,
// dropping any prerelease or build metadata from the result.
func SemverBump(v, component string) (string, error) {
	return core.SemverBump(v, component)
}

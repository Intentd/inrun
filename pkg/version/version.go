// Package version reports the build's version, commit and date, injected
// with -ldflags. String gives the full form and Short just the version.
package version

import "fmt"

// These variables are set at build time via ldflags:
//
//	-X github.com/intentd/inrun/pkg/version.Version=v1.0.0
//	-X github.com/intentd/inrun/pkg/version.Commit=abc1234
//	-X github.com/intentd/inrun/pkg/version.Date=2026-03-19T10:00:00Z
var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)

// String returns the full version string.
func String() string {
	return fmt.Sprintf("%s (commit: %s, built: %s)", Version, Commit, Date)
}

// Short returns just the version tag.
func Short() string {
	return Version
}

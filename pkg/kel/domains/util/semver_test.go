// pkg/note/semver_test.go
package note

import (
	"testing"
)

func TestNoteSemverMajor(t *testing.T) {
	tests := []struct {
		name string
		v    string
		want string
	}{
		{"valid semver", "1.2.3", "1"},
		{"with v prefix", "v2.0.0", "2"},
		{"prerelease", "1.2.3-alpha.1", "1"},
		{"build metadata", "1.2.3+build", "1"},
		{"invalid", "not-a-version", ""},
		{"empty", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := noteSemverMajor(tt.v); got != tt.want {
				t.Errorf("noteSemverMajor(%q) = %v, want %v", tt.v, got, tt.want)
			}
		})
	}
}

func TestNoteSemverMinor(t *testing.T) {
	tests := []struct {
		name string
		v    string
		want string
	}{
		{"valid semver", "1.2.3", "2"},
		{"with v prefix", "v2.0.0", "0"},
		{"prerelease", "1.2.3-alpha.1", "2"},
		{"invalid", "not-a-version", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := noteSemverMinor(tt.v); got != tt.want {
				t.Errorf("noteSemverMinor(%q) = %v, want %v", tt.v, got, tt.want)
			}
		})
	}
}

func TestNoteSemverPatch(t *testing.T) {
	tests := []struct {
		name string
		v    string
		want string
	}{
		{"valid semver", "1.2.3", "3"},
		{"with v prefix", "v2.0.0", "0"},
		{"prerelease", "1.2.3-alpha.1", "3"},
		{"invalid", "not-a-version", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := noteSemverPatch(tt.v); got != tt.want {
				t.Errorf("noteSemverPatch(%q) = %v, want %v", tt.v, got, tt.want)
			}
		})
	}
}

func TestNoteSemverValid(t *testing.T) {
	tests := []struct {
		name string
		v    string
		want bool
	}{
		{"valid semver", "1.2.3", true},
		{"with v prefix", "v2.0.0", true},
		{"prerelease", "1.2.3-alpha.1", true},
		{"build metadata", "1.2.3+build", true},
		{"invalid", "latest", false},
		{"empty", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := noteSemverValid(tt.v); got != tt.want {
				t.Errorf("noteSemverValid(%q) = %v, want %v", tt.v, got, tt.want)
			}
		})
	}
}

func TestNoteSemverCompare(t *testing.T) {
	tests := []struct {
		name string
		a    string
		b    string
		want int
	}{
		{"a < b", "1.2.3", "1.3.0", -1},
		{"a == b", "1.2.3", "1.2.3", 0},
		{"a > b", "2.0.0", "1.9.9", 1},
		{"with v prefix", "v1.2.3", "1.2.3", 0},
		{"prerelease", "1.2.3-alpha", "1.2.3", -1},
		{"invalid a", "not-valid", "1.2.3", 0},
		{"invalid b", "1.2.3", "not-valid", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := noteSemverCompare(tt.a, tt.b); got != tt.want {
				t.Errorf("noteSemverCompare(%q, %q) = %v, want %v", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

func TestNoteSemverBump(t *testing.T) {
	tests := []struct {
		name      string
		v         string
		component string
		want      string
	}{
		{"bump patch", "1.2.3", "patch", "1.2.4"},
		{"bump minor", "1.2.3", "minor", "1.3.0"},
		{"bump major", "1.2.3", "major", "2.0.0"},
		{"with v prefix", "v1.2.3", "patch", "1.2.4"},
		{"prerelease bump patch", "1.2.3-alpha", "patch", "1.2.4"},
		{"prerelease bump minor", "1.2.3-alpha", "minor", "1.3.0"},
		{"prerelease bump major", "1.2.3-alpha", "major", "2.0.0"},
		{"invalid version", "invalid", "patch", "invalid"},
		{"unknown component", "1.2.3", "unknown", "1.2.3"},
		{"empty component", "1.2.3", "", "1.2.3"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := noteSemverBump(tt.v, tt.component); got != tt.want {
				t.Errorf("noteSemverBump(%q, %q) = %v, want %v", tt.v, tt.component, got, tt.want)
			}
		})
	}
}

func TestNoteSemverConstraint(t *testing.T) {
	tests := []struct {
		name       string
		v          string
		constraint string
		want       bool
	}{
		{"satisfies >=1.0.0", "1.2.3", ">=1.0.0", true},
		{"satisfies <2.0.0", "1.2.3", "<2.0.0", true},
		{"combined constraint", "1.2.3", ">=1.0.0,<2.0.0", true},
		{"does not satisfy", "2.1.0", "^1.0", false},
		{"tilde constraint", "1.2.3", "~1.2", true},
		{"caret constraint", "1.2.3", "^1.2", true},
		{"wildcard", "1.2.3", "1.x", true},
		{"invalid version", "invalid", ">=1.0.0", false},
		{"invalid constraint", "1.2.3", "!!", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := noteSemverConstraint(tt.v, tt.constraint); got != tt.want {
				t.Errorf("noteSemverConstraint(%q, %q) = %v, want %v", tt.v, tt.constraint, got, tt.want)
			}
		})
	}
}

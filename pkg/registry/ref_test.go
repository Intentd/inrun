package registry

import "testing"

func TestExtractTagVersion(t *testing.T) {
	tests := []struct {
		ref  string
		want string
	}{
		// simple name:tag
		{"postgres:v14", "v14"},
		{"redis:7.0.1", "7.0.1"},
		// registry host + path
		{"ghcr.io/intentd/postgres:v0.1.0", "v0.1.0"},
		{"oci://ghcr.io/intentd/postgres:v0.1.0", "v0.1.0"},
		// no tag
		{"postgres", ""},
		{"ghcr.io/intentd/postgres", ""},
		// digest refs (should ignore digest)
		{"ghcr.io/intentd/postgres@sha256:abcdef", ""},
		{"ghcr.io/intentd/postgres:v0.1.0@sha256:abcdef", "v0.1.0"},
		// edge cases with multiple colons (ports in host)
		{"localhost:5000/postgres:v2", "v2"},
		{"localhost:5000/postgres", ""},
		// weird but valid-looking strings
		{"repo/name:with:colons:tag", "tag"},
		{"repo/name:tag-with:colon", "colon"},
	}

	for _, tc := range tests {
		got := ExtractTagVersion(tc.ref)
		if got != tc.want {
			t.Fatalf("ExtractTagVersion(%q) = %q; want %q", tc.ref, got, tc.want)
		}
	}
}

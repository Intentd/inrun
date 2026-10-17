// pkg/note/validation_test.go
package note

import "testing"

func TestNoteIsValidEmail(t *testing.T) {
	tests := []struct {
		name  string
		email string
		want  bool
	}{
		{"simple valid", "team@myorg.io", true},
		{"with plus tag", "team+alerts@myorg.io", true},
		{"with subdomain", "team@mail.myorg.io", true},
		{"empty", "", false},
		{"no at sign", "not-an-email", false},
		{"no domain", "team@", false},
		{"display name form rejected", "Team <team@myorg.io>", false},
		{"trailing space", "team@myorg.io ", true}, // ParseAddress trims
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := noteIsValidEmail(tt.email); got != tt.want {
				t.Errorf("noteIsValidEmail(%q) = %v, want %v", tt.email, got, tt.want)
			}
		})
	}
}

func TestNoteIsValidGitRepository(t *testing.T) {
	tests := []struct {
		name string
		repo string
		want bool
	}{
		{"https URL", "https://github.com/myorg/payments", true},
		{"https URL with .git suffix", "https://github.com/myorg/payments.git", true},
		{"http URL", "http://internal-git.myorg.io/team/app", true},
		{"git protocol", "git://github.com/myorg/payments.git", true},
		{"ssh protocol", "ssh://git@github.com/myorg/payments.git", true},
		{"scp-like shorthand", "git@github.com:myorg/payments.git", true},
		{"scp-like without .git", "git@github.com:myorg/payments", true},
		{"empty", "", false},
		{"plain text", "not a repo", false},
		{"ftp scheme unsupported", "ftp://example.com/repo", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := noteIsValidGitRepository(tt.repo); got != tt.want {
				t.Errorf("noteIsValidGitRepository(%q) = %v, want %v", tt.repo, got, tt.want)
			}
		})
	}
}

func TestNoteIsValidURL(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want bool
	}{
		{"https", "https://example.com/webhook", true},
		{"http", "http://example.com", true},
		{"with query", "https://example.com/path?x=1", true},
		{"empty", "", false},
		{"no scheme", "example.com", false},
		{"ftp unsupported", "ftp://example.com", false},
		{"no host", "https://", false},
		{"plain text", "not a url", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := noteIsValidURL(tt.url); got != tt.want {
				t.Errorf("noteIsValidURL(%q) = %v, want %v", tt.url, got, tt.want)
			}
		})
	}
}

func TestNoteIsValidImageRef(t *testing.T) {
	tests := []struct {
		name string
		ref  string
		want bool
	}{
		{"bare name", "nginx", true},
		{"org/repo", "myorg/app", true},
		{"with tag", "myorg/app:v1.2.3", true},
		{"with latest tag", "myorg/app:latest", true},
		{"registry with port", "registry.myorg.io:5000/team/app:latest", true},
		{"with digest", "myorg/app@sha256:" + fortyNineHexChars, true},
		{"empty", "", false},
		{"uppercase invalid", "MyOrg/App", false},
		{"contains space", "my org/app", false},
		{"double colon", "myorg/app::latest", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := noteIsValidImageRef(tt.ref); got != tt.want {
				t.Errorf("noteIsValidImageRef(%q) = %v, want %v", tt.ref, got, tt.want)
			}
		})
	}
}

// fortyNineHexChars is a syntactically valid sha256 hex digest (64 chars) —
// named for the regex's 32+ char minimum, but a real digest is always 64.
const fortyNineHexChars = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b85"

func TestNoteIsValidJSON(t *testing.T) {
	tests := []struct {
		name string
		s    string
		want bool
	}{
		{"object", `{"app": "payments-api"}`, true},
		{"array", `[1, 2, 3]`, true},
		{"string", `"hello"`, true},
		{"number", `42`, true},
		{"empty", "", false},
		{"malformed", `{not json`, false},
		{"trailing comma", `{"a": 1,}`, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := noteIsValidJSON(tt.s); got != tt.want {
				t.Errorf("noteIsValidJSON(%q) = %v, want %v", tt.s, got, tt.want)
			}
		})
	}
}

func TestNoteIsValidPort(t *testing.T) {
	tests := []struct {
		name string
		port string
		want bool
	}{
		{"typical", "8080", true},
		{"minimum", "1", true},
		{"maximum", "65535", true},
		{"zero invalid", "0", false},
		{"over max", "65536", false},
		{"negative", "-1", false},
		{"empty", "", false},
		{"non-numeric", "http", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := noteIsValidPort(tt.port); got != tt.want {
				t.Errorf("noteIsValidPort(%q) = %v, want %v", tt.port, got, tt.want)
			}
		})
	}
}

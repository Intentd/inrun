// pkg/note/kube_validation_test.go
package note

import "testing"

func TestNoteIsValidLabelValue(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  bool
	}{
		{"empty is valid", "", true},
		{"simple alphanumeric", "payments", true},
		{"with dash, underscore, dot", "team-payments_v1.0", true},
		{"max length 63", stringOfLen(63), true},
		{"over max length 64", stringOfLen(64), false},
		{"contains space", "team payments", false},
		{"leading dash", "-payments", false},
		{"trailing dash", "payments-", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := noteIsValidLabelValue(tt.value); got != tt.want {
				t.Errorf("noteIsValidLabelValue(%q) = %v, want %v", tt.value, got, tt.want)
			}
		})
	}
}

func TestNoteIsValidLabelKey(t *testing.T) {
	tests := []struct {
		name string
		key  string
		want bool
	}{
		{"empty is invalid", "", false},
		{"bare name", "tier", true},
		{"prefixed name", "platform.myorg.io/tier", true},
		{"contains space", "bad key", false},
		{"multiple slashes", "a/b/c", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := noteIsValidLabelKey(tt.key); got != tt.want {
				t.Errorf("noteIsValidLabelKey(%q) = %v, want %v", tt.key, got, tt.want)
			}
		})
	}
}

func TestNoteIsValidAnnotationKey(t *testing.T) {
	tests := []struct {
		name string
		key  string
		want bool
	}{
		{"empty is invalid", "", false},
		{"bare name", "jira-ticket", true},
		{"prefixed name", "platform.myorg.io/jira-ticket", true},
		{"contains space", "bad key", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := noteIsValidAnnotationKey(tt.key); got != tt.want {
				t.Errorf("noteIsValidAnnotationKey(%q) = %v, want %v", tt.key, got, tt.want)
			}
		})
	}
}

func TestNoteIsDNS1123Subdomain(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  bool
	}{
		{"empty is invalid", "", false},
		{"bare name", "my-app", true},
		{"dotted subdomain", "my-app.example.com", true},
		{"uppercase invalid", "My-App", false},
		{"underscore invalid", "my_app", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := noteIsDNS1123Subdomain(tt.value); got != tt.want {
				t.Errorf("noteIsDNS1123Subdomain(%q) = %v, want %v", tt.value, got, tt.want)
			}
		})
	}
}

func stringOfLen(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = 'a'
	}
	return string(b)
}

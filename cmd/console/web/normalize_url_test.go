package web

import "testing"

func TestNormalizeURL(t *testing.T) {
	cases := []struct{ in, want string }{
		{"", ""},
		{"  ", ""},
		{"localhost:8080", "http://localhost:8080"},
		{"http://localhost:8080", "http://localhost:8080"},
		{"https://localhost:8080", "https://localhost:8080"},
		{"http://localhost:8080/", "http://localhost:8080"},
		{"  http://localhost:8080  ", "http://localhost:8080"},
	}
	for _, c := range cases {
		if got := normalizeURL(c.in); got != c.want {
			t.Errorf("normalizeURL(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

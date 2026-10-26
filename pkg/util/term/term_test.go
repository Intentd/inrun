package term

import (
	"bytes"
	"strings"
	"testing"
)

func TestConfirmFrom(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  bool
	}{
		{"yes lowercase", "y\n", true},
		{"yes uppercase", "Y\n", true},
		{"yes word", "yes\n", true},
		{"yes word mixed case", "YeS\n", true},
		{"yes with spaces", "  y  \n", true},
		{"no", "n\n", false},
		{"no word", "no\n", false},
		{"empty", "\n", false},
		{"garbage", "maybe\n", false},
		{"eof empty", "", false},
		{"eof with y", "y", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var out bytes.Buffer
			got, err := ConfirmFrom("Push anyway?", strings.NewReader(tc.input), &out)
			if err != nil {
				t.Fatalf("ConfirmFrom: %v", err)
			}
			if got != tc.want {
				t.Fatalf("ConfirmFrom(%q) = %v, want %v", tc.input, got, tc.want)
			}
			if !strings.Contains(out.String(), "[y/N]") {
				t.Fatalf("prompt missing [y/N]: %q", out.String())
			}
		})
	}
}

func TestConfirmFromPrompt(t *testing.T) {
	var out bytes.Buffer
	_, _ = ConfirmFrom("Push anyway?", strings.NewReader("n\n"), &out)

	if got, want := out.String(), "Push anyway? [y/N]: "; got != want {
		t.Fatalf("prompt = %q, want %q", got, want)
	}
}

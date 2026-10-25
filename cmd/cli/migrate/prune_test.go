//go:build !runtime

package migrate

import (
	"strings"
	"testing"
)

func TestAskYes(t *testing.T) {
	for in, want := range map[string]bool{
		"y\n": true, "Y\n": true, "yes\n": true, " YES \n": true,
		"n\n": false, "\n": false, "": false, "maybe\n": false,
	} {
		got, err := askYes(strings.NewReader(in), "")
		if err != nil {
			t.Fatalf("%q: %v", in, err)
		}
		if got != want {
			t.Errorf("askYes(%q) = %v, want %v", in, got, want)
		}
	}
}

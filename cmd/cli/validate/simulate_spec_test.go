//go:build !runtime && !gateway

package validate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeSpec(t *testing.T, spec string) string {
	t.Helper()
	dir := t.TempDir()
	for _, f := range []string{"catalog.yaml", "intent.json", "cr.yaml"} {
		if err := os.WriteFile(filepath.Join(dir, f), []byte("{}"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	p := filepath.Join(dir, "simulate.yaml")
	doc := "apiVersion: inrun.dev/v1\nkind: Simulate\nmetadata:\n  name: s\nspec:\n  catalog: catalog.yaml\n" + spec
	if err := os.WriteFile(p, []byte(doc), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestValidateSimulate_CRSource(t *testing.T) {
	cases := []struct {
		name, spec, wantErr string
	}{
		{name: "cr", spec: "  cr: cr.yaml\n"},
		{name: "intent with token", spec: "  intent: intent.json\n  token: dev\n"},
		{name: "intent without token", spec: "  intent: intent.json\n", wantErr: "spec.token is required"},
		{name: "intent and cr", spec: "  intent: intent.json\n  token: dev\n  cr: cr.yaml\n", wantErr: "not both"},
		{name: "neither", spec: "", wantErr: "spec.cr, spec.crFiles or spec.intent is required"},
		{name: "missing intent file", spec: "  intent: nope.json\n  token: dev\n", wantErr: "intent file not found"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateSimulateFileQuiet(writeSpec(t, tc.spec))
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			// The quiet form carries the messages, not just a count.
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("err = %v, want it to mention %q", err, tc.wantErr)
			}
		})
	}
}

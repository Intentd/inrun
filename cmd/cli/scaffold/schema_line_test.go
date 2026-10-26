//go:build !runtime && !gateway

package scaffold

import (
	"strings"
	"testing"
)

func TestWithSchemaLine(t *testing.T) {
	cases := []struct {
		name, in, want string
	}{
		{"blueprint", "apiVersion: inrun.dev/v1\nkind: Blueprint\n", "# yaml-language-server: $schema=https://inrun.dev/schema/v1/blueprint.json\n"},
		{"e2e", "apiVersion: inrun.dev/v1\nkind: E2E\n", "# yaml-language-server: $schema=https://inrun.dev/schema/v1/e2e.json\n"},
		{"a CRD", "apiVersion: apiextensions.k8s.io/v1\nkind: CustomResourceDefinition\n", ""},
		{"a CR of an Inrun example group", "apiVersion: demo.inrun.dev/v1alpha1\nkind: Website\n", ""},
		{"already set", "# yaml-language-server: $schema=x\napiVersion: inrun.dev/v1\nkind: Blueprint\n", ""},
	}
	for _, c := range cases {
		got := string(withSchemaLine([]byte(c.in)))
		if c.want == "" {
			if got != c.in {
				t.Errorf("%s: changed to %q", c.name, got)
			}
			continue
		}
		if !strings.HasPrefix(got, c.want) || !strings.HasSuffix(got, c.in) {
			t.Errorf("%s: got %q", c.name, got)
		}
	}
}

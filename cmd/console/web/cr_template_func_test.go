package web

import "testing"

func TestCRTemplateFunc_PhaseColor(t *testing.T) {
	phaseColor := crTemplateFuncs["phaseColor"].(func(string) string)
	cases := []struct{ phase, want string }{
		{"Succeeded", "green"},
		{"Failed", "red"},
		{"Running", "blue"},
		{"Running: syncing", "blue"},
		{"Pending", "yellow"},
		{"Unknown", "gray"},
		{"", "gray"},
	}
	for _, c := range cases {
		if got := phaseColor(c.phase); got != c.want {
			t.Errorf("phaseColor(%q) = %q, want %q", c.phase, got, c.want)
		}
	}
}

func TestCRTemplateFunc_PhaseIcon(t *testing.T) {
	phaseIcon := crTemplateFuncs["phaseIcon"].(func(string) string)
	cases := []struct{ phase, want string }{
		{"Succeeded", "✓"},
		{"Failed", "✗"},
		{"Running", "◌"},
		{"Pending", "◷"},
		{"Unknown", "·"},
	}
	for _, c := range cases {
		if got := phaseIcon(c.phase); got != c.want {
			t.Errorf("phaseIcon(%q) = %q, want %q", c.phase, got, c.want)
		}
	}
}

func TestCRTemplateFunc_HasPrefix(t *testing.T) {
	hasPrefix := crTemplateFuncs["hasPrefix"].(func(string, string) bool)
	if !hasPrefix("Running: syncing", "Running") {
		t.Error("hasPrefix(Running: syncing, Running) = false")
	}
	if hasPrefix("Pending", "Running") {
		t.Error("hasPrefix(Pending, Running) = true")
	}
}

// init() merges crTemplateFuncs into templateFuncs, overwriting any existing
// key of the same name (both define "hasPrefix"). Confirms the merge actually
// ran and the shared map ends up with a working implementation either way —
// a regression here would mean templates silently lose a function.
func TestTemplateFuncs_CRFuncsAreMerged(t *testing.T) {
	fn, ok := templateFuncs["phaseColor"]
	if !ok {
		t.Fatal("phaseColor from crTemplateFuncs was not merged into templateFuncs")
	}
	if got := fn.(func(string) string)("Failed"); got != "red" {
		t.Errorf("merged phaseColor(Failed) = %q, want red", got)
	}
}

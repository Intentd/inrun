package web

import "strings"

// CR template function additions.
// Merge these into the existing templateFuncs map in templatefuncs.go.

var crTemplateFuncs = map[string]interface{}{
	// hasPrefix checks if a string starts with a prefix.
	// Used in cr_list.html and cr_detail.html for phase badge logic:
	//   {{ if hasPrefix .Phase "Running" }}
	"hasPrefix": func(s, prefix string) bool {
		return strings.HasPrefix(s, prefix)
	},

	// phaseColor returns a Tailwind CSS color token for a phase string.
	// Centralises the phase → colour mapping so templates stay clean.
	"phaseColor": func(phase string) string {
		switch {
		case phase == "Succeeded":
			return "green"
		case phase == "Failed":
			return "red"
		case strings.HasPrefix(phase, "Running"):
			return "blue"
		case phase == "Pending":
			return "yellow"
		default:
			return "gray"
		}
	},

	// phaseIcon returns a small unicode symbol for a phase.
	"phaseIcon": func(phase string) string {
		switch {
		case phase == "Succeeded":
			return "✓"
		case phase == "Failed":
			return "✗"
		case strings.HasPrefix(phase, "Running"):
			return "◌"
		case phase == "Pending":
			return "◷"
		default:
			return "·"
		}
	},
}

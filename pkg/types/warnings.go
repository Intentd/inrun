package types

import "strings"

// Warnings holds non‑fatal validation messages for this CRD.
// Populated during Blueprint validation (e.g., enrichment, deletion protection overrides).
type Warnings []string

// HasWarnings returns true if there are any warnings for this CRD.
func (w *Warnings) HasWarnings() bool {
	if w == nil {
		return false
	}
	return len(*w) > 0
}

// AddWarning appends a warning message.
func (w *Warnings) AddWarning(msg string) {
	*w = append(*w, msg)
}

// MergeWarnings adds all warnings from another slice.
func (w *Warnings) MergeWarnings(other Warnings) {
	*w = append(*w, other...)
}

// Contains reports whether any warning message contains text as a substring.
func (w *Warnings) Contains(text string) bool {
	for _, msg := range *w {
		if strings.Contains(msg, text) {
			return true
		}
	}
	return false
}

func (w *Warnings) String() string {
	return strings.Join(*w, ",")
}

package types

import "strings"

// Info holds non‑fatal validation messages for this CRD.
// Populated during Blueprint validation (e.g., enrichments).
type Info []string

// HasInfo returns true if there are any informational messages.
func (w *Info) HasInfo() bool {
	if w == nil {
		return false
	}
	return len(*w) > 0
}

// AddInfo appends an informational message.
func (w *Info) AddInfo(msg string) {
	*w = append(*w, msg)
}

// MergeInfo adds all info from another slice.
func (w *Info) MergeInfo(other Info) {
	*w = append(*w, other...)
}

// Contains reports whether any informational message contains text as a substring.
func (w *Info) Contains(text string) bool {
	for _, msg := range *w {
		if strings.Contains(msg, text) {
			return true
		}
	}
	return false
}

func (w *Info) String() string {
	return strings.Join(*w, ",")
}

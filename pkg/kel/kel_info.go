package note

import "strings"

// NoteInfo holds discoverable metadata for one note function.
// Populated from BuiltinNotes (list_generated.go) which is produced by
// make generate-notes — do not edit list_generated.go directly.
type NoteInfo struct {
	Name        string   // template function name, e.g. "cronToMap"
	Domain      string   // note family, e.g. "cron", "strings", "kubernetes"
	Description string   // one-line summary
	Example     string   // short usage snippet (optional)
	SeeAlso     []string // related note names
	Keywords    []string // search tags parsed from the "Keywords:" line in the doc
}

// ListNotes returns all built-in notes sorted by domain then name.
func ListNotes() []NoteInfo {
	return BuiltinNotes
}

// ListByDomain returns notes whose Domain matches the given value (case-insensitive).
func ListByDomain(domain string) []NoteInfo {
	domain = strings.ToLower(domain)
	var out []NoteInfo
	for _, n := range BuiltinNotes {
		if strings.ToLower(n.Domain) == domain {
			out = append(out, n)
		}
	}
	return out
}

// GetNote returns the NoteInfo for the given name (case-insensitive). Returns
// the zero value and false when not found.
func GetNote(name string) (NoteInfo, bool) {
	name = strings.ToLower(name)
	for _, n := range BuiltinNotes {
		if strings.ToLower(n.Name) == name {
			return n, true
		}
	}
	return NoteInfo{}, false
}

// SearchNotes returns notes whose Name, Description, Example, or Keywords
// contains term (case-insensitive).
func SearchNotes(term string) []NoteInfo {
	term = strings.ToLower(term)
	var out []NoteInfo
	for _, n := range BuiltinNotes {
		if strings.Contains(strings.ToLower(n.Name), term) ||
			strings.Contains(strings.ToLower(n.Description), term) ||
			strings.Contains(strings.ToLower(n.Example), term) {
			out = append(out, n)
			continue
		}
		for _, kw := range n.Keywords {
			if strings.Contains(kw, term) {
				out = append(out, n)
				break
			}
		}
	}
	return out
}

// Domains returns the distinct domain values present in BuiltinNotes, in the
// order they first appear.
func Domains() []string {
	seen := map[string]bool{}
	var out []string
	for _, n := range BuiltinNotes {
		if !seen[n.Domain] {
			seen[n.Domain] = true
			out = append(out, n.Domain)
		}
	}
	return out
}

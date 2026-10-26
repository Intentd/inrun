package kubectl

import "strings"

// Kind classifies a kubectl failure so callers can branch on it without
// string-matching stderr.
type Kind string

const (
	KindNotFound  Kind = "not-found"
	KindForbidden Kind = "forbidden"
	KindInvalid   Kind = "invalid"
	KindUnknown   Kind = "unknown"
)

// CommandError is a failed kubectl invocation with a classified Kind.
type CommandError struct {
	Kind   Kind
	Output string
	Err    error
}

func (e *CommandError) Error() string { return e.Err.Error() }
func (e *CommandError) Unwrap() error { return e.Err }

// Classify wraps err in a CommandError with Kind inferred from the output.
// Returns nil when err is nil.
func Classify(out string, err error) error {
	if err == nil {
		return nil
	}
	return &CommandError{Kind: kindOf(out), Output: out, Err: err}
}

func kindOf(out string) Kind {
	o := strings.ToLower(out)
	switch {
	case strings.Contains(o, "(notfound)"), strings.Contains(o, "not found"):
		return KindNotFound
	case strings.Contains(o, "forbidden"), strings.Contains(o, "cannot "):
		return KindForbidden
	case strings.Contains(o, "invalid"), strings.Contains(o, "error validating"):
		return KindInvalid
	default:
		return KindUnknown
	}
}

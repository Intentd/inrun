package blueprint

import "errors"

// ErrTypedOperator is the sentinel matched by errors.Is for typed-operator
// failures that originated from a registry source (not a local file).
var ErrTypedOperator = errors.New("typed operator requires a custom runtime image")

// TypedOperatorError wraps a typed-operator failure with the registry ref
// that sourced the CRD. Only returned when the CRD came through
// loadRegistrySource (RegistryRef is set); local development hits the raw error.
type TypedOperatorError struct {
	Ref string
	Err error
}

func (e *TypedOperatorError) Is(target error) bool { return target == ErrTypedOperator }
func (e *TypedOperatorError) Unwrap() error        { return e.Err }
func (e *TypedOperatorError) Error() string        { return e.Err.Error() }

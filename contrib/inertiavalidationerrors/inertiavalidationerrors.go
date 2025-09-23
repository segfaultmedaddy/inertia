package inertiavalidationerrors

import (
	"encoding/gob"

	"go.inout.gg/inertia"
)

var (
	_ error                     = (*MapError)(nil)
	_ inertia.ValidationErrorer = (*MapError)(nil)
)

//nolint:gochecknoinits
func init() {
	gob.Register(&MapError{})
}

// MapError is a map of key-value pairs that can be used as validation errors.
// Key is the field name and value is the error message.
type MapError map[string]string

func (m MapError) ValidationErrors() []inertia.ValidationError {
	errors := make([]inertia.ValidationError, 0, len(m))
	for k, v := range m {
		errors = append(errors, inertia.NewValidationError(k, v))
	}

	return errors
}

func (m MapError) Error() string    { return "validation errors" }
func (m MapError) Len() int         { return len(m) }
func (m MapError) ErrorBag() string { return inertia.DefaultErrorBag }

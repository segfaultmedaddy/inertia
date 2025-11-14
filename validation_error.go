package inertia

import (
	"encoding/gob"
)

var (
	_ error = (*validationError)(nil)
	_ error = (*ValidationErrors)(nil)

	_ ValidationError   = (*validationError)(nil)
	_ ValidationErrorer = (*validationError)(nil)
	_ ValidationErrorer = (*ValidationErrors)(nil)
)

const (
	DefaultErrorBag = ""
)

//nolint:gochecknoinits
func init() {
	gob.Register(&validationError{}) //nolint:exhaustruct
	gob.Register(&ValidationErrors{})
}

// ValidationError represents a single field validation failure.
type ValidationError interface {
	// Field returns the name of the field that failed validation.
	Field() string

	// Error returns the human-readable error message describing the validation failure.
	Error() string
}

// ValidationErrorer is a collection of validation errors that can be sent to the client.
type ValidationErrorer interface {
	error

	// ValidationErrors returns all validation errors in the collection.
	ValidationErrors() []ValidationError

	// Len returns the number of validation errors.
	Len() int
}

type validationError struct {
	Field_    string //nolint:revive
	Message_  string //nolint:revive
	ErrorBag_ string //nolint:revive
}

// NewValidationError creates a validation error for a specific field with a message.
// The error is associated with the default error bag.
func NewValidationError(field string, message string) *validationError { //nolint:revive
	return &validationError{
		Field_:    field,
		Message_:  message,
		ErrorBag_: DefaultErrorBag,
	}
}

func (err *validationError) Error() string                       { return err.Message_ }
func (err *validationError) Field() string                       { return err.Field_ }
func (err *validationError) ValidationErrors() []ValidationError { return []ValidationError{err} }
func (err *validationError) Len() int                            { return 1 }

type ValidationErrors []ValidationError

func (errs ValidationErrors) Error() string                       { return "validation errors" }
func (errs ValidationErrors) ValidationErrors() []ValidationError { return errs }
func (errs ValidationErrors) Len() int                            { return len(errs) }

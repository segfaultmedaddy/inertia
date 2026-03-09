package inertia

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidationError(t *testing.T) {
	t.Parallel()

	t.Run("NewValidationError", func(t *testing.T) {
		t.Parallel()

		// arrange
		// act
		err := NewValidationError("email", "Email is invalid")

		// assert
		assert.Equal(t, "email", err.Field_)
		assert.Equal(t, "Email is invalid", err.Message_)
		assert.Empty(t, err.ErrorBag_)
	})

	t.Run("Error returns the message", func(t *testing.T) {
		t.Parallel()

		// arrange
		err := NewValidationError("email", "Email is invalid")

		// act
		result := err.Error()

		// assert
		assert.Equal(t, "Email is invalid", result)
	})

	t.Run("Field returns the field name", func(t *testing.T) {
		t.Parallel()

		// arrange
		err := NewValidationError("email", "Email is invalid")

		// act
		result := err.Field()

		// assert
		assert.Equal(t, "email", result)
	})

	t.Run("ValidationErrors returns a slice with self", func(t *testing.T) {
		t.Parallel()

		// arrange
		err := NewValidationError("email", "Email is invalid")

		// act
		errors := err.ValidationErrors()

		// assert
		require.Len(t, errors, 1)
		assert.Same(t, err, errors[0])
	})

	t.Run("Len returns 1", func(t *testing.T) {
		t.Parallel()

		// arrange
		err := NewValidationError("email", "Email is invalid")

		// act
		result := err.Len()

		// assert
		assert.Equal(t, 1, result)
	})
}

func TestValidationErrors(t *testing.T) {
	t.Parallel()

	t.Run("Error returns static message", func(t *testing.T) {
		t.Parallel()

		// arrange
		errs := ValidationErrors{
			NewValidationError("email", "Email is invalid"),
		}

		// act
		result := errs.Error()

		// assert
		assert.Equal(t, "validation errors", result)
	})

	t.Run("ValidationErrors returns all errors", func(t *testing.T) {
		t.Parallel()

		// arrange
		err1 := NewValidationError("email", "Email is invalid")
		err2 := NewValidationError("password", "Password is too short")
		errs := ValidationErrors{err1, err2}

		// act
		returnedErrs := errs.ValidationErrors()

		// assert
		require.Len(t, returnedErrs, 2)
		assert.Same(t, err1, returnedErrs[0])
		assert.Same(t, err2, returnedErrs[1])
	})

	t.Run("Len returns the number of errors", func(t *testing.T) {
		t.Parallel()

		// arrange
		errs := ValidationErrors{
			NewValidationError("email", "Email is invalid"),
			NewValidationError("password", "Password is too short"),
		}

		// act
		result := errs.Len()

		// assert
		assert.Equal(t, 2, result)
	})

	t.Run("Len returns 0 for empty errors", func(t *testing.T) {
		t.Parallel()

		// arrange
		errs := ValidationErrors{}

		// act
		result := errs.Len()

		// assert
		assert.Equal(t, 0, result)
	})
}

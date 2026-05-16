package ucerr

import (
	"errors"
	"testing"

	"github.com/rotisserie/eris"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewValidationError_PanicsOnEmptyField(t *testing.T) {
	t.Parallel()
	var recovered any
	func() {
		defer func() { recovered = recover() }()
		NewValidationError("", "some message")
	}()
	require.NotNil(t, recovered)
	assert.Equal(t, "ucerr.NewValidationError: field must be non-empty", recovered)
}

func TestNewValidationError_ReturnsPointerAndSetsFields(t *testing.T) {
	t.Parallel()
	got := NewValidationError("x", "y")
	require.NotNil(t, got)
	assert.Equal(t, "x", got.Field)
	assert.Equal(t, "y", got.Message)
}

func TestNewForbiddenError_AcceptsEmptyMessage(t *testing.T) {
	t.Parallel()
	var got *ForbiddenError
	require.NotPanics(t, func() {
		got = NewForbiddenError("")
	})
	require.NotNil(t, got)
	assert.Equal(t, "", got.Message)
}

func TestNewValidationError_ErrorsAsRoundTrip(t *testing.T) {
	t.Parallel()
	orig := NewValidationError("after", "cursor not found")
	wrapped := eris.Wrap(orig, "outer: ctx")

	var ve *ValidationError
	require.True(t, errors.As(wrapped, &ve))
	assert.Equal(t, "after", ve.Field)
	assert.Equal(t, "cursor not found", ve.Message)
}

func TestNewForbiddenError_ErrorsAsRoundTrip(t *testing.T) {
	t.Parallel()
	orig := NewForbiddenError("only admin can demote")
	wrapped := eris.Wrap(orig, "outer: forbidden")

	var fe *ForbiddenError
	require.True(t, errors.As(wrapped, &fe))
	assert.Equal(t, "only admin can demote", fe.Message)
}

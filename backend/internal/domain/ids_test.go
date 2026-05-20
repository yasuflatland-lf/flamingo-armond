package domain

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestNewID(t *testing.T) {
	t.Parallel()

	t.Run("happy path: valid UUID v7 string", func(t *testing.T) {
		t.Parallel()

		got, err := NewID()
		require.NoError(t, err)
		require.NotEmpty(t, got)

		parsed, parseErr := uuid.Parse(got)
		require.NoError(t, parseErr, "returned string must be a valid UUID")
		require.Equal(t, uuid.Version(7), parsed.Version(), "UUID must be version 7")
	})
}

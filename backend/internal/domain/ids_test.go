package domain

import (
	"errors"
	"strings"
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

	// Swaps the package-level seam to simulate crypto/rand unavailability;
	// must not run in parallel because it mutates a package-level variable.
	t.Run("failure path: wrap prefix present in error chain", func(t *testing.T) {
		orig := newV7
		newV7 = func() (uuid.UUID, error) {
			return uuid.UUID{}, errors.New("rand: unavailable")
		}
		t.Cleanup(func() { newV7 = orig })

		_, err := NewID()
		require.Error(t, err)
		require.True(t, strings.Contains(err.Error(), "domain: new uuid v7"),
			"error chain must contain wrap prefix %q, got: %v", "domain: new uuid v7", err)
	})
}

package domain

import (
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// TestNewID does NOT call t.Parallel(): its "failure path" subtest swaps the
// package-level newV7 seam, and a parallel top-level test would run concurrently
// with that swap. Any other parallel test that calls NewID() (directly or via a
// constructor like NewSwipeRecord / NewMasterCard) would then race on newV7.
// Keeping the whole test sequential isolates the swap to the non-parallel phase.
func TestNewID(t *testing.T) {
	t.Run("happy path: valid UUID v7 string", func(t *testing.T) {
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

package usecase

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// TestFilterParseableIDs pins the direct contract of the shared bulk-delete
// pre-filter: malformed (non-UUID) ids are dropped, valid ids survive in their
// original order, and empty input yields an empty slice.
func TestFilterParseableIDs(t *testing.T) {
	t.Parallel()

	valid1 := uuid.NewString()
	valid2 := uuid.NewString()

	got := filterParseableIDs([]string{"not-a-uuid", valid1, "", valid2})
	require.Equal(t, []string{valid1, valid2}, got,
		"valid ids must survive in their original order")

	require.Empty(t, filterParseableIDs(nil), "nil input yields an empty slice")
	require.Empty(t, filterParseableIDs([]string{}), "empty input yields an empty slice")
	require.Empty(t, filterParseableIDs([]string{"junk", "more-junk"}),
		"all-malformed input yields an empty slice")
}

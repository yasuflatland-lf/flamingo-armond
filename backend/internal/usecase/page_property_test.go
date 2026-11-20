package usecase

import (
	"errors"
	"math"
	"slices"
	"testing"

	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"

	"backend/internal/usecase/ucerr"
)

// genPageArg draws a page-size argument; half the draws are the boundary values
// where every guard flips.
var genPageArg = rapid.OneOf(
	rapid.IntRange(-3, maxPageSize+3),
	rapid.SampledFrom([]int{math.MinInt, -1, 0, 1, maxPageSize, maxPageSize + 1, math.MaxInt}),
)

// TestTrimAndDetect_Property_TrimsOnlyOverflow: with want > 0 and len > want the
// result is items[:want] with hasMore; otherwise it is the input slice itself
// (nil stays nil) without hasMore.
func TestTrimAndDetect_Property_TrimsOnlyOverflow(t *testing.T) {
	t.Parallel()
	rapid.Check(t, func(t *rapid.T) {
		items := rapid.SliceOfN(rapid.Int(), 0, 30).Draw(t, "items")
		if rapid.IntRange(0, 9).Draw(t, "nil") == 0 {
			items = nil
		}
		want := rapid.IntRange(-3, 35).Draw(t, "want")

		got, more := TrimAndDetect(items, want)
		if want > 0 && len(items) > want {
			require.True(t, more)
			require.Equal(t, items[:want], got)
			return
		}
		require.False(t, more)
		require.Equal(t, items == nil, got == nil)
		require.Equal(t, items, got)
	})
}

// TestAssemblePage_Property_PlusOneFetch: fetch is called once with first+1
// (0 stays 0); a forward page equals TrimAndDetect(rows, first) with
// hasPrev = hasAfter; a count-only request returns the rows untouched with
// both flags false.
func TestAssemblePage_Property_PlusOneFetch(t *testing.T) {
	t.Parallel()
	rapid.Check(t, func(t *rapid.T) {
		first := 0
		if rapid.Bool().Draw(t, "forward") {
			first = rapid.IntRange(1, 20).Draw(t, "first")
		}
		hasAfter := rapid.Bool().Draw(t, "hasAfter")
		rows := rapid.SliceOfN(rapid.Int(), 0, 22).Draw(t, "rows")

		calls := 0
		items, hasNext, hasPrev, err := assemblePage(first, hasAfter, func(want int) ([]int, error) {
			calls++
			require.Equal(t, first+min(first, 1), want)
			return slices.Clone(rows), nil
		})
		require.NoError(t, err)
		require.Equal(t, 1, calls)
		if first > 0 {
			wantItems, wantNext := TrimAndDetect(rows, first)
			require.Equal(t, wantItems, items)
			require.Equal(t, [2]bool{wantNext, hasAfter}, [2]bool{hasNext, hasPrev})
			return
		}
		require.Equal(t, rows, items)
		require.False(t, hasNext || hasPrev)
	})
}

// TestValidateRelayArgs_Property_AfterRequiresPositiveFirst enumerates first in
// {nil, MinInt, -1, 0, 1, MaxInt} and after in {nil, "", "c"}: the arguments are
// rejected iff after is given without first > 0, always with the wire-visible
// ValidationError("after", "after requires first").
func TestValidateRelayArgs_Property_AfterRequiresPositiveFirst(t *testing.T) {
	t.Parallel()
	for _, first := range []*int{nil, intPtr(math.MinInt), intPtr(-1), intPtr(0), intPtr(1), intPtr(math.MaxInt)} {
		for _, after := range []*string{nil, strPtr(""), strPtr("c")} {
			err := validateRelayArgs(first, after)
			if after == nil || (first != nil && *first > 0) {
				require.NoError(t, err, "first=%v after=%v", first, after)
				continue
			}
			var ve *ucerr.ValidationError
			require.True(t, errors.As(err, &ve), "first=%v after=%v: got %v", first, after, err)
			require.Equal(t, [2]string{"after", "after requires first"}, [2]string{ve.Field, ve.Message})
		}
	}
}

// TestResolvePageSize_Property_StandardClampsAdminRejects: with first absent the
// standard resolver returns defaultPageSize and the admin resolver maxPageSize;
// otherwise the standard resolver clamps first into [0, maxPageSize] and the
// admin resolver returns it unchanged when in range and rejects it on field
// "first" otherwise.
func TestResolvePageSize_Property_StandardClampsAdminRejects(t *testing.T) {
	t.Parallel()
	rapid.Check(t, func(t *rapid.T) {
		var first *int
		if rapid.Bool().Draw(t, "set") {
			v := genPageArg.Draw(t, "first")
			first = &v
		}
		std, serr := resolveStandardPageSize(first)
		adm, aerr := resolveAdminPageSize(first)
		require.NoError(t, serr)
		if first == nil {
			require.Equal(t, [2]int{defaultPageSize, maxPageSize}, [2]int{std, adm})
			require.NoError(t, aerr)
			return
		}
		require.Equal(t, min(max(*first, 0), maxPageSize), std)
		if *first < 0 || *first > maxPageSize {
			var ve *ucerr.ValidationError
			require.True(t, errors.As(aerr, &ve), "admin must reject %d", *first)
			require.Equal(t, "first", ve.Field)
			return
		}
		require.NoError(t, aerr)
		require.Equal(t, *first, adm)
	})
}

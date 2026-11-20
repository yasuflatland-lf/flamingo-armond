package domain

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"
)

// TestLearnWindow_Property_JSTDayBounds: Start <= now < End, End-Start == 24h,
// Start is a JST midnight and a fixed point, and NewLearnWindow wires each
// field to its helper.
func TestLearnWindow_Property_JSTDayBounds(t *testing.T) {
	t.Parallel()
	rapid.Check(t, func(t *rapid.T) {
		now := genInstant().Draw(t, "now")
		start, end := StartOfLearnDay(now), EndOfLearnDay(now)
		require.False(t, now.Before(start), "now=%v start=%v", now, start)
		require.True(t, now.Before(end), "now=%v end=%v", now, end)
		require.Equal(t, 24*time.Hour, end.Sub(start))
		local := start.In(learnDayZone)
		require.Equal(t, [4]int{0, 0, 0, 0}, [4]int{local.Hour(), local.Minute(), local.Second(), local.Nanosecond()})
		require.True(t, StartOfLearnDay(start).Equal(start))
		require.Equal(t, LearnDayKey(start), LearnDayKey(now))

		w := NewLearnWindow(now)
		require.True(t, w.Now.Equal(now))
		require.True(t, w.ReviewedBefore.Equal(start))
		require.True(t, w.DueBefore.Equal(end))
		require.True(t, w.CreditReviewedBefore.Equal(CreditReviewedBefore(now)))
	})
}

// TestCreditReviewedBefore_Property_UTCMidnightAtOrBeforeNow: the credit bound is
// the UTC midnight c with c <= now < c+24h.
func TestCreditReviewedBefore_Property_UTCMidnightAtOrBeforeNow(t *testing.T) {
	t.Parallel()
	rapid.Check(t, func(t *rapid.T) {
		now := genInstant().Draw(t, "now")
		c := CreditReviewedBefore(now)
		require.False(t, now.Before(c))
		require.True(t, now.Before(c.Add(24*time.Hour)))
		u := c.UTC()
		require.Equal(t, [4]int{0, 0, 0, 0}, [4]int{u.Hour(), u.Minute(), u.Second(), u.Nanosecond()})
	})
}

// TestEarnsSchedulingCredit_Property_AgreesWithBounds: for lastReview <= now,
// credit == lastReview < CreditReviewedBefore(now) (the serving-side SQL bound),
// ReviewedWithinLearnDay is the complement of last_review < StartOfLearnDay(now),
// and credit is monotone in now.
func TestEarnsSchedulingCredit_Property_AgreesWithBounds(t *testing.T) {
	t.Parallel()
	rapid.Check(t, func(t *rapid.T) {
		now := genInstant().Draw(t, "now")
		back := rapid.Int64Range(0, int64(72*time.Hour)).Draw(t, "back")
		last := now.Add(-time.Duration(back))
		if rapid.Bool().Draw(t, "atBoundary") {
			last = rapid.SampledFrom([]time.Time{StartOfLearnDay(now), CreditReviewedBefore(now)}).Draw(t, "boundary")
		}
		require.Equal(t, last.Before(CreditReviewedBefore(now)), EarnsSchedulingCredit(last, now),
			"last=%v now=%v", last, now)
		require.Equal(t, !last.Before(StartOfLearnDay(now)), ReviewedWithinLearnDay(last, now))

		later := now.Add(time.Duration(rapid.Int64Range(0, int64(48*time.Hour)).Draw(t, "fwd")))
		if EarnsSchedulingCredit(last, now) {
			require.True(t, EarnsSchedulingCredit(last, later))
		}
		// Backward steps and a zero lastReview never earn credit.
		require.False(t, EarnsSchedulingCredit(now, last.Add(-time.Nanosecond)))
		require.False(t, EarnsSchedulingCredit(time.Time{}, now))
	})
}

package domain

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestEarnsSchedulingCredit keeps the worked examples of the UTC-date rule that
// falsified earlier wall-clock reasoning; the general law is
// TestEarnsSchedulingCredit_Property_AgreesWithBounds. The non-UTC row is
// load-bearing: utcCalendarDay must normalise before truncating.
func TestEarnsSchedulingCredit(t *testing.T) {
	t.Parallel()

	utcDate := time.Date(2026, 4, 26, 0, 30, 0, 0, time.UTC)
	cases := []struct {
		name       string
		lastReview time.Time
		now        time.Time
		want       bool
	}{
		{
			name:       "23 hours apart inside one UTC date earns no credit",
			lastReview: utcDate,
			now:        utcDate.Add(23 * time.Hour),
			want:       false,
		},
		{
			name:       "one hour apart across UTC midnight earns credit",
			lastReview: time.Date(2026, 4, 26, 23, 30, 0, 0, time.UTC),
			now:        time.Date(2026, 4, 27, 0, 30, 0, 0, time.UTC),
			want:       true,
		},
		{
			name:       "zero last review earns no credit",
			lastReview: time.Time{},
			now:        utcDate,
			want:       false,
		},
		{
			name:       "non-UTC operands on different local dates but one UTC date earn no credit",
			lastReview: time.Date(2026, 4, 26, 23, 30, 0, 0, learnDayZone),
			now:        time.Date(2026, 4, 27, 8, 30, 0, 0, learnDayZone),
			want:       false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, EarnsSchedulingCredit(tc.lastReview, tc.now))
		})
	}
}

// TestReviewedWithinLearnDay pins the truth table at the boundary instant:
// the predicate complements the serving-side JST guard
// `last_review < StartOfLearnDay(now)`, so a review exactly at the boundary
// counts as reviewed today while one a nanosecond earlier belongs to the
// previous learn day.
func TestReviewedWithinLearnDay(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 6, 5, 3, 0, 0, 0, time.UTC) // 12:00 JST
	boundary := StartOfLearnDay(now)                   // 2026-06-04T15:00Z (2026-06-05 00:00 JST)

	cases := []struct {
		name       string
		lastReview time.Time
		want       bool
	}{
		{
			name:       "exactly at StartOfLearnDay counts as reviewed today",
			lastReview: boundary,
			want:       true,
		},
		{
			name:       "one nanosecond before the boundary is the previous learn day",
			lastReview: boundary.Add(-time.Nanosecond),
			want:       false,
		},
		{
			name:       "after the boundary counts as reviewed today",
			lastReview: boundary.Add(time.Hour),
			want:       true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, ReviewedWithinLearnDay(tc.lastReview, now))
		})
	}
}

// TestDueBeforeEndOfLearnDay pins the truth table at the exclusive end-of-day
// boundary: the predicate mirrors the serving-side `ucs.due < EndOfLearnDay(now)`,
// so a card due exactly at the next JST midnight belongs to tomorrow.
func TestDueBeforeEndOfLearnDay(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 9, 29, 0, 0, 0, 0, time.UTC) // 09:00 JST
	end := EndOfLearnDay(now)                           // 2026-09-29T15:00Z (2026-09-30 00:00 JST)
	// 01:00 JST on 09-30 while the UTC date is still 09-29: a day end derived
	// from the UTC date (2026-09-29T15:00Z) would be a whole JST day too early.
	afterJSTMidnight := time.Date(2026, 9, 29, 16, 0, 0, 0, time.UTC)

	cases := []struct {
		name string
		now  time.Time
		due  time.Time
		want bool
	}{
		{
			name: "exactly at EndOfLearnDay belongs to tomorrow",
			now:  now,
			due:  end,
			want: false,
		},
		{
			name: "one nanosecond before EndOfLearnDay is due today",
			now:  now,
			due:  end.Add(-time.Nanosecond),
			want: true,
		},
		{
			name: "overdue since yesterday is due",
			now:  now,
			due:  time.Date(2026, 9, 28, 12, 0, 0, 0, time.UTC),
			want: true,
		},
		{
			name: "due later today in JST is due",
			now:  now,
			due:  time.Date(2026, 9, 29, 12, 0, 0, 0, time.UTC),
			want: true,
		},
		{
			name: "due later the same JST day before the UTC date catches up is due",
			now:  afterJSTMidnight,
			due:  time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC), // 21:00 JST on 09-30
			want: true,
		},
		{
			name: "exactly at EndOfLearnDay before the UTC date catches up belongs to tomorrow",
			now:  afterJSTMidnight,
			due:  time.Date(2026, 9, 30, 15, 0, 0, 0, time.UTC), // 2026-10-01 00:00 JST
			want: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, DueBeforeEndOfLearnDay(tc.due, tc.now))
		})
	}
}

func TestCreditReviewedBefore_AdmitsOvernightReviewUnderTwentyFourHours(t *testing.T) {
	t.Parallel()

	lastReview := time.Date(2026, 7, 18, 23, 0, 0, 0, learnDayZone)
	now := time.Date(2026, 7, 19, 9, 0, 0, 0, learnDayZone)

	require.True(t, lastReview.Before(CreditReviewedBefore(now)))
	require.True(t, EarnsSchedulingCredit(lastReview, now))
	require.Less(t, now.Sub(lastReview), 24*time.Hour)
	require.True(t, lastReview.Before(StartOfLearnDay(now)))
}

func TestCreditReviewedBefore_WithholdsSameUTCDateReview(t *testing.T) {
	t.Parallel()

	lastReview := time.Date(2026, 7, 18, 9, 0, 0, 0, learnDayZone)
	now := time.Date(2026, 7, 19, 0, 0, 0, 0, learnDayZone)

	require.False(t, lastReview.Before(CreditReviewedBefore(now)))
	require.False(t, EarnsSchedulingCredit(lastReview, now))
	require.True(t, lastReview.Before(StartOfLearnDay(now)))
	require.Equal(t, 15*time.Hour, now.Sub(lastReview))
}

// windowMembership evaluates lastReview against the review window's two last_review
// guards and against the practice pool's inclusive lower bound.
func windowMembership(w LearnWindow, lastReview time.Time) (reviewEligible, practice bool) {
	reviewEligible = lastReview.Before(w.ReviewedBefore) && lastReview.Before(w.CreditReviewedBefore)
	practice = !lastReview.Before(w.PracticeReviewedAfter())
	return reviewEligible, practice
}

// TestLearnWindow_PracticeReviewedAfter_ComplementsReviewGuards pins that practice is
// the exact complement of the review window's two last_review guards (due is not
// modelled) over a 48-hour half-hour grid, and that the practice bound agrees with
// HandleSwipe's replay-guard disjunction.
func TestLearnWindow_PracticeReviewedAfter_ComplementsReviewGuards(t *testing.T) {
	t.Parallel()

	base := time.Date(2026, 7, 18, 0, 0, 0, 0, time.UTC)
	for i := 0; i <= 96; i++ {
		for j := i; j <= 96; j++ {
			lastReview := base.Add(time.Duration(i) * 30 * time.Minute)
			now := base.Add(time.Duration(j) * 30 * time.Minute)
			reviewEligible, practice := windowMembership(NewLearnWindow(now), lastReview)
			require.NotEqual(t, reviewEligible, practice,
				"practice and review must be complements at (+%d, +%d) half-hours", i, j)
			require.Equal(t,
				ReviewedWithinLearnDay(lastReview, now) || !EarnsSchedulingCredit(lastReview, now),
				practice,
				"practice bound and swipe replay guard must agree at (+%d, +%d) half-hours", i, j)
		}
	}
}

// TestLearnWindow_PracticeReviewedAfter_Band pins the 00:00-09:00 JST band, where
// the UTC date start is the earlier bound, and the return to the JST day start at
// 09:00 JST.
func TestLearnWindow_PracticeReviewedAfter_Band(t *testing.T) {
	t.Parallel()

	inBand := time.Date(2026, 7, 18, 23, 0, 0, 0, time.UTC)   // 08:00 JST on 2026-07-19
	afterBand := time.Date(2026, 7, 19, 0, 0, 0, 0, time.UTC) // 09:00 JST on 2026-07-19
	cases := []struct {
		name            string
		now             time.Time
		lastReview      time.Time
		wantPractice    bool
		wantReview      bool
		wantCreditBound bool
	}{
		{
			name:            "08:00 JST, reviewed 23:59 JST the day before",
			now:             inBand,
			lastReview:      time.Date(2026, 7, 18, 14, 59, 0, 0, time.UTC),
			wantPractice:    true,
			wantCreditBound: true,
		},
		{
			name:            "08:00 JST, reviewed 20:00 JST the day before",
			now:             inBand,
			lastReview:      time.Date(2026, 7, 18, 11, 0, 0, 0, time.UTC),
			wantPractice:    true,
			wantCreditBound: true,
		},
		{
			name:            "08:00 JST, reviewed exactly at the UTC date start",
			now:             inBand,
			lastReview:      time.Date(2026, 7, 18, 0, 0, 0, 0, time.UTC),
			wantPractice:    true,
			wantCreditBound: true,
		},
		{
			name:            "08:00 JST, reviewed 08:59 JST the day before",
			now:             inBand,
			lastReview:      time.Date(2026, 7, 17, 23, 59, 0, 0, time.UTC),
			wantReview:      true,
			wantCreditBound: true,
		},
		{
			name:            "08:00 JST, reviewed 00:30 JST the same day",
			now:             inBand,
			lastReview:      time.Date(2026, 7, 18, 15, 30, 0, 0, time.UTC),
			wantPractice:    true,
			wantCreditBound: true,
		},
		{
			name:       "09:00 JST, reviewed 23:00 JST the day before",
			now:        afterBand,
			lastReview: time.Date(2026, 7, 18, 14, 0, 0, 0, time.UTC),
			wantReview: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			w := NewLearnWindow(tc.now)
			reviewEligible, practice := windowMembership(w, tc.lastReview)
			require.Equal(t, tc.wantPractice, practice)
			require.Equal(t, tc.wantReview, reviewEligible)
			wantBound := w.ReviewedBefore
			if tc.wantCreditBound {
				wantBound = w.CreditReviewedBefore
			}
			require.True(t, w.PracticeReviewedAfter().Equal(wantBound),
				"PracticeReviewedAfter: got %v, want %v", w.PracticeReviewedAfter(), wantBound)
		})
	}
}

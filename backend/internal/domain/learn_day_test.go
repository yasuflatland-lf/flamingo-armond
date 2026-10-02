package domain

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestEarnsSchedulingCredit pins the rule to UTC calendar dates rather than
// wall-clock distance: 23 hours inside one UTC date earns nothing while one hour
// across UTC midnight earns credit. The non-UTC row is load-bearing —
// utcCalendarDay must normalise before truncating, so a fixture whose operands
// straddle a JST date but share a UTC date would pass an implementation that
// truncated in the operand's own location. Two rows carry the zero/backward
// guard: the full-day-backward step and the zero lastReview, both of which the
// bare date comparison would credit. The one-second-backward step is a boundary
// case only — it shares a UTC date with lastReview, so the date comparison
// already rejects it and it cannot distinguish a guarded implementation.
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
			name:       "exactly 24 hours apart earns credit",
			lastReview: utcDate,
			now:        utcDate.Add(24 * time.Hour),
			want:       true,
		},
		{
			name:       "identical instants earn no credit",
			lastReview: utcDate,
			now:        utcDate,
			want:       false,
		},
		{
			name:       "one-second backward clock step earns no credit",
			lastReview: utcDate,
			now:        utcDate.Add(-time.Second),
			want:       false,
		},
		{
			name:       "full-day backward clock step earns no credit",
			lastReview: utcDate,
			now:        utcDate.Add(-24 * time.Hour),
			want:       false,
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

// TestStartOfLearnDay pins the JST (UTC+9) start-of-day boundary used by the
// learn and practice windows: midnight at or before now, returned as an
// absolute instant. Only the instant matters, not the input's location.
func TestStartOfLearnDay(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		now  time.Time
		want time.Time
	}{
		{
			name: "just before JST midnight maps to the previous day's boundary",
			now:  time.Date(2026, 6, 5, 14, 59, 59, 0, time.UTC), // 23:59:59 JST
			want: time.Date(2026, 6, 4, 15, 0, 0, 0, time.UTC),   // 2026-06-05 00:00 JST
		},
		{
			name: "just after JST midnight maps to the current day's boundary",
			now:  time.Date(2026, 6, 5, 15, 0, 1, 0, time.UTC), // 2026-06-06 00:00:01 JST
			want: time.Date(2026, 6, 5, 15, 0, 0, 0, time.UTC), // 2026-06-06 00:00 JST
		},
		{
			name: "JST noon",
			now:  time.Date(2026, 6, 5, 3, 0, 0, 0, time.UTC), // 12:00 JST
			want: time.Date(2026, 6, 4, 15, 0, 0, 0, time.UTC),
		},
		{
			name: "UTC input is converted into JST before truncation",
			now:  time.Date(2026, 6, 6, 16, 30, 0, 0, time.UTC), // 2026-06-07 01:30 JST
			want: time.Date(2026, 6, 6, 15, 0, 0, 0, time.UTC),  // 2026-06-07 00:00 JST
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := StartOfLearnDay(tc.now)
			require.True(t, got.Equal(tc.want), "got %v, want instant %v", got, tc.want)
		})
	}
}

// TestEndOfLearnDay pins the exclusive JST end-of-day boundary used by the
// review window, including the exact local-midnight transition.
func TestEndOfLearnDay(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		now  time.Time
		want time.Time
	}{
		{
			name: "23:59:59 JST ends at the next midnight",
			now:  time.Date(2026, 6, 5, 14, 59, 59, 0, time.UTC),
			want: time.Date(2026, 6, 5, 15, 0, 0, 0, time.UTC),
		},
		{
			name: "00:00:00 JST ends at the following midnight",
			now:  time.Date(2026, 6, 5, 15, 0, 0, 0, time.UTC),
			want: time.Date(2026, 6, 6, 15, 0, 0, 0, time.UTC),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := EndOfLearnDay(tc.now)
			require.True(t, got.Equal(tc.want), "got %v, want instant %v", got, tc.want)
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

// TestNewLearnWindow pins each field of the canonical window to its boundary
// formula, all derived from the same now.
func TestNewLearnWindow(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 6, 5, 3, 0, 0, 0, time.UTC) // 12:00 JST
	got := NewLearnWindow(now)

	require.True(t, got.Now.Equal(now), "Now must be the input instant")
	require.True(t, got.ReviewedBefore.Equal(StartOfLearnDay(now)),
		"ReviewedBefore must be StartOfLearnDay(now)")
	require.True(t, got.DueBefore.Equal(EndOfLearnDay(now)),
		"DueBefore must be EndOfLearnDay(now)")
	require.True(t, got.CreditReviewedBefore.Equal(CreditReviewedBefore(now)),
		"CreditReviewedBefore must be CreditReviewedBefore(now)")
}

// TestCreditReviewedBefore pins the review window's exclusive credit
// bound to UTC midnight on now's UTC calendar date.
func TestCreditReviewedBefore(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		now  time.Time
		want time.Time
	}{
		{
			name: "08:00 JST maps to the previous UTC date's midnight",
			now:  time.Date(2026, 7, 18, 23, 0, 0, 0, time.UTC),
			want: time.Date(2026, 7, 18, 0, 0, 0, 0, time.UTC),
		},
		{
			name: "09:00 JST is exactly UTC midnight",
			now:  time.Date(2026, 7, 19, 0, 0, 0, 0, time.UTC),
			want: time.Date(2026, 7, 19, 0, 0, 0, 0, time.UTC),
		},
		{
			name: "JST noon",
			now:  time.Date(2026, 7, 19, 3, 0, 0, 0, time.UTC),
			want: time.Date(2026, 7, 19, 0, 0, 0, 0, time.UTC),
		},
		{
			name: "just before JST midnight",
			now:  time.Date(2026, 7, 19, 14, 59, 59, 0, time.UTC),
			want: time.Date(2026, 7, 19, 0, 0, 0, 0, time.UTC),
		},
		{
			name: "non-UTC input is normalised to UTC before truncation",
			now:  time.Date(2026, 7, 19, 12, 0, 0, 0, learnDayZone),
			want: time.Date(2026, 7, 19, 0, 0, 0, 0, time.UTC),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := CreditReviewedBefore(tc.now)
			require.True(t, got.Equal(tc.want), "got %v, want %v", got, tc.want)
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

// TestCreditReviewedBefore_AgreesWithEarnsSchedulingCredit pins the invariant
// this boundary buys: drift on either serving or recording side must fail over
// every forward pair in a 48-hour grid.
func TestCreditReviewedBefore_AgreesWithEarnsSchedulingCredit(t *testing.T) {
	t.Parallel()

	base := time.Date(2026, 7, 18, 0, 0, 0, 0, time.UTC)
	for i := 0; i <= 48; i++ {
		for j := i; j <= 48; j++ {
			lastReview := base.Add(time.Duration(i) * time.Hour)
			now := base.Add(time.Duration(j) * time.Hour)
			require.Equal(t,
				EarnsSchedulingCredit(lastReview, now),
				lastReview.Before(CreditReviewedBefore(now)),
				"serving-side bound and recording-side credit rule must agree at (+%dh, +%dh)", i, j,
			)
		}
	}
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

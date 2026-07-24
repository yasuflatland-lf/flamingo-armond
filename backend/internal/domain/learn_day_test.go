package domain

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

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
// rescue window, including the exact local-midnight transition.
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
// the predicate is the exact complement of the serving-side SQL
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

// TestNewLearnWindow pins each field of the canonical window to its boundary
// formula, all derived from the same now.
func TestNewLearnWindow(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 6, 5, 3, 0, 0, 0, time.UTC) // 12:00 JST
	got := NewLearnWindow(now)

	require.True(t, got.Now.Equal(now), "Now must be the input instant")
	require.True(t, got.ReviewedBefore.Equal(StartOfLearnDay(now)),
		"ReviewedBefore must be StartOfLearnDay(now)")
	require.True(t, got.RescueDueBefore.Equal(EndOfLearnDay(now)),
		"RescueDueBefore must be EndOfLearnDay(now)")
	require.True(t, got.RescueReviewedBefore.Equal(RescueReviewedBefore(now)),
		"RescueReviewedBefore must be RescueReviewedBefore(now)")
}

// TestRescueReviewedBefore pins the rescue window's minimum-elapsed floor at
// exactly 24 hours before now, and pins its relation to the day boundaries: the
// floor is never later than the JST start-of-day, so it is always the tighter of
// the two last_review bounds the rescue predicate applies.
func TestRescueReviewedBefore(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		now  time.Time
	}{
		{name: "JST noon", now: time.Date(2026, 6, 5, 3, 0, 0, 0, time.UTC)},
		{name: "exactly JST midnight", now: time.Date(2026, 6, 5, 15, 0, 0, 0, time.UTC)},
		{name: "just before JST midnight", now: time.Date(2026, 6, 5, 14, 59, 59, 0, time.UTC)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := RescueReviewedBefore(tc.now)
			require.True(t, got.Equal(tc.now.Add(-24*time.Hour)),
				"got %v, want exactly 24h before %v", got, tc.now)
			require.False(t, got.After(StartOfLearnDay(tc.now)),
				"the rescue floor must never be later than the JST start-of-day")
		})
	}
}

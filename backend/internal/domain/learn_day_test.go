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

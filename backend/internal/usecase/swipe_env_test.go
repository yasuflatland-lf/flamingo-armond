package usecase_test

import (
	"log/slog"
	"testing"

	"backend/internal/usecase"
)

func TestSwipeNextBatchSize(t *testing.T) {
	cases := []struct {
		name string
		env  string
		want int
	}{
		{"empty uses default", "", 10},
		{"valid positive", "20", 20},
		{"invalid string uses default", "bad", 10},
		{"zero uses default", "0", 10},
		{"negative uses default", "-5", 10},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("SWIPE_NEXT_BATCH_SIZE", tc.env)
			got := usecase.SwipeNextBatchSize(slog.Default())
			if got != tc.want {
				t.Errorf("SwipeNextBatchSize(%q) = %d, want %d", tc.env, got, tc.want)
			}
		})
	}
}

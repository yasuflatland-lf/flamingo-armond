package domain

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRatingIsValid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input Rating
		want  bool
	}{
		{"RatingAgain", RatingAgain, true},
		{"RatingHard", RatingHard, true},
		{"RatingGood", RatingGood, true},
		{"RatingEasy", RatingEasy, true},
		{"zero", Rating(0), false},
		{"five", Rating(5), false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, tc.input.IsValid())
		})
	}
}

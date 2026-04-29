package domain

import "github.com/rotisserie/eris"

// Rating is the FSRS review grade used by the domain. Values intentionally
// match go-fsrs ratings.
type Rating int

const (
	RatingAgain Rating = 1
	RatingHard  Rating = 2
	RatingGood  Rating = 3
	RatingEasy  Rating = 4
)

// RatingFromSwipeMode maps the legacy 3-step swipe UI to FSRS ratings. The UI
// emits 1=Again, 2=Hard, and 4=Easy; it intentionally does not emit 3=Good.
func RatingFromSwipeMode(mode int) (Rating, error) {
	switch mode {
	case 1:
		return RatingAgain, nil
	case 2:
		return RatingHard, nil
	case 4:
		return RatingEasy, nil
	default:
		return 0, eris.Errorf("rating: unknown swipe mode %d", mode)
	}
}

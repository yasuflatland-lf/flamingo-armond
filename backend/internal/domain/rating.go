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

// IsValid reports whether the rating is in the recognised FSRS range.
func (r Rating) IsValid() bool {
	return r >= RatingAgain && r <= RatingEasy
}

// IsSuccess reports whether the rating counts as a successful review (Good or
// Easy). The 3-step swipe UI never emits RatingGood (RatingFromSwipe yields
// only 1/2/4), so "success" is effectively Easy today; the predicate stays
// >= RatingGood to remain correct if Good is ever wired.
func (r Rating) IsSuccess() bool { return r >= RatingGood }

// RatingFromSwipe maps the 3-step swipe UI's raw rating to an FSRS rating. The
// UI emits 1=Again, 2=Hard, and 4=Easy; it intentionally does not emit 3=Good.
func RatingFromSwipe(rating int) (Rating, error) {
	switch rating {
	case 1:
		return RatingAgain, nil
	case 2:
		return RatingHard, nil
	case 4:
		return RatingEasy, nil
	default:
		return 0, eris.Errorf("rating: unknown swipe rating %d", rating)
	}
}

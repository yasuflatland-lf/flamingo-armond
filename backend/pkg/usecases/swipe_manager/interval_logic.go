package swipe_manager

import (
	repository "backend/graph/db"
	"backend/graph/services"
	"time"
)

// IntervalLogic interface
type IntervalLogic interface {
	UpdateInterval(card *repository.Card, swipe *repository.SwipeRecord)
}

// intervalLogic struct
type intervalLogic struct {
	intervals []int
}

// NewIntervalLogic returns a new instance of intervalLogic.
func NewIntervalLogic() IntervalLogic {
	return &intervalLogic{
		intervals: []int{1, 3, 7, 14, 30},
	}
}

func (il *intervalLogic) UpdateInterval(card *repository.Card, swipe *repository.SwipeRecord) {
	switch swipe.Mode {
	case services.KNOWN:
		il.increaseInterval(card)
		break
	case services.MAYBE:
		// Do nothing, stay same Interval Days
		break
	case services.DONTKNOW:
		// If you don't know the word, decrease the interval
		il.decreaseInterval(card)
	default:
		il.resetInterval(card)
	}
	// Calculate and set the next review date
	card.ReviewDate = time.Now().AddDate(0, 0, card.IntervalDays)
}

// decreaseInterval decreases the interval to the previous step.
func (il *intervalLogic) decreaseInterval(card *repository.Card) {
	currentIndex := il.findIntervalIndex(card.IntervalDays)

	// Check if the current interval is out of the expected range and reset if necessary
	if currentIndex == 0 {
		card.IntervalDays = il.intervals[0] // Already at the minimum interval
	} else {
		card.IntervalDays = il.intervals[currentIndex-1] // Move to the previous interval
	}
}

func (il *intervalLogic) increaseInterval(card *repository.Card) {
	currentIndex := il.findIntervalIndex(card.IntervalDays)

	// Check if the current interval is out of the expected range and reset if necessary
	if currentIndex == 0 && card.IntervalDays != il.intervals[0] {
		card.IntervalDays = il.intervals[0]
		return
	}

	if currentIndex < len(il.intervals)-1 {
		card.IntervalDays = il.intervals[currentIndex+1]
	} else {
		card.IntervalDays = il.intervals[len(il.intervals)-1] // If the maximum interval is reached
	}
}

// resetInterval resets the interval to the first step.
func (il *intervalLogic) resetInterval(card *repository.Card) {
	card.IntervalDays = il.intervals[0]
}

// findIntervalIndex finds the index of the current interval in the intervals list.
func (il *intervalLogic) findIntervalIndex(days int) int {
	for i, interval := range il.intervals {
		if interval == days {
			return i
		}
	}
	return 0 // Default to the first interval
}

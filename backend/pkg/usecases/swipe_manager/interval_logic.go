package swipe_manager

import (
	"sync"
	"time"
)

// IntervalLogic interface
type IntervalLogic interface {
	UpdateInterval(IntervalDays int, reviewDate time.Time, Mode int) (int, time.Time)
}

// intervalLogic struct
type intervalLogic struct {
	intervals []int
	mu        sync.RWMutex
}

// NewIntervalLogic returns a new instance of intervalLogic.
func NewIntervalLogic() IntervalLogic {
	return &intervalLogic{
		intervals: []int{1, 3, 7, 14, 30},
	}
}

func (il *intervalLogic) UpdateInterval(IntervalDays int, reviewDate time.Time, Mode int) (int, time.Time) {
	il.mu.RLock()
	defer il.mu.RUnlock()

	switch Mode {
	case GOOD:
		fallthrough
	case EASY:
		IntervalDays = il.increaseInterval(IntervalDays)
	default:
		IntervalDays = il.resetInterval()
	}

	reviewDate = time.Now().AddDate(0, 0, IntervalDays)
	return IntervalDays, reviewDate
}

func (il *intervalLogic) decreaseInterval(IntervalDays int) int {
	currentIndex := il.findIntervalIndex(IntervalDays)

	il.mu.Lock()
	defer il.mu.Unlock()

	if currentIndex == 0 {
		return il.intervals[0] // Already at the minimum interval
	}
	return il.intervals[currentIndex-1] // Move to the previous interval
}

func (il *intervalLogic) increaseInterval(IntervalDays int) int {
	currentIndex := il.findIntervalIndex(IntervalDays)

	if currentIndex == 0 && IntervalDays != il.intervals[0] {
		return il.intervals[0]
	}

	if currentIndex < len(il.intervals)-1 {
		return il.intervals[currentIndex+1]
	}
	return il.intervals[len(il.intervals)-1] // If the maximum interval is reached
}

func (il *intervalLogic) resetInterval() int {
	return il.intervals[0]
}

func (il *intervalLogic) findIntervalIndex(days int) int {
	il.mu.RLock()
	defer il.mu.RUnlock()

	for i, interval := range il.intervals {
		if interval == days {
			return i
		}
	}
	return 0 // Default to the first interval
}

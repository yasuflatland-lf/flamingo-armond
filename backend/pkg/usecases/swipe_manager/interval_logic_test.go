package swipe_manager

import (
	repository "backend/graph/db"
	"backend/graph/services"
	"testing"
	"time"
)

func TestIncreaseInterval(t *testing.T) {
	il := NewIntervalLogic().(*intervalLogic)

	t.Run("Normal Range Value 1 to 3", func(t *testing.T) {
		card := &repository.Card{IntervalDays: 1}
		il.increaseInterval(card)
		if card.IntervalDays != 3 {
			t.Errorf("Expected IntervalDays to be 3, got %d", card.IntervalDays)
		}
	})

	t.Run("Normal Range Value 3 to 7", func(t *testing.T) {
		card := &repository.Card{IntervalDays: 3}
		il.increaseInterval(card)
		if card.IntervalDays != 7 {
			t.Errorf("Expected IntervalDays to be 7, got %d", card.IntervalDays)
		}
	})

	t.Run("Edge Case: Max Interval", func(t *testing.T) {
		card := &repository.Card{IntervalDays: 30}
		il.increaseInterval(card)
		if card.IntervalDays != 30 {
			t.Errorf("Expected IntervalDays to remain 30, got %d", card.IntervalDays)
		}
	})

	t.Run("Abnormal Range Value", func(t *testing.T) {
		card := &repository.Card{IntervalDays: 100}
		il.increaseInterval(card)
		if card.IntervalDays != 1 {
			t.Errorf("Expected IntervalDays to reset to 1 for abnormal value, got %d", card.IntervalDays)
		}
	})
}

func TestUpdateInterval(t *testing.T) {
	il := NewIntervalLogic().(*intervalLogic)
	card := &repository.Card{IntervalDays: 1}

	t.Run("Normal Value with Direction Known", func(t *testing.T) {
		swipe := &repository.SwipeRecord{Direction: services.KNOWN}
		il.UpdateInterval(card, swipe)
		expectedDate := time.Now().AddDate(0, 0, 3) // because interval should increase to 3
		if !card.ReviewDate.Truncate(time.Second).Equal(expectedDate.Truncate(time.Second)) {
			t.Errorf("Expected ReviewDate to be %v, got %v", expectedDate, card.ReviewDate)
		}
	})

	t.Run("Reset Interval with Unknown Direction", func(t *testing.T) {
		swipe := &repository.SwipeRecord{Direction: services.DONTKNOW}
		il.UpdateInterval(card, swipe)
		expectedDate := time.Now().AddDate(0, 0, 1) // reset to 1 day
		if !card.ReviewDate.Truncate(time.Second).Equal(expectedDate.Truncate(time.Second)) {
			t.Errorf("Expected ReviewDate to be %v, got %v", expectedDate, card.ReviewDate)
		}
	})

	t.Run("Edge Case with Maxed Out Interval", func(t *testing.T) {
		card.IntervalDays = 30
		swipe := &repository.SwipeRecord{Direction: services.KNOWN}
		il.UpdateInterval(card, swipe)
		expectedDate := time.Now().AddDate(0, 0, 30)
		if !card.ReviewDate.Truncate(time.Second).Equal(expectedDate.Truncate(time.Second)) {
			t.Errorf("Expected ReviewDate to be %v, got %v", expectedDate, card.ReviewDate)
		}
	})
}

func TestFindIntervalIndex(t *testing.T) {
	il := NewIntervalLogic().(*intervalLogic)

	t.Run("Find Index for Interval 1", func(t *testing.T) {
		index := il.findIntervalIndex(1)
		if index != 0 {
			t.Errorf("Expected index to be 0, got %d", index)
		}
	})

	t.Run("Find Index for Interval 7", func(t *testing.T) {
		index := il.findIntervalIndex(7)
		if index != 2 {
			t.Errorf("Expected index to be 2, got %d", index)
		}
	})

	t.Run("Find Index for Abnormal Interval", func(t *testing.T) {
		index := il.findIntervalIndex(100)
		if index != 0 {
			t.Errorf("Expected index to be 0 for abnormal value, got %d", index)
		}
	})
}

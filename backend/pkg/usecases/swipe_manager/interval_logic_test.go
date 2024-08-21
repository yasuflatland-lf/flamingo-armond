package swipe_manager

import (
	"testing"
	"time"
)

func TestIncreaseInterval(t *testing.T) {
	il := NewIntervalLogic().(*intervalLogic)

	t.Run("Normal Range Value 1 to 3", func(t *testing.T) {
		t.Parallel()
		intervalDays := 1
		intervalDays = il.increaseInterval(intervalDays)
		if intervalDays != 3 {
			t.Errorf("Expected IntervalDays to be 3, got %d", intervalDays)
		}
	})

	t.Run("Normal Range Value 3 to 7", func(t *testing.T) {
		t.Parallel()
		intervalDays := 3
		intervalDays = il.increaseInterval(intervalDays)
		if intervalDays != 7 {
			t.Errorf("Expected IntervalDays to be 7, got %d", intervalDays)
		}
	})

	t.Run("Edge Case: Max Interval", func(t *testing.T) {
		t.Parallel()
		intervalDays := 30
		intervalDays = il.increaseInterval(intervalDays)
		if intervalDays != 30 {
			t.Errorf("Expected IntervalDays to remain 30, got %d", intervalDays)
		}
	})

	t.Run("Abnormal Range Value", func(t *testing.T) {
		t.Parallel()
		intervalDays := 100
		intervalDays = il.increaseInterval(intervalDays)
		if intervalDays != 1 {
			t.Errorf("Expected IntervalDays to reset to 1 for abnormal value, got %d", intervalDays)
		}
	})
}

func TestUpdateInterval(t *testing.T) {
	il := NewIntervalLogic().(*intervalLogic)

	t.Run("Normal Value with GOOD Mode", func(t *testing.T) {
		t.Parallel()
		intervalDays := 1
		reviewDate := time.Now()
		mode := GOOD
		updatedDays, _ := il.UpdateInterval(intervalDays, reviewDate, mode)
		expectedDays := 3 // because interval should increase to 3
		if updatedDays != expectedDays {
			t.Errorf("Expected IntervalDays to be %d, got %d", expectedDays, updatedDays)
		}
	})

	t.Run("Normal Value with EASY Mode", func(t *testing.T) {
		t.Parallel()
		intervalDays := 3
		reviewDate := time.Now()
		mode := EASY
		updatedDays, _ := il.UpdateInterval(intervalDays, reviewDate, mode)
		expectedDays := 7 // because interval should increase to 7
		if updatedDays != expectedDays {
			t.Errorf("Expected IntervalDays to be %d, got %d", expectedDays, updatedDays)
		}
	})

	t.Run("Reset Interval with DIFFICULT Mode", func(t *testing.T) {
		t.Parallel()
		intervalDays := 7
		reviewDate := time.Now()
		mode := DIFFICULT
		updatedDays, _ := il.UpdateInterval(intervalDays, reviewDate, mode)
		expectedDays := 1 // because interval should reset to 1
		if updatedDays != expectedDays {
			t.Errorf("Expected IntervalDays to be %d, got %d", expectedDays, updatedDays)
		}
	})

	t.Run("Edge Case with Maxed Out Interval", func(t *testing.T) {
		t.Parallel()
		intervalDays := 30
		reviewDate := time.Now()
		mode := GOOD
		updatedDays, _ := il.UpdateInterval(intervalDays, reviewDate, mode)
		expectedDays := 30 // because it's already max
		if updatedDays != expectedDays {
			t.Errorf("Expected IntervalDays to remain %d, got %d", expectedDays, updatedDays)
		}
	})
}

func TestFindIntervalIndex(t *testing.T) {
	il := NewIntervalLogic().(*intervalLogic)

	t.Run("Find Index for Interval 1", func(t *testing.T) {
		t.Parallel()
		index := il.findIntervalIndex(1)
		if index != 0 {
			t.Errorf("Expected index to be 0, got %d", index)
		}
	})

	t.Run("Find Index for Interval 7", func(t *testing.T) {
		t.Parallel()
		index := il.findIntervalIndex(7)
		if index != 2 {
			t.Errorf("Expected index to be 2, got %d", index)
		}
	})

	t.Run("Find Index for Abnormal Interval", func(t *testing.T) {
		t.Parallel()
		index := il.findIntervalIndex(100)
		if index != 0 {
			t.Errorf("Expected index to be 0 for abnormal value, got %d", index)
		}
	})
}

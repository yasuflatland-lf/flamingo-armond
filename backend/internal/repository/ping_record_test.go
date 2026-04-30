package repository_test

import (
	"context"
	"testing"

	"backend/internal/repository"
)

func TestPingRecord_CreateOnEmpty(t *testing.T) {
	ctx := context.Background()
	repo := repository.NewPingRecordRepository(testDB.GORM)

	t.Cleanup(func() {
		testDB.GORM.Exec("DELETE FROM ping_records")
	})

	n, err := repo.Count(ctx)
	if err != nil {
		t.Fatalf("Count (before create): %v", err)
	}
	if n != 0 {
		t.Fatalf("Count: got %d, want 0", n)
	}

	if err := repo.Create(ctx); err != nil {
		t.Fatalf("Create: %v", err)
	}

	n, err = repo.Count(ctx)
	if err != nil {
		t.Fatalf("Count (after create): %v", err)
	}
	if n != 1 {
		t.Fatalf("Count: got %d, want 1", n)
	}
}

func TestPingRecord_DeleteAllOnPopulated(t *testing.T) {
	ctx := context.Background()
	repo := repository.NewPingRecordRepository(testDB.GORM)

	t.Cleanup(func() {
		testDB.GORM.Exec("DELETE FROM ping_records")
	})

	if err := repo.Create(ctx); err != nil {
		t.Fatalf("Create (seed): %v", err)
	}

	affected, err := repo.DeleteAll(ctx)
	if err != nil {
		t.Fatalf("DeleteAll: %v", err)
	}
	if affected != 1 {
		t.Fatalf("DeleteAll rows affected: got %d, want 1", affected)
	}

	n, err := repo.Count(ctx)
	if err != nil {
		t.Fatalf("Count (after delete): %v", err)
	}
	if n != 0 {
		t.Fatalf("Count: got %d, want 0", n)
	}
}

// TestPingRecord_OscillationFourCycles verifies the create/delete cycle four
// times producing the sequence 0 -> 1 -> 0 -> 1 -> 0.
func TestPingRecord_OscillationFourCycles(t *testing.T) {
	ctx := context.Background()
	repo := repository.NewPingRecordRepository(testDB.GORM)

	t.Cleanup(func() {
		testDB.GORM.Exec("DELETE FROM ping_records")
	})

	// Ensure starting from empty.
	if _, err := repo.DeleteAll(ctx); err != nil {
		t.Fatalf("initial DeleteAll: %v", err)
	}

	assertCount := func(want int64) {
		t.Helper()
		got, err := repo.Count(ctx)
		if err != nil {
			t.Fatalf("Count: %v", err)
		}
		if got != want {
			t.Fatalf("Count: got %d, want %d", got, want)
		}
	}

	assertCount(0)

	// cycle 1: create -> 1
	if err := repo.Create(ctx); err != nil {
		t.Fatalf("cycle1 Create: %v", err)
	}
	assertCount(1)

	// cycle 2: delete -> 0
	if affected, err := repo.DeleteAll(ctx); err != nil || affected == 0 {
		t.Fatalf("cycle2 DeleteAll: err=%v affected=%d", err, affected)
	}
	assertCount(0)

	// cycle 3: create -> 1
	if err := repo.Create(ctx); err != nil {
		t.Fatalf("cycle3 Create: %v", err)
	}
	assertCount(1)

	// cycle 4: delete -> 0
	if affected, err := repo.DeleteAll(ctx); err != nil || affected == 0 {
		t.Fatalf("cycle4 DeleteAll: err=%v affected=%d", err, affected)
	}
	assertCount(0)
}

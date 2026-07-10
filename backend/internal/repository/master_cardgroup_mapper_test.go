package repository

// White-box tests for masterCardgroupToDomain. The mapper is unexported and is
// a pure function of the row struct, so the tests live in the same package and
// need no live DB. They pin the DB-reconstitution guard that rejects an unknown
// MasterCardgroupStatus column value (mirrors the FSRSPhase guard in
// userCardFSRSToDomain / dueCardsFromRows): a corrupt status would otherwise
// flow silently into the domain and make IsPublished() return false, hiding the
// deck instead of surfacing the corruption.

import (
	"testing"
	"time"

	"backend/internal/domain"
)

func TestMasterCardgroupToDomain_InvalidStatus_ReturnsError(t *testing.T) {
	t.Parallel()
	row := gormMasterCardgroup{
		ID:        "00000000-0000-0000-0000-000000000001",
		Name:      "Garbage Status Deck",
		Version:   1,
		Status:    "garbage",
		SortOrder: 0,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}

	got, err := masterCardgroupToDomain(row)
	if err == nil {
		t.Fatalf("expected an error for invalid status, got nil (mapper returned %+v)", got)
	}
	if got != nil {
		t.Fatalf("expected nil aggregate on error, got %+v", got)
	}
}

func TestMasterCardgroupToDomain_ValidStatus_RoundTrips(t *testing.T) {
	t.Parallel()
	for _, status := range []domain.MasterCardgroupStatus{
		domain.MasterStatusDraft,
		domain.MasterStatusPublished,
	} {
		row := gormMasterCardgroup{
			ID:        "00000000-0000-0000-0000-000000000002",
			Name:      "Valid Status Deck",
			Version:   3,
			Status:    string(status),
			SortOrder: 7,
			CreatedAt: time.Now().UTC(),
			UpdatedAt: time.Now().UTC(),
		}

		got, err := masterCardgroupToDomain(row)
		if err != nil {
			t.Fatalf("unexpected error for status %q: %v", status, err)
		}
		if got == nil {
			t.Fatalf("expected non-nil aggregate for status %q", status)
		}
		if got.Status != status {
			t.Fatalf("status not preserved: want %q, got %q", status, got.Status)
		}
		if got.ID != row.ID {
			t.Fatalf("id not preserved: want %q, got %q", row.ID, got.ID)
		}
		if got.Version != row.Version {
			t.Fatalf("version not preserved: want %d, got %d", row.Version, got.Version)
		}
		if got.SortOrder != row.SortOrder {
			t.Fatalf("sort order not preserved: want %d, got %d", row.SortOrder, got.SortOrder)
		}
	}
}

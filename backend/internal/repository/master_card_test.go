package repository_test

// TestMain, testDB, and insertAuthUser are defined in user_test.go and shared
// across this package.

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"backend/internal/domain"
	"backend/internal/repository"
)

// newMCGForCardTest builds a minimal MasterCardgroup ready for Create.
// Distinct from newMasterCardgroup in master_cardgroup_test.go to avoid a
// redeclaration error in the same test package.
func newMCGForCardTest(name string) *domain.MasterCardgroup {
	now := time.Now().UTC().Truncate(time.Microsecond)
	return &domain.MasterCardgroup{
		ID:               uuid.NewString(),
		Name:             domain.CardgroupName(name),
		Version:          1,
		Status:           domain.MasterStatusDraft,
		IsDefaultStarter: false,
		SortOrder:        0,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
}

// insertMCGForCardTest creates a MasterCardgroup row via Create and returns it.
// Distinct from any insertMasterCardgroup helper to avoid redeclaration.
func insertMCGForCardTest(t *testing.T, ctx context.Context, name string) *domain.MasterCardgroup {
	t.Helper()
	repo := repository.NewMasterCardgroupRepository(testDB.GORM)
	mcg := newMCGForCardTest(name)
	require.NoError(t, repo.Create(ctx, mcg))
	return mcg
}

// newMasterCard builds a MasterCard ready for Create/UpsertManyTx.
func newMasterCard(masterCardgroupID, front, back string, position int) *domain.MasterCard {
	now := time.Now().UTC().Truncate(time.Microsecond)
	return &domain.MasterCard{
		ID:                uuid.NewString(),
		MasterCardgroupID: masterCardgroupID,
		Front:             domain.CardText(front),
		Back:              domain.CardText(back),
		Position:          position,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
}

// TestMasterCardRepository_CreateAndListByMasterCardgroup verifies Create +
// ListByMasterCardgroup round-trip and ordering by (position, id).
func TestMasterCardRepository_CreateAndListByMasterCardgroup(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	mcgRepo := repository.NewMasterCardgroupRepository(testDB.GORM)
	mcg := newMCGForCardTest("CreateList-Group-A")
	require.NoError(t, mcgRepo.Create(ctx, mcg))

	repo := repository.NewMasterCardRepository(testDB.GORM)

	// Insert cards at positions 2, 0, 1 — list must return them in position order.
	c2 := newMasterCard(mcg.ID, "CreateList-front-2", "back-2", 2)
	c0 := newMasterCard(mcg.ID, "CreateList-front-0", "back-0", 0)
	c1 := newMasterCard(mcg.ID, "CreateList-front-1", "back-1", 1)
	require.NoError(t, repo.Create(ctx, c2))
	require.NoError(t, repo.Create(ctx, c0))
	require.NoError(t, repo.Create(ctx, c1))

	got, err := repo.ListByMasterCardgroup(ctx, mcg.ID)
	require.NoError(t, err)
	require.Len(t, got, 3)

	// Ordered by position ASC then id ASC.
	require.Equal(t, 0, got[0].Position)
	require.Equal(t, 1, got[1].Position)
	require.Equal(t, 2, got[2].Position)

	// Front, Back, and MasterCardgroupID round-trip correctly.
	require.Equal(t, domain.CardText("CreateList-front-0"), got[0].Front)
	require.Equal(t, domain.CardText("back-0"), got[0].Back)
	require.Equal(t, mcg.ID, got[0].MasterCardgroupID)

	require.Equal(t, domain.CardText("CreateList-front-1"), got[1].Front)
	require.Equal(t, domain.CardText("back-2"), got[2].Back)
}

// TestMasterCardRepository_ListByMasterCardgroup_EmptyID verifies the empty-id
// early return: an empty masterCardgroupID must produce (nil, nil) with no error.
func TestMasterCardRepository_ListByMasterCardgroup_EmptyID(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := repository.NewMasterCardRepository(testDB.GORM)

	got, err := repo.ListByMasterCardgroup(ctx, "")
	require.NoError(t, err)
	require.Nil(t, got)
}

// TestMasterCardRepository_UpsertManyTx_AllInserts verifies that upserting N
// new master cards into a fresh group reports Inserted == N, Updated == 0 and
// that the rows exist after the transaction commits.
func TestMasterCardRepository_UpsertManyTx_AllInserts(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	mcg := insertMCGForCardTest(t, ctx, "Upsert-AllInserts-Group")
	repo := repository.NewMasterCardRepository(testDB.GORM)

	cards := []*domain.MasterCard{
		newMasterCard(mcg.ID, "UpsertIns-f1", "b1", 0),
		newMasterCard(mcg.ID, "UpsertIns-f2", "b2", 1),
		newMasterCard(mcg.ID, "UpsertIns-f3", "b3", 2),
	}

	var result repository.UpsertManyTxResult
	err := testDB.GORM.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var txErr error
		result, txErr = repo.UpsertManyTx(ctx, tx, cards)
		return txErr
	})
	require.NoError(t, err)
	require.Equal(t, int64(3), result.Inserted)
	require.Equal(t, int64(0), result.Updated)

	stored, err := repo.ListByMasterCardgroup(ctx, mcg.ID)
	require.NoError(t, err)
	require.Len(t, stored, 3)
}

// TestMasterCardRepository_UpsertManyTx_AllUpdates verifies that re-upserting
// the same fronts with changed Back values reports Inserted == 0, Updated == N
// and that the Back values in the DB are overwritten.
func TestMasterCardRepository_UpsertManyTx_AllUpdates(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	mcg := insertMCGForCardTest(t, ctx, "Upsert-AllUpdates-Group")
	repo := repository.NewMasterCardRepository(testDB.GORM)

	// Initial insert.
	initial := []*domain.MasterCard{
		newMasterCard(mcg.ID, "UpsertUpd-f1", "old-b1", 0),
		newMasterCard(mcg.ID, "UpsertUpd-f2", "old-b2", 1),
	}
	err := testDB.GORM.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		_, txErr := repo.UpsertManyTx(ctx, tx, initial)
		return txErr
	})
	require.NoError(t, err)

	// Re-upsert the same fronts with new backs.
	updated := []*domain.MasterCard{
		newMasterCard(mcg.ID, "UpsertUpd-f1", "new-b1", 0),
		newMasterCard(mcg.ID, "UpsertUpd-f2", "new-b2", 1),
	}
	var result repository.UpsertManyTxResult
	err = testDB.GORM.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var txErr error
		result, txErr = repo.UpsertManyTx(ctx, tx, updated)
		return txErr
	})
	require.NoError(t, err)
	require.Equal(t, int64(0), result.Inserted)
	require.Equal(t, int64(2), result.Updated)

	stored, err := repo.ListByMasterCardgroup(ctx, mcg.ID)
	require.NoError(t, err)
	require.Len(t, stored, 2)
	byFront := make(map[domain.CardText]*domain.MasterCard, len(stored))
	for _, c := range stored {
		byFront[c.Front] = c
	}
	require.Equal(t, domain.CardText("new-b1"), byFront[domain.CardText("UpsertUpd-f1")].Back)
	require.Equal(t, domain.CardText("new-b2"), byFront[domain.CardText("UpsertUpd-f2")].Back)
}

// TestMasterCardRepository_UpsertManyTx_Mixed verifies that a batch containing
// both new fronts and existing fronts reports the correct Inserted/Updated split.
func TestMasterCardRepository_UpsertManyTx_Mixed(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	mcg := insertMCGForCardTest(t, ctx, "Upsert-Mixed-Group")
	repo := repository.NewMasterCardRepository(testDB.GORM)

	// Pre-insert two rows whose fronts will be in the upsert batch.
	pre := []*domain.MasterCard{
		newMasterCard(mcg.ID, "UpsertMix-shared-1", "old-bs1", 0),
		newMasterCard(mcg.ID, "UpsertMix-shared-2", "old-bs2", 1),
	}
	err := testDB.GORM.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		_, txErr := repo.UpsertManyTx(ctx, tx, pre)
		return txErr
	})
	require.NoError(t, err)

	// Upsert batch: 2 existing fronts + 3 new fronts.
	batch := []*domain.MasterCard{
		newMasterCard(mcg.ID, "UpsertMix-shared-1", "new-bs1", 0),
		newMasterCard(mcg.ID, "UpsertMix-shared-2", "new-bs2", 1),
		newMasterCard(mcg.ID, "UpsertMix-new-1", "bn1", 2),
		newMasterCard(mcg.ID, "UpsertMix-new-2", "bn2", 3),
		newMasterCard(mcg.ID, "UpsertMix-new-3", "bn3", 4),
	}
	var result repository.UpsertManyTxResult
	err = testDB.GORM.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var txErr error
		result, txErr = repo.UpsertManyTx(ctx, tx, batch)
		return txErr
	})
	require.NoError(t, err)
	require.Equal(t, int64(3), result.Inserted)
	require.Equal(t, int64(2), result.Updated)

	stored, err := repo.ListByMasterCardgroup(ctx, mcg.ID)
	require.NoError(t, err)
	require.Len(t, stored, 5)
}

// TestMasterCardRepository_UpsertManyTx_EmptySlice verifies that an empty
// (nil) slice is a no-op: zero counts, no error, no rows inserted.
func TestMasterCardRepository_UpsertManyTx_EmptySlice(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	mcg := insertMCGForCardTest(t, ctx, "Upsert-Empty-Group")
	repo := repository.NewMasterCardRepository(testDB.GORM)

	var result repository.UpsertManyTxResult
	err := testDB.GORM.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var txErr error
		result, txErr = repo.UpsertManyTx(ctx, tx, nil)
		return txErr
	})
	require.NoError(t, err)
	require.Equal(t, repository.UpsertManyTxResult{}, result)

	stored, err := repo.ListByMasterCardgroup(ctx, mcg.ID)
	require.NoError(t, err)
	require.Empty(t, stored)
}

// TestMasterCardRepository_ListFrontsByMasterCardgroupTx verifies that
// ListFrontsByMasterCardgroupTx returns the sorted front values for the group.
func TestMasterCardRepository_ListFrontsByMasterCardgroupTx(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	mcg := insertMCGForCardTest(t, ctx, "ListFronts-Group")
	repo := repository.NewMasterCardRepository(testDB.GORM)

	cards := []*domain.MasterCard{
		newMasterCard(mcg.ID, "ListFronts-cherry", "back-c", 2),
		newMasterCard(mcg.ID, "ListFronts-apple", "back-a", 0),
		newMasterCard(mcg.ID, "ListFronts-banana", "back-b", 1),
	}
	err := testDB.GORM.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		_, txErr := repo.UpsertManyTx(ctx, tx, cards)
		return txErr
	})
	require.NoError(t, err)

	var fronts []string
	err = testDB.GORM.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var txErr error
		fronts, txErr = repo.ListFrontsByMasterCardgroupTx(ctx, tx, mcg.ID)
		return txErr
	})
	require.NoError(t, err)
	require.Equal(t, []string{
		"ListFronts-apple",
		"ListFronts-banana",
		"ListFronts-cherry",
	}, fronts)
}

// TestMasterCardRepository_DeleteByMasterCardgroupAndFrontsTx verifies that the
// method deletes only the matching (master_cardgroup_id, front) pairs:
//   - returns the correct deleted count
//   - does not touch fronts in a different master cardgroup
//   - empty fronts slice is a (0, nil) no-op
func TestMasterCardRepository_DeleteByMasterCardgroupAndFrontsTx(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := repository.NewMasterCardRepository(testDB.GORM)

	mcgA := insertMCGForCardTest(t, ctx, "DeleteByFronts-GroupA")
	mcgB := insertMCGForCardTest(t, ctx, "DeleteByFronts-GroupB")

	// mcgA: "shared" front (to be deleted) and "keep-a" (to survive).
	deleteA := newMasterCard(mcgA.ID, "DeleteByFronts-shared", "back-a", 0)
	keepA := newMasterCard(mcgA.ID, "DeleteByFronts-keep-a", "back-keep", 1)
	// mcgB: same "shared" front but in a different group — must not be touched.
	keepB := newMasterCard(mcgB.ID, "DeleteByFronts-shared", "back-b", 0)
	for _, c := range []*domain.MasterCard{deleteA, keepA, keepB} {
		require.NoError(t, repo.Create(ctx, c))
	}

	// Delete "shared" scoped to mcgA only.
	var affected int64
	err := testDB.GORM.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var txErr error
		affected, txErr = repo.DeleteByMasterCardgroupAndFrontsTx(ctx, tx, mcgA.ID, []string{"DeleteByFronts-shared"})
		return txErr
	})
	require.NoError(t, err)
	require.Equal(t, int64(1), affected)

	// mcgA "shared" row is gone; "keep-a" survives.
	storedA, err := repo.ListByMasterCardgroup(ctx, mcgA.ID)
	require.NoError(t, err)
	require.Len(t, storedA, 1)
	require.Equal(t, domain.CardText("DeleteByFronts-keep-a"), storedA[0].Front)

	// mcgB "shared" row is untouched.
	storedB, err := repo.ListByMasterCardgroup(ctx, mcgB.ID)
	require.NoError(t, err)
	require.Len(t, storedB, 1)
	require.Equal(t, domain.CardText("DeleteByFronts-shared"), storedB[0].Front)

	// Empty fronts slice is a no-op.
	var noopAffected int64
	err = testDB.GORM.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var txErr error
		noopAffected, txErr = repo.DeleteByMasterCardgroupAndFrontsTx(ctx, tx, mcgA.ID, nil)
		return txErr
	})
	require.NoError(t, err)
	require.Equal(t, int64(0), noopAffected)
}

// TestMasterCardRepository_Create_DuplicateFront pins the current pass-through
// behavior: Create on a duplicate (master_cardgroup_id, front) pair returns a
// non-nil error because the uq_master_cards_cg_front unique constraint is
// enforced. Unlike cardRepo.Create, masterCardRepo.Create intentionally does not
// classify the 23505 conflict into a sentinel today (deferred to the future admin
// consumer); callers receive the raw wrapped error.
func TestMasterCardRepository_Create_DuplicateFront(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	mcg := insertMCGForCardTest(t, ctx, "DupFront-Group")
	repo := repository.NewMasterCardRepository(testDB.GORM)

	first := newMasterCard(mcg.ID, "DupFront-same-front", "back-one", 0)
	require.NoError(t, repo.Create(ctx, first))

	second := newMasterCard(mcg.ID, "DupFront-same-front", "back-two", 1)
	err := repo.Create(ctx, second)
	require.Error(t, err, "Create with duplicate (master_cardgroup_id, front) must return an error")

	// Only one row for that front exists — the duplicate was rejected.
	stored, listErr := repo.ListByMasterCardgroup(ctx, mcg.ID)
	require.NoError(t, listErr)
	require.Len(t, stored, 1)
}

// TestMasterCardRepository_Delete verifies Delete by primary key: the row is
// removed on success, and a non-existent id returns ErrNotFound.
func TestMasterCardRepository_Delete(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	mcg := insertMCGForCardTest(t, ctx, "Delete-Group")
	repo := repository.NewMasterCardRepository(testDB.GORM)

	card := newMasterCard(mcg.ID, "Delete-front", "back", 0)
	require.NoError(t, repo.Create(ctx, card))

	// Confirm it exists.
	stored, err := repo.ListByMasterCardgroup(ctx, mcg.ID)
	require.NoError(t, err)
	require.Len(t, stored, 1)

	// Delete it.
	require.NoError(t, repo.Delete(ctx, card.ID))

	// Row is gone.
	after, err := repo.ListByMasterCardgroup(ctx, mcg.ID)
	require.NoError(t, err)
	require.Empty(t, after)

	// Deleting a non-existent id returns ErrNotFound.
	err = repo.Delete(ctx, uuid.NewString())
	require.ErrorIs(t, err, repository.ErrNotFound)
}

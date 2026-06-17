package repository_test

// TestMain, testDB, and insertAuthUser are defined in user_test.go and shared
// across this package.

import (
	"context"
	"fmt"
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

// TestMasterCardRepository_UpsertManyTx_CaseInsensitiveFront verifies the citext
// front column: upserting a case-variant of an existing front ("drive" when
// "Drive" exists) UPDATES the existing row rather than inserting a second one,
// and the stored case is preserved (ON CONFLICT DO UPDATE does not touch front).
func TestMasterCardRepository_UpsertManyTx_CaseInsensitiveFront(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	mcg := insertMCGForCardTest(t, ctx, "Upsert-CaseInsensitive-Group")
	repo := repository.NewMasterCardRepository(testDB.GORM)

	// Seed the capitalized variant.
	err := testDB.GORM.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		_, txErr := repo.UpsertManyTx(ctx, tx, []*domain.MasterCard{
			newMasterCard(mcg.ID, "Drive", "old-back", 0),
		})
		return txErr
	})
	require.NoError(t, err)

	// Upsert the lowercase variant with a new back.
	var result repository.UpsertManyTxResult
	err = testDB.GORM.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var txErr error
		result, txErr = repo.UpsertManyTx(ctx, tx, []*domain.MasterCard{
			newMasterCard(mcg.ID, "drive", "new-back", 0),
		})
		return txErr
	})
	require.NoError(t, err)
	// Matched case-insensitively: update, not a second insert.
	require.Equal(t, int64(0), result.Inserted)
	require.Equal(t, int64(1), result.Updated)

	stored, err := repo.ListByMasterCardgroup(ctx, mcg.ID)
	require.NoError(t, err)
	require.Len(t, stored, 1)
	// Back is overwritten; the stored front keeps its original case (citext
	// preserves storage case, and ON CONFLICT DO UPDATE leaves front untouched).
	require.Equal(t, domain.CardText("new-back"), stored[0].Back)
	require.Equal(t, domain.CardText("Drive"), stored[0].Front)
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

// insertMasterCardsSeq creates n master cards in masterCardgroupID with
// deterministic fronts ("<prefix>-front-0".."<prefix>-front-n-1") at positions
// 0..n-1 and staggered created_at so insertion order matches creation order. It
// re-fetches each row so the returned cards carry DB-rounded timestamps (Postgres
// truncates to microsecond precision; cursor comparisons must use those values).
// The prefix is the per-test isolation token: paginated queries on the shared
// parallel DB scope themselves via a search predicate on this prefix so rows from
// other parallel tests never leak into a no-cursor first=N window (see
// `.claude/rules/go-library-gotchas.md` shared-parallel-db rule).
func insertMasterCardsSeq(t *testing.T, ctx context.Context, repo repository.MasterCardRepository, mcgID, prefix string, n int) []*domain.MasterCard {
	t.Helper()
	now := time.Now().UTC()
	cards := make([]*domain.MasterCard, n)
	for i := 0; i < n; i++ {
		c := newMasterCard(mcgID, fmt.Sprintf("%s-front-%d", prefix, i), "back", i)
		// Stagger timestamps by 1 hour so created_at ordering is unambiguous.
		c.CreatedAt = now.Add(time.Duration(i) * time.Hour)
		c.UpdatedAt = c.CreatedAt
		require.NoError(t, repo.Create(ctx, c))
		cards[i] = c
	}
	for i, c := range cards {
		got, err := repo.ListByMasterCardgroup(ctx, mcgID)
		require.NoError(t, err)
		// ListByMasterCardgroup is position-ordered; find the matching id to pick
		// up the DB-rounded timestamps.
		for _, g := range got {
			if g.ID == c.ID {
				cards[i] = g
				break
			}
		}
	}
	return cards
}

// sortMasterByID returns a copy of cards sorted by ID ascending.
func sortMasterByID(cards []*domain.MasterCard) []*domain.MasterCard {
	out := make([]*domain.MasterCard, len(cards))
	copy(out, cards)
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j-1].ID > out[j].ID; j-- {
			out[j-1], out[j] = out[j], out[j-1]
		}
	}
	return out
}

// TestMasterCardRepository_FindPageByMasterCardgroup_ForwardByPosition seeds N
// master cards and asserts the forward page returns the requested window in
// position order, totalCount reflects every row in the group, and the cursor walk
// advances correctly without skipping or duplicating rows.
func TestMasterCardRepository_FindPageByMasterCardgroup_ForwardByPosition(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	mcg := insertMCGForCardTest(t, ctx, "MCPage-ForwardPos-Group")
	repo := repository.NewMasterCardRepository(testDB.GORM)

	cards := insertMasterCardsSeq(t, ctx, repo, mcg.ID, "MCPage-ForwardPos", 5)

	// Page 1: first=2, no cursor → positions 0, 1.
	got, total, err := repo.FindPageByMasterCardgroup(
		ctx, mcg.ID, nil, nil, 2, 0, repository.MasterCardOrderByPosition, repository.SortAsc, nil,
	)
	require.NoError(t, err)
	require.Equal(t, int64(5), total)
	require.Len(t, got, 2)
	require.Equal(t, cards[0].ID, got[0].ID)
	require.Equal(t, cards[1].ID, got[1].ID)

	// Page 2: after the position-1 cursor → positions 2, 3.
	pos1 := cards[1].Position
	got, total, err = repo.FindPageByMasterCardgroup(
		ctx, mcg.ID,
		&repository.MasterCardCursor{ID: cards[1].ID, Position: &pos1}, nil,
		2, 0, repository.MasterCardOrderByPosition, repository.SortAsc, nil,
	)
	require.NoError(t, err)
	require.Equal(t, int64(5), total)
	require.Len(t, got, 2)
	require.Equal(t, cards[2].ID, got[0].ID)
	require.Equal(t, cards[3].ID, got[1].ID)

	// Page 3: after position-3 → only position 4 remains.
	pos3 := cards[3].Position
	got, total, err = repo.FindPageByMasterCardgroup(
		ctx, mcg.ID,
		&repository.MasterCardCursor{ID: cards[3].ID, Position: &pos3}, nil,
		2, 0, repository.MasterCardOrderByPosition, repository.SortAsc, nil,
	)
	require.NoError(t, err)
	require.Equal(t, int64(5), total)
	require.Len(t, got, 1)
	require.Equal(t, cards[4].ID, got[0].ID)
}

// TestMasterCardRepository_FindPageByMasterCardgroup_HasNextViaPlusOne verifies
// the +1 fetch trick: requesting want+1 rows returns the trailing extra row so the
// usecase can detect hasNextPage, and the last page (want+1 rows but only `want`
// remaining) reports no extra row. The repository itself only applies the LIMIT;
// the usecase trims, so the test asserts the row-count signal the trim consumes.
func TestMasterCardRepository_FindPageByMasterCardgroup_HasNextViaPlusOne(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	mcg := insertMCGForCardTest(t, ctx, "MCPage-PlusOne-Group")
	repo := repository.NewMasterCardRepository(testDB.GORM)

	cards := insertMasterCardsSeq(t, ctx, repo, mcg.ID, "MCPage-PlusOne", 3)
	sorted := sortMasterByID(cards)

	// want=2 → request 3 (want+1). With 3 rows total the result has 3 rows, so the
	// trailing extra row signals hasNextPage=true. Order by ID for a stable expectation.
	page1, total, err := repo.FindPageByMasterCardgroup(
		ctx, mcg.ID, nil, nil, 3, 0, repository.MasterCardOrderByID, repository.SortAsc, nil,
	)
	require.NoError(t, err)
	require.Equal(t, int64(3), total)
	require.Len(t, page1, 3, "+1 fetch returns the trailing extra row")
	require.True(t, len(page1) > 2, "extra row signals hasNextPage=true")

	// The trimmed page (first 2) is the real window; the 3rd is the boundary row.
	require.Equal(t, sorted[0].ID, page1[0].ID)
	require.Equal(t, sorted[1].ID, page1[1].ID)

	// Last page: after the position-1 cursor, want=2 → request 3, only 1 row
	// remains, so no extra row and hasNextPage=false.
	page2, _, err := repo.FindPageByMasterCardgroup(
		ctx, mcg.ID,
		&repository.MasterCardCursor{ID: sorted[1].ID}, nil,
		3, 0, repository.MasterCardOrderByID, repository.SortAsc, nil,
	)
	require.NoError(t, err)
	require.Len(t, page2, 1, "last page returns fewer than want+1 rows")
	require.False(t, len(page2) > 2, "no extra row signals hasNextPage=false")
	require.Equal(t, sorted[2].ID, page2[0].ID)
}

// TestMasterCardRepository_FindPageByMasterCardgroup_BackwardByPosition verifies
// backward paging (last/before): the repository inverts the ORDER BY direction,
// applies LIMIT, then reverses the slice so the returned rows are in forward order
// with the boundary at the tail.
func TestMasterCardRepository_FindPageByMasterCardgroup_BackwardByPosition(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	mcg := insertMCGForCardTest(t, ctx, "MCPage-Backward-Group")
	repo := repository.NewMasterCardRepository(testDB.GORM)

	cards := insertMasterCardsSeq(t, ctx, repo, mcg.ID, "MCPage-Backward", 5)

	// last=2, before=nil → the final two positions in ASC order (3, 4).
	got, total, err := repo.FindPageByMasterCardgroup(
		ctx, mcg.ID, nil, nil, 0, 2, repository.MasterCardOrderByPosition, repository.SortAsc, nil,
	)
	require.NoError(t, err)
	require.Equal(t, int64(5), total)
	require.Len(t, got, 2)
	require.Equal(t, cards[3].ID, got[0].ID)
	require.Equal(t, cards[4].ID, got[1].ID)

	// last=2, before=position-3 cursor → positions 1, 2 in ASC order.
	pos3 := cards[3].Position
	got, _, err = repo.FindPageByMasterCardgroup(
		ctx, mcg.ID,
		nil, &repository.MasterCardCursor{ID: cards[3].ID, Position: &pos3},
		0, 2, repository.MasterCardOrderByPosition, repository.SortAsc, nil,
	)
	require.NoError(t, err)
	require.Len(t, got, 2)
	require.Equal(t, cards[1].ID, got[0].ID)
	require.Equal(t, cards[2].ID, got[1].ID)
}

// TestMasterCardRepository_FindPageByMasterCardgroup_OrderByCreatedAtDesc verifies
// a non-ID, non-position order (created_at DESC) with the secondary id tie-break,
// and that the created_at cursor advances correctly.
func TestMasterCardRepository_FindPageByMasterCardgroup_OrderByCreatedAtDesc(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	mcg := insertMCGForCardTest(t, ctx, "MCPage-CreatedDesc-Group")
	repo := repository.NewMasterCardRepository(testDB.GORM)

	cards := insertMasterCardsSeq(t, ctx, repo, mcg.ID, "MCPage-CreatedDesc", 4)
	// DESC: newest created_at first → cards[3], cards[2], cards[1], cards[0].

	got, total, err := repo.FindPageByMasterCardgroup(
		ctx, mcg.ID, nil, nil, 2, 0, repository.MasterCardOrderByCreatedAt, repository.SortDesc, nil,
	)
	require.NoError(t, err)
	require.Equal(t, int64(4), total)
	require.Len(t, got, 2)
	require.Equal(t, cards[3].ID, got[0].ID)
	require.Equal(t, cards[2].ID, got[1].ID)

	cur := &repository.MasterCardCursor{ID: cards[2].ID, CreatedAt: &cards[2].CreatedAt}
	got, _, err = repo.FindPageByMasterCardgroup(
		ctx, mcg.ID, cur, nil, 2, 0, repository.MasterCardOrderByCreatedAt, repository.SortDesc, nil,
	)
	require.NoError(t, err)
	require.Len(t, got, 2)
	require.Equal(t, cards[1].ID, got[0].ID)
	require.Equal(t, cards[0].ID, got[1].ID)
}

// TestMasterCardRepository_FindPageByMasterCardgroup_Search verifies the
// case-insensitive substring match on front OR back, and that the ILIKE escaping
// treats %/_ as literals (so a row whose text contains those characters is matched
// only by a search that also contains them, never by an unescaped wildcard).
func TestMasterCardRepository_FindPageByMasterCardgroup_Search(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	mcg := insertMCGForCardTest(t, ctx, "MCPage-Search-Group")
	repo := repository.NewMasterCardRepository(testDB.GORM)

	// Rows: one matches on FRONT (uppercase to prove case-insensitivity), one
	// matches on BACK, two do not match the search term at all.
	frontMatch := newMasterCard(mcg.ID, "MCPage-Search-APPLE-front", "no-hit-back", 0)
	backMatch := newMasterCard(mcg.ID, "MCPage-Search-other-front", "ripe apple here", 1)
	miss1 := newMasterCard(mcg.ID, "MCPage-Search-banana", "yellow", 2)
	miss2 := newMasterCard(mcg.ID, "MCPage-Search-cherry", "red", 3)
	for _, c := range []*domain.MasterCard{frontMatch, backMatch, miss1, miss2} {
		require.NoError(t, repo.Create(ctx, c))
	}

	search := "apple"
	got, total, err := repo.FindPageByMasterCardgroup(
		ctx, mcg.ID, nil, nil, 10, 0, repository.MasterCardOrderByPosition, repository.SortAsc, &search,
	)
	require.NoError(t, err)
	require.Equal(t, int64(2), total, "search totalCount counts only matching rows")
	require.Len(t, got, 2)
	gotIDs := map[string]bool{got[0].ID: true, got[1].ID: true}
	require.True(t, gotIDs[frontMatch.ID], "front 'APPLE' must match case-insensitively")
	require.True(t, gotIDs[backMatch.ID], "back 'apple' must match")
	require.False(t, gotIDs[miss1.ID])
	require.False(t, gotIDs[miss2.ID])

	// ILIKE escaping: a literal '%' in a front must NOT be matched by a search of
	// "%" (which, unescaped, would match every row). Seed a row containing a
	// literal percent and a literal underscore.
	pctRow := newMasterCard(mcg.ID, "MCPage-Search-50%off", "disc_ount", 4)
	require.NoError(t, repo.Create(ctx, pctRow))

	// Search for a bare "%": escaped to a literal, so it matches ONLY the row that
	// actually contains a '%' — not every row in the group.
	pct := "%"
	gotPct, totalPct, err := repo.FindPageByMasterCardgroup(
		ctx, mcg.ID, nil, nil, 10, 0, repository.MasterCardOrderByPosition, repository.SortAsc, &pct,
	)
	require.NoError(t, err)
	require.Equal(t, int64(1), totalPct, "literal '%%' must match exactly the one row containing it, not all rows")
	require.Len(t, gotPct, 1)
	require.Equal(t, pctRow.ID, gotPct[0].ID)

	// Search for a bare "_": escaped, matches only the row whose back contains '_'.
	und := "_"
	gotUnd, totalUnd, err := repo.FindPageByMasterCardgroup(
		ctx, mcg.ID, nil, nil, 10, 0, repository.MasterCardOrderByPosition, repository.SortAsc, &und,
	)
	require.NoError(t, err)
	require.Equal(t, int64(1), totalUnd, "literal '_' must not act as a single-char wildcard")
	require.Len(t, gotUnd, 1)
	require.Equal(t, pctRow.ID, gotUnd[0].ID)
}

// TestMasterCardRepository_FindPageByMasterCardgroup_TotalCountScopedToGroup
// verifies totalCount is scoped to the requested master cardgroup and does not
// count rows in other groups (which other parallel tests may also be inserting).
func TestMasterCardRepository_FindPageByMasterCardgroup_TotalCountScopedToGroup(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := repository.NewMasterCardRepository(testDB.GORM)
	mcg1 := insertMCGForCardTest(t, ctx, "MCPage-Scope-Group1")
	mcg2 := insertMCGForCardTest(t, ctx, "MCPage-Scope-Group2")

	insertMasterCardsSeq(t, ctx, repo, mcg1.ID, "MCPage-Scope1", 3)
	insertMasterCardsSeq(t, ctx, repo, mcg2.ID, "MCPage-Scope2", 5)

	_, total1, err := repo.FindPageByMasterCardgroup(
		ctx, mcg1.ID, nil, nil, 100, 0, repository.MasterCardOrderByPosition, repository.SortAsc, nil,
	)
	require.NoError(t, err)
	require.Equal(t, int64(3), total1)

	_, total2, err := repo.FindPageByMasterCardgroup(
		ctx, mcg2.ID, nil, nil, 100, 0, repository.MasterCardOrderByPosition, repository.SortAsc, nil,
	)
	require.NoError(t, err)
	require.Equal(t, int64(5), total2)
}

// TestMasterCardRepository_FindPageByMasterCardgroup_ZeroPageReturnsTotal
// verifies first=0 && last=0 short-circuits the row fetch but still returns the
// real totalCount from the separate COUNT(*), and that the slice is non-nil.
func TestMasterCardRepository_FindPageByMasterCardgroup_ZeroPageReturnsTotal(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	mcg := insertMCGForCardTest(t, ctx, "MCPage-ZeroPage-Group")
	repo := repository.NewMasterCardRepository(testDB.GORM)

	insertMasterCardsSeq(t, ctx, repo, mcg.ID, "MCPage-ZeroPage", 4)

	cards, total, err := repo.FindPageByMasterCardgroup(
		ctx, mcg.ID, nil, nil, 0, 0, repository.MasterCardOrderByPosition, repository.SortAsc, nil,
	)
	require.NoError(t, err)
	require.NotNil(t, cards)
	require.Empty(t, cards)
	require.Equal(t, int64(4), total)
}

// TestMasterCardRepository_FindPageByMasterCardgroup_PageCapAllowsMaxPlusOne
// verifies PageCap (101) accepts first=101 so the usecase's +1 trick can detect a
// next page when the caller requests the documented maximum of 100. The group is
// isolated from other parallel tests' rows by the per-test search prefix.
func TestMasterCardRepository_FindPageByMasterCardgroup_PageCapAllowsMaxPlusOne(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	mcg := insertMCGForCardTest(t, ctx, "MCPage-PageCap-Group")
	repo := repository.NewMasterCardRepository(testDB.GORM)

	insertMasterCardsSeq(t, ctx, repo, mcg.ID, "MCPage-PageCap", 101)
	// Scope every query to this test's rows via the unique search prefix so rows
	// from other parallel tests in the same shared DB cannot leak into the window.
	prefix := "MCPage-PageCap"

	cards101, total101, err := repo.FindPageByMasterCardgroup(
		ctx, mcg.ID, nil, nil, 101, 0, repository.MasterCardOrderByPosition, repository.SortAsc, &prefix,
	)
	require.NoError(t, err)
	require.Equal(t, int64(101), total101)
	require.Len(t, cards101, 101, "first=101 returns all rows because PageCap == 101")

	cards100, total100, err := repo.FindPageByMasterCardgroup(
		ctx, mcg.ID, nil, nil, 100, 0, repository.MasterCardOrderByPosition, repository.SortAsc, &prefix,
	)
	require.NoError(t, err)
	require.Equal(t, int64(101), total100)
	require.Len(t, cards100, 100, "first=100 is capped to 100 rows")
}

// TestMasterCardRepository_FindByID verifies the single-row PK lookup: an
// existing id returns the row with its columns round-tripped, and an unknown id
// returns ErrNotFound.
func TestMasterCardRepository_FindByID(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	mcg := insertMCGForCardTest(t, ctx, "MCFindByID-Group")
	repo := repository.NewMasterCardRepository(testDB.GORM)

	card := newMasterCard(mcg.ID, "MCFindByID-front", "back", 5)
	require.NoError(t, repo.Create(ctx, card))

	got, err := repo.FindByID(ctx, card.ID)
	require.NoError(t, err)
	require.Equal(t, card.ID, got.ID)
	require.Equal(t, mcg.ID, got.MasterCardgroupID)
	require.Equal(t, domain.CardText("MCFindByID-front"), got.Front)
	require.Equal(t, 5, got.Position)

	// Unknown id returns ErrNotFound.
	_, err = repo.FindByID(ctx, uuid.NewString())
	require.ErrorIs(t, err, repository.ErrNotFound)
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

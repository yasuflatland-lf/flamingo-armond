package repository_test

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"backend/internal/domain"
	"backend/internal/repository"
)

// TestCardRepository_UpsertManyTx covers the four scenarios for UpsertManyTx:
// pure inserts, mixed insert+update, empty input, and the per-cardgroup
// uniqueness boundary. The unique index that backs the ON CONFLICT clause is
// migration 20260503000000_add_cards_upsert_index.
func TestCardRepository_UpsertManyTx(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("all inserts", func(t *testing.T) {
		t.Parallel()
		repo := repository.NewCardRepository(testDB.GORM)
		ownerID := insertAuthUser(t, ctx)
		cg := insertCardgroup(t, ctx, ownerID)

		cards := []*domain.Card{
			newCard(cg.ID, "front-1", "back-1"),
			newCard(cg.ID, "front-2", "back-2"),
			newCard(cg.ID, "front-3", "back-3"),
			newCard(cg.ID, "front-4", "back-4"),
			newCard(cg.ID, "front-5", "back-5"),
		}

		var result repository.UpsertManyTxResult
		err := testDB.GORM.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			var txErr error
			result, txErr = repo.UpsertManyTx(ctx, tx, cards)
			return txErr
		})
		require.NoError(t, err)
		require.Equal(t, int64(5), result.Inserted)
		require.Equal(t, int64(0), result.Updated)

		stored, err := repo.ListByCardgroup(ctx, string(cg.ID))
		require.NoError(t, err)
		require.Len(t, stored, 5)
	})

	t.Run("mixed inserts and updates", func(t *testing.T) {
		t.Parallel()
		repo := repository.NewCardRepository(testDB.GORM)
		ownerID := insertAuthUser(t, ctx)
		cg := insertCardgroup(t, ctx, ownerID)

		// Pre-insert three rows whose fronts will overlap with the upsert batch.
		pre1 := newCard(cg.ID, "shared-1", "old-back-1")
		pre2 := newCard(cg.ID, "shared-2", "old-back-2")
		pre3 := newCard(cg.ID, "shared-3", "old-back-3")
		for _, c := range []*domain.Card{pre1, pre2, pre3} {
			require.NoError(t, repo.Create(ctx, c))
		}

		// Upsert batch: 3 same fronts (with new backs) + 2 brand new fronts.
		// The IDs supplied here for the existing fronts MUST be ignored by the
		// ON CONFLICT path — the conflict key is (cardgroup_id, front).
		upsertBatch := []*domain.Card{
			newCard(cg.ID, "shared-1", "new-back-1"),
			newCard(cg.ID, "shared-2", "new-back-2"),
			newCard(cg.ID, "shared-3", "new-back-3"),
			newCard(cg.ID, "fresh-1", "fresh-back-1"),
			newCard(cg.ID, "fresh-2", "fresh-back-2"),
		}

		var result repository.UpsertManyTxResult
		err := testDB.GORM.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			var txErr error
			result, txErr = repo.UpsertManyTx(ctx, tx, upsertBatch)
			return txErr
		})
		require.NoError(t, err)
		require.Equal(t, int64(2), result.Inserted)
		require.Equal(t, int64(3), result.Updated)

		// All three existing rows have their `back` text replaced; the row id
		// (the original PK) stays intact because ON CONFLICT only updates `back`
		// and `position`; the database trigger advances updated_at.
		for _, original := range []*domain.Card{pre1, pre2, pre3} {
			got, err := repo.FindByID(ctx, original.ID)
			require.NoError(t, err)
			require.Equal(t, original.Front, got.Front)
			require.NotEqual(t, original.Back, got.Back, "back must be overwritten")
		}

		// Final cardgroup row count: 3 pre-existing + 2 newly inserted.
		stored, err := repo.ListByCardgroup(ctx, string(cg.ID))
		require.NoError(t, err)
		require.Len(t, stored, 5)
	})

	t.Run("empty input is a no-op", func(t *testing.T) {
		t.Parallel()
		repo := repository.NewCardRepository(testDB.GORM)
		ownerID := insertAuthUser(t, ctx)
		cg := insertCardgroup(t, ctx, ownerID)

		var result repository.UpsertManyTxResult
		err := testDB.GORM.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			var txErr error
			result, txErr = repo.UpsertManyTx(ctx, tx, nil)
			return txErr
		})
		require.NoError(t, err)
		require.Equal(t, repository.UpsertManyTxResult{}, result)

		stored, err := repo.ListByCardgroup(ctx, string(cg.ID))
		require.NoError(t, err)
		require.Empty(t, stored)
	})

	t.Run("same front in different cardgroups coexist", func(t *testing.T) {
		t.Parallel()
		repo := repository.NewCardRepository(testDB.GORM)
		ownerID := insertAuthUser(t, ctx)
		cgA := insertCardgroup(t, ctx, ownerID)
		cgB := insertCardgroup(t, ctx, ownerID)

		// Insert (group A, "hello").
		err := testDB.GORM.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			_, txErr := repo.UpsertManyTx(ctx, tx, []*domain.Card{
				newCard(cgA.ID, "hello", "back-A"),
			})
			return txErr
		})
		require.NoError(t, err)

		// Upsert (group B, "hello") — must be classified as an INSERT because
		// the unique key is the (cardgroup_id, front) pair, not just `front`.
		var result repository.UpsertManyTxResult
		err = testDB.GORM.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			var txErr error
			result, txErr = repo.UpsertManyTx(ctx, tx, []*domain.Card{
				newCard(cgB.ID, "hello", "back-B"),
			})
			return txErr
		})
		require.NoError(t, err)
		require.Equal(t, int64(1), result.Inserted)
		require.Equal(t, int64(0), result.Updated)

		storedA, err := repo.ListByCardgroup(ctx, string(cgA.ID))
		require.NoError(t, err)
		require.Len(t, storedA, 1)
		require.Equal(t, domain.CardText("hello"), storedA[0].Front)
		require.Equal(t, domain.CardText("back-A"), storedA[0].Back)

		storedB, err := repo.ListByCardgroup(ctx, string(cgB.ID))
		require.NoError(t, err)
		require.Len(t, storedB, 1)
		require.Equal(t, domain.CardText("hello"), storedB[0].Front)
		require.Equal(t, domain.CardText("back-B"), storedB[0].Back)
	})
}

// TestCardRepository_UpsertManyTx_UpdatesPositionOnConflict proves the
// ON CONFLICT DO UPDATE path refreshes `position`. The first batch writes
// fronts F1,F2,F3 at positions 0,1,2; the second batch re-upserts the SAME
// fronts at positions 10,11,12. Because cardToDomain carries Position, a read
// after the second upsert must reflect the new positions.
func TestCardRepository_UpsertManyTx_UpdatesPositionOnConflict(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := repository.NewCardRepository(testDB.GORM)
	ownerID := insertAuthUser(t, ctx)
	cg := insertCardgroup(t, ctx, ownerID)

	fronts := []string{"F1", "F2", "F3"}

	makeBatch := func(positions []int) []*domain.Card {
		batch := make([]*domain.Card, len(fronts))
		for i, front := range fronts {
			c := newCard(cg.ID, front, "back")
			c.Position = positions[i]
			batch[i] = c
		}
		return batch
	}

	// First batch: positions 0,1,2.
	err := testDB.GORM.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		_, txErr := repo.UpsertManyTx(ctx, tx, makeBatch([]int{0, 1, 2}))
		return txErr
	})
	require.NoError(t, err)

	for i, front := range fronts {
		got, err := repo.FindByCardgroupAndFront(ctx, string(cg.ID), front)
		require.NoError(t, err)
		require.Equal(t, i, got.Position, "initial position for %q", front)
	}

	// Second batch: SAME fronts, DIFFERENT positions 10,11,12. The conflict
	// path must refresh position.
	err = testDB.GORM.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		_, txErr := repo.UpsertManyTx(ctx, tx, makeBatch([]int{10, 11, 12}))
		return txErr
	})
	require.NoError(t, err)

	for i, front := range fronts {
		got, err := repo.FindByCardgroupAndFront(ctx, string(cg.ID), front)
		require.NoError(t, err)
		require.Equal(t, 10+i, got.Position,
			"position for %q must be refreshed by the conflict path", front)
	}
}

func TestCardRepository_UpsertManyTx_SpansChunks(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := repository.NewCardRepository(testDB.GORM)
	ownerID := insertAuthUser(t, ctx)
	cg := insertCardgroup(t, ctx, ownerID)

	// Three times bulkStatementChunkRows (5000) + 1: a 1-row final chunk, and 90,006 bind
	// parameters as one statement, above pgx's 65,535 cap. Not 10,001: that fits in one statement.
	const count = 15001
	cards := make([]*domain.Card, count)
	for i := range cards {
		cards[i] = newCard(cg.ID, fmt.Sprintf("front-%05d", i), "back-1")
	}

	var result repository.UpsertManyTxResult
	err := testDB.GORM.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var txErr error
		result, txErr = repo.UpsertManyTx(ctx, tx, cards)
		return txErr
	})
	require.NoError(t, err)
	require.Equal(t, repository.UpsertManyTxResult{Inserted: count}, result)

	for i := range cards {
		cards[i] = newCard(cg.ID, fmt.Sprintf("front-%05d", i), "back-2")
	}
	err = testDB.GORM.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var txErr error
		result, txErr = repo.UpsertManyTx(ctx, tx, cards)
		return txErr
	})
	require.NoError(t, err)
	require.Equal(t, repository.UpsertManyTxResult{Updated: count}, result)

	sqlDB := sqlDBHandle(t)
	var total int
	require.NoError(t, sqlDB.QueryRowContext(ctx,
		`SELECT count(*) FROM cards WHERE cardgroup_id = $1`, cg.ID).Scan(&total))
	require.Equal(t, count, total)
	require.NoError(t, sqlDB.QueryRowContext(ctx,
		`SELECT count(*) FROM cards WHERE cardgroup_id = $1 AND back = 'back-2'`, cg.ID).Scan(&total))
	require.Equal(t, count, total)
}

func TestCardRepository_FoldFrontCaseToTx_SpansChunks(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := repository.NewCardRepository(testDB.GORM)
	ownerID := insertAuthUser(t, ctx)
	cg := insertCardgroup(t, ctx, ownerID)

	// Not 15,001 fronts alone: that fits in one statement. Lead with 55,000 unmatched fronts
	// (11 full chunks) so one unchunked statement would bind 70,002 parameters, above pgx's
	// 65,535 cap. Not matches first: the 1-row final chunk would then hold an unmatched front.
	const count = 15001
	const pad = 55000
	cards := make([]*domain.Card, count)
	fronts := make([]string, pad+count)
	for i := range pad {
		fronts[i] = fmt.Sprintf("MISSING-%05d", i)
	}
	for i := range cards {
		front := fmt.Sprintf("front-%05d", i)
		cards[i] = newCard(cg.ID, front, "back-1")
		fronts[pad+i] = strings.ToUpper(front)
	}
	err := testDB.GORM.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		_, txErr := repo.UpsertManyTx(ctx, tx, cards)
		return txErr
	})
	require.NoError(t, err)

	var folded int64
	err = testDB.GORM.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var txErr error
		folded, txErr = repo.FoldFrontCaseToTx(ctx, tx, string(cg.ID), fronts)
		return txErr
	})
	require.NoError(t, err)
	require.Equal(t, int64(count), folded)

	sqlDB := sqlDBHandle(t)
	var total int
	require.NoError(t, sqlDB.QueryRowContext(ctx,
		`SELECT count(*) FROM cards WHERE cardgroup_id = $1 AND front LIKE 'FRONT-%'`, cg.ID).Scan(&total))
	require.Equal(t, count, total)
	require.NoError(t, sqlDB.QueryRowContext(ctx,
		`SELECT count(*) FROM cards WHERE cardgroup_id = $1`, cg.ID).Scan(&total))
	require.Equal(t, count, total)
}

func TestCardRepository_FoldFrontCaseToTx(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("renames single variant before upsert", func(t *testing.T) {
		t.Parallel()
		repo := repository.NewCardRepository(testDB.GORM)
		ownerID := insertAuthUser(t, ctx)
		cg := insertCardgroup(t, ctx, ownerID)
		original := newCard(cg.ID, "apple", "old")
		require.NoError(t, repo.Create(ctx, original))

		var folded int64
		var result repository.UpsertManyTxResult
		err := testDB.GORM.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			var txErr error
			folded, txErr = repo.FoldFrontCaseToTx(ctx, tx, string(cg.ID), []string{"Apple"})
			if txErr != nil {
				return txErr
			}
			result, txErr = repo.UpsertManyTx(ctx, tx, []*domain.Card{newCard(cg.ID, "Apple", "catalog")})
			return txErr
		})
		require.NoError(t, err)
		require.Equal(t, int64(1), folded)
		require.Equal(t, repository.UpsertManyTxResult{Updated: 1}, result)

		stored, err := repo.ListByCardgroup(ctx, string(cg.ID))
		require.NoError(t, err)
		require.Len(t, stored, 1)
		require.Equal(t, original.ID, stored[0].ID)
		require.Equal(t, domain.CardText("Apple"), stored[0].Front)
		require.Equal(t, domain.CardText("catalog"), stored[0].Back)
	})

	t.Run("exact variant prevents rename", func(t *testing.T) {
		t.Parallel()
		repo := repository.NewCardRepository(testDB.GORM)
		ownerID := insertAuthUser(t, ctx)
		cg := insertCardgroup(t, ctx, ownerID)
		lower := newCard(cg.ID, "apple", "lower-old")
		exact := newCard(cg.ID, "Apple", "exact-old")
		require.NoError(t, repo.Create(ctx, lower))
		require.NoError(t, repo.Create(ctx, exact))

		var folded int64
		err := testDB.GORM.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			var txErr error
			folded, txErr = repo.FoldFrontCaseToTx(ctx, tx, string(cg.ID), []string{"Apple"})
			if txErr != nil {
				return txErr
			}
			_, txErr = repo.UpsertManyTx(ctx, tx, []*domain.Card{newCard(cg.ID, "Apple", "catalog")})
			return txErr
		})
		require.NoError(t, err)
		require.Zero(t, folded)

		gotLower, err := repo.FindByID(ctx, lower.ID)
		require.NoError(t, err)
		require.Equal(t, domain.CardText("apple"), gotLower.Front)
		require.Equal(t, domain.CardText("lower-old"), gotLower.Back)
		gotExact, err := repo.FindByID(ctx, exact.ID)
		require.NoError(t, err)
		require.Equal(t, domain.CardText("Apple"), gotExact.Front)
		require.Equal(t, domain.CardText("catalog"), gotExact.Back)
	})

	t.Run("renames oldest variant only", func(t *testing.T) {
		t.Parallel()
		repo := repository.NewCardRepository(testDB.GORM)
		ownerID := insertAuthUser(t, ctx)
		cg := insertCardgroup(t, ctx, ownerID)
		older := newCard(cg.ID, "APPLE", "older")
		older.CreatedAt = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
		newer := newCard(cg.ID, "apple", "newer")
		newer.CreatedAt = older.CreatedAt.Add(time.Hour)
		require.NoError(t, repo.Create(ctx, older))
		require.NoError(t, repo.Create(ctx, newer))

		var folded int64
		err := testDB.GORM.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			var txErr error
			folded, txErr = repo.FoldFrontCaseToTx(ctx, tx, string(cg.ID), []string{"Apple"})
			if txErr != nil {
				return txErr
			}
			_, txErr = repo.UpsertManyTx(ctx, tx, []*domain.Card{newCard(cg.ID, "Apple", "catalog")})
			return txErr
		})
		require.NoError(t, err)
		require.Equal(t, int64(1), folded)

		gotOlder, err := repo.FindByID(ctx, older.ID)
		require.NoError(t, err)
		require.Equal(t, domain.CardText("Apple"), gotOlder.Front)
		require.Equal(t, domain.CardText("catalog"), gotOlder.Back)
		gotNewer, err := repo.FindByID(ctx, newer.ID)
		require.NoError(t, err)
		require.Equal(t, domain.CardText("apple"), gotNewer.Front)
		require.Equal(t, domain.CardText("newer"), gotNewer.Back)
	})

	t.Run("cancelled context returns the bare context error", func(t *testing.T) {
		t.Parallel()
		repo := repository.NewCardRepository(testDB.GORM)
		ownerID := insertAuthUser(t, ctx)
		cg := insertCardgroup(t, ctx, ownerID)
		cancelledCtx, cancel := context.WithCancel(ctx)
		cancel()

		folded, err := repo.FoldFrontCaseToTx(cancelledCtx, testDB.GORM, string(cg.ID), []string{"Apple"})
		require.Equal(t, context.Canceled, err)
		require.Zero(t, folded)
	})

	t.Run("empty fronts does not access database", func(t *testing.T) {
		t.Parallel()
		repo := repository.NewCardRepository(nil)
		folded, err := repo.FoldFrontCaseToTx(ctx, nil, "unused", nil)
		require.NoError(t, err)
		require.Zero(t, folded)
	})
}

// TestCardRepository_FoldFrontCaseToTx_ConcurrentExactInsert_ReturnsDuplicateFront stages an
// uncommitted exact-front insert, lets the fold's rename block on it, then commits it: the fold's
// NOT EXISTS ran on a snapshot without that row, so the rename collides on uq_cards_cardgroup_front.
// Waiting is confirmed via pg_blocking_pids, not a sleep: a fold that starts after the commit would
// see the row, skip the rename and never collide.
func TestCardRepository_FoldFrontCaseToTx_ConcurrentExactInsert_ReturnsDuplicateFront(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := repository.NewCardRepository(testDB.GORM)
	ownerID := insertAuthUser(t, ctx)
	cg := insertCardgroup(t, ctx, ownerID)
	require.NoError(t, repo.Create(ctx, newCard(cg.ID, "apple", "old")))

	stage, err := sqlDBHandle(t).BeginTx(ctx, nil)
	require.NoError(t, err)
	defer func() { _ = stage.Rollback() }()
	var stagePID int
	require.NoError(t, stage.QueryRowContext(ctx, "SELECT pg_backend_pid()").Scan(&stagePID))
	_, err = stage.ExecContext(ctx,
		"INSERT INTO cards (id, cardgroup_id, front, back) VALUES ($1, $2, 'Apple', 'concurrent')",
		uuid.NewString(), string(cg.ID))
	require.NoError(t, err)

	done := make(chan error, 1)
	go func() {
		done <- testDB.GORM.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			_, foldErr := repo.FoldFrontCaseToTx(ctx, tx, string(cg.ID), []string{"Apple"})
			return foldErr
		})
	}()

	deadline := time.Now().Add(10 * time.Second)
	for {
		var blocked int
		require.NoError(t, sqlDBHandle(t).QueryRowContext(ctx,
			"SELECT count(*) FROM pg_stat_activity WHERE $1 = ANY(pg_blocking_pids(pid))", stagePID).Scan(&blocked))
		if blocked == 1 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the fold never blocked on the uncommitted exact-front insert")
		}
		time.Sleep(20 * time.Millisecond)
	}

	require.NoError(t, stage.Commit())

	select {
	case err := <-done:
		require.ErrorIs(t, err, repository.ErrCardDuplicateFront)
	case <-time.After(30 * time.Second):
		t.Fatal("the fold did not finish after the concurrent insert committed")
	}

	stored, err := repo.ListByCardgroup(ctx, string(cg.ID))
	require.NoError(t, err)
	backs := make(map[domain.CardText]domain.CardText, len(stored))
	for _, c := range stored {
		backs[c.Front] = c.Back
	}
	require.Equal(t, map[domain.CardText]domain.CardText{"apple": "old", "Apple": "concurrent"}, backs,
		"the fold rolls back and the concurrent card stays")
}

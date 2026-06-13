package repository_test

// Real-DB integration test for the master-deck copy primitive
// (usecase.CopyMasterToUser). It runs against the testcontainer Postgres with
// all migrations applied, so it catches column-mapping and constraint bugs the
// no-DB usecase fakes cannot. TestMain, testDB, and insertAuthUser are defined
// in sibling files and shared across this package.

import (
	"context"
	"log/slog"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"backend/internal/domain"
	"backend/internal/repository"
	"backend/internal/usecase"
)

// seedMasterDeck creates a published master cardgroup plus its master cards in
// the catalog tables and returns the master cardgroup id.
func seedMasterDeck(t *testing.T, ctx context.Context, name string, cards []*domain.MasterCard) string {
	t.Helper()
	mcgRepo := repository.NewMasterCardgroupRepository(testDB.GORM)
	mcRepo := repository.NewMasterCardRepository(testDB.GORM)

	mcg := &domain.MasterCardgroup{
		ID:               uuid.NewString(),
		Name:             domain.CardgroupName(name),
		Status:           domain.MasterStatusPublished,
		IsDefaultStarter: true,
	}
	require.NoError(t, mcgRepo.Create(ctx, mcg))
	for _, c := range cards {
		c.MasterCardgroupID = mcg.ID
		require.NoError(t, mcRepo.Create(ctx, c))
	}
	return mcg.ID
}

func masterCardFixture(front, back string, position int) *domain.MasterCard {
	return &domain.MasterCard{
		ID:       uuid.NewString(),
		Front:    domain.CardText(front),
		Back:     domain.CardText(back),
		Position: position,
	}
}

// countCardgroupsForOwner returns the number of public.cardgroups rows owned by
// ownerID, read straight from SQL to avoid coupling the assertion to a repo
// method's filtering.
func countCardgroupsForOwner(t *testing.T, ctx context.Context, ownerID string) int {
	t.Helper()
	var n int
	row := sqlDBHandle(t).QueryRowContext(ctx,
		`SELECT count(*) FROM cardgroups WHERE owner_id = $1`, ownerID)
	require.NoError(t, row.Scan(&n))
	return n
}

// countFSRSRowsForUser returns the number of user_card_fsrs rows for ownerID.
func countFSRSRowsForUser(t *testing.T, ctx context.Context, ownerID string) int {
	t.Helper()
	var n int
	row := sqlDBHandle(t).QueryRowContext(ctx,
		`SELECT count(*) FROM user_card_fsrs WHERE user_id = $1`, ownerID)
	require.NoError(t, row.Scan(&n))
	return n
}

func newMasterDeckUsecaseForTest(t *testing.T) usecase.CopyMasterToUserUsecase {
	t.Helper()
	logger := slog.New(slog.DiscardHandler)
	return usecase.NewMasterDeckUsecase(
		repository.NewMasterCardgroupRepository(testDB.GORM),
		repository.NewMasterCardRepository(testDB.GORM),
		repository.NewCardRepository(testDB.GORM),
		repository.NewCardgroupRepository(testDB.GORM),
		testDB.GORM,
		logger,
	)
}

// TestCopyMasterToUser_Integration_CopiesDeckWithNoFSRSState runs the copy
// primitive against a real database and asserts the full row-level outcome:
// a new user cardgroup, reparented cards with fresh ids and preserved
// front/back/position, and zero FSRS rows for the new owner.
func TestCopyMasterToUser_Integration_CopiesDeckWithNoFSRSState(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ownerID := insertAuthUser(t, ctx)

	masterID := seedMasterDeck(t, ctx, "Integration Starter", []*domain.MasterCard{
		masterCardFixture("front-1", "back-1", 0),
		masterCardFixture("front-2", "back-2", 1),
		masterCardFixture("front-3", "back-3", 2),
	})

	uc := newMasterDeckUsecaseForTest(t)
	cg, err := uc.CopyMasterToUser(ctx, masterID, ownerID)
	require.NoError(t, err)
	require.NotNil(t, cg)

	// The new cardgroup row landed with the correct owner and copied name.
	require.NotEmpty(t, cg.ID)
	assert.Equal(t, ownerID, cg.OwnerID)
	assert.Equal(t, domain.CardgroupName("Integration Starter"), cg.Name)
	assert.Equal(t, 1, countCardgroupsForOwner(t, ctx, ownerID))

	cgRepo := repository.NewCardgroupRepository(testDB.GORM)
	persisted, err := cgRepo.FindByID(ctx, cg.ID)
	require.NoError(t, err)
	assert.Equal(t, ownerID, persisted.OwnerID)
	assert.Equal(t, domain.CardgroupName("Integration Starter"), persisted.Name)

	// The cards landed in public.cards: reparented, fresh ids, content preserved.
	cardRepo := repository.NewCardRepository(testDB.GORM)
	got, err := cardRepo.FindByCardgroup(ctx, cg.ID)
	require.NoError(t, err)
	require.Len(t, got, 3)

	byFront := map[domain.CardText]*domain.Card{}
	for _, c := range got {
		assert.Equal(t, cg.ID, c.CardgroupID, "card reparented to the new cardgroup")
		assert.NotEmpty(t, c.ID)
		byFront[c.Front] = c
	}
	require.Contains(t, byFront, domain.CardText("front-1"))
	assert.Equal(t, domain.CardText("back-1"), byFront["front-1"].Back)
	assert.Equal(t, 0, byFront["front-1"].Position)
	assert.Equal(t, domain.CardText("back-2"), byFront["front-2"].Back)
	assert.Equal(t, 1, byFront["front-2"].Position)
	assert.Equal(t, domain.CardText("back-3"), byFront["front-3"].Back)
	assert.Equal(t, 2, byFront["front-3"].Position)

	// New ids differ from the master cards' ids: read the master cards back and
	// confirm no id overlap with the copied cards.
	mcRepo := repository.NewMasterCardRepository(testDB.GORM)
	masterCards, err := mcRepo.ListByMasterCardgroup(ctx, masterID)
	require.NoError(t, err)
	masterIDs := map[string]struct{}{}
	for _, mc := range masterCards {
		masterIDs[mc.ID] = struct{}{}
	}
	for _, c := range got {
		_, clash := masterIDs[c.ID]
		assert.False(t, clash, "copied card id %s must not reuse a master card id", c.ID)
	}

	// No FSRS rows are written for the new owner: the snapshot is empty state.
	assert.Equal(t, 0, countFSRSRowsForUser(t, ctx, ownerID),
		"the copy primitive writes no per-user FSRS rows")
}

// TestCopyMasterToUser_Integration_RollsBackOnMissingMaster proves the
// transaction boundary: a copy of a non-existent master deck leaves zero
// cardgroups for the user (no partial cardgroup insert survives).
func TestCopyMasterToUser_Integration_RollsBackOnMissingMaster(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ownerID := insertAuthUser(t, ctx)

	uc := newMasterDeckUsecaseForTest(t)
	cg, err := uc.CopyMasterToUser(ctx, uuid.NewString(), ownerID)
	require.Error(t, err)
	assert.Nil(t, cg)

	assert.Equal(t, 0, countCardgroupsForOwner(t, ctx, ownerID),
		"a failed copy leaves no cardgroup behind")
}

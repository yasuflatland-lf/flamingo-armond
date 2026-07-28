package repository_test

// Parity invariant integration test: preview tally MUST equal merge tally.
//
// This test runs against the testcontainer Postgres with all migrations applied.
// TestMain, testDB, insertAuthUser, and seedMasterDeck are defined in sibling
// files and shared across this package.

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"backend/internal/auth"
	"backend/internal/domain"
	"backend/internal/repository"
	"backend/internal/usecase"
)

// stubParityAdminChecker always returns non-admin. The parity test exercises
// MergeMaster and PreviewMergeMaster, neither of which needs admin access.
type stubParityAdminChecker struct{}

func (stubParityAdminChecker) IsAdmin(_ context.Context, _ string) (bool, error) {
	return false, nil
}

// newMasterCatalogUsecaseForParityTest constructs a full MasterCatalogUsecase
// wired to real repositories, mirroring the production wiring in cmd/server/main.go.
func newMasterCatalogUsecaseForParityTest(t *testing.T) usecase.MasterCatalogUsecase {
	t.Helper()
	logger := slog.New(slog.DiscardHandler)
	mcgRepo := repository.NewMasterCardgroupRepository(testDB.GORM)
	mcRepo := repository.NewMasterCardRepository(testDB.GORM)
	cardRepo := repository.NewCardRepository(testDB.GORM)
	cgRepo := repository.NewCardgroupRepository(testDB.GORM)
	deckUC := usecase.NewMasterDeckUsecase(mcgRepo, mcRepo, cardRepo, cgRepo, testDB.GORM, logger)
	adminGate := usecase.NewAdminGate(stubParityAdminChecker{})
	return usecase.NewMasterCatalogUsecase(mcgRepo, deckUC, cgRepo, adminGate, logger)
}

// seedOwnedCardgroupWithCards creates a user-owned cardgroup and inserts the given
// cards into it. Returns the cardgroup ID.
func seedOwnedCardgroupWithCards(
	t *testing.T, ctx context.Context,
	ownerID, name string,
	fronts []struct{ front, back string },
) domain.CardgroupID {
	t.Helper()
	cgRepo := repository.NewCardgroupRepository(testDB.GORM)
	cardRepo := repository.NewCardRepository(testDB.GORM)
	now := time.Date(2026, 4, 26, 0, 0, 0, 0, time.UTC)

	cg, err := domain.NewCardgroup(domain.UserID(ownerID), domain.CardgroupName(name), now)
	require.NoError(t, err)
	require.NoError(t, cgRepo.Create(ctx, cg))

	for i, f := range fronts {
		card, err := domain.NewCard(cg.ID, f.front, f.back, i, now)
		require.NoError(t, err)
		require.NoError(t, cardRepo.Create(ctx, card))
	}
	return cg.ID
}

func authenticatedContext(ctx context.Context, ownerID string) context.Context {
	return auth.ContextWithUser(ctx, &auth.AuthUser{
		Sub:           ownerID,
		Email:         ownerID + "@test.example",
		EmailVerified: true,
	})
}

func requirePreviewMergeParity(t *testing.T, previewAdded, previewUpdated, mergeAdded, mergeUpdated int64) {
	t.Helper()
	require.Equal(t, mergeAdded, previewAdded, "preview.Added must equal merge.Added")
	require.Equal(t, mergeUpdated, previewUpdated, "preview.Updated must equal merge.Updated")
}

// TestMergeCaseFold_PreservesCardIDAndFSRS proves a case-only catalog match is
// renamed in place before upsert. The preview and merge tally it as one update,
// and the scheduling row remains byte-for-byte attached to the original card id.
func TestMergeCaseFold_PreservesCardIDAndFSRS(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	ownerID := insertAuthUser(t, ctx)
	authedCtx := authenticatedContext(ctx, ownerID)
	masterID := seedMasterDeck(t, authedCtx, "Fold Master "+uuid.NewString(), []*domain.MasterCard{
		{ID: uuid.NewString(), Front: domain.CardText("Apple"), Back: domain.CardText("catalog-back"), Position: 0},
	})
	destID := seedOwnedCardgroupWithCards(t, ctx, ownerID, "My Deck "+uuid.NewString(), []struct{ front, back string }{
		{front: "apple", back: "learner-back"},
	})

	cardRepo := repository.NewCardRepository(testDB.GORM)
	fsrsRepo := repository.NewUserCardFSRSRepository(testDB.GORM)
	original, err := cardRepo.FindByCardgroupAndFront(ctx, string(destID), "apple")
	require.NoError(t, err)
	studiedAt := time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC)
	state := domain.NewUserCardFSRSForNewCard(domain.UserID(ownerID), original.ID, studiedAt)
	state.State = domain.FSRSState{
		Due:           studiedAt.Add(21 * 24 * time.Hour),
		Stability:     19.5,
		Difficulty:    4.2,
		ScheduledDays: 21,
		Reps:          8,
		Lapses:        1,
		Phase:         domain.FSRSPhaseReview,
		LastReview:    studiedAt,
		LastRating:    domain.RatingGood,
	}
	require.NoError(t, testDB.GORM.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fsrsRepo.UpsertTx(ctx, tx, state)
	}))
	before, err := fsrsRepo.FindByUserAndCardIDs(ctx, ownerID, []string{original.ID})
	require.NoError(t, err)
	require.Contains(t, before, original.ID)

	uc := newMasterCatalogUsecaseForParityTest(t)
	preview, err := uc.PreviewMergeMaster(authedCtx, masterID, string(destID))
	require.NoError(t, err)
	require.Equal(t, int64(0), preview.Added)
	require.Equal(t, int64(1), preview.Updated)
	merge, err := uc.MergeMaster(authedCtx, masterID, string(destID))
	require.NoError(t, err)
	requirePreviewMergeParity(t, preview.Added, preview.Updated, merge.Added, merge.Updated)

	stored, err := cardRepo.ListByCardgroup(ctx, string(destID))
	require.NoError(t, err)
	require.Len(t, stored, 1)
	require.Equal(t, original.ID, stored[0].ID)
	require.Equal(t, domain.CardText("Apple"), stored[0].Front)
	require.Equal(t, domain.CardText("catalog-back"), stored[0].Back)
	after, err := fsrsRepo.FindByUserAndCardIDs(ctx, ownerID, []string{original.ID})
	require.NoError(t, err)
	require.Equal(t, before[original.ID], after[original.ID])
}

// TestMergeCaseFold_ExactVariantWins proves an exact catalog front suppresses
// folding even when another stored case variant exists. Only the exact row is
// updated, both rows remain, and preview counts one distinct folded update.
func TestMergeCaseFold_ExactVariantWins(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ownerID := insertAuthUser(t, ctx)
	authedCtx := authenticatedContext(ctx, ownerID)
	masterID := seedMasterDeck(t, authedCtx, "Exact Master "+uuid.NewString(), []*domain.MasterCard{
		{ID: uuid.NewString(), Front: domain.CardText("Apple"), Back: domain.CardText("catalog-back"), Position: 0},
	})
	destID := seedOwnedCardgroupWithCards(t, ctx, ownerID, "Exact Deck "+uuid.NewString(), []struct{ front, back string }{
		{front: "apple", back: "lower-back"},
		{front: "Apple", back: "exact-back"},
	})
	cardRepo := repository.NewCardRepository(testDB.GORM)
	lower, err := cardRepo.FindByCardgroupAndFront(ctx, string(destID), "apple")
	require.NoError(t, err)
	exact, err := cardRepo.FindByCardgroupAndFront(ctx, string(destID), "Apple")
	require.NoError(t, err)

	uc := newMasterCatalogUsecaseForParityTest(t)
	preview, err := uc.PreviewMergeMaster(authedCtx, masterID, string(destID))
	require.NoError(t, err)
	merge, err := uc.MergeMaster(authedCtx, masterID, string(destID))
	require.NoError(t, err)
	requirePreviewMergeParity(t, preview.Added, preview.Updated, merge.Added, merge.Updated)
	require.Equal(t, int64(0), merge.Added)
	require.Equal(t, int64(1), merge.Updated)

	stored, err := cardRepo.ListByCardgroup(ctx, string(destID))
	require.NoError(t, err)
	require.Len(t, stored, 2)
	gotLower, err := cardRepo.FindByID(ctx, lower.ID)
	require.NoError(t, err)
	require.Equal(t, domain.CardText("apple"), gotLower.Front)
	require.Equal(t, domain.CardText("lower-back"), gotLower.Back)
	gotExact, err := cardRepo.FindByID(ctx, exact.ID)
	require.NoError(t, err)
	require.Equal(t, domain.CardText("Apple"), gotExact.Front)
	require.Equal(t, domain.CardText("catalog-back"), gotExact.Back)
}

// TestMergeCaseFold_OldestVariantWins proves the deterministic boundary when
// several case variants exist and none is exact. Only the oldest row is renamed
// and updated; the newer variant remains untouched and preview stays in parity.
func TestMergeCaseFold_OldestVariantWins(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ownerID := insertAuthUser(t, ctx)
	authedCtx := authenticatedContext(ctx, ownerID)
	masterID := seedMasterDeck(t, authedCtx, "Oldest Master "+uuid.NewString(), []*domain.MasterCard{
		{ID: uuid.NewString(), Front: domain.CardText("Apple"), Back: domain.CardText("catalog-back"), Position: 0},
	})
	destID := seedOwnedCardgroupWithCards(t, ctx, ownerID, "Oldest Deck "+uuid.NewString(), nil)
	cardRepo := repository.NewCardRepository(testDB.GORM)
	older := newCard(destID, "APPLE", "older-back")
	older.CreatedAt = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	newer := newCard(destID, "apple", "newer-back")
	newer.CreatedAt = older.CreatedAt.Add(time.Hour)
	require.NoError(t, cardRepo.Create(ctx, older))
	require.NoError(t, cardRepo.Create(ctx, newer))

	uc := newMasterCatalogUsecaseForParityTest(t)
	preview, err := uc.PreviewMergeMaster(authedCtx, masterID, string(destID))
	require.NoError(t, err)
	merge, err := uc.MergeMaster(authedCtx, masterID, string(destID))
	require.NoError(t, err)
	requirePreviewMergeParity(t, preview.Added, preview.Updated, merge.Added, merge.Updated)
	require.Equal(t, int64(0), merge.Added)
	require.Equal(t, int64(1), merge.Updated)

	gotOlder, err := cardRepo.FindByID(ctx, older.ID)
	require.NoError(t, err)
	require.Equal(t, domain.CardText("Apple"), gotOlder.Front)
	require.Equal(t, domain.CardText("catalog-back"), gotOlder.Back)
	gotNewer, err := cardRepo.FindByID(ctx, newer.ID)
	require.NoError(t, err)
	require.Equal(t, domain.CardText("apple"), gotNewer.Front)
	require.Equal(t, domain.CardText("newer-back"), gotNewer.Back)
}

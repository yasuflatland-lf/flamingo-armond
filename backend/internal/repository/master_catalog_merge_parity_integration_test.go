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

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

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
	return usecase.NewMasterCatalogUsecase(mcgRepo, deckUC, adminGate, logger)
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

	cg, err := domain.NewCardgroup(domain.UserID(ownerID), domain.CardgroupName(name))
	require.NoError(t, err)
	require.NoError(t, cgRepo.Create(ctx, cg))

	for i, f := range fronts {
		card, err := domain.NewCard(cg.ID, f.front, f.back, i)
		require.NoError(t, err)
		require.NoError(t, cardRepo.Create(ctx, card))
	}
	return cg.ID
}

// TestMergePreviewEqualsMergeTally_CaseSensitive is the load-bearing parity
// invariant test. It asserts that PreviewMergeMaster and MergeMaster compute
// identical Add/Update tallies for the same fixture, and pins the expected
// case-sensitive split:
//
//   - "Apple"  (exact match)  → Updated = 1
//   - "banana" (case differs) → does NOT match master "Banana" → master Banana is Added
//   - "Cherry" (absent)       → Added
//
// Total: Added = 2, Updated = 1.
func TestMergePreviewEqualsMergeTally_CaseSensitive(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	ownerID := insertAuthUser(t, ctx)
	authedCtx := auth.ContextWithUser(ctx, &auth.AuthUser{
		Sub:           ownerID,
		Email:         ownerID + "@test.example",
		EmailVerified: true,
	})

	// Seed a PUBLISHED master deck with three cards.
	masterID := seedMasterDeck(t, authedCtx, "Parity Master "+uuid.NewString(), []*domain.MasterCard{
		{ID: uuid.NewString(), Front: domain.CardText("Apple"), Back: domain.CardText("apple-back"), Position: 0},
		{ID: uuid.NewString(), Front: domain.CardText("Banana"), Back: domain.CardText("banana-back"), Position: 1},
		{ID: uuid.NewString(), Front: domain.CardText("Cherry"), Back: domain.CardText("cherry-back"), Position: 2},
	})

	// Seed the destination cardgroup:
	//   "Apple"  → exact front match with master "Apple"  → Updated
	//   "banana" → LOWERCASE; cards front is case-sensitive, so this is NOT a match
	//              for master "Banana"; master "Banana" will be Added.
	destID := seedOwnedCardgroupWithCards(t, ctx, ownerID, "My Deck "+uuid.NewString(), []struct{ front, back string }{
		{front: "Apple", back: "old-apple"},
		{front: "banana", back: "old-banana"},
	})

	uc := newMasterCatalogUsecaseForParityTest(t)

	// --- preview (dry run) ---
	preview, err := uc.PreviewMergeMaster(authedCtx, masterID, string(destID))
	require.NoError(t, err)
	require.False(t, preview.NotFound, "master deck must be found")

	// Pin the case-sensitive expectation before calling the real merge.
	// Added = 2: master "Banana" + master "Cherry" are new (banana != Banana).
	// Updated = 1: master "Apple" overwrites dest "Apple" (exact match).
	require.Equal(t, int64(2), preview.Added, "preview: Added must be 2 (Banana + Cherry)")
	require.Equal(t, int64(1), preview.Updated, "preview: Updated must be 1 (Apple exact)")

	// --- real merge ---
	merge, err := uc.MergeMaster(authedCtx, masterID, string(destID))
	require.NoError(t, err)
	require.False(t, merge.NotFound, "master deck must be found")

	// Parity invariant: preview tally MUST equal merge tally.
	require.Equal(t, merge.Added, preview.Added, "preview.Added must equal merge.Added")
	require.Equal(t, merge.Updated, preview.Updated, "preview.Updated must equal merge.Updated")

	// Belt-and-suspenders: confirm merge itself also matches the pin.
	require.Equal(t, int64(2), merge.Added, "merge: Added must be 2")
	require.Equal(t, int64(1), merge.Updated, "merge: Updated must be 1")

}

package repository_test

// Real-DB concurrency test for the master-deck unpublish window. The catalog
// gate and the write that follows it are two separate reads; this file pins that
// an unpublish landing between them can no longer produce an import. It needs a
// real Postgres because the guarantee is a row lock, not a code path: the merge's
// in-transaction FOR SHARE read is what makes the unpublish wait, and no fake can
// model that. TestMain, testDB, insertAuthUser, sqlDBHandle, seedMasterDeck,
// seedOwnedCardgroupWithCards and newMasterCatalogUsecaseForParityTest are
// defined in sibling files and shared across this package.

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"backend/internal/auth"
	"backend/internal/domain"
	"backend/internal/repository"
	"backend/internal/usecase"
)

// TestMergeMaster_Integration_UnpublishCommitsBeforeTxBody_NoCardsImported
// sequences an unpublish into the exact instant the race used to live in: after
// the catalog gate has read the deck as published, and before the merge
// transaction body writes anything.
//
// The unpublish is staged inside an open transaction, so it holds an exclusive
// lock on the master row while remaining invisible to every other connection.
// The catalog gate therefore still sees a published deck, and the merge's
// in-transaction FOR SHARE probe blocks on the staged lock — proof the probe runs
// on the transaction connection, since a pooled read would sail past and import
// the cards. Committing the unpublish releases the probe, which re-checks the new
// row version under READ COMMITTED, fails the published filter, and collapses the
// merge into the non-disclosure not-found outcome with nothing written.
//
// The mirror interleaving is also correct and deliberately not asserted here: an
// unpublish arriving after the probe blocks until the merge commits, so the deck
// was published for the whole import.
func TestMergeMaster_Integration_UnpublishCommitsBeforeTxBody_NoCardsImported(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	ownerID := insertAuthUser(t, ctx)
	authedCtx := auth.ContextWithUser(ctx, &auth.AuthUser{
		Sub:           ownerID,
		Email:         ownerID + "@test.example",
		EmailVerified: true,
	})

	masterID := seedMasterDeck(t, ctx, "Unpublish Race "+uuid.NewString(), []*domain.MasterCard{
		{ID: uuid.NewString(), Front: domain.CardText("alpha"), Back: domain.CardText("alpha-back"), Position: 0},
		{ID: uuid.NewString(), Front: domain.CardText("beta"), Back: domain.CardText("beta-back"), Position: 1},
	})
	destID := seedOwnedCardgroupWithCards(t, ctx, ownerID, "Race Dest "+uuid.NewString(), nil)

	// Stage the unpublish: the row now carries an exclusive lock, but no other
	// connection can see the new status until this transaction commits.
	unpublishTx, err := sqlDBHandle(t).BeginTx(ctx, nil)
	require.NoError(t, err)
	defer func() { _ = unpublishTx.Rollback() }()
	_, err = unpublishTx.ExecContext(ctx,
		`UPDATE master_cardgroups SET status = 'draft' WHERE id = $1`, masterID)
	require.NoError(t, err)

	uc := newMasterCatalogUsecaseForParityTest(t)

	type mergeResult struct {
		out usecase.MergeMasterOutcome
		err error
	}
	done := make(chan mergeResult, 1)
	go func() {
		out, mergeErr := uc.MergeMaster(authedCtx, masterID, string(destID))
		done <- mergeResult{out: out, err: mergeErr}
	}()

	select {
	case r := <-done:
		t.Fatalf("merge completed while an uncommitted unpublish held the master row: out=%+v err=%v", r.out, r.err)
	case <-time.After(500 * time.Millisecond):
		// Still blocked on the row lock, which is the expected state. A slower
		// machine that has not reached the probe yet is equally fine: the
		// unpublish commits below either way, and the probe then reads it.
	}

	require.NoError(t, unpublishTx.Commit())

	select {
	case r := <-done:
		require.NoError(t, r.err)
		assert.True(t, r.out.NotFound,
			"an unpublish committed before the merge writes must yield the not-found outcome")
		assert.Nil(t, r.out.Cardgroup)
		assert.Zero(t, r.out.Added)
		assert.Zero(t, r.out.Updated)
	case <-time.After(30 * time.Second):
		t.Fatal("merge did not finish after the unpublish committed")
	}

	cardRepo := repository.NewCardRepository(testDB.GORM)
	got, err := cardRepo.ListByCardgroup(ctx, string(destID))
	require.NoError(t, err)
	assert.Empty(t, got, "no master cards may be imported into the destination")
}

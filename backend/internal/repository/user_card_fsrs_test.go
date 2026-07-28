package repository_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"backend/internal/domain"
	"backend/internal/repository"
)

func TestUserCardFSRSRepository_UpsertTxAndFindByUserAndCardIDs(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ownerID := insertAuthUser(t, ctx)
	cg := insertCardgroup(t, ctx, ownerID)
	cardRepo := repository.NewCardRepository(testDB.GORM)
	ucsRepo := repository.NewUserCardFSRSRepository(testDB.GORM)

	card := newCard(cg.ID, "front", "back")
	require.NoError(t, cardRepo.Create(ctx, card))

	now := time.Now().UTC().Truncate(time.Microsecond)
	first := domain.NewUserCardFSRSForNewCard(domain.UserID(ownerID), card.ID, now)
	first.State.Reps = 1
	first.State.Due = now.Add(time.Hour)
	first.UpdatedAt = time.Unix(1, 0).UTC()

	require.NoError(t, testDB.GORM.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return ucsRepo.UpsertTx(ctx, tx, first)
	}))

	got, err := ucsRepo.FindByUserAndCardIDs(ctx, ownerID, []string{card.ID})
	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Equal(t, 1, got[card.ID].State.Reps)
	require.True(t, got[card.ID].State.Due.Equal(first.State.Due))
	require.Equal(t, domain.Rating(0), got[card.ID].State.LastRating)
	require.Equal(t, first.UpdatedAt, got[card.ID].UpdatedAt,
		"UpsertTx must copy the database-assigned updated_at back into the aggregate")
	require.NotEqual(t, time.Unix(1, 0).UTC(), first.UpdatedAt)

	second := domain.NewUserCardFSRSForNewCard(domain.UserID(ownerID), card.ID, now.Add(time.Minute))
	second.State.Reps = 2
	second.State.Lapses = 1
	second.State.Due = now.Add(24 * time.Hour)
	second.State.LastRating = domain.RatingEasy
	require.NoError(t, testDB.GORM.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return ucsRepo.UpsertTx(ctx, tx, second)
	}))

	got, err = ucsRepo.FindByUserAndCardIDs(ctx, ownerID, []string{card.ID})
	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Equal(t, 2, got[card.ID].State.Reps)
	require.Equal(t, 1, got[card.ID].State.Lapses)
	require.True(t, got[card.ID].State.Due.Equal(second.State.Due))
	require.Equal(t, domain.RatingEasy, got[card.ID].State.LastRating)
	require.Equal(t, second.UpdatedAt, got[card.ID].UpdatedAt,
		"conflict updates must return the database-assigned updated_at")
}

func TestUserCardFSRSRepository_FindByUserAndCardIDs_InvalidLastRating(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ownerID := insertAuthUser(t, ctx)
	cg := insertCardgroup(t, ctx, ownerID)
	cardRepo := repository.NewCardRepository(testDB.GORM)
	ucsRepo := repository.NewUserCardFSRSRepository(testDB.GORM)

	card := newCard(cg.ID, "invalid-last-rating", "back")
	require.NoError(t, cardRepo.Create(ctx, card))

	now := time.Now().UTC().Truncate(time.Microsecond)
	_, err := sqlDBHandle(t).ExecContext(ctx,
		`INSERT INTO public.user_card_fsrs
				(user_id, card_id, state, due, stability, difficulty, reps, lapses, last_review, last_rating, scheduled_days)
			 VALUES ($1, $2, $3, $4, 6.9, 5.0, 1, 0, $4, 9, 1)`,
		ownerID, card.ID, int(domain.FSRSPhaseReview), now)
	require.NoError(t, err, "seed a row with an invalid last_rating value via raw SQL")

	_, err = ucsRepo.FindByUserAndCardIDs(ctx, ownerID, []string{card.ID})
	require.ErrorContains(t, err, "repository: invalid last_rating value 9 for card "+card.ID)
}

// TestUserCardFSRSRepository_FindByUserAndCardIDs_InvalidStabilityOrDifficulty
// proves the reconstitution guard rejects a persisted stability the scheduler
// can never produce (non-finite or non-positive) and a difficulty outside the
// [MinDifficulty, MaxDifficulty] FSRS scale. Neither column carries a CHECK
// constraint and no application write path yields such a value, so raw SQL is
// the only way to seed one — the guard is defence in depth against a row edited
// outside the application. Without it a NaN stability reconstitutes silently and
// breaks JSON marshalling of the UserCardState GraphQL Float it feeds. The
// mastery-tier harm travels through the sibling ListFSRSStatesByUser projection,
// which carries its own stability guard.
func TestUserCardFSRSRepository_FindByUserAndCardIDs_InvalidStabilityOrDifficulty(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ownerID := insertAuthUser(t, ctx)
	cg := insertCardgroup(t, ctx, ownerID)
	cardRepo := repository.NewCardRepository(testDB.GORM)
	ucsRepo := repository.NewUserCardFSRSRepository(testDB.GORM)

	cases := []struct {
		name string
		// stability and difficulty are SQL literals so the non-finite values can
		// be seeded exactly as Postgres stores them in a double precision column.
		stability  string
		difficulty string
		wantErr    string
	}{
		{name: "stability_nan", stability: "'NaN'", difficulty: "5.0", wantErr: "repository: invalid stability value"},
		{name: "stability_positive_infinity", stability: "'Infinity'", difficulty: "5.0", wantErr: "repository: invalid stability value"},
		{name: "stability_negative_infinity", stability: "'-Infinity'", difficulty: "5.0", wantErr: "repository: invalid stability value"},
		{name: "stability_zero", stability: "0", difficulty: "5.0", wantErr: "repository: invalid stability value"},
		{name: "stability_negative", stability: "-1.5", difficulty: "5.0", wantErr: "repository: invalid stability value"},
		{name: "difficulty_below_scale", stability: "6.9", difficulty: "0.5", wantErr: "repository: invalid difficulty value"},
		{name: "difficulty_above_scale", stability: "6.9", difficulty: "10.5", wantErr: "repository: invalid difficulty value"},
		{name: "difficulty_nan", stability: "6.9", difficulty: "'NaN'", wantErr: "repository: invalid difficulty value"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			card := newCard(cg.ID, "fsrs-range-"+tc.name, "back")
			require.NoError(t, cardRepo.Create(ctx, card))

			now := time.Now().UTC().Truncate(time.Microsecond)
			_, err := sqlDBHandle(t).ExecContext(ctx,
				`INSERT INTO public.user_card_fsrs
						(user_id, card_id, state, due, stability, difficulty, reps, lapses, last_review, last_rating, scheduled_days)
					 VALUES ($1, $2, $3, $4, `+tc.stability+`::double precision, `+tc.difficulty+`::double precision, 1, 0, $4, NULL, 1)`,
				ownerID, card.ID, int(domain.FSRSPhaseReview), now)
			require.NoError(t, err, "seed an out-of-range scheduling row via raw SQL")

			_, err = ucsRepo.FindByUserAndCardIDs(ctx, ownerID, []string{card.ID})
			require.ErrorContains(t, err, tc.wantErr)
			require.ErrorContains(t, err, card.ID, "the rejection must name the offending card")
		})
	}
}

// TestUserCardFSRSRepository_OnCardDelete_CascadesFSRSRow proves the
// user_card_fsrs.card_id -> cards(id) FK is ON DELETE CASCADE: deleting a card
// removes every user's FSRS row for it. This is the runtime behaviour that
// idx_user_card_fsrs_card_id backs — the composite PK (user_id, card_id) cannot
// serve a card_id-only lookup, so without the index this cascade sequential-scans
// the whole table.
//   - SET NULL would violate the NOT NULL on user_card_fsrs.card_id.
//   - RESTRICT would block the card DELETE, breaking the cardgroup- and
//     user-deletion cascades that pass through cards.
func TestUserCardFSRSRepository_OnCardDelete_CascadesFSRSRow(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ownerID := insertAuthUser(t, ctx)
	cg := insertCardgroup(t, ctx, ownerID)
	cardRepo := repository.NewCardRepository(testDB.GORM)
	ucsRepo := repository.NewUserCardFSRSRepository(testDB.GORM)

	card := newCard(cg.ID, "front", "back")
	require.NoError(t, cardRepo.Create(ctx, card))

	now := time.Now().UTC().Truncate(time.Microsecond)
	state := domain.NewUserCardFSRSForNewCard(domain.UserID(ownerID), card.ID, now)
	require.NoError(t, testDB.GORM.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return ucsRepo.UpsertTx(ctx, tx, state)
	}))

	// Precondition: the FSRS row exists before the card is deleted.
	got, err := ucsRepo.FindByUserAndCardIDs(ctx, ownerID, []string{card.ID})
	require.NoError(t, err)
	require.Len(t, got, 1, "expected the FSRS row to exist before card deletion")

	// Delete the card; the ON DELETE CASCADE FK on user_card_fsrs.card_id must
	// remove the dependent FSRS row.
	require.NoError(t, cardRepo.Delete(ctx, card.ID))

	got, err = ucsRepo.FindByUserAndCardIDs(ctx, ownerID, []string{card.ID})
	require.NoError(t, err)
	require.Len(t, got, 0, "FSRS row still exists after card deletion: FK is not ON DELETE CASCADE")
}

func TestUserCardFSRSRepository_FindByUserAndCardIDs_ScopesByViewer(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ownerID := insertAuthUser(t, ctx)
	otherUserID := insertAuthUser(t, ctx)
	cg := insertCardgroup(t, ctx, ownerID)
	cardRepo := repository.NewCardRepository(testDB.GORM)
	ucsRepo := repository.NewUserCardFSRSRepository(testDB.GORM)

	card := newCard(cg.ID, "shared", "back")
	require.NoError(t, cardRepo.Create(ctx, card))

	now := time.Now().UTC().Truncate(time.Microsecond)
	ownerState := domain.NewUserCardFSRSForNewCard(domain.UserID(ownerID), card.ID, now)
	ownerState.State.Reps = 1
	otherState := domain.NewUserCardFSRSForNewCard(domain.UserID(otherUserID), card.ID, now)
	otherState.State.Reps = 7

	require.NoError(t, testDB.GORM.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := ucsRepo.UpsertTx(ctx, tx, ownerState); err != nil {
			return err
		}
		return ucsRepo.UpsertTx(ctx, tx, otherState)
	}))

	got, err := ucsRepo.FindByUserAndCardIDs(ctx, ownerID, []string{card.ID})
	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Equal(t, 1, got[card.ID].State.Reps)
}

func TestUserCardFSRSRepository_FindByUserAndCardIDs_EmptySlice(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ucsRepo := repository.NewUserCardFSRSRepository(testDB.GORM)

	got, err := ucsRepo.FindByUserAndCardIDs(ctx, "00000000-0000-0000-0000-000000000001", nil)
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Empty(t, got)
}

func TestUserCardFSRSRepository_FindByUserAndCardIDsTx_EmptySlice(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ucsRepo := repository.NewUserCardFSRSRepository(testDB.GORM)

	var got map[string]*domain.UserCardFSRS
	err := testDB.GORM.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var inner error
		got, inner = ucsRepo.FindByUserAndCardIDsTx(ctx, tx, "00000000-0000-0000-0000-000000000001", nil)
		return inner
	})
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Empty(t, got)
}

// TestUserCardFSRSRepository_ListFSRSStatesByUser_ScopesByViewer proves
// ListFSRSStatesByUser returns only the caller's studied rows, joined to cards
// for the owning cardgroup, and never leaks another user's FSRS row — even when
// both users studied the same card. Cross-tenant negative test per
// docs/backend/library-gotchas/repository-cross-tenant-negative-test.md.
func TestUserCardFSRSRepository_ListFSRSStatesByUser_ScopesByViewer(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ownerID := insertAuthUser(t, ctx)
	otherUserID := insertAuthUser(t, ctx)
	cgA := insertCardgroupForUser(t, ctx, ownerID, "Stats Deck A")
	cgOther := insertCardgroupForUser(t, ctx, otherUserID, "Other Stats Deck")
	cardRepo := repository.NewCardRepository(testDB.GORM)
	ucsRepo := repository.NewUserCardFSRSRepository(testDB.GORM)

	mature := newCard(domain.CardgroupID(cgA), "mature", "back")
	learned := newCard(domain.CardgroupID(cgA), "learned", "back")
	otherCard := newCard(domain.CardgroupID(cgOther), "other", "back")
	require.NoError(t, cardRepo.Create(ctx, mature))
	require.NoError(t, cardRepo.Create(ctx, learned))
	require.NoError(t, cardRepo.Create(ctx, otherCard))

	now := time.Now().UTC().Truncate(time.Microsecond)

	// Owner: one mature (Review, 25d) and one learned (Review, 5d) card.
	ownerMature := domain.NewUserCardFSRSForNewCard(domain.UserID(ownerID), mature.ID, now)
	ownerMature.State.Phase = domain.FSRSPhaseReview
	ownerMature.State.Stability = 25
	ownerLearned := domain.NewUserCardFSRSForNewCard(domain.UserID(ownerID), learned.ID, now)
	ownerLearned.State.Phase = domain.FSRSPhaseReview
	ownerLearned.State.Stability = 5
	ownerLearned.State.Lapses = 3

	// Other user studies their own card AND the owner's mature card, with a
	// distinct stability so a leak would be detectable.
	otherOwn := domain.NewUserCardFSRSForNewCard(domain.UserID(otherUserID), otherCard.ID, now)
	otherOwn.State.Phase = domain.FSRSPhaseReview
	otherOwn.State.Stability = 30
	otherOnOwnerCard := domain.NewUserCardFSRSForNewCard(domain.UserID(otherUserID), mature.ID, now)
	otherOnOwnerCard.State.Phase = domain.FSRSPhaseReview
	otherOnOwnerCard.State.Stability = 99

	require.NoError(t, testDB.GORM.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, s := range []*domain.UserCardFSRS{ownerMature, ownerLearned, otherOwn, otherOnOwnerCard} {
			if err := ucsRepo.UpsertTx(ctx, tx, s); err != nil {
				return err
			}
		}
		return nil
	}))

	rows, err := ucsRepo.ListFSRSStatesByUser(ctx, ownerID)
	require.NoError(t, err)
	require.Len(t, rows, 2, "owner has exactly two studied cards")

	byCard := make(map[string]domain.FSRSStat, len(rows))
	for _, r := range rows {
		byCard[r.CardID] = r
	}
	require.NotContains(t, byCard, otherCard.ID, "another user's card must never leak")

	gotMature, ok := byCard[mature.ID]
	require.True(t, ok)
	require.Equal(t, cgA, gotMature.CardgroupID)
	require.Equal(t, domain.FSRSPhaseReview, gotMature.Phase)
	require.InDelta(t, 25.0, gotMature.Stability, 1e-9,
		"owner's own stability (25), not the other user's row on the same card (99)")

	gotLearned, ok := byCard[learned.ID]
	require.True(t, ok)
	require.Equal(t, cgA, gotLearned.CardgroupID)
	require.Equal(t, 3, gotLearned.Lapses)
	require.InDelta(t, 5.0, gotLearned.Stability, 1e-9)

	// The other user sees their own two rows, keyed correctly.
	otherRows, err := ucsRepo.ListFSRSStatesByUser(ctx, otherUserID)
	require.NoError(t, err)
	require.Len(t, otherRows, 2)
}

// TestUserCardFSRSRepository_ListFSRSStatesByUser_InvalidPhaseErrors proves
// ListFSRSStatesByUser rejects a row whose persisted state column does not map
// to a known FSRSPhase, mirroring the userCardFSRSToDomain IsValid guard used
// by FindByUserAndCardIDs. A raw SQL insert is required to seed the invalid
// value because the domain constructor and UpsertTx never produce one.
func TestUserCardFSRSRepository_ListFSRSStatesByUser_InvalidPhaseErrors(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ownerID := insertAuthUser(t, ctx)
	cg := insertCardgroupForUser(t, ctx, ownerID, "Invalid Phase Deck")
	cardRepo := repository.NewCardRepository(testDB.GORM)
	ucsRepo := repository.NewUserCardFSRSRepository(testDB.GORM)

	card := newCard(domain.CardgroupID(cg), "bad-state", "back")
	require.NoError(t, cardRepo.Create(ctx, card))

	sqlDB := sqlDBHandle(t)
	now := time.Now().UTC().Truncate(time.Microsecond)
	_, err := sqlDB.ExecContext(ctx,
		`INSERT INTO public.user_card_fsrs
				(user_id, card_id, state, due, stability, difficulty, reps, lapses, last_review, scheduled_days)
			 VALUES ($1, $2, 99, $3, 0, 0, 0, 0, $3, 0)`,
		ownerID, card.ID, now)
	require.NoError(t, err, "seed a row with an invalid FSRSPhase value via raw SQL")

	_, err = ucsRepo.ListFSRSStatesByUser(ctx, ownerID)
	require.Error(t, err, "an invalid persisted FSRSPhase must be rejected, not silently reconstituted")
}

// TestUserCardFSRSRepository_ListFSRSStatesByUser_InvalidStabilityErrors proves
// the stability guard mirrors the userCardFSRSToDomain one on the /stats
// projection. This is the path domain.ClassifyMastery consumes, so an unchecked
// NaN would be silently bucketed into the Learned mastery tier (NaN fails both
// of its comparisons) and would break JSON marshalling of the GraphQL Float
// StrugglingCard.stability feeds. Raw SQL is the only way to seed such a value:
// the column carries no CHECK constraint and no application write path yields
// one.
func TestUserCardFSRSRepository_ListFSRSStatesByUser_InvalidStabilityErrors(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	cases := []struct {
		name string
		// stability is a SQL literal so the non-finite values can be seeded
		// exactly as Postgres stores them in a double precision column.
		stability string
	}{
		{name: "nan", stability: "'NaN'"},
		{name: "positive_infinity", stability: "'Infinity'"},
		{name: "negative_infinity", stability: "'-Infinity'"},
		{name: "zero", stability: "0"},
		{name: "negative", stability: "-1.5"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ownerID := insertAuthUser(t, ctx)
			cg := insertCardgroupForUser(t, ctx, ownerID, "Invalid Stability Deck "+tc.name)
			cardRepo := repository.NewCardRepository(testDB.GORM)
			ucsRepo := repository.NewUserCardFSRSRepository(testDB.GORM)

			card := newCard(domain.CardgroupID(cg), "bad-stability-"+tc.name, "back")
			require.NoError(t, cardRepo.Create(ctx, card))

			now := time.Now().UTC().Truncate(time.Microsecond)
			_, err := sqlDBHandle(t).ExecContext(ctx,
				`INSERT INTO public.user_card_fsrs
						(user_id, card_id, state, due, stability, difficulty, reps, lapses, last_review, scheduled_days)
					 VALUES ($1, $2, $3, $4, `+tc.stability+`::double precision, 5.0, 1, 0, $4, 1)`,
				ownerID, card.ID, int(domain.FSRSPhaseReview), now)
			require.NoError(t, err, "seed a row with an invalid stability value via raw SQL")

			_, err = ucsRepo.ListFSRSStatesByUser(ctx, ownerID)
			require.ErrorContains(t, err, "repository: user card fsrs: invalid stability value",
				"an invalid persisted stability must be rejected, not fed to ClassifyMastery")
		})
	}
}

// TestUserCardFSRSRepository_CountCardsByCardgroupForUser_ScopesByOwner proves
// CountCardsByCardgroupForUser returns per-deck totals only for cardgroups the
// user owns (cardgroups.owner_id = ?) that have at least one card — a deck
// with cards but zero studied cards is included, a deck with zero cards is
// omitted (no acquisition denominator) — and another user's deck never leaks.
func TestUserCardFSRSRepository_CountCardsByCardgroupForUser_ScopesByOwner(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ownerID := insertAuthUser(t, ctx)
	otherUserID := insertAuthUser(t, ctx)
	cgA := insertCardgroupForUser(t, ctx, ownerID, "Count Deck A")
	cgB := insertCardgroupForUser(t, ctx, ownerID, "Count Deck B")
	cgEmpty := insertCardgroupForUser(t, ctx, ownerID, "Count Deck Empty")
	cgOther := insertCardgroupForUser(t, ctx, otherUserID, "Other Count Deck")
	cardRepo := repository.NewCardRepository(testDB.GORM)
	ucsRepo := repository.NewUserCardFSRSRepository(testDB.GORM)

	// Deck A: two cards. Deck B: one card. Deck Empty: no cards. Other deck: one card.
	require.NoError(t, cardRepo.Create(ctx, newCard(domain.CardgroupID(cgA), "a1", "back")))
	require.NoError(t, cardRepo.Create(ctx, newCard(domain.CardgroupID(cgA), "a2", "back")))
	require.NoError(t, cardRepo.Create(ctx, newCard(domain.CardgroupID(cgB), "b1", "back")))
	require.NoError(t, cardRepo.Create(ctx, newCard(domain.CardgroupID(cgOther), "o1", "back")))

	totals, err := ucsRepo.CountCardsByCardgroupForUser(ctx, ownerID)
	require.NoError(t, err)
	require.Len(t, totals, 2, "owner owns exactly two decks with at least one card")
	require.Equal(t, 2, totals[cgA])
	require.Equal(t, 1, totals[cgB], "a deck with zero studied cards still reports its total")
	require.NotContains(t, totals, cgOther, "another user's deck must never leak")
	require.NotContains(t, totals, cgEmpty, "an owned deck with zero cards is omitted (no acquisition denominator)")
}

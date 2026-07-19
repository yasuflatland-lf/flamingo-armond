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

func TestSwipeRecordRepository_CreateTxAndFind(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ownerID := insertAuthUser(t, ctx)
	cg := insertCardgroup(t, ctx, ownerID)
	cardRepo := repository.NewCardRepository(testDB.GORM)
	swipeRepo := repository.NewSwipeRecordRepository(testDB.GORM)

	card := newCard(cg.ID, "front", "back")
	require.NoError(t, cardRepo.Create(ctx, card))
	reviewedAt := time.Now().UTC()
	stateBefore := domain.NewFSRSStateForNewCard(reviewedAt)
	stateBefore.Phase = domain.FSRSPhaseReview
	stateBefore.ScheduledDays = 3
	stateBefore.Stability = 6.6
	state := domain.NewFSRSStateForNewCard(reviewedAt)
	state.Reps = 1
	sr, err := domain.NewSwipeRecord(domain.UserID(ownerID), card.ID, cg.ID, domain.RatingEasy, reviewedAt, stateBefore, state)
	require.NoError(t, err)

	require.NoError(t, testDB.GORM.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return swipeRepo.CreateTx(ctx, tx, sr)
	}))

	byID, err := swipeRepo.FindByIDs(ctx, []string{sr.ID})
	require.NoError(t, err)
	require.Len(t, byID, 1)
	require.Equal(t, sr.ID, byID[sr.ID].ID)
	require.Equal(t, cg.ID, byID[sr.ID].CardgroupID)
	require.Equal(t, domain.RatingEasy, byID[sr.ID].Rating)
	require.Equal(t, state.Reps, byID[sr.ID].StateAfter.Reps)
	// The pre-swipe snapshot survives the CreateTx -> read roundtrip.
	require.NotNil(t, byID[sr.ID].PhaseBefore)
	require.Equal(t, domain.FSRSPhaseReview, *byID[sr.ID].PhaseBefore)
	require.NotNil(t, byID[sr.ID].ScheduledDaysBefore)
	require.Equal(t, 3, *byID[sr.ID].ScheduledDaysBefore)
	require.NotNil(t, byID[sr.ID].StabilityBefore)
	require.InDelta(t, 6.6, *byID[sr.ID].StabilityBefore, 0.000000001)

	history, err := swipeRepo.FindByUserAndCardgroup(ctx, ownerID, string(cg.ID))
	require.NoError(t, err)
	require.Len(t, history, 1)
	require.Equal(t, sr.ID, history[0].ID)
	require.Equal(t, cg.ID, history[0].CardgroupID)
}

func TestSwipeRecordRepository_FindByUserAndCardgroup_UsesDenormalizedCardgroup(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ownerID := insertAuthUser(t, ctx)
	cgAtSwipe := insertCardgroup(t, ctx, ownerID)
	cgCurrent := insertCardgroup(t, ctx, ownerID)
	cardRepo := repository.NewCardRepository(testDB.GORM)
	swipeRepo := repository.NewSwipeRecordRepository(testDB.GORM)

	card := newCard(cgCurrent.ID, "front denormalized", "back")
	require.NoError(t, cardRepo.Create(ctx, card))
	reviewedAt := time.Now().UTC().Truncate(time.Microsecond)
	state := domain.NewFSRSStateForNewCard(reviewedAt)
	sr, err := domain.NewSwipeRecord(domain.UserID(ownerID), card.ID, cgAtSwipe.ID, domain.RatingGood, reviewedAt, state, state)
	require.NoError(t, err)

	require.NoError(t, testDB.GORM.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return swipeRepo.CreateTx(ctx, tx, sr)
	}))

	historyAtSwipe, err := swipeRepo.FindByUserAndCardgroup(ctx, ownerID, string(cgAtSwipe.ID))
	require.NoError(t, err)
	require.Len(t, historyAtSwipe, 1)
	require.Equal(t, sr.ID, historyAtSwipe[0].ID)
	require.Equal(t, cgAtSwipe.ID, historyAtSwipe[0].CardgroupID)

	historyCurrent, err := swipeRepo.FindByUserAndCardgroup(ctx, ownerID, string(cgCurrent.ID))
	require.NoError(t, err)
	require.Empty(t, historyCurrent)
}

func TestSwipeRecordRepository_ListRecentByUser_OrdersAndScopes(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ownerID := insertAuthUser(t, ctx)
	otherUserID := insertAuthUser(t, ctx)
	cg := insertCardgroup(t, ctx, ownerID)
	cardRepo := repository.NewCardRepository(testDB.GORM)
	swipeRepo := repository.NewSwipeRecordRepository(testDB.GORM)

	ownerCard1 := newCard(cg.ID, "owner 1", "back 1")
	ownerCard2 := newCard(cg.ID, "owner 2", "back 2")
	otherCard := newCard(cg.ID, "other", "back 3")
	require.NoError(t, cardRepo.Create(ctx, ownerCard1))
	require.NoError(t, cardRepo.Create(ctx, ownerCard2))
	require.NoError(t, cardRepo.Create(ctx, otherCard))

	base := time.Date(2026, 4, 30, 12, 0, 0, 0, time.UTC)
	earlier := base.Add(-time.Hour)
	sameReviewTime := base
	newest := base.Add(time.Hour)

	seed := func(id, userID, cardID string, cardgroupID domain.CardgroupID, reviewedAt time.Time) *domain.SwipeRecord {
		return &domain.SwipeRecord{
			ID:          id,
			UserID:      domain.UserID(userID),
			CardID:      cardID,
			CardgroupID: cardgroupID,
			Rating:      domain.RatingEasy,
			ReviewedAt:  reviewedAt,
			StateAfter:  domain.NewFSRSStateForNewCard(reviewedAt),
		}
	}

	require.NoError(t, testDB.GORM.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, sr := range []*domain.SwipeRecord{
			seed("00000000-0000-0000-0000-000000000001", ownerID, ownerCard1.ID, cg.ID, earlier),
			seed("00000000-0000-0000-0000-000000000002", ownerID, ownerCard1.ID, cg.ID, sameReviewTime),
			seed("00000000-0000-0000-0000-000000000003", ownerID, ownerCard2.ID, cg.ID, sameReviewTime),
			seed("00000000-0000-0000-0000-000000000004", ownerID, ownerCard2.ID, cg.ID, newest),
			seed("00000000-0000-0000-0000-000000000005", otherUserID, otherCard.ID, cg.ID, newest),
		} {
			if err := swipeRepo.CreateTx(ctx, tx, sr); err != nil {
				return err
			}
		}
		return nil
	}))

	got, err := swipeRepo.ListRecentByUser(ctx, ownerID, 10)
	require.NoError(t, err)
	require.Len(t, got, 4)
	require.Equal(t, "00000000-0000-0000-0000-000000000004", got[0].ID)
	require.Equal(t, "00000000-0000-0000-0000-000000000003", got[1].ID)
	require.Equal(t, "00000000-0000-0000-0000-000000000002", got[2].ID)
	require.Equal(t, "00000000-0000-0000-0000-000000000001", got[3].ID)
	for _, sr := range got {
		require.Equal(t, ownerID, string(sr.UserID))
	}

	limited, err := swipeRepo.ListRecentByUser(ctx, ownerID, 2)
	require.NoError(t, err)
	require.Len(t, limited, 2)
	require.Equal(t, got[0].ID, limited[0].ID)
	require.Equal(t, got[1].ID, limited[1].ID)

	empty, err := swipeRepo.ListRecentByUser(ctx, ownerID, 0)
	require.NoError(t, err)
	require.Empty(t, empty)
}

// TestSwipeRecordRepository_ListByUserSince_InclusiveBoundaryAndScopes proves
// ListByUserSince applies a `reviewed_at >= since` (inclusive) cutoff — the row
// at exactly `since` is returned, the row one microsecond before is excluded —
// and never leaks another user's swipe. Exact-boundary fixture per
// docs/backend/library-gotchas/strict-cutoff-boundary-fixture-and-mutation-proof.md:
// the Len==2 assertion discriminates `>=` from a strict `>` (which would drop
// the == since row).
func TestSwipeRecordRepository_ListByUserSince_InclusiveBoundaryAndScopes(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ownerID := insertAuthUser(t, ctx)
	otherUserID := insertAuthUser(t, ctx)
	cg := insertCardgroup(t, ctx, ownerID)
	cgOther := insertCardgroup(t, ctx, otherUserID)
	cardRepo := repository.NewCardRepository(testDB.GORM)
	swipeRepo := repository.NewSwipeRecordRepository(testDB.GORM)

	ownerCard := newCard(cg.ID, "owner since front", "back")
	otherCard := newCard(cgOther.ID, "other since front", "back")
	require.NoError(t, cardRepo.Create(ctx, ownerCard))
	require.NoError(t, cardRepo.Create(ctx, otherCard))

	since := time.Date(2026, 5, 1, 9, 0, 0, 0, time.UTC)
	before := since.Add(-time.Microsecond) // strictly before the cutoff (Postgres µs precision)
	after := since.Add(time.Hour)

	seed := func(id, userID, cardID string, cardgroupID domain.CardgroupID, reviewedAt time.Time) *domain.SwipeRecord {
		return &domain.SwipeRecord{
			ID:          id,
			UserID:      domain.UserID(userID),
			CardID:      cardID,
			CardgroupID: cardgroupID,
			Rating:      domain.RatingEasy,
			ReviewedAt:  reviewedAt,
			StateAfter:  domain.NewFSRSStateForNewCard(reviewedAt),
		}
	}

	const (
		idAtSince      = "00000000-0000-0000-0000-0000000000b1"
		idBeforeSince  = "00000000-0000-0000-0000-0000000000b2"
		idAfterSince   = "00000000-0000-0000-0000-0000000000b3"
		idOtherAtSince = "00000000-0000-0000-0000-0000000000b4"
	)

	require.NoError(t, testDB.GORM.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, sr := range []*domain.SwipeRecord{
			seed(idAtSince, ownerID, ownerCard.ID, cg.ID, since),               // == since: included (>=)
			seed(idBeforeSince, ownerID, ownerCard.ID, cg.ID, before),          // < since: excluded
			seed(idAfterSince, ownerID, ownerCard.ID, cg.ID, after),            // > since: included
			seed(idOtherAtSince, otherUserID, otherCard.ID, cgOther.ID, since), // cross-tenant: excluded
		} {
			if err := swipeRepo.CreateTx(ctx, tx, sr); err != nil {
				return err
			}
		}
		return nil
	}))

	got, err := swipeRepo.ListByUserSince(ctx, ownerID, since)
	require.NoError(t, err)

	ids := make(map[string]struct{}, len(got))
	for _, sr := range got {
		ids[sr.ID] = struct{}{}
		require.Equal(t, ownerID, string(sr.UserID), "another user's swipe must never leak")
	}
	require.Len(t, got, 2, "reviewed_at == since is included (>=) and the strictly-before row is excluded")
	require.Contains(t, ids, idAtSince, "reviewed_at == since is inclusive")
	require.Contains(t, ids, idAfterSince)
	require.NotContains(t, ids, idBeforeSince, "reviewed_at strictly before since is excluded")
	require.NotContains(t, ids, idOtherAtSince, "another user's swipe is excluded")
}

// TestSwipeRecordRepository_OutOfRangeState_ReturnsError proves the reconstitution
// guard in swipeRecordToDomain rejects a corrupt enum value instead of silently
// miscounting it in the performance metrics, per
// docs/backend/library-gotchas/gorm-enum-cast-isvalid.md. A row whose `state`
// column holds an out-of-range FSRSPhase (99) must surface a non-nil error from
// every read path, not reconstitute a SwipeRecord carrying an invalid Phase.
//
// The `rating` column is not exercised here because a persisted out-of-range
// rating is impossible: the swipe_records schema enforces
// `CHECK (rating BETWEEN 1 AND 4)`, so the rating-side IsValid() guard is
// defense-in-depth against future schema drift or a manual SQL edit, not a
// condition a persisted row can reach today.
func TestSwipeRecordRepository_OutOfRangeState_ReturnsError(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ownerID := insertAuthUser(t, ctx)
	cg := insertCardgroup(t, ctx, ownerID)
	cardRepo := repository.NewCardRepository(testDB.GORM)
	swipeRepo := repository.NewSwipeRecordRepository(testDB.GORM)

	card := newCard(cg.ID, "out of range state", "back")
	require.NoError(t, cardRepo.Create(ctx, card))

	reviewedAt := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	state := domain.NewFSRSStateForNewCard(reviewedAt)
	state.Phase = domain.FSRSPhase(99) // out of range; no DB CHECK on the state column
	corrupt := &domain.SwipeRecord{
		ID:          "00000000-0000-0000-0000-0000000000c1",
		UserID:      domain.UserID(ownerID),
		CardID:      card.ID,
		CardgroupID: cg.ID,
		Rating:      domain.RatingEasy,
		ReviewedAt:  reviewedAt,
		StateAfter:  state,
	}
	require.NoError(t, testDB.GORM.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return swipeRepo.CreateTx(ctx, tx, corrupt)
	}))

	reads := map[string]func() error{
		"FindByIDs": func() error {
			_, err := swipeRepo.FindByIDs(ctx, []string{corrupt.ID})
			return err
		},
		"FindByUserAndCardgroup": func() error {
			_, err := swipeRepo.FindByUserAndCardgroup(ctx, ownerID, string(cg.ID))
			return err
		},
		"ListRecentByUser": func() error {
			_, err := swipeRepo.ListRecentByUser(ctx, ownerID, 10)
			return err
		},
		"ListByUserSince": func() error {
			_, err := swipeRepo.ListByUserSince(ctx, ownerID, reviewedAt.Add(-time.Hour))
			return err
		},
	}
	for name, read := range reads {
		t.Run(name, func(t *testing.T) {
			err := read()
			require.Error(t, err, "an out-of-range FSRSPhase must propagate as an error, not a silent success")
			require.Contains(t, err.Error(), "invalid FSRSPhase value 99")
		})
	}
}

// TestSwipeRecordRepository_LegacyRowWithoutSnapshot_ReadsBackNil proves a row
// inserted without the phase_before / scheduled_days_before / stability_before
// columns (a legacy row recorded before the migrations) reads back with every
// snapshot pointer nil — the sentinel the metrics layer branches on.
func TestSwipeRecordRepository_LegacyRowWithoutSnapshot_ReadsBackNil(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ownerID := insertAuthUser(t, ctx)
	cg := insertCardgroup(t, ctx, ownerID)
	cardRepo := repository.NewCardRepository(testDB.GORM)
	swipeRepo := repository.NewSwipeRecordRepository(testDB.GORM)

	card := newCard(cg.ID, "legacy snapshot front", "back")
	require.NoError(t, cardRepo.Create(ctx, card))

	reviewedAt := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	const rowID = "00000000-0000-0000-0000-0000000000d1"
	// Raw insert omitting phase_before / scheduled_days_before / stability_before;
	// all three default to NULL.
	require.NoError(t, testDB.GORM.WithContext(ctx).Exec(
		`INSERT INTO swipe_records
		   (id, user_id, card_id, cardgroup_id, rating, reviewed_at,
		    due, stability, difficulty, elapsed_days, scheduled_days,
		    reps, lapses, state, last_review)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		rowID, ownerID, card.ID, string(cg.ID), int(domain.RatingGood), reviewedAt,
		reviewedAt, 2.5, 5.0, 0, 0, 0, 0, int(domain.FSRSPhaseNew), reviewedAt,
	).Error)

	byID, err := swipeRepo.FindByIDs(ctx, []string{rowID})
	require.NoError(t, err)
	require.Len(t, byID, 1)
	require.Nil(t, byID[rowID].PhaseBefore, "legacy row's phase_before reads back as nil")
	require.Nil(t, byID[rowID].ScheduledDaysBefore, "legacy row's scheduled_days_before reads back as nil")
	require.Nil(t, byID[rowID].StabilityBefore, "legacy row's stability_before reads back as nil")
}

// TestSwipeRecordRepository_OutOfRangePhaseBefore_ReturnsError proves a non-nil
// phase_before holding a value outside domain.FSRSPhase.IsValid surfaces a
// repository error rather than reconstituting a SwipeRecord with an invalid
// snapshot phase.
func TestSwipeRecordRepository_OutOfRangePhaseBefore_ReturnsError(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ownerID := insertAuthUser(t, ctx)
	cg := insertCardgroup(t, ctx, ownerID)
	cardRepo := repository.NewCardRepository(testDB.GORM)
	swipeRepo := repository.NewSwipeRecordRepository(testDB.GORM)

	card := newCard(cg.ID, "out of range phase_before", "back")
	require.NoError(t, cardRepo.Create(ctx, card))

	reviewedAt := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	const rowID = "00000000-0000-0000-0000-0000000000d2"
	// state stays valid (FSRSPhaseNew); only phase_before is corrupt (99).
	require.NoError(t, testDB.GORM.WithContext(ctx).Exec(
		`INSERT INTO swipe_records
		   (id, user_id, card_id, cardgroup_id, rating, reviewed_at,
		    due, stability, difficulty, elapsed_days, scheduled_days,
		    reps, lapses, state, last_review, phase_before, scheduled_days_before)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		rowID, ownerID, card.ID, string(cg.ID), int(domain.RatingGood), reviewedAt,
		reviewedAt, 2.5, 5.0, 0, 0, 0, 0, int(domain.FSRSPhaseNew), reviewedAt, 99, 3,
	).Error)

	_, err := swipeRepo.FindByIDs(ctx, []string{rowID})
	require.Error(t, err, "an out-of-range phase_before must propagate as an error")
	require.Contains(t, err.Error(), "swipe record: invalid phase_before value 99")
}

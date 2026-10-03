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
	reviewedAt := time.Now().UTC().Truncate(time.Microsecond)
	stateBefore := domain.NewFSRSStateForNewCard(reviewedAt)
	stateBefore.Phase = domain.FSRSPhaseReview
	stateBefore.ScheduledDays = 3
	stateBefore.Stability = 6.6
	stateBefore.Due = reviewedAt.Add(3 * 24 * time.Hour)
	state := domain.NewFSRSStateForNewCard(reviewedAt)
	state.Reps = 1
	sr, err := domain.NewSwipeRecord(domain.UserID(ownerID), card.ID, cg.ID, domain.RatingEasy, reviewedAt, stateBefore, state)
	require.NoError(t, err)

	require.NoError(t, testDB.GORM.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return swipeRepo.CreateTx(ctx, tx, sr)
	}))

	var rows []struct {
		ID              string
		CardgroupID     string
		Rating          int
		Reps            int
		PhaseBefore     int16
		StabilityBefore float64
		DueBefore       time.Time
	}
	require.NoError(t, testDB.GORM.WithContext(ctx).Table("swipe_records").
		Where("id = ?", sr.ID).Find(&rows).Error)
	require.Len(t, rows, 1)
	require.Equal(t, sr.ID, rows[0].ID)
	require.Equal(t, string(cg.ID), rows[0].CardgroupID)
	require.Equal(t, int(domain.RatingEasy), rows[0].Rating)
	require.Equal(t, int(state.Reps), rows[0].Reps)
	require.Equal(t, int16(domain.FSRSPhaseReview), rows[0].PhaseBefore)
	require.InDelta(t, 6.6, rows[0].StabilityBefore, 0.000000001)
	require.True(t, stateBefore.Due.Equal(rows[0].DueBefore))

	records, err := swipeRepo.ListByUserSince(ctx, ownerID, reviewedAt.Add(-time.Microsecond))
	require.NoError(t, err)
	require.Len(t, records, 1)
	require.Equal(t, sr.ID, records[0].ID)
	require.Equal(t, cg.ID, records[0].CardgroupID)
	require.Equal(t, domain.RatingEasy, records[0].Rating)
	require.Equal(t, state.Reps, records[0].StateAfter.Reps)
	require.Equal(t, domain.FSRSPhaseReview, records[0].PhaseBefore)
	require.InDelta(t, 6.6, records[0].StabilityBefore, 0.000000001)
	require.True(t, stateBefore.Due.Equal(records[0].DueBefore))

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
		"FindByUserAndCardgroup": func() error {
			_, err := swipeRepo.FindByUserAndCardgroup(ctx, ownerID, string(cg.ID))
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

// TestSwipeRecordRepository_OutOfRangePhaseBefore_ReturnsError proves a
// phase_before value outside domain.FSRSPhase.IsValid surfaces a
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
		    due, stability, difficulty, scheduled_days, reps, lapses, state,
		    last_review, phase_before, stability_before, due_before)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		rowID, ownerID, card.ID, string(cg.ID), int(domain.RatingGood), reviewedAt,
		reviewedAt, 2.5, 5.0, 0, 0, 0, int(domain.FSRSPhaseNew), reviewedAt, 99, 2.5, reviewedAt,
	).Error)

	_, err := swipeRepo.ListByUserSince(ctx, ownerID, reviewedAt.Add(-time.Microsecond))
	require.Error(t, err, "an out-of-range phase_before must propagate as an error")
	require.Contains(t, err.Error(), "swipe record: invalid phase_before value 99")
}

// TestSwipeRecordRepository_OnDeleteCardgroup_CascadesSwipeRecords proves the
// swipe_records.cardgroup_id foreign key carries ON DELETE CASCADE, per
// docs/backend/library-gotchas/fk-action-integration-test.md.
//
// The fixture deliberately records the swipe against a DIFFERENT deck than the
// one holding the card: deleting a deck that owns the card would remove the swipe
// through the cards -> card_id cascade even with no cardgroup_id foreign key at
// all, so a same-deck fixture cannot tell the two constraints apart. Here the
// card survives in cgCurrent and only the cardgroup_id reference reaches the
// deleted deck, so the assertion isolates the new constraint.
//
// The alternatives this rules out:
//   - NO ACTION / RESTRICT would make the cardgroup DELETE fail outright,
//     breaking deck deletion for every user with review history.
//   - SET NULL would violate the column's NOT NULL and abort the DELETE.
func TestSwipeRecordRepository_OnDeleteCardgroup_CascadesSwipeRecords(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ownerID := insertAuthUser(t, ctx)
	cgAtSwipe := insertCardgroup(t, ctx, ownerID)
	cgCurrent := insertCardgroup(t, ctx, ownerID)
	cardRepo := repository.NewCardRepository(testDB.GORM)
	swipeRepo := repository.NewSwipeRecordRepository(testDB.GORM)

	card := newCard(cgCurrent.ID, "fk cascade front", "back")
	require.NoError(t, cardRepo.Create(ctx, card))
	reviewedAt := time.Now().UTC().Truncate(time.Microsecond)
	state := domain.NewFSRSStateForNewCard(reviewedAt)
	sr, err := domain.NewSwipeRecord(domain.UserID(ownerID), card.ID, cgAtSwipe.ID, domain.RatingGood, reviewedAt, state, state)
	require.NoError(t, err)
	require.NoError(t, testDB.GORM.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return swipeRepo.CreateTx(ctx, tx, sr)
	}))

	sqlDB := sqlDBHandle(t)
	var count int
	require.NoError(t, sqlDB.QueryRowContext(ctx, `SELECT COUNT(*) FROM public.swipe_records WHERE id = $1`, sr.ID).Scan(&count))
	require.Equal(t, 1, count, "swipe record must exist before the cardgroup delete")

	_, err = sqlDB.ExecContext(ctx, `DELETE FROM public.cardgroups WHERE id = $1`, string(cgAtSwipe.ID))
	require.NoError(t, err, "deleting the cardgroup must not be blocked by the foreign key")

	require.NoError(t, sqlDB.QueryRowContext(ctx, `SELECT COUNT(*) FROM public.swipe_records WHERE id = $1`, sr.ID).Scan(&count))
	require.Zero(t, count, "swipe record survived the cardgroup delete: the FK is not ON DELETE CASCADE")

	// The card itself lives in a different deck and must be untouched, proving the
	// cascade travelled through cardgroup_id rather than through cards.card_id.
	survivor, err := cardRepo.FindByID(ctx, string(card.ID))
	require.NoError(t, err)
	require.Equal(t, card.ID, survivor.ID)
}

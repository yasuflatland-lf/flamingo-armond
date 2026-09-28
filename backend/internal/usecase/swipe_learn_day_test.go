package usecase

import (
	"bytes"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"backend/internal/domain"
	"backend/internal/domain/service"
)

// newSwipeLearnDayFixture builds a swipe usecase whose card and cardgroup
// repositories resolve "card-1" in "cg-1" owned by "user-1". existing is the
// FSRS row FindByUserAndCardIDsTx returns; pass nil for the brand-new-card path
// where no row exists yet. The concrete usecase receives the fixed clock after
// construction so the public constructor signatures remain unchanged.
func newSwipeLearnDayFixture(existing *domain.UserCardFSRS, now time.Time, logger *slog.Logger) (SwipeUsecase, *mockUserCardFSRSRepository, *mockSwipeRecordRepoForSwipe) {
	byCardID := map[string]*domain.UserCardFSRS{}
	if existing != nil {
		byCardID[existing.CardID] = existing
	}
	if logger == nil {
		logger = newTestLogger()
	}
	userFSRSRepo := &mockUserCardFSRSRepository{byCardID: byCardID}
	swipeRepo := &mockSwipeRecordRepoForSwipe{}
	tx, _ := fakeTxRunner()
	uc := NewSwipeUsecaseWithTx(
		&mockCardRepository{
			findResult: &domain.Card{
				ID:          "card-1",
				CardgroupID: domain.CardgroupID("cg-1"),
			},
		},
		&mockCardgroupRepoForCard{
			findResult: &domain.Cardgroup{ID: domain.CardgroupID("cg-1"), OwnerID: "user-1"},
		},
		swipeRepo,
		service.NewFSRSScheduler(),
		tx,
		userFSRSRepo,
		logger,
	)
	uc.(*swipeUsecase).clock = fixedClock{now: now}
	return uc, userFSRSRepo, swipeRepo
}

// reviewedCardFSRS returns an already-reviewed FSRS row for "card-1" whose
// LastReview is lastReview and whose Due is one day later, with non-zero counters.
// A same-learn-day fixture is therefore also not due before the learn-day end, so
// replay-guard skip tests assert which log line fired to pin that the replay
// guard runs first; the not-due guard has its own tests that set Due explicitly.
func reviewedCardFSRS(lastReview time.Time) *domain.UserCardFSRS {
	return &domain.UserCardFSRS{
		UserID: domain.UserID("user-1"),
		CardID: "card-1",
		State: domain.FSRSState{
			Due:           lastReview.Add(24 * time.Hour),
			Stability:     12.5,
			Difficulty:    5.5,
			ScheduledDays: 1,
			Reps:          3,
			Lapses:        1,
			Phase:         domain.FSRSPhaseReview,
			LastReview:    lastReview,
			LastRating:    domain.RatingGood,
		},
	}
}

func swipeCard1(uc SwipeUsecase) (HandleSwipeOutcome, error) {
	return swipeCard1WithRating(uc, domain.RatingEasy)
}

func swipeCard1WithRating(uc SwipeUsecase, rating domain.Rating) (HandleSwipeOutcome, error) {
	return uc.HandleSwipe(authedCtx("user-1"), HandleSwipeInput{
		CardID:      "card-1",
		CardgroupID: "cg-1",
		Rating:      int(rating),
	})
}

// TestSwipeUsecase_HandleSwipe_SameLearnDayRepeat_IsSuccessShapedNoOp pins the
// write-side complement of the learn-queue invariant: a second review of the
// same card within the same JST learn day neither advances the schedule nor
// writes a second swipe record, yet still returns the success outcome so a
// replayed request needs no special client handling.
func TestSwipeUsecase_HandleSwipe_SameLearnDayRepeat_IsSuccessShapedNoOp(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 6, 5, 3, 0, 0, 0, time.UTC)
	lastReview := domain.StartOfLearnDay(now).Add(time.Minute)
	existing := reviewedCardFSRS(lastReview)
	before := existing.State
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	uc, userFSRSRepo, swipeRepo := newSwipeLearnDayFixture(existing, now, logger)

	outcome, err := swipeCard1(uc)

	require.NoError(t, err)
	require.Contains(t, buf.String(), `"msg":"swipe: repeat review ignored"`)
	require.NotContains(t, buf.String(), `"msg":"swipe: not-due review ignored"`)
	require.NotNil(t, outcome.Swipe, "an ignored repeat must still return the success variant")
	require.Equal(t, "card-1", outcome.Swipe.CardID, "an ignored repeat still echoes the card id")
	require.Nil(t, outcome.Validation, "an ignored repeat is not a user-input error")
	require.Nil(t, userFSRSRepo.upserted, "the FSRS row must not be re-upserted")
	require.Nil(t, swipeRepo.created, "no second swipe record may be written")
	require.Equal(t, before, existing.State, "the scheduling state must be left byte-identical")
	require.Equal(t, 3, existing.State.Reps)
	require.Equal(t, 1, existing.State.Lapses)
	require.InDelta(t, 12.5, existing.State.Stability, 0)
}

// TestSwipeUsecase_HandleSwipe_LearnDayBoundary pins the strict/non-strict
// comparison at JST midnight. StartOfLearnDay documents the serving-side window
// as "last_review strictly before this boundary", so a row whose LastReview sits
// exactly on the boundary belongs to today and must be ignored, while one
// nanosecond earlier belongs to the previous learn day and must apply.
func TestSwipeUsecase_HandleSwipe_LearnDayBoundary(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 6, 5, 3, 0, 0, 0, time.UTC)
	boundary := domain.StartOfLearnDay(now)
	cases := []struct {
		name       string
		lastReview time.Time
		wantSkip   bool
	}{
		{
			name:       "exactly at the JST midnight boundary is the current learn day",
			lastReview: boundary,
			wantSkip:   true,
		},
		{
			name:       "one nanosecond before the boundary is the previous learn day",
			lastReview: boundary.Add(-time.Nanosecond),
			wantSkip:   false,
		},
		{
			name:       "a full day before the boundary applies normally",
			lastReview: boundary.Add(-24 * time.Hour),
			wantSkip:   false,
		},
		{
			name:       "a zero-value last review earns no scheduling credit",
			lastReview: time.Time{},
			wantSkip:   true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			existing := reviewedCardFSRS(tc.lastReview)
			var buf bytes.Buffer
			logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
			uc, userFSRSRepo, swipeRepo := newSwipeLearnDayFixture(existing, now, logger)

			outcome, err := swipeCard1(uc)

			require.NoError(t, err)
			require.NotNil(t, outcome.Swipe)
			if tc.wantSkip {
				require.Contains(t, buf.String(), `"msg":"swipe: repeat review ignored"`)
				require.NotContains(t, buf.String(), `"msg":"swipe: not-due review ignored"`)
				require.Nil(t, userFSRSRepo.upserted, "same-learn-day repeat must not upsert")
				require.Nil(t, swipeRepo.created, "same-learn-day repeat must not write a swipe record")
				require.Equal(t, 3, existing.State.Reps, "reps must not advance")
				return
			}
			require.NotNil(t, userFSRSRepo.upserted, "a swipe on a later learn day must upsert the FSRS row")
			require.NotNil(t, swipeRepo.created, "a swipe on a later learn day must write a swipe record")
			require.Equal(t, 4, userFSRSRepo.upserted.State.Reps, "reps must advance")
			require.False(t, userFSRSRepo.upserted.State.LastReview.Before(boundary),
				"the applied swipe must stamp LastReview inside the current learn day")
		})
	}
}

// TestSwipeUsecase_HandleSwipe_ZeroCreditRepeatAcrossLearnDayIgnored pins the
// UTC-date half of the replay guard. A repeat can cross JST midnight while
// remaining inside one UTC date, so learn-day membership alone is insufficient.
func TestSwipeUsecase_HandleSwipe_ZeroCreditRepeatAcrossLearnDayIgnored(t *testing.T) {
	t.Parallel()

	lastReview := time.Date(2026, 4, 26, 0, 30, 0, 0, time.UTC)
	zeroCreditNow := time.Date(2026, 4, 26, 23, 30, 0, 0, time.UTC)
	existing := reviewedCardFSRS(lastReview)
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	uc, userFSRSRepo, swipeRepo := newSwipeLearnDayFixture(existing, zeroCreditNow, logger)

	outcome, err := swipeCard1(uc)

	require.NoError(t, err)
	require.NotNil(t, outcome.Swipe, "an ignored repeat must still return the success variant")
	require.Nil(t, outcome.Validation, "an ignored repeat is not a user-input error")
	require.Nil(t, userFSRSRepo.upserted, "a zero-credit repeat must not upsert the FSRS row")
	require.Nil(t, swipeRepo.created, "a zero-credit repeat must not write a swipe record")
	require.Contains(t, buf.String(), `"msg":"swipe: repeat review ignored"`)
	require.Contains(t, buf.String(), `"card_id":"card-1"`)

	creditNow := time.Date(2026, 4, 27, 0, 30, 0, 0, time.UTC)
	uc.(*swipeUsecase).clock = fixedClock{now: creditNow}

	outcome, err = swipeCard1(uc)

	require.NoError(t, err)
	require.NotNil(t, outcome.Swipe, "a credited review must return the success variant")
	require.Nil(t, outcome.Validation)
	require.NotNil(t, userFSRSRepo.upserted, "a review on a different UTC date must upsert the FSRS row")
	require.NotNil(t, swipeRepo.created, "a review on a different UTC date must write a swipe record")
	require.Equal(t, 4, userFSRSRepo.upserted.State.Reps, "the credited review must advance reps")
	require.True(t, userFSRSRepo.upserted.State.LastReview.Equal(creditNow),
		"the applied swipe must use the injected clock instant")
}

// TestSwipeUsecase_HandleSwipe_BrandNewCard_IsNotSkipped guards the trap the
// same-learn-day check invites: NewUserCardFSRSForNewCard stamps LastReview with
// now, so a guard reading the synthesized state instead of the loaded row would
// silently swallow the very first swipe of every card.
func TestSwipeUsecase_HandleSwipe_BrandNewCard_IsNotSkipped(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 6, 5, 3, 0, 0, 0, time.UTC)
	uc, userFSRSRepo, swipeRepo := newSwipeLearnDayFixture(nil, now, nil)

	outcome, err := swipeCard1(uc)

	require.NoError(t, err)
	require.NotNil(t, outcome.Swipe)
	require.NotNil(t, userFSRSRepo.upserted, "the first swipe of a brand-new card must persist its FSRS row")
	require.NotNil(t, swipeRepo.created, "the first swipe of a brand-new card must write a swipe record")
	require.Equal(t, 1, userFSRSRepo.upserted.State.Reps)
	require.Equal(t, domain.FSRSPhaseNew, swipeRepo.created.PhaseBefore,
		"the recorded pre-swipe phase must be the synthesized new-card phase")
}

// TestSwipeUsecase_HandleSwipe_StaleTabNextMorning_NotDueIgnored replays the
// stale-tab counterexample: a Hard at 21:00 JST schedules the card two days out,
// and a second rating from the same tab at 09:00 JST the next morning passes
// both replay rules yet targets a card the learn queue does not serve. It must
// be ignored without moving the due date forward.
func TestSwipeUsecase_HandleSwipe_StaleTabNextMorning_NotDueIgnored(t *testing.T) {
	t.Parallel()

	day0 := time.Date(2026, 9, 28, 12, 0, 0, 0, time.UTC) // 21:00 JST
	uc0, repo0, _ := newSwipeLearnDayFixture(nil, day0, nil)
	_, err := swipeCard1WithRating(uc0, domain.RatingHard)
	require.NoError(t, err)
	existing := repo0.upserted
	require.NotNil(t, existing)
	require.True(t, existing.State.Due.Equal(time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC)),
		"a first Hard at 21:00 JST must schedule the card two days out, got %s", existing.State.Due)
	before := existing.State

	day1 := time.Date(2026, 9, 29, 0, 0, 0, 0, time.UTC) // 09:00 JST
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	uc1, repo1, swipe1 := newSwipeLearnDayFixture(existing, day1, logger)

	outcome, err := swipeCard1WithRating(uc1, domain.RatingHard)

	require.NoError(t, err)
	require.NotNil(t, outcome.Swipe, "an ignored not-due rating must still return the success variant")
	require.Nil(t, outcome.Validation, "an ignored not-due rating is not a user-input error")
	require.Nil(t, repo1.upserted, "a not-due rating must not upsert the FSRS row")
	require.Nil(t, swipe1.created, "a not-due rating must not write a swipe record")
	require.Equal(t, before, existing.State, "the scheduling state must be left byte-identical")
	require.Contains(t, buf.String(), `"msg":"swipe: not-due review ignored"`)
	require.Contains(t, buf.String(), `"card_id":"card-1"`)
	require.NotContains(t, buf.String(), `"msg":"swipe: repeat review ignored"`)
}

// TestSwipeUsecase_HandleSwipe_AgainNextMorning_DueTodayApplies is the servable
// counterpart: an Again at 21:00 JST makes the card due at 21:00 JST the next
// day, before that learn day ends, so a rating the next morning is applied.
func TestSwipeUsecase_HandleSwipe_AgainNextMorning_DueTodayApplies(t *testing.T) {
	t.Parallel()

	day0 := time.Date(2026, 9, 28, 12, 0, 0, 0, time.UTC) // 21:00 JST
	uc0, repo0, _ := newSwipeLearnDayFixture(nil, day0, nil)
	_, err := swipeCard1WithRating(uc0, domain.RatingAgain)
	require.NoError(t, err)
	existing := repo0.upserted
	require.NotNil(t, existing)
	require.True(t, existing.State.Due.Equal(time.Date(2026, 9, 29, 12, 0, 0, 0, time.UTC)),
		"a first Again at 21:00 JST must schedule the card for the next day, got %s", existing.State.Due)

	day1 := time.Date(2026, 9, 29, 0, 0, 0, 0, time.UTC) // 09:00 JST
	uc1, repo1, swipe1 := newSwipeLearnDayFixture(existing, day1, nil)

	outcome, err := swipeCard1WithRating(uc1, domain.RatingHard)

	require.NoError(t, err)
	require.NotNil(t, outcome.Swipe)
	require.NotNil(t, repo1.upserted, "a rating for a card due today must upsert the FSRS row")
	require.NotNil(t, swipe1.created, "a rating for a card due today must write a swipe record")
	require.Equal(t, 2, repo1.upserted.State.Reps, "the applied rating must advance reps")
	require.True(t, repo1.upserted.State.LastReview.Equal(day1),
		"the applied swipe must use the injected clock instant")
}

// TestSwipeUsecase_HandleSwipe_DueBoundary pins the exclusive comparison at the
// next JST midnight. The serving side admits ucs.due < EndOfLearnDay(now), so a
// row due exactly at that instant belongs to tomorrow and must be ignored, while
// one due a nanosecond earlier must apply.
func TestSwipeUsecase_HandleSwipe_DueBoundary(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 9, 29, 0, 0, 0, 0, time.UTC) // 09:00 JST
	end := domain.EndOfLearnDay(now)
	require.True(t, end.Equal(time.Date(2026, 9, 29, 15, 0, 0, 0, time.UTC)))
	lastReview := time.Date(2026, 9, 27, 12, 0, 0, 0, time.UTC)

	cases := []struct {
		name     string
		due      time.Time
		wantSkip bool
	}{
		{
			name:     "exactly at EndOfLearnDay is tomorrow",
			due:      end,
			wantSkip: true,
		},
		{
			name:     "one nanosecond before EndOfLearnDay is due today",
			due:      end.Add(-time.Nanosecond),
			wantSkip: false,
		},
		{
			name:     "overdue since yesterday applies",
			due:      time.Date(2026, 9, 28, 12, 0, 0, 0, time.UTC),
			wantSkip: false,
		},
		{
			name:     "due two days later is ignored",
			due:      time.Date(2026, 10, 1, 15, 0, 0, 0, time.UTC),
			wantSkip: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			existing := reviewedCardFSRS(lastReview)
			existing.State.Due = tc.due
			uc, userFSRSRepo, swipeRepo := newSwipeLearnDayFixture(existing, now, nil)

			outcome, err := swipeCard1(uc)

			require.NoError(t, err)
			require.NotNil(t, outcome.Swipe)
			if tc.wantSkip {
				require.Nil(t, userFSRSRepo.upserted, "a not-due rating must not upsert")
				require.Nil(t, swipeRepo.created, "a not-due rating must not write a swipe record")
				require.Equal(t, 3, existing.State.Reps, "reps must not advance")
				require.True(t, existing.State.Due.Equal(tc.due), "due must not move")
				return
			}
			require.NotNil(t, userFSRSRepo.upserted, "a rating for a due card must upsert the FSRS row")
			require.NotNil(t, swipeRepo.created, "a rating for a due card must write a swipe record")
			require.Equal(t, 4, userFSRSRepo.upserted.State.Reps, "reps must advance")
		})
	}
}

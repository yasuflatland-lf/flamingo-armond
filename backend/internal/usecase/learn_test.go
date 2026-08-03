package usecase

import (
	"context"
	"errors"
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"backend/internal/domain"
	"backend/internal/domain/service"
	"backend/internal/repository"
)

type mockLearnCardRepo struct {
	rows []domain.DueCard
	err  error

	cardgroupID string
	userID      string
	window      domain.LearnWindow
	limit       int
	calls       int

	// Practice-mode capture fields, separate from the due-mode captures so a
	// test exercising one window cannot read a value written by the other.
	practiceRows          []domain.DueCard
	practiceErr           error
	practiceCardgroupID   string
	practiceUserID        string
	practiceReviewedAfter time.Time
	practiceLimit         int
	practiceCalls         int
}

func (m *mockLearnCardRepo) FindDueCardsForUser(_ context.Context, userID, cardgroupID string, window domain.LearnWindow, limit int) ([]domain.DueCard, error) {
	m.calls++
	m.userID = userID
	m.cardgroupID = cardgroupID
	m.window = window
	m.limit = limit
	return m.rows, m.err
}

func (m *mockLearnCardRepo) FindPracticeCardsForUser(_ context.Context, userID, cardgroupID string, reviewedAfter time.Time, limit int) ([]domain.DueCard, error) {
	m.practiceCalls++
	m.practiceUserID = userID
	m.practiceCardgroupID = cardgroupID
	m.practiceReviewedAfter = reviewedAfter
	m.practiceLimit = limit
	return m.practiceRows, m.practiceErr
}

type mockLearnCardgroupRepo struct {
	cardgroup *domain.Cardgroup
	err       error
}

func (m *mockLearnCardgroupRepo) FindByID(_ context.Context, _ string) (*domain.Cardgroup, error) {
	return m.cardgroup, m.err
}

// mockLearnUserPrefs satisfies UserPrefsForLearn. When err is set it is returned
// verbatim (e.g. repository.ErrNotFound to exercise the default-ratio path, or
// context.Canceled to exercise the pass-through branch); otherwise pref is
// returned so a stored non-default ratio can reach OrderingPolicy.Apply.
type mockLearnUserPrefs struct {
	pref  *domain.UserPreference
	err   error
	calls int
}

func (m *mockLearnUserPrefs) FindByUserID(_ context.Context, _ string) (*domain.UserPreference, error) {
	m.calls++
	return m.pref, m.err
}

// notFoundPrefs returns a userPrefs stub reporting no stored preference row, so
// NextDueCards falls back to domain.DefaultNewCardRatio (the default 4:1 order).
func notFoundPrefs() *mockLearnUserPrefs {
	return &mockLearnUserPrefs{err: repository.ErrNotFound}
}

type fixedClock struct{ now time.Time }

func (c fixedClock) Now() time.Time { return c.now }

func TestLearnUsecaseNextDueCards(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 13, 9, 0, 0, 0, time.UTC)
	// Two review cards in the same (Review) phase; under a seeded rng the
	// in-phase run may be permuted, so assert set equality, not order.
	first := learnDueCard("repo-first", now.Add(-2*time.Hour), domain.FSRSPhaseReview)
	second := learnDueCard("repo-second", now.Add(-time.Hour), domain.FSRSPhaseReview)
	cardRepo := &mockLearnCardRepo{rows: []domain.DueCard{first, second}}
	uc := NewLearnUsecase(
		cardRepo,
		&mockLearnCardgroupRepo{cardgroup: &domain.Cardgroup{ID: domain.CardgroupID("cg-1"), OwnerID: "u-1"}},
		notFoundPrefs(),
		service.NewOrderingPolicy(),
		func() *rand.Rand { return rand.New(rand.NewSource(1)) },
		20,
		100,
		fixedClock{now: now},
		newTestLogger(),
	)

	got, err := uc.NextDueCards(authedCtx("u-1"), "cg-1", learnIntPtr(5))

	require.NoError(t, err)
	require.Equal(t, 1, cardRepo.calls)
	require.Equal(t, "u-1", cardRepo.userID)
	require.Equal(t, "cg-1", cardRepo.cardgroupID)
	require.Equal(t, now, cardRepo.window.Now)
	require.Equal(t, 5, cardRepo.limit)
	require.True(t, cardRepo.window.ReviewedBefore.Equal(time.Date(2026, 5, 12, 15, 0, 0, 0, time.UTC)),
		"JST start-of-day for 2026-05-13T09:00Z")
	require.True(t, cardRepo.window.RescueDueBefore.Equal(time.Date(2026, 5, 13, 15, 0, 0, 0, time.UTC)),
		"JST end-of-day for 2026-05-13T09:00Z")
	require.True(t, cardRepo.window.RescueReviewedBefore.Equal(time.Date(2026, 5, 13, 0, 0, 0, 0, time.UTC)),
		"rescue early-serve bound is UTC midnight of now's UTC calendar date")
	require.ElementsMatch(t, []string{"repo-first", "repo-second"}, learnCardIDs(got))
}

func TestLearnUsecaseNextDueCardsAuthAndCardgroupErrors(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 13, 9, 0, 0, 0, time.UTC)

	t.Run("anonymous", func(t *testing.T) {
		t.Parallel()
		uc := NewLearnUsecase(
			&mockLearnCardRepo{},
			&mockLearnCardgroupRepo{},
			notFoundPrefs(),
			service.NewOrderingPolicy(),
			func() *rand.Rand { return rand.New(rand.NewSource(1)) },
			20,
			100,
			fixedClock{now: now},
			newTestLogger(),
		)
		_, err := uc.NextDueCards(anonCtx(), "cg-1", learnIntPtr(5))
		assertUnauthenticated(t, err)
	})

	t.Run("missing cardgroup", func(t *testing.T) {
		t.Parallel()
		uc := NewLearnUsecase(
			&mockLearnCardRepo{},
			&mockLearnCardgroupRepo{err: repository.ErrNotFound},
			notFoundPrefs(),
			service.NewOrderingPolicy(),
			func() *rand.Rand { return rand.New(rand.NewSource(1)) },
			20,
			100,
			fixedClock{now: now},
			newTestLogger(),
		)
		_, err := uc.NextDueCards(authedCtx("u-1"), "missing", learnIntPtr(5))
		assertValidationError(t, err, "cardgroupId", "")
	})

	t.Run("non owner", func(t *testing.T) {
		t.Parallel()
		uc := NewLearnUsecase(
			&mockLearnCardRepo{},
			&mockLearnCardgroupRepo{cardgroup: &domain.Cardgroup{ID: domain.CardgroupID("cg-1"), OwnerID: "u-2"}},
			notFoundPrefs(),
			service.NewOrderingPolicy(),
			func() *rand.Rand { return rand.New(rand.NewSource(1)) },
			20,
			100,
			fixedClock{now: now},
			newTestLogger(),
		)
		_, err := uc.NextDueCards(authedCtx("u-1"), "cg-1", learnIntPtr(5))
		assertUnauthenticated(t, err)
	})
}

func TestLearnUsecaseNextDueCardsLimitClampAndEmpty(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 13, 9, 0, 0, 0, time.UTC)
	cases := []struct {
		name      string
		in        *int
		wantLimit int
	}{
		{"nil uses default", nil, 20},
		{"zero uses default", learnIntPtr(0), 20},
		{"negative uses default", learnIntPtr(-1), 20},
		{"over max clamps", learnIntPtr(1000), 100},
		{"explicit passes through", learnIntPtr(5), 5},
		{"max passes through", learnIntPtr(100), 100},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			cardRepo := &mockLearnCardRepo{rows: []domain.DueCard{}}
			uc := NewLearnUsecase(
				cardRepo,
				&mockLearnCardgroupRepo{cardgroup: &domain.Cardgroup{ID: domain.CardgroupID("cg-1"), OwnerID: "u-1"}},
				notFoundPrefs(),
				service.NewOrderingPolicy(),
				func() *rand.Rand { return rand.New(rand.NewSource(1)) },
				20,
				100,
				fixedClock{now: now},
				newTestLogger(),
			)

			got, err := uc.NextDueCards(authedCtx("u-1"), "cg-1", tc.in)

			require.NoError(t, err)
			require.Empty(t, got)
			require.Equal(t, tc.wantLimit, cardRepo.limit)
			require.Equal(t, now, cardRepo.window.Now)
		})
	}
}

func TestLearnUsecaseNextDueCardsRepoError(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 13, 9, 0, 0, 0, time.UTC)
	uc := NewLearnUsecase(
		&mockLearnCardRepo{err: errors.New("db down")},
		&mockLearnCardgroupRepo{cardgroup: &domain.Cardgroup{ID: domain.CardgroupID("cg-1"), OwnerID: "u-1"}},
		notFoundPrefs(),
		service.NewOrderingPolicy(),
		func() *rand.Rand { return rand.New(rand.NewSource(1)) },
		20,
		100,
		fixedClock{now: now},
		newTestLogger(),
	)

	_, err := uc.NextDueCards(authedCtx("u-1"), "cg-1", learnIntPtr(5))

	assertInternalChain(t, err, "usecase: learn: find due cards")
}

func TestLearnUsecaseNextDueCardsCardgroupRepoInternalError(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 13, 9, 0, 0, 0, time.UTC)
	uc := NewLearnUsecase(
		&mockLearnCardRepo{},
		&mockLearnCardgroupRepo{err: errors.New("db down")},
		notFoundPrefs(),
		service.NewOrderingPolicy(),
		func() *rand.Rand { return rand.New(rand.NewSource(1)) },
		20,
		100,
		fixedClock{now: now},
		newTestLogger(),
	)

	_, err := uc.NextDueCards(authedCtx("u-1"), "cg-1", learnIntPtr(5))

	assertInternalChain(t, err, "usecase: authorize cardgroup: find by id")
}

func TestNewLearnUsecase_PanicsOnInvalidDeps(t *testing.T) {
	t.Parallel()
	cardRepo := &mockLearnCardRepo{}
	cgRepo := &mockLearnCardgroupRepo{}
	t.Run("nil cardRepo", func(t *testing.T) {
		t.Parallel()
		require.Panics(t, func() {
			NewLearnUsecase(nil, cgRepo, notFoundPrefs(), nil, nil, 20, 100, nil, newTestLogger())
		})
	})
	t.Run("nil cardgroupRepo", func(t *testing.T) {
		t.Parallel()
		require.Panics(t, func() {
			NewLearnUsecase(cardRepo, nil, notFoundPrefs(), nil, nil, 20, 100, nil, newTestLogger())
		})
	})
	t.Run("nil userPrefs", func(t *testing.T) {
		t.Parallel()
		require.Panics(t, func() {
			NewLearnUsecase(cardRepo, cgRepo, nil, nil, nil, 20, 100, nil, newTestLogger())
		})
	})
	t.Run("defaultLimit greater than maxLimit", func(t *testing.T) {
		t.Parallel()
		require.Panics(t, func() {
			NewLearnUsecase(cardRepo, cgRepo, notFoundPrefs(), nil, nil, 30, 20, nil, newTestLogger())
		})
	})
}

// TestLearnUsecaseNextDueCards_TruncatesToDueLimit verifies that NextDueCards
// truncates the ordered result to the requested limit when the mock repository
// returns more rows than requested, exercising the guard at the usecase layer.
func TestLearnUsecaseNextDueCards_TruncatesToDueLimit(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 13, 9, 0, 0, 0, time.UTC)
	// 3 new + 3 review cards. With the default ratio (new share 4 : review
	// share 1) largest-remainder distribution emits [new, new, review, ...], so
	// the first three slots are [new, new, review]. Truncating at limit=3 keeps
	// that composition; the exact ids within a phase are shuffled, so assert the
	// SHAPE (which slot is review vs new), not specific ids.
	rows := []domain.DueCard{
		learnDueCard("new-1", now.Add(-3*time.Hour), domain.FSRSPhaseNew),
		learnDueCard("new-2", now.Add(-2*time.Hour), domain.FSRSPhaseNew),
		learnDueCard("new-3", now.Add(-time.Hour), domain.FSRSPhaseNew),
		learnDueCard("rev-1", now.Add(-6*time.Hour), domain.FSRSPhaseReview),
		learnDueCard("rev-2", now.Add(-5*time.Hour), domain.FSRSPhaseReview),
		learnDueCard("rev-3", now.Add(-4*time.Hour), domain.FSRSPhaseReview),
	}
	cardRepo := &mockLearnCardRepo{rows: rows}
	uc := NewLearnUsecase(
		cardRepo,
		&mockLearnCardgroupRepo{cardgroup: &domain.Cardgroup{ID: domain.CardgroupID("cg-1"), OwnerID: "u-1"}},
		notFoundPrefs(),
		service.NewOrderingPolicy(),
		func() *rand.Rand { return rand.New(rand.NewSource(42)) },
		20,
		100,
		fixedClock{now: now},
		newTestLogger(),
	)

	got, err := uc.NextDueCards(authedCtx("u-1"), "cg-1", learnIntPtr(3))

	require.NoError(t, err)
	require.Len(t, got, 3, "result must be truncated to the requested limit")
	reviewSet := map[string]bool{"rev-1": true, "rev-2": true, "rev-3": true}
	ids := learnCardIDs(got)
	require.False(t, reviewSet[ids[0]], "slot 0 must be a new card, got %q", ids[0])
	require.False(t, reviewSet[ids[1]], "slot 1 must be a new card, got %q", ids[1])
	require.True(t, reviewSet[ids[2]], "slot 2 must be a review card, got %q", ids[2])
}

// TestLearnUsecaseNextDueCards_HappyPathReviewOnly verifies the simple path
// where the repository returns only review cards and the limit is lower than
// the number returned, so the result is truncated.
func TestLearnUsecaseNextDueCards_HappyPathReviewOnly(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 13, 9, 0, 0, 0, time.UTC)
	// 5 review cards; request limit=3 → expect 3 back.
	rows := []domain.DueCard{
		learnDueCard("rev-1", now.Add(-5*time.Hour), domain.FSRSPhaseReview),
		learnDueCard("rev-2", now.Add(-4*time.Hour), domain.FSRSPhaseReview),
		learnDueCard("rev-3", now.Add(-3*time.Hour), domain.FSRSPhaseReview),
		learnDueCard("rev-4", now.Add(-2*time.Hour), domain.FSRSPhaseReview),
		learnDueCard("rev-5", now.Add(-time.Hour), domain.FSRSPhaseReview),
	}
	cardRepo := &mockLearnCardRepo{rows: rows}
	uc := NewLearnUsecase(
		cardRepo,
		&mockLearnCardgroupRepo{cardgroup: &domain.Cardgroup{ID: domain.CardgroupID("cg-1"), OwnerID: "u-1"}},
		notFoundPrefs(),
		service.NewOrderingPolicy(),
		func() *rand.Rand { return rand.New(rand.NewSource(7)) },
		20,
		100,
		fixedClock{now: now},
		newTestLogger(),
	)

	got, err := uc.NextDueCards(authedCtx("u-1"), "cg-1", learnIntPtr(3))

	require.NoError(t, err)
	require.Len(t, got, 3, "result must contain exactly the requested limit")
}

// ratioRows returns 3 new + 3 review cards, the fixture used by the ratio tests.
// The slot composition (review vs new) after Apply is deterministic; the ids
// within a phase are shuffled, so tests assert membership, not order.
func ratioRows(now time.Time) []domain.DueCard {
	return []domain.DueCard{
		learnDueCard("new-1", now.Add(-3*time.Hour), domain.FSRSPhaseNew),
		learnDueCard("new-2", now.Add(-2*time.Hour), domain.FSRSPhaseNew),
		learnDueCard("new-3", now.Add(-time.Hour), domain.FSRSPhaseNew),
		learnDueCard("rev-1", now.Add(-6*time.Hour), domain.FSRSPhaseReview),
		learnDueCard("rev-2", now.Add(-5*time.Hour), domain.FSRSPhaseReview),
		learnDueCard("rev-3", now.Add(-4*time.Hour), domain.FSRSPhaseReview),
	}
}

// TestLearnUsecaseNextDueCards_UsesStoredRatio verifies that a stored non-default
// ratio (1/2) reaches OrderingPolicy.Apply: the 1:1 alternation puts new cards in
// the even slots and review cards in the odd slots, distinct from the default
// 4:1 order [N,N,R,N,R,R].
func TestLearnUsecaseNextDueCards_UsesStoredRatio(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 13, 9, 0, 0, 0, time.UTC)
	ratio, err := domain.ParseNewCardRatio(1, 2)
	require.NoError(t, err)
	prefs := &mockLearnUserPrefs{pref: &domain.UserPreference{
		UserID:       domain.UserID("u-1"),
		NewCardRatio: ratio,
	}}
	cardRepo := &mockLearnCardRepo{rows: ratioRows(now)}
	uc := NewLearnUsecase(
		cardRepo,
		&mockLearnCardgroupRepo{cardgroup: &domain.Cardgroup{ID: domain.CardgroupID("cg-1"), OwnerID: "u-1"}},
		prefs,
		service.NewOrderingPolicy(),
		func() *rand.Rand { return rand.New(rand.NewSource(42)) },
		20,
		100,
		fixedClock{now: now},
		newTestLogger(),
	)

	got, err := uc.NextDueCards(authedCtx("u-1"), "cg-1", learnIntPtr(6))

	require.NoError(t, err)
	require.Equal(t, 1, prefs.calls, "the stored preference must be read once")
	require.Len(t, got, 6)
	reviewSet := map[string]bool{"rev-1": true, "rev-2": true, "rev-3": true}
	ids := learnCardIDs(got)
	for i, id := range ids {
		if i%2 == 0 {
			require.False(t, reviewSet[id], "slot %d must be a new card, got %q", i, id)
		} else {
			require.True(t, reviewSet[id], "slot %d must be a review card, got %q", i, id)
		}
	}
}

// TestLearnUsecaseNextDueCards_ErrNotFoundUsesDefaultRatio verifies that a
// missing preference row falls back to domain.DefaultNewCardRatio (4:1), yielding
// the default [N,N,R,N,R,R] order for the 3-new/3-review fixture.
func TestLearnUsecaseNextDueCards_ErrNotFoundUsesDefaultRatio(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 13, 9, 0, 0, 0, time.UTC)
	cardRepo := &mockLearnCardRepo{rows: ratioRows(now)}
	uc := NewLearnUsecase(
		cardRepo,
		&mockLearnCardgroupRepo{cardgroup: &domain.Cardgroup{ID: domain.CardgroupID("cg-1"), OwnerID: "u-1"}},
		notFoundPrefs(),
		service.NewOrderingPolicy(),
		func() *rand.Rand { return rand.New(rand.NewSource(42)) },
		20,
		100,
		fixedClock{now: now},
		newTestLogger(),
	)

	got, err := uc.NextDueCards(authedCtx("u-1"), "cg-1", learnIntPtr(6))

	require.NoError(t, err)
	require.Len(t, got, 6)
	reviewSet := map[string]bool{"rev-1": true, "rev-2": true, "rev-3": true}
	ids := learnCardIDs(got)
	// Default 4:1 largest remainder: [N, N, R, N, R, R].
	require.False(t, reviewSet[ids[0]], "slot 0 must be a new card, got %q", ids[0])
	require.False(t, reviewSet[ids[1]], "slot 1 must be a new card, got %q", ids[1])
	require.True(t, reviewSet[ids[2]], "slot 2 must be a review card, got %q", ids[2])
	require.False(t, reviewSet[ids[3]], "slot 3 must be a new card, got %q", ids[3])
	require.True(t, reviewSet[ids[4]], "slot 4 must be a review card, got %q", ids[4])
	require.True(t, reviewSet[ids[5]], "slot 5 must be a review card, got %q", ids[5])
}

// --- PracticeTodaysCards tests ---

// newPracticeUsecase builds a learnUsecase wired for practice-mode tests.
func newPracticeUsecase(cardRepo *mockLearnCardRepo, cgRepo *mockLearnCardgroupRepo, now time.Time) LearnUsecase {
	return NewLearnUsecase(
		cardRepo,
		cgRepo,
		notFoundPrefs(),
		service.NewOrderingPolicy(),
		func() *rand.Rand { return rand.New(rand.NewSource(1)) },
		20,
		100,
		fixedClock{now: now},
		newTestLogger(),
	)
}

func TestLearnUsecasePracticeTodaysCardsAuthAndCardgroupErrors(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 13, 9, 0, 0, 0, time.UTC)

	t.Run("anonymous", func(t *testing.T) {
		t.Parallel()
		uc := newPracticeUsecase(&mockLearnCardRepo{}, &mockLearnCardgroupRepo{}, now)
		_, err := uc.PracticeTodaysCards(anonCtx(), "cg-1", learnIntPtr(5))
		assertUnauthenticated(t, err)
	})

	t.Run("missing cardgroup", func(t *testing.T) {
		t.Parallel()
		uc := newPracticeUsecase(
			&mockLearnCardRepo{},
			&mockLearnCardgroupRepo{err: repository.ErrNotFound},
			now,
		)
		_, err := uc.PracticeTodaysCards(authedCtx("u-1"), "missing", learnIntPtr(5))
		assertValidationError(t, err, "cardgroupId", "")
	})

	t.Run("non owner", func(t *testing.T) {
		t.Parallel()
		uc := newPracticeUsecase(
			&mockLearnCardRepo{},
			&mockLearnCardgroupRepo{cardgroup: &domain.Cardgroup{ID: domain.CardgroupID("cg-1"), OwnerID: "u-2"}},
			now,
		)
		_, err := uc.PracticeTodaysCards(authedCtx("u-1"), "cg-1", learnIntPtr(5))
		assertUnauthenticated(t, err)
	})

	t.Run("cardgroup repo infra error", func(t *testing.T) {
		t.Parallel()
		uc := newPracticeUsecase(
			&mockLearnCardRepo{},
			&mockLearnCardgroupRepo{err: errors.New("db down")},
			now,
		)
		_, err := uc.PracticeTodaysCards(authedCtx("u-1"), "cg-1", learnIntPtr(5))
		assertInternalChain(t, err, "usecase: authorize cardgroup: find by id")
	})
}

func TestLearnUsecasePracticeTodaysCardsCardRepoInfraError(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 13, 9, 0, 0, 0, time.UTC)
	uc := newPracticeUsecase(
		&mockLearnCardRepo{practiceErr: errors.New("db down")},
		&mockLearnCardgroupRepo{cardgroup: &domain.Cardgroup{ID: domain.CardgroupID("cg-1"), OwnerID: "u-1"}},
		now,
	)
	_, err := uc.PracticeTodaysCards(authedCtx("u-1"), "cg-1", learnIntPtr(5))
	assertInternalChain(t, err, "usecase: learn: find practice cards")
}

// TestLearnUsecasePracticeTodaysCards_PropagatesCancelled verifies that
// context.Canceled returned by the card repository is propagated unwrapped,
// preserving its identity for errors.Is at the resolver boundary.
func TestLearnUsecasePracticeTodaysCards_PropagatesCancelled(t *testing.T) {
	t.Parallel()
	cardRepo := &mockLearnCardRepo{practiceErr: context.Canceled}
	uc := newPracticeUsecase(
		cardRepo,
		&mockLearnCardgroupRepo{cardgroup: &domain.Cardgroup{ID: domain.CardgroupID("cg-1"), OwnerID: "u-1"}},
		time.Date(2026, 5, 13, 9, 0, 0, 0, time.UTC),
	)
	_, err := uc.PracticeTodaysCards(authedCtx("u-1"), "cg-1", nil)
	assertCancelled(t, err)
	require.Equal(t, context.Canceled, err, "expected unwrapped context.Canceled, got %v", err)
}

func TestLearnUsecasePracticeTodaysCardsLimitClamp(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 13, 9, 0, 0, 0, time.UTC)
	cases := []struct {
		name      string
		in        *int
		wantLimit int
	}{
		// Practice default == cap == 100: the whole day's pool is the unit.
		{"nil uses cap", nil, 100},
		{"zero uses cap", learnIntPtr(0), 100},
		{"over cap clamps", learnIntPtr(250), 100},
		{"explicit passes through", learnIntPtr(7), 7},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			cardRepo := &mockLearnCardRepo{practiceRows: []domain.DueCard{}}
			uc := newPracticeUsecase(
				cardRepo,
				&mockLearnCardgroupRepo{cardgroup: &domain.Cardgroup{ID: domain.CardgroupID("cg-1"), OwnerID: "u-1"}},
				now,
			)
			_, err := uc.PracticeTodaysCards(authedCtx("u-1"), "cg-1", tc.in)
			require.NoError(t, err)
			require.Equal(t, tc.wantLimit, cardRepo.practiceLimit)
		})
	}
}

// TestLearnUsecasePracticeTodaysCards_PassesJSTStartOfDayAsReviewedAfter pins
// that the usecase computes the JST start-of-day boundary and the repository
// receives it verbatim as reviewedAfter (the inverse window of NextDueCards),
// and that the authenticated user's Sub is forwarded. The chosen instant has
// distinct UTC and JST dates: 2026-06-06T16:30Z = 2026-06-07 01:30 JST.
func TestLearnUsecasePracticeTodaysCards_PassesJSTStartOfDayAsReviewedAfter(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 6, 6, 16, 30, 0, 0, time.UTC)
	wantBoundary := time.Date(2026, 6, 7, 0, 0, 0, 0, time.FixedZone("JST", 9*60*60))
	cardRepo := &mockLearnCardRepo{}
	uc := newPracticeUsecase(
		cardRepo,
		&mockLearnCardgroupRepo{cardgroup: &domain.Cardgroup{ID: domain.CardgroupID("cg-1"), OwnerID: "u-1"}},
		now,
	)
	_, err := uc.PracticeTodaysCards(authedCtx("u-1"), "cg-1", learnIntPtr(5))
	require.NoError(t, err)
	require.True(t, cardRepo.practiceReviewedAfter.Equal(wantBoundary),
		"reviewedAfter: got %v, want instant %v", cardRepo.practiceReviewedAfter, wantBoundary)
	require.Equal(t, "u-1", cardRepo.practiceUserID)
	require.Equal(t, "cg-1", cardRepo.practiceCardgroupID)
}

// TestLearnUsecasePracticeTodaysCards_EmptyIsNonNilSlice verifies that an empty
// repository result maps to a non-nil empty slice, not nil.
func TestLearnUsecasePracticeTodaysCards_EmptyIsNonNilSlice(t *testing.T) {
	t.Parallel()

	cardRepo := &mockLearnCardRepo{practiceRows: []domain.DueCard{}}
	uc := newPracticeUsecase(
		cardRepo,
		&mockLearnCardgroupRepo{cardgroup: &domain.Cardgroup{ID: domain.CardgroupID("cg-1"), OwnerID: "u-1"}},
		time.Date(2026, 5, 13, 9, 0, 0, 0, time.UTC),
	)
	got, err := uc.PracticeTodaysCards(authedCtx("u-1"), "cg-1", nil)
	require.NoError(t, err)
	require.NotNil(t, got, "empty result must be a non-nil slice")
	require.Len(t, got, 0)
}

// TestLearnUsecasePracticeTodaysCards_PreservesRepoOrderAndPointers verifies the
// mapping contract: the result holds the exact same *domain.Card pointers the
// repository returned, in the same order, with no OrderingPolicy reshuffle.
func TestLearnUsecasePracticeTodaysCards_PreservesRepoOrderAndPointers(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 13, 9, 0, 0, 0, time.UTC)
	// Distinct pointers; order chosen so that an OrderingPolicy pass (new before
	// review or vice-versa) would visibly reorder them — proving Apply is NOT run.
	c1 := &domain.Card{ID: "p-1"}
	c2 := &domain.Card{ID: "p-2"}
	c3 := &domain.Card{ID: "p-3"}
	rows := []domain.DueCard{
		{Card: c1, Phase: domain.FSRSPhaseReview, Due: now.Add(-time.Hour)},
		{Card: c2, Phase: domain.FSRSPhaseNew, Due: now.Add(-2 * time.Hour)},
		{Card: c3, Phase: domain.FSRSPhaseReview, Due: now.Add(-3 * time.Hour)},
	}
	cardRepo := &mockLearnCardRepo{practiceRows: rows}
	uc := newPracticeUsecase(
		cardRepo,
		&mockLearnCardgroupRepo{cardgroup: &domain.Cardgroup{ID: domain.CardgroupID("cg-1"), OwnerID: "u-1"}},
		now,
	)
	got, err := uc.PracticeTodaysCards(authedCtx("u-1"), "cg-1", nil)
	require.NoError(t, err)
	require.Len(t, got, 3)
	require.Same(t, c1, got[0], "slot 0 pointer must match repo row 0")
	require.Same(t, c2, got[1], "slot 1 pointer must match repo row 1")
	require.Same(t, c3, got[2], "slot 2 pointer must match repo row 2")
}

// learnDueCard constructs a DueCard for use in learn tests.
// state controls the partition (FSRSPhaseNew vs review).
// due sets the DueCard.Due timestamp.
func learnDueCard(id string, due time.Time, state domain.FSRSPhase) domain.DueCard {
	return domain.DueCard{
		Card:  &domain.Card{ID: id},
		Phase: state,
		Due:   due,
	}
}

func learnCardIDs(cards []*domain.Card) []string {
	out := make([]string, len(cards))
	for i, card := range cards {
		out[i] = card.ID
	}
	return out
}

func learnIntPtr(v int) *int { return &v }

// TestLearnUsecase_NextDueCards_FindCardgroup_PropagatesCancelled verifies that
// context.Canceled returned by the cardgroup repository is propagated unwrapped.
func TestLearnUsecase_NextDueCards_FindCardgroup_PropagatesCancelled(t *testing.T) {
	t.Parallel()
	cgRepo := &mockLearnCardgroupRepo{err: context.Canceled}
	uc := NewLearnUsecase(
		&mockLearnCardRepo{},
		cgRepo,
		notFoundPrefs(),
		nil,
		nil,
		20,
		100,
		fixedClock{now: time.Now()},
		newTestLogger(),
	)
	_, err := uc.NextDueCards(authedCtx("u-1"), "cg-1", nil)
	assertCancelled(t, err)
	require.Equal(t, context.Canceled, err, "expected unwrapped context.Canceled, got %v", err)
}

// TestLearnUsecase_NextDueCards_FindDueCards_PropagatesDeadlineExceeded verifies
// that context.DeadlineExceeded returned by the card repository is propagated
// unwrapped after a successful cardgroup lookup.
func TestLearnUsecase_NextDueCards_FindDueCards_PropagatesDeadlineExceeded(t *testing.T) {
	t.Parallel()
	cardRepo := &mockLearnCardRepo{err: context.DeadlineExceeded}
	cgRepo := &mockLearnCardgroupRepo{cardgroup: &domain.Cardgroup{ID: domain.CardgroupID("cg-1"), OwnerID: "u-1"}}
	uc := NewLearnUsecase(
		cardRepo,
		cgRepo,
		notFoundPrefs(),
		nil,
		nil,
		20,
		100,
		fixedClock{now: time.Now()},
		newTestLogger(),
	)
	_, err := uc.NextDueCards(authedCtx("u-1"), "cg-1", nil)
	assertCancelled(t, err)
}

// TestLearnUsecaseNextDueCards_LoadUserPreferenceInternalError verifies that a
// generic (non-context, non-ErrNotFound) error from the user-preference read is
// wrapped with the usecase layer prefix rather than silently falling back to the
// default ratio.
func TestLearnUsecaseNextDueCards_LoadUserPreferenceInternalError(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 13, 9, 0, 0, 0, time.UTC)
	uc := NewLearnUsecase(
		&mockLearnCardRepo{},
		&mockLearnCardgroupRepo{cardgroup: &domain.Cardgroup{ID: domain.CardgroupID("cg-1"), OwnerID: "u-1"}},
		&mockLearnUserPrefs{err: errors.New("db down")},
		service.NewOrderingPolicy(),
		func() *rand.Rand { return rand.New(rand.NewSource(1)) },
		20,
		100,
		fixedClock{now: now},
		newTestLogger(),
	)

	_, err := uc.NextDueCards(authedCtx("u-1"), "cg-1", learnIntPtr(5))

	assertInternalChain(t, err, "usecase: learn: load user preference")
}

// TestLearnUsecaseNextDueCards_LoadUserPreferencePropagatesCancelled verifies that
// context.Canceled from the user-preference read is propagated unwrapped, matching
// the cardgroup/card repository context-done pass-through.
func TestLearnUsecaseNextDueCards_LoadUserPreferencePropagatesCancelled(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 13, 9, 0, 0, 0, time.UTC)
	uc := NewLearnUsecase(
		&mockLearnCardRepo{},
		&mockLearnCardgroupRepo{cardgroup: &domain.Cardgroup{ID: domain.CardgroupID("cg-1"), OwnerID: "u-1"}},
		&mockLearnUserPrefs{err: context.Canceled},
		service.NewOrderingPolicy(),
		func() *rand.Rand { return rand.New(rand.NewSource(1)) },
		20,
		100,
		fixedClock{now: now},
		newTestLogger(),
	)

	_, err := uc.NextDueCards(authedCtx("u-1"), "cg-1", learnIntPtr(5))

	assertCancelled(t, err)
	require.Equal(t, context.Canceled, err, "expected unwrapped context.Canceled, got %v", err)
}

// TestLearnUsecaseNextDueCards_PassesJSTStartOfDayAsReviewedBefore pins the
// contract that the usecase computes the JST previous-day boundary and the
// repository receives it verbatim.
func TestLearnUsecaseNextDueCards_PassesJSTStartOfDayAsReviewedBefore(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		now  time.Time
		want time.Time
	}{
		{
			name: "before JST midnight",
			now:  time.Date(2026, 6, 5, 14, 59, 0, 0, time.UTC),
			want: time.Date(2026, 6, 4, 15, 0, 0, 0, time.UTC),
		},
		{
			name: "after JST midnight",
			now:  time.Date(2026, 6, 5, 15, 0, 0, 0, time.UTC),
			want: time.Date(2026, 6, 5, 15, 0, 0, 0, time.UTC),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			cardRepo := &mockLearnCardRepo{}
			uc := NewLearnUsecase(
				cardRepo,
				&mockLearnCardgroupRepo{cardgroup: &domain.Cardgroup{ID: domain.CardgroupID("cg-1"), OwnerID: "u-1"}},
				notFoundPrefs(),
				service.NewOrderingPolicy(),
				func() *rand.Rand { return rand.New(rand.NewSource(1)) },
				20,
				100,
				fixedClock{now: tc.now},
				newTestLogger(),
			)
			_, err := uc.NextDueCards(authedCtx("u-1"), "cg-1", learnIntPtr(5))
			require.NoError(t, err)
			require.True(t, cardRepo.window.ReviewedBefore.Equal(tc.want),
				"reviewedBefore: got %v, want instant %v", cardRepo.window.ReviewedBefore, tc.want)
		})
	}
}

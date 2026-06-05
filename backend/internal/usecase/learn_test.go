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
	now         time.Time
	limit       int
	calls       int
}

func (m *mockLearnCardRepo) FindDueCardsForUser(_ context.Context, userID, cardgroupID string, now time.Time, limit int) ([]domain.DueCard, error) {
	m.calls++
	m.userID = userID
	m.cardgroupID = cardgroupID
	m.now = now
	m.limit = limit
	return m.rows, m.err
}

type mockLearnCardgroupRepo struct {
	cardgroup *domain.Cardgroup
	err       error
}

func (m *mockLearnCardgroupRepo) FindByID(_ context.Context, _ string) (*domain.Cardgroup, error) {
	return m.cardgroup, m.err
}

type fixedClock struct{ now time.Time }

func (c fixedClock) Now() time.Time { return c.now }

func TestLearnUsecaseNextDueCards(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 13, 9, 0, 0, 0, time.UTC)
	// Two review cards with distinct Due times so shuffleSameDue leaves order stable.
	first := learnDueCard("repo-first", now.Add(-2*time.Hour), domain.FSRSStateReview)
	second := learnDueCard("repo-second", now.Add(-time.Hour), domain.FSRSStateReview)
	cardRepo := &mockLearnCardRepo{rows: []domain.DueCard{first, second}}
	uc := NewLearnUsecase(
		cardRepo,
		&mockLearnCardgroupRepo{cardgroup: &domain.Cardgroup{ID: "cg-1", OwnerID: "u-1"}},
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
	require.Equal(t, now, cardRepo.now)
	require.Equal(t, 5, cardRepo.limit)
	require.Equal(t, []string{"repo-first", "repo-second"}, learnCardIDs(got))
}

func TestLearnUsecaseNextDueCardsAuthAndCardgroupErrors(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 13, 9, 0, 0, 0, time.UTC)

	t.Run("anonymous", func(t *testing.T) {
		t.Parallel()
		uc := NewLearnUsecase(
			&mockLearnCardRepo{},
			&mockLearnCardgroupRepo{},
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
			&mockLearnCardgroupRepo{cardgroup: &domain.Cardgroup{ID: "cg-1", OwnerID: "u-2"}},
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
				&mockLearnCardgroupRepo{cardgroup: &domain.Cardgroup{ID: "cg-1", OwnerID: "u-1"}},
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
			require.Equal(t, now, cardRepo.now)
		})
	}
}

func TestLearnUsecaseNextDueCardsRepoError(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 13, 9, 0, 0, 0, time.UTC)
	uc := NewLearnUsecase(
		&mockLearnCardRepo{err: errors.New("db down")},
		&mockLearnCardgroupRepo{cardgroup: &domain.Cardgroup{ID: "cg-1", OwnerID: "u-1"}},
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
		service.NewOrderingPolicy(),
		func() *rand.Rand { return rand.New(rand.NewSource(1)) },
		20,
		100,
		fixedClock{now: now},
		newTestLogger(),
	)

	_, err := uc.NextDueCards(authedCtx("u-1"), "cg-1", learnIntPtr(5))

	assertInternalChain(t, err, "usecase: find cardgroup by id")
}

func TestNewLearnUsecase_PanicsOnInvalidDeps(t *testing.T) {
	t.Parallel()
	cardRepo := &mockLearnCardRepo{}
	cgRepo := &mockLearnCardgroupRepo{}
	t.Run("nil cardRepo", func(t *testing.T) {
		t.Parallel()
		require.Panics(t, func() {
			NewLearnUsecase(nil, cgRepo, nil, nil, 20, 100, nil, newTestLogger())
		})
	})
	t.Run("nil cardgroupRepo", func(t *testing.T) {
		t.Parallel()
		require.Panics(t, func() {
			NewLearnUsecase(cardRepo, nil, nil, nil, 20, 100, nil, newTestLogger())
		})
	})
	t.Run("defaultLimit greater than maxLimit", func(t *testing.T) {
		t.Parallel()
		require.Panics(t, func() {
			NewLearnUsecase(cardRepo, cgRepo, nil, nil, 30, 20, nil, newTestLogger())
		})
	})
}

// TestLearnUsecaseNextDueCards_TruncatesToDueLimit verifies that NextDueCards
// truncates the ordered result to the requested limit when the mock repository
// returns more rows than requested, exercising the guard at the usecase layer.
func TestLearnUsecaseNextDueCards_TruncatesToDueLimit(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 13, 9, 0, 0, 0, time.UTC)
	// 3 new + 3 review cards, all with distinct Due timestamps so shuffleSameDue
	// is a no-op and the interleave order is fully deterministic.
	// With ReviewCardRatio=4 and 3 reviews, all reviews emit before any new card:
	// rev-1, rev-2, rev-3, new-1, new-2, new-3.  Truncating at limit=3 yields
	// the first three review cards in their Due-sorted order.
	rows := []domain.DueCard{
		learnDueCard("new-1", now.Add(-3*time.Hour), domain.FSRSStateNew),
		learnDueCard("new-2", now.Add(-2*time.Hour), domain.FSRSStateNew),
		learnDueCard("new-3", now.Add(-time.Hour), domain.FSRSStateNew),
		learnDueCard("rev-1", now.Add(-6*time.Hour), domain.FSRSStateReview),
		learnDueCard("rev-2", now.Add(-5*time.Hour), domain.FSRSStateReview),
		learnDueCard("rev-3", now.Add(-4*time.Hour), domain.FSRSStateReview),
	}
	cardRepo := &mockLearnCardRepo{rows: rows}
	uc := NewLearnUsecase(
		cardRepo,
		&mockLearnCardgroupRepo{cardgroup: &domain.Cardgroup{ID: "cg-1", OwnerID: "u-1"}},
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
	require.Equal(t, []string{"rev-1", "rev-2", "rev-3"}, learnCardIDs(got),
		"truncated result must contain the first three cards from the ordered set")
}

// TestLearnUsecaseNextDueCards_HappyPathReviewOnly verifies the simple path
// where the repository returns only review cards and the limit is lower than
// the number returned, so the result is truncated.
func TestLearnUsecaseNextDueCards_HappyPathReviewOnly(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 13, 9, 0, 0, 0, time.UTC)
	// 5 review cards; request limit=3 → expect 3 back.
	rows := []domain.DueCard{
		learnDueCard("rev-1", now.Add(-5*time.Hour), domain.FSRSStateReview),
		learnDueCard("rev-2", now.Add(-4*time.Hour), domain.FSRSStateReview),
		learnDueCard("rev-3", now.Add(-3*time.Hour), domain.FSRSStateReview),
		learnDueCard("rev-4", now.Add(-2*time.Hour), domain.FSRSStateReview),
		learnDueCard("rev-5", now.Add(-time.Hour), domain.FSRSStateReview),
	}
	cardRepo := &mockLearnCardRepo{rows: rows}
	uc := NewLearnUsecase(
		cardRepo,
		&mockLearnCardgroupRepo{cardgroup: &domain.Cardgroup{ID: "cg-1", OwnerID: "u-1"}},
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

// learnDueCard constructs a DueCard for use in learn tests.
// state controls the partition (FSRSStateNew vs review).
// due sets the DueCard.Due timestamp so shuffleSameDue groups cards correctly.
func learnDueCard(id string, due time.Time, state domain.FSRSCardState) domain.DueCard {
	return domain.DueCard{
		Card:  &domain.Card{ID: id},
		State: state,
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
	cgRepo := &mockLearnCardgroupRepo{cardgroup: &domain.Cardgroup{ID: "cg-1", OwnerID: "u-1"}}
	uc := NewLearnUsecase(
		cardRepo,
		cgRepo,
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

// TestStartOfDayJST pins the "previous day" boundary used by the review
// slots: JST (UTC+9) midnight at or before now, returned as an instant.
func TestStartOfDayJST(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		now  time.Time
		want time.Time
	}{
		{
			name: "just before JST midnight",
			now:  time.Date(2026, 6, 5, 14, 59, 59, 0, time.UTC), // 23:59:59 JST
			want: time.Date(2026, 6, 4, 15, 0, 0, 0, time.UTC),   // 2026-06-05 00:00 JST
		},
		{
			name: "exactly JST midnight",
			now:  time.Date(2026, 6, 5, 15, 0, 0, 0, time.UTC), // 2026-06-06 00:00 JST
			want: time.Date(2026, 6, 5, 15, 0, 0, 0, time.UTC),
		},
		{
			name: "JST noon",
			now:  time.Date(2026, 6, 5, 3, 0, 0, 0, time.UTC), // 12:00 JST
			want: time.Date(2026, 6, 4, 15, 0, 0, 0, time.UTC),
		},
		{
			name: "input zone does not matter, only the instant",
			now:  time.Date(2026, 6, 5, 12, 0, 0, 0, time.FixedZone("JST", 9*60*60)), // = 03:00 UTC
			want: time.Date(2026, 6, 4, 15, 0, 0, 0, time.UTC),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := startOfDayJST(tc.now)
			require.True(t, got.Equal(tc.want), "got %v, want instant %v", got, tc.want)
		})
	}
}

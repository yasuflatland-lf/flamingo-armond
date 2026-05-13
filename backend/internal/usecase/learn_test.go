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
	rows []*domain.Card
	err  error

	cardgroupID string
	userID      string
	now         time.Time
	limit       int
	calls       int
}

func (m *mockLearnCardRepo) FindDueCards(_ context.Context, cardgroupID string, now time.Time, limit int) ([]*domain.Card, error) {
	m.calls++
	m.cardgroupID = cardgroupID
	m.now = now
	m.limit = limit
	return m.rows, m.err
}

func (m *mockLearnCardRepo) FindDueCardsForUser(_ context.Context, userID, cardgroupID string, now time.Time, limit int) ([]*domain.Card, error) {
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

func TestLearnUsecaseNextDueCards(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 13, 9, 0, 0, 0, time.UTC)
	earlier := learnCard("earlier", now.Add(-time.Hour))
	later := learnCard("later", now)
	cardRepo := &mockLearnCardRepo{rows: []*domain.Card{later, earlier}}
	uc := NewLearnUsecase(
		cardRepo,
		&mockLearnCardgroupRepo{cardgroup: &domain.Cardgroup{ID: "cg-1", OwnerID: "u-1"}},
		service.NewOrderingPolicy(),
		func() *rand.Rand { return rand.New(rand.NewSource(1)) },
		20,
		100,
	)

	got, err := uc.NextDueCards(authedCtx("u-1"), "cg-1", now, 5)

	require.NoError(t, err)
	require.Equal(t, 1, cardRepo.calls)
	require.Equal(t, "u-1", cardRepo.userID)
	require.Equal(t, "cg-1", cardRepo.cardgroupID)
	require.Equal(t, now, cardRepo.now)
	require.Equal(t, 5, cardRepo.limit)
	require.Equal(t, []string{"earlier", "later"}, learnCardIDs(got))
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
		)
		_, err := uc.NextDueCards(anonCtx(), "cg-1", now, 5)
		assertGQLErr(t, err, "UNAUTHENTICATED", "")
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
		)
		_, err := uc.NextDueCards(authedCtx("u-1"), "missing", now, 5)
		assertGQLErr(t, err, "BAD_USER_INPUT", "cardgroupId")
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
		)
		_, err := uc.NextDueCards(authedCtx("u-1"), "cg-1", now, 5)
		assertGQLErr(t, err, "UNAUTHENTICATED", "")
	})
}

func TestLearnUsecaseNextDueCardsLimitClampAndEmpty(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 13, 9, 0, 0, 0, time.UTC)
	cases := []struct {
		name      string
		in        int
		wantLimit int
	}{
		{"zero uses default", 0, 20},
		{"negative uses default", -1, 20},
		{"over max clamps", 1000, 100},
		{"explicit passes through", 5, 5},
		{"max passes through", 100, 100},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			cardRepo := &mockLearnCardRepo{rows: []*domain.Card{}}
			uc := NewLearnUsecase(
				cardRepo,
				&mockLearnCardgroupRepo{cardgroup: &domain.Cardgroup{ID: "cg-1", OwnerID: "u-1"}},
				service.NewOrderingPolicy(),
				func() *rand.Rand { return rand.New(rand.NewSource(1)) },
				20,
				100,
			)

			got, err := uc.NextDueCards(authedCtx("u-1"), "cg-1", now, tc.in)

			require.NoError(t, err)
			require.Empty(t, got)
			require.Equal(t, tc.wantLimit, cardRepo.limit)
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
	)

	_, err := uc.NextDueCards(authedCtx("u-1"), "cg-1", now, 5)

	assertGQLErr(t, err, "INTERNAL", "")
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
	)

	_, err := uc.NextDueCards(authedCtx("u-1"), "cg-1", now, 5)

	assertGQLErr(t, err, "INTERNAL", "")
}

func TestNewLearnUsecase_PanicsOnInvalidDeps(t *testing.T) {
	t.Parallel()
	cardRepo := &mockLearnCardRepo{}
	cgRepo := &mockLearnCardgroupRepo{}
	t.Run("nil cardRepo", func(t *testing.T) {
		t.Parallel()
		require.Panics(t, func() {
			NewLearnUsecase(nil, cgRepo, nil, nil, 20, 100)
		})
	})
	t.Run("nil cardgroupRepo", func(t *testing.T) {
		t.Parallel()
		require.Panics(t, func() {
			NewLearnUsecase(cardRepo, nil, nil, nil, 20, 100)
		})
	})
	t.Run("defaultLimit greater than maxLimit", func(t *testing.T) {
		t.Parallel()
		require.Panics(t, func() {
			NewLearnUsecase(cardRepo, cgRepo, nil, nil, 30, 20)
		})
	})
}

func learnCard(id string, due time.Time) *domain.Card {
	return &domain.Card{
		ID:   id,
		FSRS: domain.FSRSState{Due: due},
	}
}

func learnCardIDs(cards []*domain.Card) []string {
	out := make([]string, len(cards))
	for i, card := range cards {
		out[i] = card.ID
	}
	return out
}

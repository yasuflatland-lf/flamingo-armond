package repository_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"backend/internal/domain"
	"backend/internal/repository"
)

// insertCards creates n cards with deterministic Front values ("c0".."cn-1").
// CreatedAt is staggered so insertion order matches creation order. Due is set
// to now+i*hour so DUE-ordered tests have a clear ASC sequence.
func insertCards(t *testing.T, ctx context.Context, repo repository.CardRepository, cgID string, n int) []*domain.Card {
	t.Helper()
	now := time.Now().UTC()
	cards := make([]*domain.Card, n)
	for i := 0; i < n; i++ {
		c := newCard(cgID, "front", "back")
		// Stagger timestamps by 1 hour so ordering is unambiguous.
		c.CreatedAt = now.Add(time.Duration(i) * time.Hour)
		c.UpdatedAt = c.CreatedAt
		c.FSRS.Due = now.Add(time.Duration(i) * time.Hour)
		require.NoError(t, repo.Create(ctx, c))
		cards[i] = c
	}
	// Re-fetch so we observe DB-rounded timestamps (Postgres truncates to
	// microsecond precision; cursor comparisons must use those values).
	for i, c := range cards {
		got, err := repo.FindByID(ctx, c.ID)
		require.NoError(t, err)
		cards[i] = got
	}
	return cards
}

func TestCardRepository_FindPageByCardgroup_ForwardByID(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ownerID := insertAuthUser(t, ctx)
	cg := insertCardgroup(t, ctx, ownerID)
	repo := repository.NewCardRepository(testDB.GORM)

	cards := insertCards(t, ctx, repo, cg.ID, 5)
	// Sort the inserted cards by their UUID so we know what "id ASC" returns.
	sorted := sortByID(cards)

	got, total, err := repo.FindPageByCardgroup(
		ctx, cg.ID, nil, nil, 2, 0, repository.CardOrderByID, repository.SortAsc,
	)
	require.NoError(t, err)
	require.Equal(t, int64(5), total)
	require.Len(t, got, 2)
	require.Equal(t, sorted[0].ID, got[0].ID)
	require.Equal(t, sorted[1].ID, got[1].ID)

	got, total, err = repo.FindPageByCardgroup(
		ctx, cg.ID,
		&repository.CardCursor{ID: sorted[1].ID}, nil,
		2, 0, repository.CardOrderByID, repository.SortAsc,
	)
	require.NoError(t, err)
	require.Equal(t, int64(5), total)
	require.Len(t, got, 2)
	require.Equal(t, sorted[2].ID, got[0].ID)
	require.Equal(t, sorted[3].ID, got[1].ID)

	got, total, err = repo.FindPageByCardgroup(
		ctx, cg.ID,
		&repository.CardCursor{ID: sorted[3].ID}, nil,
		2, 0, repository.CardOrderByID, repository.SortAsc,
	)
	require.NoError(t, err)
	require.Equal(t, int64(5), total)
	require.Len(t, got, 1)
	require.Equal(t, sorted[4].ID, got[0].ID)
}

func TestCardRepository_FindPageByCardgroup_BackwardByID(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ownerID := insertAuthUser(t, ctx)
	cg := insertCardgroup(t, ctx, ownerID)
	repo := repository.NewCardRepository(testDB.GORM)

	cards := insertCards(t, ctx, repo, cg.ID, 5)
	sorted := sortByID(cards)

	// last=2, before=nil should return the final two ids in ASC order.
	got, total, err := repo.FindPageByCardgroup(
		ctx, cg.ID, nil, nil, 0, 2, repository.CardOrderByID, repository.SortAsc,
	)
	require.NoError(t, err)
	require.Equal(t, int64(5), total)
	require.Len(t, got, 2)
	require.Equal(t, sorted[3].ID, got[0].ID)
	require.Equal(t, sorted[4].ID, got[1].ID)

	// last=2, before=cards[3] should return cards[1..2] in ASC order.
	got, _, err = repo.FindPageByCardgroup(
		ctx, cg.ID,
		nil, &repository.CardCursor{ID: sorted[3].ID},
		0, 2, repository.CardOrderByID, repository.SortAsc,
	)
	require.NoError(t, err)
	require.Len(t, got, 2)
	require.Equal(t, sorted[1].ID, got[0].ID)
	require.Equal(t, sorted[2].ID, got[1].ID)
}

func TestCardRepository_FindPageByCardgroup_EmptyGroup(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ownerID := insertAuthUser(t, ctx)
	cg := insertCardgroup(t, ctx, ownerID)
	repo := repository.NewCardRepository(testDB.GORM)

	got, total, err := repo.FindPageByCardgroup(
		ctx, cg.ID, nil, nil, 10, 0, repository.CardOrderByID, repository.SortAsc,
	)
	require.NoError(t, err)
	require.Equal(t, int64(0), total)
	require.NotNil(t, got)
	require.Empty(t, got)
}

func TestCardRepository_FindPageByCardgroup_SinglePage(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ownerID := insertAuthUser(t, ctx)
	cg := insertCardgroup(t, ctx, ownerID)
	repo := repository.NewCardRepository(testDB.GORM)

	insertCards(t, ctx, repo, cg.ID, 3)

	got, total, err := repo.FindPageByCardgroup(
		ctx, cg.ID, nil, nil, 10, 0, repository.CardOrderByID, repository.SortAsc,
	)
	require.NoError(t, err)
	require.Equal(t, int64(3), total)
	require.Len(t, got, 3)
}

func TestCardRepository_FindPageByCardgroup_OrderByDue(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ownerID := insertAuthUser(t, ctx)
	cg := insertCardgroup(t, ctx, ownerID)
	repo := repository.NewCardRepository(testDB.GORM)

	cards := insertCards(t, ctx, repo, cg.ID, 3)
	// insertCards staggers Due by +1h per index, so cards[0] is earliest.

	got, total, err := repo.FindPageByCardgroup(
		ctx, cg.ID, nil, nil, 2, 0, repository.CardOrderByDue, repository.SortAsc,
	)
	require.NoError(t, err)
	require.Equal(t, int64(3), total)
	require.Len(t, got, 2)
	require.Equal(t, cards[0].ID, got[0].ID)
	require.Equal(t, cards[1].ID, got[1].ID)

	// Advance via cursor populated with the second card's Due value.
	cursor := &repository.CardCursor{
		ID:  cards[1].ID,
		Due: &cards[1].FSRS.Due,
	}
	got, _, err = repo.FindPageByCardgroup(
		ctx, cg.ID, cursor, nil, 2, 0, repository.CardOrderByDue, repository.SortAsc,
	)
	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Equal(t, cards[2].ID, got[0].ID)
}

func TestCardRepository_FindPageByCardgroup_OrderByCreatedAtDesc(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ownerID := insertAuthUser(t, ctx)
	cg := insertCardgroup(t, ctx, ownerID)
	repo := repository.NewCardRepository(testDB.GORM)

	cards := insertCards(t, ctx, repo, cg.ID, 4)
	// DESC: newest first → cards[3], cards[2], cards[1], cards[0].

	got, total, err := repo.FindPageByCardgroup(
		ctx, cg.ID, nil, nil, 2, 0, repository.CardOrderByCreatedAt, repository.SortDesc,
	)
	require.NoError(t, err)
	require.Equal(t, int64(4), total)
	require.Len(t, got, 2)
	require.Equal(t, cards[3].ID, got[0].ID)
	require.Equal(t, cards[2].ID, got[1].ID)

	cursor := &repository.CardCursor{
		ID:        cards[2].ID,
		CreatedAt: &cards[2].CreatedAt,
	}
	got, _, err = repo.FindPageByCardgroup(
		ctx, cg.ID, cursor, nil, 2, 0, repository.CardOrderByCreatedAt, repository.SortDesc,
	)
	require.NoError(t, err)
	require.Len(t, got, 2)
	require.Equal(t, cards[1].ID, got[0].ID)
	require.Equal(t, cards[0].ID, got[1].ID)
}

func TestCardRepository_FindPageByCardgroup_TotalCountIsScopedToCardgroup(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ownerID := insertAuthUser(t, ctx)
	cg1 := insertCardgroup(t, ctx, ownerID)
	cg2 := insertCardgroup(t, ctx, ownerID)
	repo := repository.NewCardRepository(testDB.GORM)

	insertCards(t, ctx, repo, cg1.ID, 3)
	insertCards(t, ctx, repo, cg2.ID, 5)

	_, total, err := repo.FindPageByCardgroup(
		ctx, cg1.ID, nil, nil, 10, 0, repository.CardOrderByID, repository.SortAsc,
	)
	require.NoError(t, err)
	require.Equal(t, int64(3), total)

	_, total, err = repo.FindPageByCardgroup(
		ctx, cg2.ID, nil, nil, 10, 0, repository.CardOrderByID, repository.SortAsc,
	)
	require.NoError(t, err)
	require.Equal(t, int64(5), total)
}

// TestCardRepository_FindPageByCardgroup_OrderByDue_TieBreakOnEqualDue
// verifies the secondary `id` key keeps order deterministic when multiple
// cards share the same Due value — pages must not skip or duplicate.
func TestCardRepository_FindPageByCardgroup_OrderByDue_TieBreakOnEqualDue(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ownerID := insertAuthUser(t, ctx)
	cg := insertCardgroup(t, ctx, ownerID)
	repo := repository.NewCardRepository(testDB.GORM)

	now := time.Now().UTC()
	cards := make([]*domain.Card, 3)
	for i := 0; i < 3; i++ {
		c := newCard(cg.ID, "front", "back")
		c.CreatedAt = now
		c.UpdatedAt = now
		c.FSRS.Due = now
		require.NoError(t, repo.Create(ctx, c))
		cards[i] = c
	}
	// Re-fetch so we observe DB-rounded timestamps.
	for i, c := range cards {
		got, err := repo.FindByID(ctx, c.ID)
		require.NoError(t, err)
		cards[i] = got
	}

	// (due, id) lexicographic sort — all dues equal so order is by ID.
	sorted := sortByID(cards)
	dueT := sorted[0].FSRS.Due

	page1, total, err := repo.FindPageByCardgroup(
		ctx, cg.ID, nil, nil, 2, 0, repository.CardOrderByDue, repository.SortAsc,
	)
	require.NoError(t, err)
	require.Equal(t, int64(3), total)
	require.Len(t, page1, 2)
	require.Equal(t, sorted[0].ID, page1[0].ID)
	require.Equal(t, sorted[1].ID, page1[1].ID)

	cursor := &repository.CardCursor{
		ID:  page1[1].ID,
		Due: &dueT,
	}
	page2, _, err := repo.FindPageByCardgroup(
		ctx, cg.ID, cursor, nil, 2, 0, repository.CardOrderByDue, repository.SortAsc,
	)
	require.NoError(t, err)
	require.Len(t, page2, 1, "third card should appear exactly once")
	require.Equal(t, sorted[2].ID, page2[0].ID)
}

// sortByID returns a copy of cards sorted by ID ascending.
func sortByID(cards []*domain.Card) []*domain.Card {
	out := make([]*domain.Card, len(cards))
	copy(out, cards)
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j-1].ID > out[j].ID; j-- {
			out[j-1], out[j] = out[j], out[j-1]
		}
	}
	return out
}


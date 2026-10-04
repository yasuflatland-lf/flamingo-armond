package repository_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"backend/internal/domain"
	"backend/internal/repository"
)

// insertCards creates n cards with deterministic front values.
func insertCards(t *testing.T, ctx context.Context, repo repository.CardRepository, cgID domain.CardgroupID, n int) []*domain.Card {
	t.Helper()
	cards := make([]*domain.Card, n)
	for i := range cards {
		c := newCard(cgID, fmt.Sprintf("front-%d", i), "back")
		require.NoError(t, repo.Create(ctx, c))
		cards[i] = c
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
		ctx, string(cg.ID), nil, 2, nil,
	)
	require.NoError(t, err)
	require.Equal(t, int64(5), total)
	require.Len(t, got, 2)
	require.Equal(t, sorted[0].ID, got[0].ID)
	require.Equal(t, sorted[1].ID, got[1].ID)

	got, total, err = repo.FindPageByCardgroup(
		ctx, string(cg.ID),
		&repository.CardCursor{ID: sorted[1].ID},
		2, nil,
	)
	require.NoError(t, err)
	require.Equal(t, int64(5), total)
	require.Len(t, got, 2)
	require.Equal(t, sorted[2].ID, got[0].ID)
	require.Equal(t, sorted[3].ID, got[1].ID)

	got, total, err = repo.FindPageByCardgroup(
		ctx, string(cg.ID),
		&repository.CardCursor{ID: sorted[3].ID},
		2, nil,
	)
	require.NoError(t, err)
	require.Equal(t, int64(5), total)
	require.Len(t, got, 1)
	require.Equal(t, sorted[4].ID, got[0].ID)
}

func TestCardRepository_FindPageByCardgroup_EmptyGroup(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ownerID := insertAuthUser(t, ctx)
	cg := insertCardgroup(t, ctx, ownerID)
	repo := repository.NewCardRepository(testDB.GORM)

	got, total, err := repo.FindPageByCardgroup(
		ctx, string(cg.ID), nil, 10, nil,
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
		ctx, string(cg.ID), nil, 10, nil,
	)
	require.NoError(t, err)
	require.Equal(t, int64(3), total)
	require.Len(t, got, 3)
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
		ctx, string(cg1.ID), nil, 10, nil,
	)
	require.NoError(t, err)
	require.Equal(t, int64(3), total)

	_, total, err = repo.FindPageByCardgroup(
		ctx, string(cg2.ID), nil, 10, nil,
	)
	require.NoError(t, err)
	require.Equal(t, int64(5), total)
}

// TestCardRepository_FindPageByCardgroup_PageCapAllowsMaxPlusOne verifies that
// pageCap (101) accepts first=101 so the usecase's +1 trick can detect a next
// page when the caller requests the documented maximum of 100.
func TestCardRepository_FindPageByCardgroup_PageCapAllowsMaxPlusOne(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ownerID := insertAuthUser(t, ctx)
	cg := insertCardgroup(t, ctx, ownerID)
	repo := repository.NewCardRepository(testDB.GORM)

	// Insert 101 cards so both first=101 and first=100 queries have enough rows.
	insertCards(t, ctx, repo, cg.ID, 101)

	// first=101 must return all 101 rows because pageCap == 101.
	cards101, total101, err := repo.FindPageByCardgroup(
		ctx, string(cg.ID), nil, 101, nil,
	)
	require.NoError(t, err)
	require.Equal(t, int64(101), total101)
	require.Len(t, cards101, 101)

	// first=100 must be limited to 100 rows, confirming the cap still applies.
	cards100, total100, err := repo.FindPageByCardgroup(
		ctx, string(cg.ID), nil, 100, nil,
	)
	require.NoError(t, err)
	require.Equal(t, int64(101), total100)
	require.Len(t, cards100, 100)
}

// TestCardRepository_FindPageByCardgroup_ZeroPageReturnsTotal verifies the C2
// fix: first=0 short-circuits the row fetch but still returns the
// real totalCount from the separate COUNT(*) query.
func TestCardRepository_FindPageByCardgroup_ZeroPageReturnsTotal(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ownerID := insertAuthUser(t, ctx)
	cg := insertCardgroup(t, ctx, ownerID)
	repo := repository.NewCardRepository(testDB.GORM)

	insertCards(t, ctx, repo, cg.ID, 5)

	cards, total, err := repo.FindPageByCardgroup(
		ctx, string(cg.ID), nil, 0, nil,
	)
	require.NoError(t, err)
	// No rows requested, but the slice must be non-nil and empty.
	require.NotNil(t, cards)
	require.Len(t, cards, 0)
	// totalCount must reflect the actual number of cards in the group.
	require.Equal(t, int64(5), total)
}

// TestCardRepo_FindPageByCardgroup_Search_WithAfter checks that search filters
// both pages and totalCount while an ID cursor advances through matching cards.
func TestCardRepo_FindPageByCardgroup_Search_WithAfter(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ownerID := insertAuthUser(t, ctx)
	cg := insertCardgroup(t, ctx, ownerID)
	repo := repository.NewCardRepository(testDB.GORM)

	// 3 cards whose front matches "apple".
	matchingFronts := []string{"front-apple-0", "front-apple-1", "front-apple-2"}
	matching := make([]*domain.Card, 3)
	for i, front := range matchingFronts {
		c := newCard(cg.ID, front, "back")
		require.NoError(t, repo.Create(ctx, c))
		matching[i] = c
	}

	// 2 cards that do not match "apple".
	nonMatching := make([]*domain.Card, 2)
	for i, front := range []string{"front-cherry", "front-banana"} {
		c := newCard(cg.ID, front, "back")
		require.NoError(t, repo.Create(ctx, c))
		nonMatching[i] = c
	}

	matching = sortByID(matching)

	search := "apple"

	// --- Page 1 ---
	page1, total1, err := repo.FindPageByCardgroup(
		ctx, string(cg.ID), nil,
		// Request 2+1 (the +1 trick) so the usecase can detect hasNextPage.
		3,

		&search,
	)
	require.NoError(t, err)
	require.Equal(t, int64(3), total1, "totalCount must count only matching cards")
	require.Len(t, page1, 3)
	require.True(t, len(page1) > 2, "page 1 should signal hasNextPage=true")
	page1 = page1[:2]

	require.Equal(t, matching[0].ID, page1[0].ID)
	require.Equal(t, matching[1].ID, page1[1].ID)

	for _, c := range page1 {
		for _, nm := range nonMatching {
			require.NotEqual(t, nm.ID, c.ID,
				"non-matching card %s must not appear in page 1", nm.ID)
		}
	}

	// --- Cursor from the last edge of page 1 ---
	last1 := page1[len(page1)-1]
	cursor := &repository.CardCursor{
		ID: last1.ID,
	}

	// --- Page 2 ---
	page2, total2, err := repo.FindPageByCardgroup(
		ctx, string(cg.ID), cursor,
		3,

		&search,
	)
	require.NoError(t, err)
	require.Equal(t, int64(3), total2, "totalCount must still be 3 on page 2")
	require.Len(t, page2, 1, "page 2 must return exactly 1 matching card")
	require.Equal(t, matching[2].ID, page2[0].ID)
	require.False(t, len(page2) > 2, "page 2 should signal hasNextPage=false")

	for _, c := range page2 {
		for _, nm := range nonMatching {
			require.NotEqual(t, nm.ID, c.ID,
				"non-matching card %s must not appear in page 2", nm.ID)
		}
	}
}

// TestCardRepo_FindPageByCardgroup_EmptySearchTreatedAsNil verifies that an
// all-whitespace search has no effect — same as nil — because searchLikePattern
// returns ok=false for trimmed-empty input. The whitespace term must NOT reach
// SQL as "%   %" (which would filter out every card) and must NOT zero the page
// or totalCount. Mirrors the cardgroup blank-search guard.
func TestCardRepo_FindPageByCardgroup_EmptySearchTreatedAsNil(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ownerID := insertAuthUser(t, ctx)
	cg := insertCardgroup(t, ctx, ownerID)
	repo := repository.NewCardRepository(testDB.GORM)

	insertCards(t, ctx, repo, cg.ID, 3)

	whitespace := "   "
	got, total, err := repo.FindPageByCardgroup(
		ctx, string(cg.ID), nil, 100,
		&whitespace,
	)
	require.NoError(t, err)
	require.Equal(t, int64(3), total,
		"all-whitespace search must NOT filter totalCount (treated as no search)")
	require.Len(t, got, 3,
		"all-whitespace search must NOT filter the page (treated as no search)")
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

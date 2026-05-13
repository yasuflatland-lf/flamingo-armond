package repository_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"backend/internal/domain"
	"backend/internal/repository"
)

// insertCards creates n cards with deterministic Front values ("front-0".."front-n-1").
// CreatedAt is staggered so insertion order matches creation order.
func insertCards(t *testing.T, ctx context.Context, repo repository.CardRepository, cgID string, n int) []*domain.Card {
	t.Helper()
	now := time.Now().UTC()
	cards := make([]*domain.Card, n)
	for i := 0; i < n; i++ {
		c := newCard(cgID, fmt.Sprintf("front-%d", i), "back")
		// Stagger timestamps by 1 hour so ordering is unambiguous.
		c.CreatedAt = now.Add(time.Duration(i) * time.Hour)
		c.UpdatedAt = c.CreatedAt
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
		ctx, cg.ID, nil, nil, 2, 0, repository.CardOrderByID, repository.SortAsc, nil,
	)
	require.NoError(t, err)
	require.Equal(t, int64(5), total)
	require.Len(t, got, 2)
	require.Equal(t, sorted[0].ID, got[0].ID)
	require.Equal(t, sorted[1].ID, got[1].ID)

	got, total, err = repo.FindPageByCardgroup(
		ctx, cg.ID,
		&repository.CardCursor{ID: sorted[1].ID}, nil,
		2, 0, repository.CardOrderByID, repository.SortAsc, nil,
	)
	require.NoError(t, err)
	require.Equal(t, int64(5), total)
	require.Len(t, got, 2)
	require.Equal(t, sorted[2].ID, got[0].ID)
	require.Equal(t, sorted[3].ID, got[1].ID)

	got, total, err = repo.FindPageByCardgroup(
		ctx, cg.ID,
		&repository.CardCursor{ID: sorted[3].ID}, nil,
		2, 0, repository.CardOrderByID, repository.SortAsc, nil,
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
		ctx, cg.ID, nil, nil, 0, 2, repository.CardOrderByID, repository.SortAsc, nil,
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
		0, 2, repository.CardOrderByID, repository.SortAsc, nil,
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
		ctx, cg.ID, nil, nil, 10, 0, repository.CardOrderByID, repository.SortAsc, nil,
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
		ctx, cg.ID, nil, nil, 10, 0, repository.CardOrderByID, repository.SortAsc, nil,
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
	ucsRepo := repository.NewUserCardFSRSRepository(testDB.GORM)

	cards := insertCards(t, ctx, repo, cg.ID, 3)
	dueValues := []time.Time{
		cards[0].CreatedAt,
		cards[1].CreatedAt,
		cards[2].CreatedAt,
	}
	require.NoError(t, testDB.GORM.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for i, card := range cards {
			state := domain.NewUserCardFSRSForNewCard(ownerID, card.ID, dueValues[i])
			state.State.Due = dueValues[i]
			if err := ucsRepo.UpsertTx(ctx, tx, state); err != nil {
				return err
			}
		}
		return nil
	}))

	got, total, err := repo.FindPageByCardgroupForUser(
		ctx, ownerID, cg.ID, nil, nil, 2, 0, repository.CardOrderByDue, repository.SortAsc, nil,
	)
	require.NoError(t, err)
	require.Equal(t, int64(3), total)
	require.Len(t, got, 2)
	require.Equal(t, cards[0].ID, got[0].ID)
	require.Equal(t, cards[1].ID, got[1].ID)

	// Advance via cursor populated with the second card's Due value.
	cursor := &repository.CardCursor{
		ID:  cards[1].ID,
		Due: &dueValues[1],
	}
	got, _, err = repo.FindPageByCardgroupForUser(
		ctx, ownerID, cg.ID, cursor, nil, 2, 0, repository.CardOrderByDue, repository.SortAsc, nil,
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
		ctx, cg.ID, nil, nil, 2, 0, repository.CardOrderByCreatedAt, repository.SortDesc, nil,
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
		ctx, cg.ID, cursor, nil, 2, 0, repository.CardOrderByCreatedAt, repository.SortDesc, nil,
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
		ctx, cg1.ID, nil, nil, 10, 0, repository.CardOrderByID, repository.SortAsc, nil,
	)
	require.NoError(t, err)
	require.Equal(t, int64(3), total)

	_, total, err = repo.FindPageByCardgroup(
		ctx, cg2.ID, nil, nil, 10, 0, repository.CardOrderByID, repository.SortAsc, nil,
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
	ucsRepo := repository.NewUserCardFSRSRepository(testDB.GORM)

	now := time.Now().UTC().Truncate(time.Microsecond)
	cards := make([]*domain.Card, 3)
	for i := 0; i < 3; i++ {
		c := newCard(cg.ID, fmt.Sprintf("front-%d", i), "back")
		c.CreatedAt = now
		c.UpdatedAt = now
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
	dueT := now
	require.NoError(t, testDB.GORM.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, card := range cards {
			state := domain.NewUserCardFSRSForNewCard(ownerID, card.ID, now)
			state.State.Due = dueT
			if err := ucsRepo.UpsertTx(ctx, tx, state); err != nil {
				return err
			}
		}
		return nil
	}))

	page1, total, err := repo.FindPageByCardgroupForUser(
		ctx, ownerID, cg.ID, nil, nil, 2, 0, repository.CardOrderByDue, repository.SortAsc, nil,
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
	page2, _, err := repo.FindPageByCardgroupForUser(
		ctx, ownerID, cg.ID, cursor, nil, 2, 0, repository.CardOrderByDue, repository.SortAsc, nil,
	)
	require.NoError(t, err)
	require.Len(t, page2, 1, "third card should appear exactly once")
	require.Equal(t, sorted[2].ID, page2[0].ID)
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
		ctx, cg.ID, nil, nil, 101, 0, repository.CardOrderByID, repository.SortAsc, nil,
	)
	require.NoError(t, err)
	require.Equal(t, int64(101), total101)
	require.Len(t, cards101, 101)

	// first=100 must be limited to 100 rows, confirming the cap still applies.
	cards100, total100, err := repo.FindPageByCardgroup(
		ctx, cg.ID, nil, nil, 100, 0, repository.CardOrderByID, repository.SortAsc, nil,
	)
	require.NoError(t, err)
	require.Equal(t, int64(101), total100)
	require.Len(t, cards100, 100)
}

// TestCardRepository_FindPageByCardgroup_ZeroPageReturnsTotal verifies the C2
// fix: first=0 && last=0 short-circuits the row fetch but still returns the
// real totalCount from the separate COUNT(*) query.
func TestCardRepository_FindPageByCardgroup_ZeroPageReturnsTotal(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ownerID := insertAuthUser(t, ctx)
	cg := insertCardgroup(t, ctx, ownerID)
	repo := repository.NewCardRepository(testDB.GORM)

	insertCards(t, ctx, repo, cg.ID, 5)

	cards, total, err := repo.FindPageByCardgroup(
		ctx, cg.ID, nil, nil, 0, 0, repository.CardOrderByID, repository.SortAsc, nil,
	)
	require.NoError(t, err)
	// No rows requested, but the slice must be non-nil and empty.
	require.NotNil(t, cards)
	require.Len(t, cards, 0)
	// totalCount must reflect the actual number of cards in the group.
	require.Equal(t, int64(5), total)
}

// TestCardRepo_FindPageByCardgroup_Search_WithAfter verifies that the ILIKE
// search filter is preserved when an `after` cursor is present. A bug that
// drops the predicate on the cursor branch would return non-matching cards on
// the second page, causing both the wrong IDs and a wrong page-3 hasNextPage
// signal.
//
// Setup: 5 cards, 3 of which match "apple" in their front text (front-apple-0,
// front-apple-1, front-apple-2). The remaining 2 do not match.
// Order: created_at ASC so the cursor walk is deterministic.
//
// Steps:
//  1. Page 1 (first=2, after=nil, search="apple") → 2 matching cards,
//     totalCount=3, the third matching card is the next page.
//  2. Take the cursor from the last returned edge (front-apple-1).
//  3. Page 2 (first=2, after=cursor, search="apple") → exactly 1 card
//     (front-apple-2), totalCount=3, hasNextPage=false.
//  4. Assert no non-matching card ever appears in either page.
func TestCardRepo_FindPageByCardgroup_Search_WithAfter(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ownerID := insertAuthUser(t, ctx)
	cg := insertCardgroup(t, ctx, ownerID)
	repo := repository.NewCardRepository(testDB.GORM)

	now := time.Now().UTC()

	// 3 cards whose front matches "apple", staggered so created_at order is
	// deterministic.
	matchingFronts := []string{"front-apple-0", "front-apple-1", "front-apple-2"}
	matching := make([]*domain.Card, 3)
	for i, front := range matchingFronts {
		c := newCard(cg.ID, front, "back")
		c.CreatedAt = now.Add(time.Duration(i) * time.Hour)
		c.UpdatedAt = c.CreatedAt
		require.NoError(t, repo.Create(ctx, c))
		matching[i] = c
	}

	// 2 cards that do NOT match "apple" — inserted after the matching ones so
	// they sort last by created_at.
	nonMatching := make([]*domain.Card, 2)
	for i, front := range []string{"front-cherry", "front-banana"} {
		c := newCard(cg.ID, front, "back")
		c.CreatedAt = now.Add(time.Duration(3+i) * time.Hour)
		c.UpdatedAt = c.CreatedAt
		require.NoError(t, repo.Create(ctx, c))
		nonMatching[i] = c
	}

	// Re-fetch so DB-rounded timestamps are used in the cursor comparisons.
	for i, c := range matching {
		got, err := repo.FindByID(ctx, c.ID)
		require.NoError(t, err)
		matching[i] = got
	}

	search := "apple"

	// --- Page 1 ---
	page1, total1, err := repo.FindPageByCardgroup(
		ctx, cg.ID, nil, nil,
		// Request 2+1 (the +1 trick) so the usecase can detect hasNextPage.
		3, 0,
		repository.CardOrderByCreatedAt, repository.SortAsc,
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
		ID:        last1.ID,
		CreatedAt: &last1.CreatedAt,
	}

	// --- Page 2 ---
	page2, total2, err := repo.FindPageByCardgroup(
		ctx, cg.ID, cursor, nil,
		3, 0,
		repository.CardOrderByCreatedAt, repository.SortAsc,
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

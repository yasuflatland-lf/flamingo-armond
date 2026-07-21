package repository_test

// Integration tests for the published master-catalog read path
// (FindPublishedPage / FindPublishedByID). FindPublishedPage carries the
// search-aware total as its second return value, so there is no separate count
// method. They run against the shared testcontainers Postgres provisioned by
// TestMain in user_test.go.
//
// Shared helpers reused from master_cardgroup_test.go:
//   - newMasterCardgroupMinimal
//   - filterMasterCardgroupsByIDs

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"backend/internal/domain"
	"backend/internal/repository"
)

// insertPublishedMCG creates a published master cardgroup with the given name
// and sort_order and returns it. The deck holds ZERO cards, so it is invisible
// to the public catalog reads (FindPublishedPage / FindPublishedByID) and
// visible only to the admin any-status list. Use insertCatalogVisibleMCG when
// the fixture is meant to appear in the public catalog.
func insertPublishedMCG(t *testing.T, ctx context.Context, name string, sortOrder int) *domain.MasterCardgroup {
	t.Helper()
	repo := repository.NewMasterCardgroupRepository(testDB.GORM)
	m := newMasterCardgroupMinimal(name)
	m.Status = domain.MasterStatusPublished
	m.SortOrder = sortOrder
	require.NoError(t, repo.Create(ctx, m))
	return m
}

// insertCatalogVisibleMCG creates a master cardgroup that satisfies the full
// public-catalog visibility predicate: status = published AND at least one
// master card. Published-but-card-less decks are deliberately excluded from the
// catalog, so every fixture that expects to be listed, found by id, imported or
// seeded must be created through this helper rather than insertPublishedMCG.
func insertCatalogVisibleMCG(t *testing.T, ctx context.Context, name string, sortOrder int) *domain.MasterCardgroup {
	t.Helper()
	m := insertPublishedMCG(t, ctx, name, sortOrder)
	insertMasterCards(t, ctx, m.ID, 1)
	return m
}

// insertDraftMCG creates a draft master cardgroup and returns it.
func insertDraftMCG(t *testing.T, ctx context.Context, name string) *domain.MasterCardgroup {
	t.Helper()
	repo := repository.NewMasterCardgroupRepository(testDB.GORM)
	m := newMasterCardgroupMinimal(name)
	m.Status = domain.MasterStatusDraft
	require.NoError(t, repo.Create(ctx, m))
	return m
}

// insertMasterCards inserts n master cards into the given master cardgroup so
// the cardCount aggregation has rows to count.
func insertMasterCards(t *testing.T, ctx context.Context, masterCardgroupID string, n int) {
	t.Helper()
	cardRepo := repository.NewMasterCardRepository(testDB.GORM)
	for i := 0; i < n; i++ {
		c := &domain.MasterCard{
			MasterCardgroupID: masterCardgroupID,
			Front:             domain.CardText("front-" + uuid.NewString()),
			Back:              domain.CardText("back"),
			Position:          i,
		}
		require.NoError(t, cardRepo.Create(ctx, c))
	}
}

// catalogIDs extracts the cardgroup IDs from a page of catalog items.
func catalogIDs(items []*repository.MasterCatalogItem) []string {
	out := make([]string, len(items))
	for i, it := range items {
		out[i] = it.Cardgroup.ID
	}
	return out
}

// filterCatalogByIDs keeps only the catalog items whose cardgroup ID is in want,
// preserving order. Isolates parallel-test cross-pollution from the shared DB.
func filterCatalogByIDs(items []*repository.MasterCatalogItem, want []string) []*repository.MasterCatalogItem {
	set := make(map[string]struct{}, len(want))
	for _, id := range want {
		set[id] = struct{}{}
	}
	out := make([]*repository.MasterCatalogItem, 0, len(want))
	for _, it := range items {
		if _, ok := set[it.Cardgroup.ID]; ok {
			out = append(out, it)
		}
	}
	return out
}

// ---------------------------------------------------------------------------
// Published-only filter: draft never leaks
// ---------------------------------------------------------------------------

func TestMasterCardgroupRepository_FindPublishedPage_DraftNeverLeaks(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := repository.NewMasterCardgroupRepository(testDB.GORM)

	base := uuid.NewString()
	pub := insertCatalogVisibleMCG(t, ctx, "Pub "+base, 1)
	draft := insertDraftMCG(t, ctx, "Draft "+base)

	page, _, err := repo.FindPublishedPage(ctx, nil, nil, repository.PageCap, 0,
		repository.MasterCatalogOrderBySortOrder, repository.SortAsc, nil)
	require.NoError(t, err)

	ours := filterCatalogByIDs(page, []string{pub.ID, draft.ID})
	ids := catalogIDs(ours)
	require.Contains(t, ids, pub.ID, "published row must be returned")
	require.NotContains(t, ids, draft.ID, "draft row must NOT be returned")
}

// ---------------------------------------------------------------------------
// cardCount aggregation
// ---------------------------------------------------------------------------

func TestMasterCardgroupRepository_FindPublishedPage_CardCount(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := repository.NewMasterCardgroupRepository(testDB.GORM)

	base := uuid.NewString()
	threeCards := insertPublishedMCG(t, ctx, "ThreeCards "+base, 1)
	insertMasterCards(t, ctx, threeCards.ID, 3)
	oneCard := insertCatalogVisibleMCG(t, ctx, "OneCard "+base, 2)

	page, _, err := repo.FindPublishedPage(ctx, nil, nil, repository.PageCap, 0,
		repository.MasterCatalogOrderBySortOrder, repository.SortAsc, nil)
	require.NoError(t, err)

	ours := filterCatalogByIDs(page, []string{threeCards.ID, oneCard.ID})
	counts := map[string]int64{}
	for _, it := range ours {
		counts[it.Cardgroup.ID] = it.CardCount
	}
	require.Equal(t, int64(3), counts[threeCards.ID], "deck with 3 cards reports cardCount=3")
	require.Equal(t, int64(1), counts[oneCard.ID], "deck with 1 card reports cardCount=1")
}

// ---------------------------------------------------------------------------
// Catalog visibility: published-but-card-less decks are excluded, and the page
// and the total are excluded together
// ---------------------------------------------------------------------------

// TestMasterCardgroupRepository_FindPublishedPage_EmptyDeckExcludedFromPageAndTotal
// pins the read-side catalog predicate: a published deck whose cards have all
// been deleted is not something a learner can study, so it leaves the catalog
// without any admin action. The total is asserted alongside the rows because
// findCatalogPage builds the COUNT and the page as two independent queries over
// the same logical base — filtering only one of them would strand the caller on
// a "showing 1 of 2" page that can never reach 2.
func TestMasterCardgroupRepository_FindPublishedPage_EmptyDeckExcludedFromPageAndTotal(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := repository.NewMasterCardgroupRepository(testDB.GORM)

	base := uuid.NewString()
	withCards := insertCatalogVisibleMCG(t, ctx, "VisibleDeck "+base, 1)
	empty := insertPublishedMCG(t, ctx, "EmptyDeck "+base, 2)

	// The shared base UUID suffix isolates this test's two rows from the other
	// tests running against the same database.
	search := base
	page, total, err := repo.FindPublishedPage(ctx, nil, nil, repository.PageCap, 0,
		repository.MasterCatalogOrderBySortOrder, repository.SortAsc, &search)
	require.NoError(t, err)

	require.Equal(t, []string{withCards.ID}, catalogIDs(page),
		"published deck with zero cards must not appear in the catalog page")
	require.Equal(t, int64(1), total,
		"totalCount must exclude the empty deck too, so it matches the returned rows")
	require.Equal(t, int64(len(page)), total,
		"page length and totalCount must agree for a single-page result")

	// Restoring one card brings the deck back with no admin action: visibility is
	// a read-side predicate, so there is no stale write-side state to repair.
	insertMasterCards(t, ctx, empty.ID, 1)

	page, total, err = repo.FindPublishedPage(ctx, nil, nil, repository.PageCap, 0,
		repository.MasterCatalogOrderBySortOrder, repository.SortAsc, &search)
	require.NoError(t, err)
	require.Equal(t, []string{withCards.ID, empty.ID}, catalogIDs(page),
		"restoring a card makes the deck visible again (self-healing)")
	require.Equal(t, int64(2), total,
		"totalCount recovers together with the page")
}

// ---------------------------------------------------------------------------
// FindPublishedPage total: excludes drafts, applies the search filter
// ---------------------------------------------------------------------------

func TestMasterCardgroupRepository_FindPublishedPage_TotalExcludesDraft(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := repository.NewMasterCardgroupRepository(testDB.GORM)

	base := uuid.NewString()
	insertCatalogVisibleMCG(t, ctx, "CountPub "+base, 1)
	insertDraftMCG(t, ctx, "CountDraft "+base)

	// A name search isolates this test's rows from the shared DB.
	search := "CountPub " + base
	_, total, err := repo.FindPublishedPage(ctx, nil, nil, repository.PageCap, 0,
		repository.MasterCatalogOrderBySortOrder, repository.SortAsc, &search)
	require.NoError(t, err)
	require.Equal(t, int64(1), total, "only the published row matching the search counts")
}

// ---------------------------------------------------------------------------
// FindPublishedByID
// ---------------------------------------------------------------------------

func TestMasterCardgroupRepository_FindPublishedByID(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := repository.NewMasterCardgroupRepository(testDB.GORM)

	base := uuid.NewString()
	pub := insertCatalogVisibleMCG(t, ctx, "FindPub "+base, 1)
	draft := insertDraftMCG(t, ctx, "FindDraft "+base)

	got, err := repo.FindPublishedByID(ctx, pub.ID)
	require.NoError(t, err)
	require.Equal(t, pub.ID, got.ID)

	_, err = repo.FindPublishedByID(ctx, draft.ID)
	require.True(t, errors.Is(err, repository.ErrNotFound),
		"draft id must return ErrNotFound (not part of the public catalog)")

	_, err = repo.FindPublishedByID(ctx, uuid.NewString())
	require.True(t, errors.Is(err, repository.ErrNotFound),
		"absent id must return ErrNotFound")
}

// TestMasterCardgroupRepository_FindPublishedByID_EmptyDeckNotFound covers the
// detail / import / merge gate: FindPublishedByID is the single lookup every one
// of those paths funnels through, so a published deck with zero cards must be
// indistinguishable from an unknown id there. Restoring one card makes it
// resolvable again with no admin action.
func TestMasterCardgroupRepository_FindPublishedByID_EmptyDeckNotFound(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := repository.NewMasterCardgroupRepository(testDB.GORM)

	empty := insertPublishedMCG(t, ctx, "FindEmpty "+uuid.NewString(), 1)

	_, err := repo.FindPublishedByID(ctx, empty.ID)
	require.True(t, errors.Is(err, repository.ErrNotFound),
		"published deck with zero cards must return ErrNotFound")

	insertMasterCards(t, ctx, empty.ID, 1)

	got, err := repo.FindPublishedByID(ctx, empty.ID)
	require.NoError(t, err, "restoring a card makes the deck resolvable again")
	require.Equal(t, empty.ID, got.ID)
}

// ---------------------------------------------------------------------------
// Forward + backward pagination round-trip
// ---------------------------------------------------------------------------

func TestMasterCardgroupRepository_FindPublishedPage_ForwardAndBackward(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := repository.NewMasterCardgroupRepository(testDB.GORM)

	base := uuid.NewString()
	// Three published decks with distinct sort_order so the (sort_order, id)
	// ordering is total and deterministic.
	m1 := insertCatalogVisibleMCG(t, ctx, "Page1 "+base, 1)
	m2 := insertCatalogVisibleMCG(t, ctx, "Page2 "+base, 2)
	m3 := insertCatalogVisibleMCG(t, ctx, "Page3 "+base, 3)
	ourIDs := []string{m1.ID, m2.ID, m3.ID}

	// A name search on the shared base UUID isolates this test's three rows from
	// the shared parallel DB — every name ends with " "+base. Without it, a
	// no-cursor first=N query returns the globally-lowest sort_order rows, which
	// other parallel tests' published rows can occupy (the same isolation pattern
	// used by the FindPublishedPage total test above).
	search := base

	// Forward page 1: first=2, no cursor. The two lowest sort_order rows of the
	// isolated set come back in order: [m1, m2].
	fwd1, _, err := repo.FindPublishedPage(ctx, nil, nil, 2, 0,
		repository.MasterCatalogOrderBySortOrder, repository.SortAsc, &search)
	require.NoError(t, err)
	ours1 := filterCatalogByIDs(fwd1, ourIDs)
	require.Equal(t, []string{m1.ID, m2.ID}, catalogIDs(ours1),
		"forward first=2 yields the two lowest sort_order rows [m1, m2]")

	// Forward from m1: cursor after m1 → [m2, m3].
	afterM1 := &repository.MasterCatalogCursor{ID: m1.ID, SortOrder: &m1.SortOrder}
	fwd2, _, err := repo.FindPublishedPage(ctx, afterM1, nil, repository.PageCap, 0,
		repository.MasterCatalogOrderBySortOrder, repository.SortAsc, &search)
	require.NoError(t, err)
	ours2 := filterCatalogByIDs(fwd2, ourIDs)
	require.Equal(t, []string{m2.ID, m3.ID}, catalogIDs(ours2),
		"forward after m1 yields [m2, m3] in sort order")

	// Backward before m3: should yield [m1, m2] in display order after the
	// internal direction-flip + reverse.
	beforeM3 := &repository.MasterCatalogCursor{ID: m3.ID, SortOrder: &m3.SortOrder}
	bwd, _, err := repo.FindPublishedPage(ctx, nil, beforeM3, 0, repository.PageCap,
		repository.MasterCatalogOrderBySortOrder, repository.SortAsc, &search)
	require.NoError(t, err)
	oursBwd := filterCatalogByIDs(bwd, ourIDs)
	require.Equal(t, []string{m1.ID, m2.ID}, catalogIDs(oursBwd),
		"backward before m3 yields [m1, m2] in display order")
}

// ---------------------------------------------------------------------------
// NAME + DESC: alternate order column and the DESC `<` cursor operator
// ---------------------------------------------------------------------------

func TestMasterCardgroupRepository_FindPublishedPage_OrderByNameDesc(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := repository.NewMasterCardgroupRepository(testDB.GORM)

	base := uuid.NewString()
	// Identical " "+base suffix makes the lexical order depend only on the
	// distinct prefixes, so DESC is deterministic regardless of the base value.
	a := insertCatalogVisibleMCG(t, ctx, "AName "+base, 1)
	b := insertCatalogVisibleMCG(t, ctx, "BName "+base, 2)
	c := insertCatalogVisibleMCG(t, ctx, "CName "+base, 3)
	ourIDs := []string{a.ID, b.ID, c.ID}
	search := base

	// NAME DESC, first page of 2: highest name first → [CName, BName].
	fwd, _, err := repo.FindPublishedPage(ctx, nil, nil, 2, 0,
		repository.MasterCatalogOrderByName, repository.SortDesc, &search)
	require.NoError(t, err)
	require.Equal(t, []string{c.ID, b.ID}, catalogIDs(filterCatalogByIDs(fwd, ourIDs)),
		"NAME DESC first page yields [CName, BName]")

	// Cursor after CName (NAME column hydrated) exercises the DESC `<` operator in
	// masterCatalogCursorWhere → [BName, AName].
	cName := string(c.Name)
	afterC := &repository.MasterCatalogCursor{ID: c.ID, Name: &cName}
	next, _, err := repo.FindPublishedPage(ctx, afterC, nil, repository.PageCap, 0,
		repository.MasterCatalogOrderByName, repository.SortDesc, &search)
	require.NoError(t, err)
	require.Equal(t, []string{b.ID, a.ID}, catalogIDs(filterCatalogByIDs(next, ourIDs)),
		"NAME DESC after CName yields [BName, AName]")
}

// ---------------------------------------------------------------------------
// Cursor missing the column required by the active orderBy is a caller bug
// ---------------------------------------------------------------------------

func TestMasterCardgroupRepository_FindPublishedPage_MissingCursorColumn(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := repository.NewMasterCardgroupRepository(testDB.GORM)

	// orderBy NAME but the cursor carries only ID (Name unset). A silent
	// zero-value fallback would emit a wrong-but-valid predicate and skip rows;
	// the repository must surface this caller bug as an error instead.
	badCursor := &repository.MasterCatalogCursor{ID: uuid.NewString()} // Name == nil
	_, _, err := repo.FindPublishedPage(ctx, badCursor, nil, 10, 0,
		repository.MasterCatalogOrderByName, repository.SortAsc, nil)
	require.Error(t, err)
}

// ---------------------------------------------------------------------------
// Search escapes ILIKE metacharacters so a literal '%' matches literally
// ---------------------------------------------------------------------------

func TestMasterCardgroupRepository_SearchEscapesLikeMetacharacters(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := repository.NewMasterCardgroupRepository(testDB.GORM)

	base := uuid.NewString()
	// Deck A's name contains a literal '%' immediately before base; deck B has a
	// plain letter there instead. A search term carrying the literal '%' must
	// match only A. If '%' were treated as an ILIKE wildcard, the term would also
	// match B (… "50" <anything> base …), so the count would be 2.
	a := insertCatalogVisibleMCG(t, ctx, "pct50%"+base, 1)
	b := insertCatalogVisibleMCG(t, ctx, "pct50x"+base, 2)

	search := "50%" + base
	page, total, err := repo.FindPublishedPage(ctx, nil, nil, repository.PageCap, 0,
		repository.MasterCatalogOrderBySortOrder, repository.SortAsc, &search)
	require.NoError(t, err)
	require.Equal(t, int64(1), total,
		"literal '%' search counts only the deck whose name contains '50%'+base")
	require.Equal(t, []string{a.ID}, catalogIDs(filterCatalogByIDs(page, []string{a.ID, b.ID})),
		"literal '%' search returns only deck A, not B")
}

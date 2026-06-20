package repository_test

// Integration tests for the admin master-catalog write and list operations:
//   - FindAdminPage total (includes draft + published; applies the search filter)
//   - CountCards (correlated count over master_cards)
//   - FindAdminPage (no status filter, otherwise identical to FindPublishedPage)
//   - Publish (sets status=published, bumps version)
//   - Unpublish (sets status=draft, version unchanged)
//
// All tests run against the shared testcontainers Postgres provisioned by
// TestMain in user_test.go. Shared helpers (newMasterCardgroupMinimal,
// insertPublishedMCG, insertDraftMCG, insertMasterCards, filterCatalogByIDs,
// catalogIDs) are defined in master_cardgroup_test.go / master_catalog_test.go.

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"backend/internal/domain"
	"backend/internal/repository"
)

// ---------------------------------------------------------------------------
// FindAdminPage total — includes both draft and published rows
// ---------------------------------------------------------------------------

func TestMasterCardgroupRepository_FindAdminPage_TotalIncludesDraftAndPublished(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := repository.NewMasterCardgroupRepository(testDB.GORM)

	base := uuid.NewString()
	insertPublishedMCG(t, ctx, "AdminCount Pub "+base, 1)
	insertDraftMCG(t, ctx, "AdminCount Draft "+base)

	// Search on the base UUID suffix so both names match (ILIKE %base%).
	search := base
	_, total, err := repo.FindAdminPage(ctx, nil, nil, repository.PageCap, 0,
		repository.MasterCatalogOrderBySortOrder, repository.SortAsc, &search)
	require.NoError(t, err)
	require.Equal(t, int64(2), total,
		"FindAdminPage total must count both draft and published rows")
}

func TestMasterCardgroupRepository_FindAdminPage_TotalSearchFilter(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := repository.NewMasterCardgroupRepository(testDB.GORM)

	base := uuid.NewString()
	insertDraftMCG(t, ctx, "AdminFilter Match "+base)
	insertDraftMCG(t, ctx, "AdminFilter NoMatch "+uuid.NewString())

	search := "AdminFilter Match " + base
	_, total, err := repo.FindAdminPage(ctx, nil, nil, repository.PageCap, 0,
		repository.MasterCatalogOrderBySortOrder, repository.SortAsc, &search)
	require.NoError(t, err)
	require.Equal(t, int64(1), total, "search filters the total by name ILIKE")
}

// ---------------------------------------------------------------------------
// CountCards — correlated count over master_cards
// ---------------------------------------------------------------------------

func TestMasterCardgroupRepository_CountCards_ZeroWhenEmpty(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := repository.NewMasterCardgroupRepository(testDB.GORM)

	m := insertDraftMCG(t, ctx, "CountCards Empty "+uuid.NewString())

	total, err := repo.CountCards(ctx, m.ID)
	require.NoError(t, err)
	require.Equal(t, int64(0), total, "cardgroup with no cards must return 0")
}

func TestMasterCardgroupRepository_CountCards_CountsOwnedCards(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := repository.NewMasterCardgroupRepository(testDB.GORM)

	m := insertDraftMCG(t, ctx, "CountCards Owned "+uuid.NewString())
	insertMasterCards(t, ctx, m.ID, 5)

	// Another deck with 2 cards to confirm the count is scoped to m.ID.
	other := insertDraftMCG(t, ctx, "CountCards Other "+uuid.NewString())
	insertMasterCards(t, ctx, other.ID, 2)

	total, err := repo.CountCards(ctx, m.ID)
	require.NoError(t, err)
	require.Equal(t, int64(5), total,
		"CountCards must return the card count scoped to the given cardgroup")
}

// ---------------------------------------------------------------------------
// FindAdminPage — draft rows visible, pagination mirrors FindPublishedPage
// ---------------------------------------------------------------------------

func TestMasterCardgroupRepository_FindAdminPage_DraftVisible(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := repository.NewMasterCardgroupRepository(testDB.GORM)

	base := uuid.NewString()
	pub := insertPublishedMCG(t, ctx, "AdminPage Pub "+base, 1)
	draft := insertDraftMCG(t, ctx, "AdminPage Draft "+base)
	ourIDs := []string{pub.ID, draft.ID}

	// Search on the base UUID suffix so both names match (ILIKE %base%).
	search := base
	page, _, err := repo.FindAdminPage(ctx, nil, nil, repository.PageCap, 0,
		repository.MasterCatalogOrderBySortOrder, repository.SortAsc, &search)
	require.NoError(t, err)

	ours := filterCatalogByIDs(page, ourIDs)
	ids := catalogIDs(ours)
	require.Contains(t, ids, pub.ID, "published row must appear in admin page")
	require.Contains(t, ids, draft.ID, "draft row must appear in admin page")
}

func TestMasterCardgroupRepository_FindAdminPage_OrderBySortOrder(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := repository.NewMasterCardgroupRepository(testDB.GORM)

	base := uuid.NewString()
	// Use distinct sort_order values so the ordering is total and deterministic.
	m1 := insertPublishedMCG(t, ctx, "AdminOrder1 "+base, 10)
	m2 := insertDraftMCG(t, ctx, "AdminOrder2 "+base)
	m2SortOrder := 20
	// Update sort_order for the draft row via Update to keep the fixture simple.
	_, err := repo.Update(context.Background(), m2.ID, repository.MasterCardgroupUpdate{
		SortOrder: &m2SortOrder,
	})
	require.NoError(t, err)
	m3 := insertPublishedMCG(t, ctx, "AdminOrder3 "+base, 30)
	ourIDs := []string{m1.ID, m2.ID, m3.ID}

	search := base
	page, _, err := repo.FindAdminPage(ctx, nil, nil, repository.PageCap, 0,
		repository.MasterCatalogOrderBySortOrder, repository.SortAsc, &search)
	require.NoError(t, err)

	ours := filterCatalogByIDs(page, ourIDs)
	require.Equal(t, []string{m1.ID, m2.ID, m3.ID}, catalogIDs(ours),
		"FindAdminPage must return rows in sort_order ASC order")
}

func TestMasterCardgroupRepository_FindAdminPage_CardCountAggregation(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := repository.NewMasterCardgroupRepository(testDB.GORM)

	base := uuid.NewString()
	withCards := insertDraftMCG(t, ctx, "AdminCards WithCards "+base)
	insertMasterCards(t, ctx, withCards.ID, 4)
	empty := insertDraftMCG(t, ctx, "AdminCards Empty "+base)

	// Search on the base UUID suffix so both names match (ILIKE %base%).
	search := base
	page, _, err := repo.FindAdminPage(ctx, nil, nil, repository.PageCap, 0,
		repository.MasterCatalogOrderBySortOrder, repository.SortAsc, &search)
	require.NoError(t, err)

	ours := filterCatalogByIDs(page, []string{withCards.ID, empty.ID})
	counts := map[string]int64{}
	for _, it := range ours {
		counts[it.Cardgroup.ID] = it.CardCount
	}
	require.Equal(t, int64(4), counts[withCards.ID],
		"draft deck with 4 cards must report cardCount=4")
	require.Equal(t, int64(0), counts[empty.ID],
		"draft deck with no cards must report cardCount=0")
}

func TestMasterCardgroupRepository_FindAdminPage_ForwardAndBackwardPagination(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := repository.NewMasterCardgroupRepository(testDB.GORM)

	base := uuid.NewString()
	m1 := insertPublishedMCG(t, ctx, "AdminPag1 "+base, 1)
	m2 := insertDraftMCG(t, ctx, "AdminPag2 "+base)
	// Set m2 sort_order to 2 so the ordering is deterministic.
	sortOrder2 := 2
	_, err := repo.Update(ctx, m2.ID, repository.MasterCardgroupUpdate{SortOrder: &sortOrder2})
	require.NoError(t, err)
	m3 := insertPublishedMCG(t, ctx, "AdminPag3 "+base, 3)
	ourIDs := []string{m1.ID, m2.ID, m3.ID}

	search := base

	// Forward page 1: first=2 yields [m1, m2].
	fwd1, _, err := repo.FindAdminPage(ctx, nil, nil, 2, 0,
		repository.MasterCatalogOrderBySortOrder, repository.SortAsc, &search)
	require.NoError(t, err)
	ours1 := filterCatalogByIDs(fwd1, ourIDs)
	require.Equal(t, []string{m1.ID, m2.ID}, catalogIDs(ours1),
		"forward first=2 yields [m1, m2]")

	// Backward before m3 yields [m1, m2] in display order.
	beforeM3 := &repository.MasterCatalogCursor{ID: m3.ID, SortOrder: &m3.SortOrder}
	bwd, _, err := repo.FindAdminPage(ctx, nil, beforeM3, 0, repository.PageCap,
		repository.MasterCatalogOrderBySortOrder, repository.SortAsc, &search)
	require.NoError(t, err)
	oursBwd := filterCatalogByIDs(bwd, ourIDs)
	require.Equal(t, []string{m1.ID, m2.ID}, catalogIDs(oursBwd),
		"backward before m3 yields [m1, m2] in display order")
}

// ---------------------------------------------------------------------------
// Publish — sets status=published, increments version
// ---------------------------------------------------------------------------

func TestMasterCardgroupRepository_Publish_SetsPublishedAndBumpsVersion(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := repository.NewMasterCardgroupRepository(testDB.GORM)

	// Start with a draft at version 1.
	m := newMasterCardgroupMinimal("Publish Me " + uuid.NewString())
	m.Version = 1
	m.Status = domain.MasterStatusDraft
	require.NoError(t, repo.Create(ctx, m))

	got, err := repo.Publish(ctx, m.ID)
	require.NoError(t, err)
	require.Equal(t, domain.MasterStatusPublished, got.Status,
		"Publish must set status to published")
	require.Equal(t, 2, got.Version,
		"Publish must increment version by 1 (1 → 2)")

	// Verify the DB state is consistent with the returned value.
	refetched, err := repo.FindByID(ctx, m.ID)
	require.NoError(t, err)
	require.Equal(t, domain.MasterStatusPublished, refetched.Status)
	require.Equal(t, 2, refetched.Version)
}

func TestMasterCardgroupRepository_Publish_NotFound(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := repository.NewMasterCardgroupRepository(testDB.GORM)

	_, err := repo.Publish(ctx, uuid.NewString())
	require.True(t, errors.Is(err, repository.ErrNotFound),
		"Publish on a non-existent id must return ErrNotFound")
}

// TestMasterCardgroupRepository_Publish_ConcurrentBumpsVersionExactlyOnce proves
// the version invariant under contention. Two goroutines publish the same draft
// at the same time; the repository loads the row FOR UPDATE inside a tx and the
// aggregate's idempotent Publish() makes the second call a no-op. The version
// therefore bumps exactly once (1 -> 2): the row lock prevents a lost update and
// the idempotency guard prevents a double bump.
func TestMasterCardgroupRepository_Publish_ConcurrentBumpsVersionExactlyOnce(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := repository.NewMasterCardgroupRepository(testDB.GORM)

	m := newMasterCardgroupMinimal("Concurrent Publish " + uuid.NewString())
	m.Version = 1
	m.Status = domain.MasterStatusDraft
	require.NoError(t, repo.Create(ctx, m))

	const n = 2
	var wg sync.WaitGroup
	errc := make(chan error, n)
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			_, err := repo.Publish(ctx, m.ID)
			errc <- err
		}()
	}
	wg.Wait()
	close(errc)
	for err := range errc {
		require.NoError(t, err)
	}

	got, err := repo.FindByID(ctx, m.ID)
	require.NoError(t, err)
	require.Equal(t, domain.MasterStatusPublished, got.Status)
	require.Equal(t, 2, got.Version,
		"concurrent publishes must bump version exactly once (no double, no lost update)")
}

// ---------------------------------------------------------------------------
// Unpublish — sets status=draft, version unchanged
// ---------------------------------------------------------------------------

func TestMasterCardgroupRepository_Unpublish_SetsDraftVersionUnchanged(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := repository.NewMasterCardgroupRepository(testDB.GORM)

	// Create as published at version 3.
	m := newMasterCardgroupMinimal("Unpublish Me " + uuid.NewString())
	m.Version = 3
	m.Status = domain.MasterStatusPublished
	require.NoError(t, repo.Create(ctx, m))

	got, err := repo.Unpublish(ctx, m.ID)
	require.NoError(t, err)
	require.Equal(t, domain.MasterStatusDraft, got.Status,
		"Unpublish must set status to draft")
	require.Equal(t, 3, got.Version,
		"Unpublish must NOT change the version (expected 3, no bump)")

	// Verify the DB state matches the returned value.
	refetched, err := repo.FindByID(ctx, m.ID)
	require.NoError(t, err)
	require.Equal(t, domain.MasterStatusDraft, refetched.Status)
	require.Equal(t, 3, refetched.Version)
}

func TestMasterCardgroupRepository_Unpublish_NotFound(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := repository.NewMasterCardgroupRepository(testDB.GORM)

	_, err := repo.Unpublish(ctx, uuid.NewString())
	require.True(t, errors.Is(err, repository.ErrNotFound),
		"Unpublish on a non-existent id must return ErrNotFound")
}

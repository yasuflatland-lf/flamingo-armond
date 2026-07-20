package repository_test

// TestMain, testDB, and sqlDBHandle are defined in user_test.go and shared
// across this package. Master cardgroup tests do NOT need insertAuthUser because
// master_cardgroups has no owner_id column — the privileged testDB connection
// bypasses RLS and can insert/query directly.

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"backend/internal/domain"
	"backend/internal/repository"
)

// newMasterCardgroup builds a fully-populated MasterCardgroup ready for Create.
// All optional pointer fields are set to non-nil values to exercise the full
// round-trip path. Use newMasterCardgroupMinimal for the nil-field path.
func newMasterCardgroup(name string) *domain.MasterCardgroup {
	now := time.Now().UTC()
	desc := "Test description"
	ver := 2
	sortOrder := 3
	status := string(domain.MasterStatusPublished)
	isStarter := true
	return &domain.MasterCardgroup{
		ID:               uuid.NewString(),
		Name:             domain.CardgroupName(name),
		Description:      domain.DescriptionFromPtr(&desc),
		Version:          ver,
		Status:           domain.MasterCardgroupStatus(status),
		IsDefaultStarter: isStarter,
		SortOrder:        sortOrder,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
}

// newMasterCardgroupMinimal builds a MasterCardgroup with the optional Description
// field cleared. This exercises the NULL round-trip path.
func newMasterCardgroupMinimal(name string) *domain.MasterCardgroup {
	now := time.Now().UTC()
	return &domain.MasterCardgroup{
		ID:               uuid.NewString(),
		Name:             domain.CardgroupName(name),
		Description:      domain.Description{},
		Version:          1,
		Status:           domain.MasterStatusDraft,
		IsDefaultStarter: false,
		SortOrder:        0,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
}

// ---------------------------------------------------------------------------
// Create + FindByID round-trip
// ---------------------------------------------------------------------------

func TestMasterCardgroupRepository_CreateAndFindByID_AllFields(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := repository.NewMasterCardgroupRepository(testDB.GORM)

	m := newMasterCardgroup("All Fields " + uuid.NewString())
	require.NoError(t, repo.Create(ctx, m))

	got, err := repo.FindByID(ctx, m.ID)
	require.NoError(t, err)

	require.Equal(t, m.ID, got.ID)
	require.Equal(t, m.Name.String(), got.Name.String())
	require.NotNil(t, got.Description.Ptr())
	require.Equal(t, *m.Description.Ptr(), *got.Description.Ptr())
	require.Equal(t, m.Version, got.Version)
	require.Equal(t, m.Status, got.Status)
	require.Equal(t, m.IsDefaultStarter, got.IsDefaultStarter)
	require.Equal(t, m.SortOrder, got.SortOrder)
	require.False(t, got.CreatedAt.IsZero())
	require.False(t, got.UpdatedAt.IsZero())
}

func TestMasterCardgroupRepository_CreateAndFindByID_NullOptionalFields(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := repository.NewMasterCardgroupRepository(testDB.GORM)

	m := newMasterCardgroupMinimal("Nil Fields " + uuid.NewString())
	require.NoError(t, repo.Create(ctx, m))

	got, err := repo.FindByID(ctx, m.ID)
	require.NoError(t, err)

	require.Equal(t, m.ID, got.ID)
	require.Equal(t, m.Name.String(), got.Name.String())
	require.Nil(t, got.Description.Ptr())
	require.Equal(t, 1, got.Version)
	require.Equal(t, domain.MasterStatusDraft, got.Status)
	require.False(t, got.IsDefaultStarter)
	require.Equal(t, 0, got.SortOrder)
}

// ---------------------------------------------------------------------------
// FindByID absent
// ---------------------------------------------------------------------------

func TestMasterCardgroupRepository_FindByID_NotFound(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := repository.NewMasterCardgroupRepository(testDB.GORM)

	_, err := repo.FindByID(ctx, uuid.NewString())
	require.True(t, errors.Is(err, repository.ErrNotFound),
		"want ErrNotFound, got %v", err)
}

// ---------------------------------------------------------------------------
// EnsureByName idempotency
// ---------------------------------------------------------------------------

func TestMasterCardgroupRepository_EnsureByName_Idempotent(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := repository.NewMasterCardgroupRepository(testDB.GORM)

	name := "Ensure Idempotent " + uuid.NewString()

	first, err := repo.EnsureByName(ctx, name)
	require.NoError(t, err)
	require.NotEmpty(t, first.ID)
	require.Equal(t, name, first.Name.String())

	// Call again with the SAME name — must return same ID, no duplicate row.
	second, err := repo.EnsureByName(ctx, name)
	require.NoError(t, err)
	require.Equal(t, first.ID, second.ID, "second call must return the same ID")

	// Verify default values on create.
	require.Equal(t, 1, first.Version)
	require.Equal(t, domain.MasterStatusDraft, first.Status)
	require.False(t, first.IsDefaultStarter)
	require.Equal(t, 0, first.SortOrder)
}

func TestMasterCardgroupRepository_EnsureByName_DifferentNames_DifferentIDs(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := repository.NewMasterCardgroupRepository(testDB.GORM)

	base := uuid.NewString()
	nameA := "Ensure A " + base
	nameB := "Ensure B " + base

	a, err := repo.EnsureByName(ctx, nameA)
	require.NoError(t, err)

	b, err := repo.EnsureByName(ctx, nameB)
	require.NoError(t, err)

	require.NotEqual(t, a.ID, b.ID, "different names must yield different IDs")
}

func TestMasterCardgroupRepository_EnsureByName_Race(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := repository.NewMasterCardgroupRepository(testDB.GORM)

	name := "Ensure Race Master " + uuid.NewString()

	const workers = 2
	results := make([]*domain.MasterCardgroup, workers)
	errs := make([]error, workers)
	var wg sync.WaitGroup
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func(i int) {
			defer wg.Done()
			results[i], errs[i] = repo.EnsureByName(ctx, name)
		}(i)
	}
	wg.Wait()

	for _, err := range errs {
		require.NoError(t, err)
	}
	require.NotNil(t, results[0])
	require.NotNil(t, results[1])
	require.Equal(t, results[0].ID, results[1].ID,
		"concurrent EnsureByName must produce exactly one row")
}

// ---------------------------------------------------------------------------
// Update patch
// ---------------------------------------------------------------------------

func TestMasterCardgroupRepository_Update_PartialPatch(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := repository.NewMasterCardgroupRepository(testDB.GORM)

	m := newMasterCardgroupMinimal("Update Patch " + uuid.NewString())
	require.NoError(t, repo.Create(ctx, m))

	statusPublished := string(domain.MasterStatusPublished)
	sortOrder := 5
	got, err := repo.Update(ctx, m.ID, repository.MasterCardgroupUpdate{
		Status:    &statusPublished,
		SortOrder: &sortOrder,
	})
	require.NoError(t, err)

	// Changed fields.
	require.Equal(t, domain.MasterStatusPublished, got.Status)
	require.Equal(t, 5, got.SortOrder)

	// Unchanged fields should be preserved.
	require.Equal(t, m.Name.String(), got.Name.String())
	require.Equal(t, m.Version, got.Version)
	require.Nil(t, got.Description.Ptr())
	require.False(t, got.IsDefaultStarter)

	// Verify the returned row is from DB (re-fetched state).
	refetched, err := repo.FindByID(ctx, m.ID)
	require.NoError(t, err)
	require.Equal(t, got.Status, refetched.Status)
	require.Equal(t, got.SortOrder, refetched.SortOrder)
}

func TestMasterCardgroupRepository_Update_NotFound(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := repository.NewMasterCardgroupRepository(testDB.GORM)

	statusDraft := string(domain.MasterStatusDraft)
	_, err := repo.Update(ctx, uuid.NewString(), repository.MasterCardgroupUpdate{
		Status: &statusDraft,
	})
	require.True(t, errors.Is(err, repository.ErrNotFound),
		"want ErrNotFound for non-existent id, got %v", err)
}

// ---------------------------------------------------------------------------
// Delete
// ---------------------------------------------------------------------------

func TestMasterCardgroupRepository_Delete_Success(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := repository.NewMasterCardgroupRepository(testDB.GORM)

	m := newMasterCardgroupMinimal("Delete Me " + uuid.NewString())
	require.NoError(t, repo.Create(ctx, m))

	require.NoError(t, repo.Delete(ctx, m.ID))

	_, err := repo.FindByID(ctx, m.ID)
	require.True(t, errors.Is(err, repository.ErrNotFound),
		"deleted master cardgroup should not be found")
}

func TestMasterCardgroupRepository_Delete_NotFound(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := repository.NewMasterCardgroupRepository(testDB.GORM)

	err := repo.Delete(ctx, uuid.NewString())
	require.True(t, errors.Is(err, repository.ErrNotFound),
		"want ErrNotFound for non-existent id, got %v", err)
}

// ---------------------------------------------------------------------------
// ListPublishedDefaultStarters filter + ordering
// ---------------------------------------------------------------------------

func TestMasterCardgroupRepository_ListPublishedDefaultStarters_FilterAndOrder(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := repository.NewMasterCardgroupRepository(testDB.GORM)

	// (a) published + starter + 1 card, sort_order=2
	mA := newMasterCardgroupMinimal("Starter A " + uuid.NewString())
	mA.Status = domain.MasterStatusPublished
	mA.IsDefaultStarter = true
	mA.SortOrder = 2
	require.NoError(t, repo.Create(ctx, mA))
	insertMasterCards(t, ctx, mA.ID, 1)

	// (b) published + starter + 1 card, sort_order=1
	mB := newMasterCardgroupMinimal("Starter B " + uuid.NewString())
	mB.Status = domain.MasterStatusPublished
	mB.IsDefaultStarter = true
	mB.SortOrder = 1
	require.NoError(t, repo.Create(ctx, mB))
	insertMasterCards(t, ctx, mB.ID, 1)

	// (c) published + NOT starter
	mC := newMasterCardgroupMinimal("Not Starter " + uuid.NewString())
	mC.Status = domain.MasterStatusPublished
	mC.IsDefaultStarter = false
	require.NoError(t, repo.Create(ctx, mC))
	insertMasterCards(t, ctx, mC.ID, 1)

	// (d) draft + starter
	mD := newMasterCardgroupMinimal("Draft Starter " + uuid.NewString())
	mD.Status = domain.MasterStatusDraft
	mD.IsDefaultStarter = true
	require.NoError(t, repo.Create(ctx, mD))
	insertMasterCards(t, ctx, mD.ID, 1)

	// (e) published + starter but ZERO cards. Seeding a learner with an empty
	// deck is worse than seeding them with nothing, so the catalog-visibility
	// predicate excludes it here exactly as it does in the catalog listing.
	mE := newMasterCardgroupMinimal("Empty Starter " + uuid.NewString())
	mE.Status = domain.MasterStatusPublished
	mE.IsDefaultStarter = true
	mE.SortOrder = 3
	require.NoError(t, repo.Create(ctx, mE))

	all, err := repo.ListPublishedDefaultStarters(ctx)
	require.NoError(t, err)

	// Filter down to only the rows we created in this test.
	ours := filterMasterCardgroupsByIDs(all, []string{mA.ID, mB.ID, mC.ID, mD.ID, mE.ID})

	// Only (a) and (b) should be in the result.
	require.Len(t, ours, 2, "only published+starter rows holding at least one card should be returned")

	ids := make([]string, len(ours))
	for i, row := range ours {
		ids[i] = row.ID
	}
	assert.Contains(t, ids, mA.ID, "published+starter+cards (a) must be in list")
	assert.Contains(t, ids, mB.ID, "published+starter+cards (b) must be in list")
	assert.NotContains(t, ids, mC.ID, "published+non-starter (c) must NOT be in list")
	assert.NotContains(t, ids, mD.ID, "draft+starter (d) must NOT be in list")
	assert.NotContains(t, ids, mE.ID, "published+starter with zero cards (e) must NOT be in list")

	// Ordering: sort_order ASC — (b) sort_order=1 must come before (a) sort_order=2.
	require.Equal(t, mB.ID, ours[0].ID, "lower sort_order (b) must come first")
	require.Equal(t, mA.ID, ours[1].ID, "higher sort_order (a) must come second")

	// Self-healing: adding a card to (e) makes it a valid starter again with no
	// admin action, and it lands last by sort_order.
	insertMasterCards(t, ctx, mE.ID, 1)
	all, err = repo.ListPublishedDefaultStarters(ctx)
	require.NoError(t, err)
	healed := filterMasterCardgroupsByIDs(all, []string{mA.ID, mB.ID, mE.ID})
	require.Equal(t, []string{mB.ID, mA.ID, mE.ID}, masterCardgroupIDs(healed),
		"restoring a card returns the starter to the seed set (self-healing)")
}

// masterCardgroupIDs extracts the ids from a slice of master cardgroups.
func masterCardgroupIDs(rows []*domain.MasterCardgroup) []string {
	out := make([]string, len(rows))
	for i, row := range rows {
		out[i] = row.ID
	}
	return out
}

// filterMasterCardgroupsByIDs returns only the rows whose IDs appear in wantIDs,
// preserving the original order. This isolates parallel-test cross-pollution
// from the shared testcontainers DB.
func filterMasterCardgroupsByIDs(rows []*domain.MasterCardgroup, wantIDs []string) []*domain.MasterCardgroup {
	set := make(map[string]struct{}, len(wantIDs))
	for _, id := range wantIDs {
		set[id] = struct{}{}
	}
	out := make([]*domain.MasterCardgroup, 0, len(wantIDs))
	for _, row := range rows {
		if _, ok := set[row.ID]; ok {
			out = append(out, row)
		}
	}
	return out
}

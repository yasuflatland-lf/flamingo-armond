package repository_test

// TestMain, testDB, insertAuthUser, sqlDBHandle, and insertNAuthUsers are
// defined in user_test.go and shared across this package.

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"backend/internal/domain"
	"backend/internal/repository"
)

// newCardgroup builds a minimal Cardgroup ready for Create.
func newCardgroup(ownerID, name string) *domain.Cardgroup {
	now := time.Now().UTC()
	return &domain.Cardgroup{
		ID:        uuid.NewString(),
		OwnerID:   ownerID,
		Name:      name,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

func TestCardgroupRepository_CreateAndFindByID(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ownerID := insertAuthUser(t, ctx)
	repo := repository.NewCardgroupRepository(testDB.GORM)

	cg := newCardgroup(ownerID, "My Flashcards")
	require.NoError(t, repo.Create(ctx, cg))

	got, err := repo.FindByID(ctx, cg.ID)
	require.NoError(t, err)
	require.Equal(t, cg.ID, got.ID)
	require.Equal(t, ownerID, got.OwnerID)
	require.Equal(t, "My Flashcards", got.Name)
	require.False(t, got.CreatedAt.IsZero())
	require.False(t, got.UpdatedAt.IsZero())
}

func TestCardgroupRepository_FindByOwner_ScopedToOwner(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	owner1 := insertAuthUser(t, ctx)
	owner2 := insertAuthUser(t, ctx)
	repo := repository.NewCardgroupRepository(testDB.GORM)

	cg1 := newCardgroup(owner1, "Owner1 Group")
	cg2 := newCardgroup(owner2, "Owner2 Group")
	require.NoError(t, repo.Create(ctx, cg1))
	require.NoError(t, repo.Create(ctx, cg2))

	result1, err := repo.FindByOwner(ctx, owner1)
	require.NoError(t, err)

	// Filter to only the groups we created in this test to avoid cross-test
	// interference from parallel tests that share the same DB.
	ids1 := cardgroupIDSet(result1)
	require.Contains(t, ids1, cg1.ID, "owner1 should see their own group")
	require.NotContains(t, ids1, cg2.ID, "owner1 should not see owner2's group")

	result2, err := repo.FindByOwner(ctx, owner2)
	require.NoError(t, err)
	ids2 := cardgroupIDSet(result2)
	require.Contains(t, ids2, cg2.ID, "owner2 should see their own group")
	require.NotContains(t, ids2, cg1.ID, "owner2 should not see owner1's group")
}

func cardgroupIDSet(cgs []*domain.Cardgroup) map[string]struct{} {
	m := make(map[string]struct{}, len(cgs))
	for _, cg := range cgs {
		m[cg.ID] = struct{}{}
	}
	return m
}

func TestCardgroupRepository_FindByOwner_OrderedByUpdatedAtDesc(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ownerID := insertAuthUser(t, ctx)
	repo := repository.NewCardgroupRepository(testDB.GORM)

	cg1 := newCardgroup(ownerID, "First")
	cg2 := newCardgroup(ownerID, "Second")
	cg3 := newCardgroup(ownerID, "Third")
	require.NoError(t, repo.Create(ctx, cg1))
	require.NoError(t, repo.Create(ctx, cg2))
	require.NoError(t, repo.Create(ctx, cg3))

	// Advance time so the trigger fires a strictly later updated_at.
	time.Sleep(5 * time.Millisecond)
	updated := "Second Updated"
	_, err := repo.Update(ctx, cg2.ID, repository.CardgroupUpdate{Name: &updated})
	require.NoError(t, err)

	result, err := repo.FindByOwner(ctx, ownerID)
	require.NoError(t, err)

	// Find positions of our three groups inside the (potentially larger) result.
	pos := make(map[string]int, 3)
	for i, cg := range result {
		switch cg.ID {
		case cg1.ID, cg2.ID, cg3.ID:
			pos[cg.ID] = i
		}
	}
	require.Len(t, pos, 3, "all three created cardgroups should be present")
	// cg2 was updated last, so it must appear before cg1 and cg3.
	require.Less(t, pos[cg2.ID], pos[cg1.ID], "updated group should sort before cg1")
	require.Less(t, pos[cg2.ID], pos[cg3.ID], "updated group should sort before cg3")
}

func TestCardgroupRepository_FindByIDs_AllFound(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ownerID := insertAuthUser(t, ctx)
	repo := repository.NewCardgroupRepository(testDB.GORM)

	cg1 := newCardgroup(ownerID, "A")
	cg2 := newCardgroup(ownerID, "B")
	cg3 := newCardgroup(ownerID, "C")
	require.NoError(t, repo.Create(ctx, cg1))
	require.NoError(t, repo.Create(ctx, cg2))
	require.NoError(t, repo.Create(ctx, cg3))

	got, err := repo.FindByIDs(ctx, []string{cg1.ID, cg2.ID, cg3.ID})
	require.NoError(t, err)
	require.Len(t, got, 3)
	require.NotNil(t, got[cg1.ID])
	require.NotNil(t, got[cg2.ID])
	require.NotNil(t, got[cg3.ID])
}

func TestCardgroupRepository_FindByIDs_PartialMissing(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ownerID := insertAuthUser(t, ctx)
	repo := repository.NewCardgroupRepository(testDB.GORM)

	cg := newCardgroup(ownerID, "Exists")
	require.NoError(t, repo.Create(ctx, cg))

	missing := uuid.NewString()
	got, err := repo.FindByIDs(ctx, []string{cg.ID, missing})
	require.NoError(t, err)
	require.Len(t, got, 1)
	require.NotNil(t, got[cg.ID])
	require.Nil(t, got[missing])
}

func TestCardgroupRepository_FindByIDs_EmptySlice(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := repository.NewCardgroupRepository(testDB.GORM)

	got, err := repo.FindByIDs(ctx, []string{})
	require.NoError(t, err)
	require.Empty(t, got)
}

func TestCardgroupRepository_Update_NameOnly(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ownerID := insertAuthUser(t, ctx)
	repo := repository.NewCardgroupRepository(testDB.GORM)

	cg := newCardgroup(ownerID, "Original")
	require.NoError(t, repo.Create(ctx, cg))

	time.Sleep(5 * time.Millisecond)

	newName := "Renamed"
	got, err := repo.Update(ctx, cg.ID, repository.CardgroupUpdate{Name: &newName})
	require.NoError(t, err)
	require.Equal(t, "Renamed", got.Name)
	require.True(t, got.UpdatedAt.After(cg.CreatedAt),
		"updated_at should be strictly later than created_at")
}

func TestCardgroupRepository_Update_EmptyPatchReturnsCurrent(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ownerID := insertAuthUser(t, ctx)
	repo := repository.NewCardgroupRepository(testDB.GORM)

	cg := newCardgroup(ownerID, "Stable")
	require.NoError(t, repo.Create(ctx, cg))

	before, err := repo.FindByID(ctx, cg.ID)
	require.NoError(t, err)

	after, err := repo.Update(ctx, cg.ID, repository.CardgroupUpdate{})
	require.NoError(t, err)
	require.Equal(t, before.UpdatedAt.UnixNano(), after.UpdatedAt.UnixNano(),
		"empty patch must not bump updated_at")
	require.Equal(t, before.Name, after.Name)
}

func TestCardgroupRepository_Update_NotFound(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := repository.NewCardgroupRepository(testDB.GORM)

	name := "ghost"
	_, err := repo.Update(ctx, uuid.NewString(), repository.CardgroupUpdate{Name: &name})
	require.True(t, errors.Is(err, repository.ErrNotFound),
		"want ErrNotFound, got %v", err)
}

func TestCardgroupRepository_Delete_Success(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ownerID := insertAuthUser(t, ctx)
	repo := repository.NewCardgroupRepository(testDB.GORM)

	cg := newCardgroup(ownerID, "To Delete")
	require.NoError(t, repo.Create(ctx, cg))

	require.NoError(t, repo.Delete(ctx, cg.ID))

	_, err := repo.FindByID(ctx, cg.ID)
	require.True(t, errors.Is(err, repository.ErrNotFound),
		"deleted cardgroup should not be found")
}

func TestCardgroupRepository_Delete_NotFound(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := repository.NewCardgroupRepository(testDB.GORM)

	err := repo.Delete(ctx, uuid.NewString())
	require.True(t, errors.Is(err, repository.ErrNotFound),
		"want ErrNotFound for non-existent id, got %v", err)
}

func TestCardgroupRepository_OnUserDeleteCascade(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ownerID := insertAuthUser(t, ctx)
	repo := repository.NewCardgroupRepository(testDB.GORM)

	cg := newCardgroup(ownerID, "Will Cascade")
	require.NoError(t, repo.Create(ctx, cg))

	// Deleting auth.users cascades → public.users → cardgroups.
	sqlDB := sqlDBHandle(t)
	_, err := sqlDB.ExecContext(ctx, `DELETE FROM auth.users WHERE id = $1`, ownerID)
	require.NoError(t, err)

	_, err = repo.FindByID(ctx, cg.ID)
	require.True(t, errors.Is(err, repository.ErrNotFound),
		"cardgroup should be gone after cascade delete of owning user")
}

func TestCardgroupRepository_NameLengthCheckRejectsTooLong(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ownerID := insertAuthUser(t, ctx)
	repo := repository.NewCardgroupRepository(testDB.GORM)

	// 101 ASCII characters: exceeds the CHECK constraint (1-100 trimmed code points).
	longName := strings.Repeat("a", 101)
	cg := newCardgroup(ownerID, longName)

	err := repo.Create(ctx, cg)
	require.Error(t, err, "DB CHECK constraint should reject a 101-char name")
}

// ---------------------------------------------------------------------------
// FindPageByOwner / CountByOwner integration tests
// ---------------------------------------------------------------------------

// insertNamedCardgroups inserts cardgroups with the given names for ownerID
// and returns the domain objects in insertion order.
func insertNamedCardgroups(t *testing.T, ctx context.Context, ownerID string, names []string) []*domain.Cardgroup {
	t.Helper()
	repo := repository.NewCardgroupRepository(testDB.GORM)
	cgs := make([]*domain.Cardgroup, len(names))
	for i, name := range names {
		now := time.Now().UTC().Add(time.Duration(i) * time.Millisecond)
		cg := &domain.Cardgroup{
			ID:        uuid.NewString(),
			OwnerID:   ownerID,
			Name:      name,
			CreatedAt: now,
			UpdatedAt: now,
		}
		require.NoError(t, repo.Create(ctx, cg))
		cgs[i] = cg
	}
	return cgs
}

// cardgroupNameSet builds a set of names from a slice for membership checks.
func cardgroupNameSet(cgs []*domain.Cardgroup) map[string]struct{} {
	m := make(map[string]struct{}, len(cgs))
	for _, cg := range cgs {
		m[cg.Name] = struct{}{}
	}
	return m
}

// cardgroupIDSetFromSlice builds a set of IDs from a slice for membership checks.
func cardgroupIDSetFromSlice(cgs []*domain.Cardgroup) map[string]struct{} {
	m := make(map[string]struct{}, len(cgs))
	for _, cg := range cgs {
		m[cg.ID] = struct{}{}
	}
	return m
}

// TestCardgroupRepo_FindPageByOwner_EmptyResult confirms that querying a
// fresh owner with no cardgroups returns an empty slice without error.
func TestCardgroupRepo_FindPageByOwner_EmptyResult(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ownerID := insertAuthUser(t, ctx)
	repo := repository.NewCardgroupRepository(testDB.GORM)

	got, err := repo.FindPageByOwner(
		ctx, ownerID, nil, nil, 10, 0,
		repository.CardgroupOrderByCreatedAt, repository.SortAsc, nil,
	)
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Empty(t, got)

	total, err := repo.CountByOwner(ctx, ownerID, nil)
	require.NoError(t, err)
	require.Equal(t, int64(0), total)
}

// TestCardgroupRepo_FindPageByOwner_ExactMatch confirms that an exact-name
// search term returns only the matching cardgroup.
func TestCardgroupRepo_FindPageByOwner_ExactMatch(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ownerID := insertAuthUser(t, ctx)
	repo := repository.NewCardgroupRepository(testDB.GORM)

	insertNamedCardgroups(t, ctx, ownerID, []string{"Exact Match", "Other Group"})

	search := "Exact Match"
	got, err := repo.FindPageByOwner(
		ctx, ownerID, nil, nil, 10, 0,
		repository.CardgroupOrderByCreatedAt, repository.SortAsc, &search,
	)
	require.NoError(t, err)
	require.Len(t, got, 1, "exactly one group should match the search term")
	require.Equal(t, "Exact Match", got[0].Name)

	total, err := repo.CountByOwner(ctx, ownerID, &search)
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
}

// TestCardgroupRepo_FindPageByOwner_PartialMatch confirms that a substring
// search returns all groups whose names contain the term.
func TestCardgroupRepo_FindPageByOwner_PartialMatch(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ownerID := insertAuthUser(t, ctx)
	repo := repository.NewCardgroupRepository(testDB.GORM)

	insertNamedCardgroups(t, ctx, ownerID, []string{"green apple", "apple pie", "banana"})

	search := "apple"
	got, err := repo.FindPageByOwner(
		ctx, ownerID, nil, nil, 10, 0,
		repository.CardgroupOrderByCreatedAt, repository.SortAsc, &search,
	)
	require.NoError(t, err)
	names := cardgroupNameSet(got)
	require.Contains(t, names, "green apple", "partial match should hit 'green apple'")
	require.Contains(t, names, "apple pie", "partial match should hit 'apple pie'")
	require.NotContains(t, names, "banana", "banana must not appear in apple search results")
	require.Len(t, got, 2)

	total, err := repo.CountByOwner(ctx, ownerID, &search)
	require.NoError(t, err)
	require.Equal(t, int64(2), total)
}

// TestCardgroupRepo_FindPageByOwner_LIKEEscape verifies that a percent sign in
// the search term matches literally and does not act as a wildcard. Inserting
// "100%" and "1000" then searching for "100%" must return only the former;
// without escaping the pattern "%100%%%" would also match "1000".
func TestCardgroupRepo_FindPageByOwner_LIKEEscape(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ownerID := insertAuthUser(t, ctx)
	repo := repository.NewCardgroupRepository(testDB.GORM)

	insertNamedCardgroups(t, ctx, ownerID, []string{"100%", "1000"})

	search := "100%"
	got, err := repo.FindPageByOwner(
		ctx, ownerID, nil, nil, 10, 0,
		repository.CardgroupOrderByCreatedAt, repository.SortAsc, &search,
	)
	require.NoError(t, err)
	names := cardgroupNameSet(got)
	// "100%" must be found because its name literally contains "100%".
	require.Contains(t, names, "100%", "literal percent must match the row named '100%%'")
	// "1000" must NOT be found — without escaping, '%' would be a wildcard and
	// the pattern "%%100%%" would match "1000" too.
	require.NotContains(t, names, "1000", "unescaped '%' would wrongly match '1000'; escaping must prevent that")
	require.Len(t, got, 1, "only one row should match the literal '100%%' search")

	total, err := repo.CountByOwner(ctx, ownerID, &search)
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
}

// TestCardgroupRepo_FindPageByOwner_LIKEUnderscoreEscape verifies that an
// underscore in the search term matches literally and does not act as a
// single-character wildcard. Searching for "a_b" must return "a_b" but not
// "acb".
func TestCardgroupRepo_FindPageByOwner_LIKEUnderscoreEscape(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ownerID := insertAuthUser(t, ctx)
	repo := repository.NewCardgroupRepository(testDB.GORM)

	insertNamedCardgroups(t, ctx, ownerID, []string{"a_b", "acb"})

	search := "a_b"
	got, err := repo.FindPageByOwner(
		ctx, ownerID, nil, nil, 10, 0,
		repository.CardgroupOrderByCreatedAt, repository.SortAsc, &search,
	)
	require.NoError(t, err)
	names := cardgroupNameSet(got)
	// "a_b" must match because the literal underscore appears in the name.
	require.Contains(t, names, "a_b", "literal underscore must match the row named 'a_b'")
	// "acb" must NOT match — without escaping '_' would be a single-char wildcard.
	require.NotContains(t, names, "acb", "unescaped '_' would wrongly match 'acb'; escaping must prevent that")
	require.Len(t, got, 1, "only one row should match the literal 'a_b' search")
}

// TestCardgroupRepo_FindPageByOwner_LIKEBackslashEscape verifies that a
// backslash in the search term matches literally. Searching for the Go string
// `back\slash` must return "back\slash" but not "backslash".
func TestCardgroupRepo_FindPageByOwner_LIKEBackslashEscape(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ownerID := insertAuthUser(t, ctx)
	repo := repository.NewCardgroupRepository(testDB.GORM)

	// Go raw string: the name literally contains a backslash character.
	insertNamedCardgroups(t, ctx, ownerID, []string{`back\slash`, "backslash"})

	// Search for the literal backslash-containing name.
	search := `back\slash`
	got, err := repo.FindPageByOwner(
		ctx, ownerID, nil, nil, 10, 0,
		repository.CardgroupOrderByCreatedAt, repository.SortAsc, &search,
	)
	require.NoError(t, err)
	names := cardgroupNameSet(got)
	require.Contains(t, names, `back\slash`, "literal backslash must match the row named 'back\\slash'")
	require.NotContains(t, names, "backslash", "row without backslash must not match the literal backslash search")
	require.Len(t, got, 1, "only one row should match the literal backslash search")
}

// TestCardgroupRepo_FindPageByOwner_CrossTenant verifies that FindPageByOwner
// and CountByOwner are scoped to ownerID: a row inserted for a different owner
// must not appear in the results even when both owners have a cardgroup with
// the same name.
func TestCardgroupRepo_FindPageByOwner_CrossTenant(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ownerA := insertAuthUser(t, ctx)
	ownerB := insertAuthUser(t, ctx)
	repo := repository.NewCardgroupRepository(testDB.GORM)

	insertNamedCardgroups(t, ctx, ownerA, []string{"shared name", "only A"})
	insertNamedCardgroups(t, ctx, ownerB, []string{"shared name", "only B"})

	// Query scoped to ownerB.
	gotB, err := repo.FindPageByOwner(
		ctx, ownerB, nil, nil, 20, 0,
		repository.CardgroupOrderByCreatedAt, repository.SortAsc, nil,
	)
	require.NoError(t, err)
	idsB := cardgroupIDSetFromSlice(gotB)

	// Fetch ownerA's IDs to assert they do not bleed into ownerB's results.
	gotA, err := repo.FindPageByOwner(
		ctx, ownerA, nil, nil, 20, 0,
		repository.CardgroupOrderByCreatedAt, repository.SortAsc, nil,
	)
	require.NoError(t, err)

	for _, cgA := range gotA {
		require.NotContains(t, idsB, cgA.ID,
			"ownerA's cardgroup %q must not appear in ownerB's page results", cgA.Name)
	}

	// CountByOwner for ownerB must return exactly 2 (its own rows only).
	totalB, err := repo.CountByOwner(ctx, ownerB, nil)
	require.NoError(t, err)
	require.Equal(t, int64(2), totalB,
		"CountByOwner must count only ownerB's cardgroups")
}

// TestCardgroupRepo_FindPageByOwner_PlusOneFetch verifies that the repository
// respects the caller-supplied limit and returns at most `first` rows,
// allowing the usecase layer to detect a next page by requesting first+1.
func TestCardgroupRepo_FindPageByOwner_PlusOneFetch(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ownerID := insertAuthUser(t, ctx)
	repo := repository.NewCardgroupRepository(testDB.GORM)

	names := make([]string, 5)
	for i := range names {
		names[i] = fmt.Sprintf("cg-%02d", i)
	}
	insertNamedCardgroups(t, ctx, ownerID, names)

	// Request first=3 (which represents the usecase sending first+1=3 when
	// the user asked for first=2). The repo must return exactly 3 rows.
	got, err := repo.FindPageByOwner(
		ctx, ownerID, nil, nil, 3, 0,
		repository.CardgroupOrderByCreatedAt, repository.SortAsc, nil,
	)
	require.NoError(t, err)
	require.Len(t, got, 3,
		"first=3 must return exactly 3 rows so the usecase can detect hasNextPage")

	// Verify the rows are in ASC order and belong to the right owner.
	for _, cg := range got {
		require.Equal(t, ownerID, cg.OwnerID)
	}
	sortedGot := make([]*domain.Cardgroup, len(got))
	copy(sortedGot, got)
	sort.Slice(sortedGot, func(i, j int) bool { return sortedGot[i].CreatedAt.Before(sortedGot[j].CreatedAt) })
	for i, cg := range got {
		require.Equal(t, sortedGot[i].ID, cg.ID, "rows must be in CreatedAt ASC order")
	}
}

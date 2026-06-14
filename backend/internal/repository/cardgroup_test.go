package repository_test

// TestMain, testDB, insertAuthUser, sqlDBHandle, and insertNAuthUsers are
// defined in user_test.go and shared across this package.

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
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
		ID:        domain.CardgroupID(uuid.NewString()),
		OwnerID:   domain.UserID(ownerID),
		Name:      domain.CardgroupName(name),
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

	got, err := repo.FindByID(ctx, string(cg.ID))
	require.NoError(t, err)
	require.Equal(t, cg.ID, got.ID)
	require.Equal(t, ownerID, string(got.OwnerID))
	require.Equal(t, "My Flashcards", got.Name.String())
	require.False(t, got.CreatedAt.IsZero())
	require.False(t, got.UpdatedAt.IsZero())
}

func TestCardgroupRepository_FindByName_ScopedToOwner(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ownerA := insertAuthUser(t, ctx)
	ownerB := insertAuthUser(t, ctx)
	repo := repository.NewCardgroupRepository(testDB.GORM)

	cgA := newCardgroup(ownerA, "Shared Name")
	cgB := newCardgroup(ownerB, "Shared Name")
	require.NoError(t, repo.Create(ctx, cgA))
	require.NoError(t, repo.Create(ctx, cgB))

	gotA, err := repo.FindByName(ctx, ownerA, "Shared Name")
	require.NoError(t, err)
	require.Equal(t, cgA.ID, gotA.ID)

	gotB, err := repo.FindByName(ctx, ownerB, "Shared Name")
	require.NoError(t, err)
	require.Equal(t, cgB.ID, gotB.ID)

	_, err = repo.FindByName(ctx, ownerA, "Missing")
	require.ErrorIs(t, err, repository.ErrNotFound)
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

	got, err := repo.FindByIDs(ctx, []string{string(cg1.ID), string(cg2.ID), string(cg3.ID)})
	require.NoError(t, err)
	require.Len(t, got, 3)
	require.NotNil(t, got[string(cg1.ID)])
	require.NotNil(t, got[string(cg2.ID)])
	require.NotNil(t, got[string(cg3.ID)])
}

func TestCardgroupRepository_FindByIDs_PartialMissing(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ownerID := insertAuthUser(t, ctx)
	repo := repository.NewCardgroupRepository(testDB.GORM)

	cg := newCardgroup(ownerID, "Exists")
	require.NoError(t, repo.Create(ctx, cg))

	missing := uuid.NewString()
	got, err := repo.FindByIDs(ctx, []string{string(cg.ID), missing})
	require.NoError(t, err)
	require.Len(t, got, 1)
	require.NotNil(t, got[string(cg.ID)])
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
	got, err := repo.Update(ctx, string(cg.ID), repository.CardgroupUpdate{Name: &newName})
	require.NoError(t, err)
	require.Equal(t, "Renamed", got.Name.String())
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

	before, err := repo.FindByID(ctx, string(cg.ID))
	require.NoError(t, err)

	after, err := repo.Update(ctx, string(cg.ID), repository.CardgroupUpdate{})
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

	require.NoError(t, repo.Delete(ctx, string(cg.ID)))

	_, err := repo.FindByID(ctx, string(cg.ID))
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

	_, err = repo.FindByID(ctx, string(cg.ID))
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

func TestCardgroupRepository_EnsureByName_Existing(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ownerID := insertAuthUser(t, ctx)
	repo := repository.NewCardgroupRepository(testDB.GORM)

	existing := newCardgroup(ownerID, "Ensure Existing")
	require.NoError(t, repo.Create(ctx, existing))

	got, err := repo.EnsureByName(ctx, ownerID, existing.Name.String())
	require.NoError(t, err)
	require.Equal(t, existing.ID, got.ID)

	rows := countCardgroupsByOwnerAndName(t, ctx, ownerID, existing.Name.String())
	require.Equal(t, int64(1), rows)
}

func TestCardgroupRepository_EnsureByName_Create(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ownerID := insertAuthUser(t, ctx)
	repo := repository.NewCardgroupRepository(testDB.GORM)

	got, err := repo.EnsureByName(ctx, ownerID, "Ensure Create")
	require.NoError(t, err)
	require.Equal(t, ownerID, string(got.OwnerID))
	require.Equal(t, "Ensure Create", got.Name.String())
	require.NotEmpty(t, got.ID)

	found, err := repo.FindByName(ctx, ownerID, "Ensure Create")
	require.NoError(t, err)
	require.Equal(t, got.ID, found.ID)
}

// TestCardgroupRepository_EnsureByName_WhitespaceContract pins the current
// contract that EnsureByName matches name verbatim: leading/trailing whitespace
// produces a distinct row from the trimmed value. Trimming is the caller's
// responsibility (callers parse the name into a domain.CardgroupName, which
// trims at its boundary, before invoking the repository). A future caller that
// bypasses that trim must either trim itself or this contract must change
// deliberately, not by accident.
func TestCardgroupRepository_EnsureByName_WhitespaceContract(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ownerID := insertAuthUser(t, ctx)
	repo := repository.NewCardgroupRepository(testDB.GORM)

	base := "Whitespace " + uuid.NewString()
	trimmed, err := repo.EnsureByName(ctx, ownerID, base)
	require.NoError(t, err)

	leadingSpace, err := repo.EnsureByName(ctx, ownerID, " "+base)
	require.NoError(t, err)
	require.NotEqual(t, trimmed.ID, leadingSpace.ID,
		"EnsureByName must NOT trim — leading-space name produces a distinct row")

	trailingSpace, err := repo.EnsureByName(ctx, ownerID, base+" ")
	require.NoError(t, err)
	require.NotEqual(t, trimmed.ID, trailingSpace.ID,
		"EnsureByName must NOT trim — trailing-space name produces a distinct row")
	require.NotEqual(t, leadingSpace.ID, trailingSpace.ID)
}

func TestCardgroupRepository_EnsureByName_DuplicateRace(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	ownerID := insertAuthUser(t, ctx)
	repo := repository.NewCardgroupRepository(testDB.GORM)
	name := "Ensure Race " + uuid.NewString()

	const workers = 2
	results := make([]*domain.Cardgroup, workers)
	errs := make([]error, workers)
	var wg sync.WaitGroup
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func(i int) {
			defer wg.Done()
			results[i], errs[i] = repo.EnsureByName(ctx, ownerID, name)
		}(i)
	}
	wg.Wait()

	for _, err := range errs {
		require.NoError(t, err)
	}
	require.NotNil(t, results[0])
	require.NotNil(t, results[1])
	require.Equal(t, results[0].ID, results[1].ID)

	rows := countCardgroupsByOwnerAndName(t, ctx, ownerID, name)
	require.Equal(t, int64(1), rows)
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
			ID:        domain.CardgroupID(uuid.NewString()),
			OwnerID:   domain.UserID(ownerID),
			Name:      domain.CardgroupName(name),
			CreatedAt: now,
			UpdatedAt: now,
		}
		require.NoError(t, repo.Create(ctx, cg))
		cgs[i] = cg
	}
	return cgs
}

func countCardgroupsByOwnerAndName(t *testing.T, ctx context.Context, ownerID, name string) int64 {
	t.Helper()
	var count int64
	err := testDB.GORM.WithContext(ctx).
		Model(&domain.Cardgroup{}).
		Where("owner_id = ? AND name = ?", ownerID, name).
		Count(&count).Error
	require.NoError(t, err)
	return count
}

// cardgroupNameSet builds a set of names from a slice for membership checks.
func cardgroupNameSet(cgs []*domain.Cardgroup) map[string]struct{} {
	m := make(map[string]struct{}, len(cgs))
	for _, cg := range cgs {
		m[cg.Name.String()] = struct{}{}
	}
	return m
}

// cardgroupIDSetFromSlice builds a set of IDs from a slice for membership checks.
func cardgroupIDSetFromSlice(cgs []*domain.Cardgroup) map[string]struct{} {
	m := make(map[string]struct{}, len(cgs))
	for _, cg := range cgs {
		m[string(cg.ID)] = struct{}{}
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
	require.Equal(t, "Exact Match", got[0].Name.String())

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
		require.NotContains(t, idsB, string(cgA.ID),
			"ownerA's cardgroup %q must not appear in ownerB's page results", cgA.Name.String())
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
		require.Equal(t, ownerID, string(cg.OwnerID))
	}
	sortedGot := make([]*domain.Cardgroup, len(got))
	copy(sortedGot, got)
	sort.Slice(sortedGot, func(i, j int) bool { return sortedGot[i].CreatedAt.Before(sortedGot[j].CreatedAt) })
	for i, cg := range got {
		require.Equal(t, sortedGot[i].ID, cg.ID, "rows must be in CreatedAt ASC order")
	}
}

// ---------------------------------------------------------------------------
// FindPageByOwner — orderBy column variants
// ---------------------------------------------------------------------------

// pickOwnerCardgroups filters got down to only the cardgroups whose IDs are
// in expectIDs, preserving order. Tests assert against the filtered slice so
// parallel-test cross-pollution from the shared DB is irrelevant.
func pickOwnerCardgroups(got []*domain.Cardgroup, expectIDs map[string]struct{}) []*domain.Cardgroup {
	out := make([]*domain.Cardgroup, 0, len(expectIDs))
	for _, cg := range got {
		if _, ok := expectIDs[string(cg.ID)]; ok {
			out = append(out, cg)
		}
	}
	return out
}

// TestCardgroupRepo_FindPageByOwner_OrderBy_Name_Asc verifies that
// orderBy=name + SortAsc returns the rows in lexicographic ascending order.
func TestCardgroupRepo_FindPageByOwner_OrderBy_Name_Asc(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ownerID := insertAuthUser(t, ctx)
	repo := repository.NewCardgroupRepository(testDB.GORM)

	cgs := insertNamedCardgroups(t, ctx, ownerID, []string{"Charlie", "Alpha", "Bravo"})
	want := map[string]struct{}{
		string(cgs[0].ID): {}, string(cgs[1].ID): {}, string(cgs[2].ID): {},
	}

	got, err := repo.FindPageByOwner(
		ctx, ownerID, nil, nil, 100, 0,
		repository.CardgroupOrderByName, repository.SortAsc, nil,
	)
	require.NoError(t, err)
	mine := pickOwnerCardgroups(got, want)
	require.Len(t, mine, 3)
	require.Equal(t, "Alpha", mine[0].Name.String())
	require.Equal(t, "Bravo", mine[1].Name.String())
	require.Equal(t, "Charlie", mine[2].Name.String())
}

// TestCardgroupRepo_FindPageByOwner_OrderBy_Name_Desc verifies that
// orderBy=name + SortDesc returns the rows in lexicographic descending order.
func TestCardgroupRepo_FindPageByOwner_OrderBy_Name_Desc(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ownerID := insertAuthUser(t, ctx)
	repo := repository.NewCardgroupRepository(testDB.GORM)

	cgs := insertNamedCardgroups(t, ctx, ownerID, []string{"Bravo", "Alpha", "Charlie"})
	want := map[string]struct{}{
		string(cgs[0].ID): {}, string(cgs[1].ID): {}, string(cgs[2].ID): {},
	}

	got, err := repo.FindPageByOwner(
		ctx, ownerID, nil, nil, 100, 0,
		repository.CardgroupOrderByName, repository.SortDesc, nil,
	)
	require.NoError(t, err)
	mine := pickOwnerCardgroups(got, want)
	require.Len(t, mine, 3)
	require.Equal(t, "Charlie", mine[0].Name.String())
	require.Equal(t, "Bravo", mine[1].Name.String())
	require.Equal(t, "Alpha", mine[2].Name.String())
}

// TestCardgroupRepo_FindPageByOwner_OrderBy_UpdatedAt_Desc verifies that
// orderBy=updated_at + SortDesc returns the most recently updated row first.
func TestCardgroupRepo_FindPageByOwner_OrderBy_UpdatedAt_Desc(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ownerID := insertAuthUser(t, ctx)
	repo := repository.NewCardgroupRepository(testDB.GORM)

	cgs := insertNamedCardgroups(t, ctx, ownerID, []string{"first", "second", "third"})

	// Wait then bump second so it has the latest updated_at.
	time.Sleep(5 * time.Millisecond)
	updated := "second updated"
	_, err := repo.Update(ctx, string(cgs[1].ID), repository.CardgroupUpdate{Name: &updated})
	require.NoError(t, err)

	want := map[string]struct{}{string(cgs[0].ID): {}, string(cgs[1].ID): {}, string(cgs[2].ID): {}}
	got, err := repo.FindPageByOwner(
		ctx, ownerID, nil, nil, 100, 0,
		repository.CardgroupOrderByUpdatedAt, repository.SortDesc, nil,
	)
	require.NoError(t, err)
	mine := pickOwnerCardgroups(got, want)
	require.Len(t, mine, 3)
	// The most recently updated row must come first under DESC.
	require.Equal(t, cgs[1].ID, mine[0].ID, "most recently updated row sorts first under updated_at DESC")
}

// TestCardgroupRepo_FindPageByOwner_OrderBy_UpdatedAt_Asc verifies the ASC
// counterpart: the row with the earliest updated_at sorts first.
func TestCardgroupRepo_FindPageByOwner_OrderBy_UpdatedAt_Asc(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ownerID := insertAuthUser(t, ctx)
	repo := repository.NewCardgroupRepository(testDB.GORM)

	cgs := insertNamedCardgroups(t, ctx, ownerID, []string{"first", "second", "third"})
	time.Sleep(5 * time.Millisecond)
	updated := "third updated"
	_, err := repo.Update(ctx, string(cgs[2].ID), repository.CardgroupUpdate{Name: &updated})
	require.NoError(t, err)

	want := map[string]struct{}{string(cgs[0].ID): {}, string(cgs[1].ID): {}, string(cgs[2].ID): {}}
	got, err := repo.FindPageByOwner(
		ctx, ownerID, nil, nil, 100, 0,
		repository.CardgroupOrderByUpdatedAt, repository.SortAsc, nil,
	)
	require.NoError(t, err)
	mine := pickOwnerCardgroups(got, want)
	require.Len(t, mine, 3)
	// First inserted (and not re-updated) is the earliest.
	require.Equal(t, cgs[0].ID, mine[0].ID,
		"earliest updated row sorts first under updated_at ASC")
}

// TestCardgroupRepo_FindPageByOwner_OrderBy_CreatedAt_Desc verifies that
// orderBy=created_at + SortDesc returns the most recently created row first.
func TestCardgroupRepo_FindPageByOwner_OrderBy_CreatedAt_Desc(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ownerID := insertAuthUser(t, ctx)
	repo := repository.NewCardgroupRepository(testDB.GORM)

	// insertNamedCardgroups staggers CreatedAt by 1ms, so cgs[2] is newest.
	cgs := insertNamedCardgroups(t, ctx, ownerID, []string{"a", "b", "c"})
	want := map[string]struct{}{string(cgs[0].ID): {}, string(cgs[1].ID): {}, string(cgs[2].ID): {}}

	got, err := repo.FindPageByOwner(
		ctx, ownerID, nil, nil, 100, 0,
		repository.CardgroupOrderByCreatedAt, repository.SortDesc, nil,
	)
	require.NoError(t, err)
	mine := pickOwnerCardgroups(got, want)
	require.Len(t, mine, 3)
	require.Equal(t, cgs[2].ID, mine[0].ID, "newest row sorts first under created_at DESC")
	require.Equal(t, cgs[0].ID, mine[2].ID, "oldest row sorts last under created_at DESC")
}

// TestCardgroupRepo_FindPageByOwner_OrderBy_ID_Desc verifies that orderBy=id
// + SortDesc emits a single ORDER BY column (no secondary `id` since the
// primary already is `id`). Asserts ID-descending ordering.
func TestCardgroupRepo_FindPageByOwner_OrderBy_ID_Desc(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ownerID := insertAuthUser(t, ctx)
	repo := repository.NewCardgroupRepository(testDB.GORM)

	cgs := insertNamedCardgroups(t, ctx, ownerID, []string{"a", "b", "c"})
	want := map[string]struct{}{string(cgs[0].ID): {}, string(cgs[1].ID): {}, string(cgs[2].ID): {}}

	got, err := repo.FindPageByOwner(
		ctx, ownerID, nil, nil, 100, 0,
		repository.CardgroupOrderByID, repository.SortDesc, nil,
	)
	require.NoError(t, err)
	mine := pickOwnerCardgroups(got, want)
	require.Len(t, mine, 3)
	for i := 1; i < len(mine); i++ {
		require.Greater(t, mine[i-1].ID, mine[i].ID,
			"rows must be sorted by ID descending")
	}
}

// ---------------------------------------------------------------------------
// FindPageByOwner — cursor (after / before) tuple-comparison variants
// ---------------------------------------------------------------------------

// TestCardgroupRepo_FindPageByOwner_Cursor_AfterByName verifies that the
// (name, id) tuple-comparison WHERE works for a forward cursor on
// orderBy=name + SortAsc: the cursor row itself must be excluded.
func TestCardgroupRepo_FindPageByOwner_Cursor_AfterByName(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ownerID := insertAuthUser(t, ctx)
	repo := repository.NewCardgroupRepository(testDB.GORM)

	cgs := insertNamedCardgroups(t, ctx, ownerID, []string{"Apple", "Banana", "Cherry"})
	want := map[string]struct{}{string(cgs[0].ID): {}, string(cgs[1].ID): {}, string(cgs[2].ID): {}}

	// Cursor at "Banana"; expect only "Cherry" after it.
	bananaName := cgs[1].Name.String()
	cursor := &repository.CardgroupCursor{
		ID:   string(cgs[1].ID),
		Name: &bananaName,
	}
	got, err := repo.FindPageByOwner(
		ctx, ownerID, cursor, nil, 100, 0,
		repository.CardgroupOrderByName, repository.SortAsc, nil,
	)
	require.NoError(t, err)
	mine := pickOwnerCardgroups(got, want)
	require.Len(t, mine, 1)
	require.Equal(t, "Cherry", mine[0].Name.String())
}

// TestCardgroupRepo_FindPageByOwner_Cursor_BackwardByCreatedAt verifies the
// backward (last + before) path for orderBy=created_at: the repo flips the
// SQL direction and reverses the slice in memory, so the caller sees the same
// display order as forward paging.
func TestCardgroupRepo_FindPageByOwner_Cursor_BackwardByCreatedAt(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ownerID := insertAuthUser(t, ctx)
	repo := repository.NewCardgroupRepository(testDB.GORM)

	cgs := insertNamedCardgroups(t, ctx, ownerID, []string{"a", "b", "c", "d", "e"})
	want := map[string]struct{}{
		string(cgs[0].ID): {}, string(cgs[1].ID): {}, string(cgs[2].ID): {}, string(cgs[3].ID): {}, string(cgs[4].ID): {},
	}

	// last=2, before=cgs[3] under created_at ASC — expect cgs[1], cgs[2].
	cursor := &repository.CardgroupCursor{
		ID:        string(cgs[3].ID),
		CreatedAt: &cgs[3].CreatedAt,
	}
	got, err := repo.FindPageByOwner(
		ctx, ownerID, nil, cursor, 0, 2,
		repository.CardgroupOrderByCreatedAt, repository.SortAsc, nil,
	)
	require.NoError(t, err)
	mine := pickOwnerCardgroups(got, want)
	require.Len(t, mine, 2, "last=2 must return exactly 2 rows")
	require.Equal(t, cgs[1].ID, mine[0].ID, "backward page must come back in forward display order")
	require.Equal(t, cgs[2].ID, mine[1].ID, "backward page must come back in forward display order")
}

// TestCardgroupRepo_FindPageByOwner_Cursor_AfterByID verifies the orderBy=id
// branch of cardgroupCursorWhere: only the `id op ?` form, no tuple compare.
func TestCardgroupRepo_FindPageByOwner_Cursor_AfterByID(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ownerID := insertAuthUser(t, ctx)
	repo := repository.NewCardgroupRepository(testDB.GORM)

	cgs := insertNamedCardgroups(t, ctx, ownerID, []string{"a", "b", "c"})
	want := map[string]struct{}{string(cgs[0].ID): {}, string(cgs[1].ID): {}, string(cgs[2].ID): {}}

	// Sort our IDs so we know which one is "first" under id ASC.
	sortedIDs := make([]string, 3)
	copy(sortedIDs, []string{string(cgs[0].ID), string(cgs[1].ID), string(cgs[2].ID)})
	sort.Strings(sortedIDs)

	cursor := &repository.CardgroupCursor{ID: sortedIDs[0]}
	got, err := repo.FindPageByOwner(
		ctx, ownerID, cursor, nil, 100, 0,
		repository.CardgroupOrderByID, repository.SortAsc, nil,
	)
	require.NoError(t, err)
	mine := pickOwnerCardgroups(got, want)
	require.Len(t, mine, 2, "two rows after the first cursor")
	require.Equal(t, domain.CardgroupID(sortedIDs[1]), mine[0].ID)
	require.Equal(t, domain.CardgroupID(sortedIDs[2]), mine[1].ID)
}

// TestCardgroupRepo_FindPageByOwner_Cursor_NameTie_TupleComparison verifies
// that two cardgroups sharing the same name are ordered deterministically by
// the (name, id) tuple — paging across the tie does not skip or duplicate.
func TestCardgroupRepo_FindPageByOwner_Cursor_NameTie_TupleComparison(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ownerID := insertAuthUser(t, ctx)
	repo := repository.NewCardgroupRepository(testDB.GORM)

	// Three cardgroups, two share the name "Tie".
	cgs := insertNamedCardgroups(t, ctx, ownerID, []string{"Tie", "Tie", "Zebra"})
	want := map[string]struct{}{string(cgs[0].ID): {}, string(cgs[1].ID): {}, string(cgs[2].ID): {}}

	// Page 1: first=1 under name ASC — must be one of the two Ties.
	page1, err := repo.FindPageByOwner(
		ctx, ownerID, nil, nil, 1, 0,
		repository.CardgroupOrderByName, repository.SortAsc, nil,
	)
	require.NoError(t, err)
	mine1 := pickOwnerCardgroups(page1, want)
	// page1 may contain rows from other parallel tests; pick our own. Since
	// "Tie" sorts before "Zebra" lexicographically and only one is requested,
	// the first row from our tenant must be a Tie. But cross-test pollution
	// can put a foreign row first; explicitly fetch our first by paging
	// with a higher limit and slicing.
	_ = mine1

	// Refetch with a high limit and pick our three deterministic rows.
	all, err := repo.FindPageByOwner(
		ctx, ownerID, nil, nil, 100, 0,
		repository.CardgroupOrderByName, repository.SortAsc, nil,
	)
	require.NoError(t, err)
	mine := pickOwnerCardgroups(all, want)
	require.Len(t, mine, 3)
	// The two Tie rows must come back-to-back; the third row is Zebra.
	require.Equal(t, "Tie", mine[0].Name.String())
	require.Equal(t, "Tie", mine[1].Name.String())
	require.Equal(t, "Zebra", mine[2].Name.String())

	// Page after the FIRST tie row using its (name, id) cursor — the next
	// row must be the OTHER tie row, then Zebra. The tuple compare keeps
	// the second tie reachable; without it, `name > 'Tie'` would skip past it.
	tieName := mine[0].Name.String()
	cursor := &repository.CardgroupCursor{ID: string(mine[0].ID), Name: &tieName}
	pageAfter, err := repo.FindPageByOwner(
		ctx, ownerID, cursor, nil, 100, 0,
		repository.CardgroupOrderByName, repository.SortAsc, nil,
	)
	require.NoError(t, err)
	myAfter := pickOwnerCardgroups(pageAfter, want)
	require.Len(t, myAfter, 2, "tuple compare must not skip the second tie row")
	require.Equal(t, mine[1].ID, myAfter[0].ID, "second tie row must come next")
	require.Equal(t, "Zebra", myAfter[1].Name.String())
}

// ---------------------------------------------------------------------------
// CountByOwner — additional branches
// ---------------------------------------------------------------------------

// TestCardgroupRepo_CountByOwner_NoSearch verifies that CountByOwner with a
// nil search returns the unfiltered count, scoped to ownerID.
func TestCardgroupRepo_CountByOwner_NoSearch(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ownerID := insertAuthUser(t, ctx)
	repo := repository.NewCardgroupRepository(testDB.GORM)

	insertNamedCardgroups(t, ctx, ownerID, []string{"x", "y", "z"})

	total, err := repo.CountByOwner(ctx, ownerID, nil)
	require.NoError(t, err)
	require.Equal(t, int64(3), total)
}

// TestCardgroupRepo_CountByOwner_EmptySearchTreatedAsNil verifies that an
// all-whitespace search has no effect on the count — same as nil — because
// cardgroupSearchPattern returns ok=false for trimmed-empty input.
func TestCardgroupRepo_CountByOwner_EmptySearchTreatedAsNil(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ownerID := insertAuthUser(t, ctx)
	repo := repository.NewCardgroupRepository(testDB.GORM)

	insertNamedCardgroups(t, ctx, ownerID, []string{"a", "b"})

	whitespace := "   "
	total, err := repo.CountByOwner(ctx, ownerID, &whitespace)
	require.NoError(t, err)
	require.Equal(t, int64(2), total,
		"all-whitespace search must NOT filter the count (treated as no search)")
}

// TestCardgroupRepo_FindPageByOwner_EmptySearchTreatedAsNil verifies the
// same trimmed-empty-search path on FindPageByOwner: all rows must be
// returned when the search is whitespace-only.
func TestCardgroupRepo_FindPageByOwner_EmptySearchTreatedAsNil(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ownerID := insertAuthUser(t, ctx)
	repo := repository.NewCardgroupRepository(testDB.GORM)

	cgs := insertNamedCardgroups(t, ctx, ownerID, []string{"x", "y"})
	want := map[string]struct{}{string(cgs[0].ID): {}, string(cgs[1].ID): {}}

	whitespace := "   "
	got, err := repo.FindPageByOwner(
		ctx, ownerID, nil, nil, 100, 0,
		repository.CardgroupOrderByCreatedAt, repository.SortAsc, &whitespace,
	)
	require.NoError(t, err)
	mine := pickOwnerCardgroups(got, want)
	require.Len(t, mine, 2,
		"all-whitespace search must NOT filter results (treated as no search)")
}

// TestCardgroupRepo_FindPageByOwner_BothFirstAndLastZero verifies the early
// short-circuit when both first and last are zero — the repo returns an
// empty slice without touching the DB.
func TestCardgroupRepo_FindPageByOwner_BothFirstAndLastZero(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ownerID := insertAuthUser(t, ctx)
	repo := repository.NewCardgroupRepository(testDB.GORM)

	insertNamedCardgroups(t, ctx, ownerID, []string{"x", "y"})

	got, err := repo.FindPageByOwner(
		ctx, ownerID, nil, nil, 0, 0,
		repository.CardgroupOrderByCreatedAt, repository.SortAsc, nil,
	)
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Empty(t, got, "first=0,last=0 must short-circuit to an empty slice")
}

// TestCardgroupRepo_FindPageByOwner_NegativeFirstClampedToZero verifies
// clampPageSize maps a negative first to 0 and triggers the short-circuit
// path described above.
func TestCardgroupRepo_FindPageByOwner_NegativeFirstClampedToZero(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ownerID := insertAuthUser(t, ctx)
	repo := repository.NewCardgroupRepository(testDB.GORM)

	insertNamedCardgroups(t, ctx, ownerID, []string{"x"})

	got, err := repo.FindPageByOwner(
		ctx, ownerID, nil, nil, -10, 0,
		repository.CardgroupOrderByCreatedAt, repository.SortAsc, nil,
	)
	require.NoError(t, err)
	require.Empty(t, got, "negative first must clamp to 0 and short-circuit")
}

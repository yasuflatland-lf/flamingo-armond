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
	cg.UpdatedAt = time.Unix(1, 0).UTC()
	require.NoError(t, repo.Create(ctx, cg))

	got, err := repo.FindByID(ctx, string(cg.ID))
	require.NoError(t, err)
	require.Equal(t, cg.ID, got.ID)
	require.Equal(t, ownerID, string(got.OwnerID))
	require.Equal(t, "My Flashcards", got.Name.String())
	require.False(t, got.CreatedAt.IsZero())
	require.False(t, got.UpdatedAt.IsZero())
	require.Equal(t, cg.UpdatedAt, got.UpdatedAt,
		"Create must copy the database-assigned updated_at back into the aggregate")
	require.NotEqual(t, time.Unix(1, 0).UTC(), cg.UpdatedAt)
}

// TestCardgroupRepository_Create_DeletedOwner_ReturnsOwnerNotFound exercises the
// real 23503 path end-to-end: the owner's auth.users row is deleted (cascading
// public.users away) while a still-valid JWT would keep authenticating them, so
// the insert violates cardgroups_owner_id_fkey. The classification must surface
// as ErrCardgroupOwnerNotFound rather than an opaque wrapped error.
func TestCardgroupRepository_Create_DeletedOwner_ReturnsOwnerNotFound(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ownerID := insertAuthUser(t, ctx)
	repo := repository.NewCardgroupRepository(testDB.GORM)

	userRepo := repository.NewUserRepository(testDB.GORM)
	require.NoError(t, deleteAuthUserInTx(ctx, userRepo, ownerID))

	err := repo.Create(ctx, newCardgroup(ownerID, "Deck for a deleted account"))
	require.ErrorIs(t, err, repository.ErrCardgroupOwnerNotFound)
	// Standalone sentinel: a missing owner must not read as a missing cardgroup.
	require.NotErrorIs(t, err, repository.ErrNotFound)
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
	before, err := repo.FindByID(ctx, string(cg.ID))
	require.NoError(t, err)

	time.Sleep(5 * time.Millisecond)

	newName := "Renamed"
	got, err := repo.Update(ctx, string(cg.ID), repository.CardgroupUpdate{Name: &newName})
	require.NoError(t, err)
	require.Equal(t, "Renamed", got.Name.String())
	require.True(t, got.UpdatedAt.After(before.UpdatedAt),
		"updated_at should advance from the post-Create database value")
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

	// 2001 ASCII characters: exceeds the CHECK constraint, whose upper bound is
	// 20x the 100-grapheme domain cap so a ZWJ-emoji name at the domain cap still
	// fits. The domain (ParseCardgroupName) rejects anything past 100 graphemes
	// long before this bound; the CHECK is a storage backstop.
	longName := strings.Repeat("a", 2001)
	cg := newCardgroup(ownerID, longName)

	err := repo.Create(ctx, cg)
	require.Error(t, err, "DB CHECK constraint should reject a 2001-char name")
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
		}
		require.NoError(t, repo.Create(ctx, cg))
		persisted, err := repo.FindByID(ctx, string(cg.ID))
		require.NoError(t, err)
		cgs[i] = persisted
	}
	return cgs
}

// sortCardgroupsByUpdatedAt returns a copy of cgs in the connection's fixed
// (updated_at DESC, id DESC) order.
func sortCardgroupsByUpdatedAt(cgs []*domain.Cardgroup) []*domain.Cardgroup {
	sorted := append([]*domain.Cardgroup(nil), cgs...)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].UpdatedAt.Equal(sorted[j].UpdatedAt) {
			return sorted[i].ID > sorted[j].ID
		}
		return sorted[i].UpdatedAt.After(sorted[j].UpdatedAt)
	})
	return sorted
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

	got, _, err := repo.FindPageByOwner(
		ctx, ownerID, nil, 10, nil,
	)
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Empty(t, got)

	total, err := repo.CountByOwner(ctx, ownerID)
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
	got, _, err := repo.FindPageByOwner(
		ctx, ownerID, nil, 10, &search,
	)
	require.NoError(t, err)
	require.Len(t, got, 1, "exactly one group should match the search term")
	require.Equal(t, "Exact Match", got[0].Name.String())
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
	got, total, err := repo.FindPageByOwner(
		ctx, ownerID, nil, 10, &search,
	)
	require.NoError(t, err)
	names := cardgroupNameSet(got)
	require.Contains(t, names, "green apple", "partial match should hit 'green apple'")
	require.Contains(t, names, "apple pie", "partial match should hit 'apple pie'")
	require.NotContains(t, names, "banana", "banana must not appear in apple search results")
	require.Len(t, got, 2)

	// The folded totalCount must reflect the active search filter (2 of the 3
	// seeded rows match "apple"), not the unfiltered owner total.
	require.Equal(t, int64(2), total,
		"totalCount from FindPageByOwner must honour the search filter, not the unfiltered total")

	unfiltered, err := repo.CountByOwner(ctx, ownerID)
	require.NoError(t, err)
	require.Equal(t, int64(3), unfiltered, "sanity: the owner really has 3 cardgroups")
	require.NotEqual(t, unfiltered, total,
		"filtered total must differ from the unfiltered total to prove the filter is applied")
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
	got, _, err := repo.FindPageByOwner(
		ctx, ownerID, nil, 10, &search,
	)
	require.NoError(t, err)
	names := cardgroupNameSet(got)
	// "100%" must be found because its name literally contains "100%".
	require.Contains(t, names, "100%", "literal percent must match the row named '100%%'")
	// "1000" must NOT be found — without escaping, '%' would be a wildcard and
	// the pattern "%%100%%" would match "1000" too.
	require.NotContains(t, names, "1000", "unescaped '%' would wrongly match '1000'; escaping must prevent that")
	require.Len(t, got, 1, "only one row should match the literal '100%%' search")
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
	got, _, err := repo.FindPageByOwner(
		ctx, ownerID, nil, 10, &search,
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
	got, _, err := repo.FindPageByOwner(
		ctx, ownerID, nil, 10, &search,
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
	gotB, _, err := repo.FindPageByOwner(
		ctx, ownerB, nil, 20, nil,
	)
	require.NoError(t, err)
	idsB := cardgroupIDSetFromSlice(gotB)

	// Fetch ownerA's IDs to assert they do not bleed into ownerB's results.
	gotA, _, err := repo.FindPageByOwner(
		ctx, ownerA, nil, 20, nil,
	)
	require.NoError(t, err)

	for _, cgA := range gotA {
		require.NotContains(t, idsB, string(cgA.ID),
			"ownerA's cardgroup %q must not appear in ownerB's page results", cgA.Name.String())
	}

	// CountByOwner for ownerB must return exactly 2 (its own rows only).
	totalB, err := repo.CountByOwner(ctx, ownerB)
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
	cgs := insertNamedCardgroups(t, ctx, ownerID, names)

	// Request first=3 (which represents the usecase sending first+1=3 when
	// the user asked for first=2). The repo must return exactly 3 rows.
	got, _, err := repo.FindPageByOwner(
		ctx, ownerID, nil, 3, nil,
	)
	require.NoError(t, err)
	require.Len(t, got, 3,
		"first=3 must return exactly 3 rows so the usecase can detect hasNextPage")

	// Verify the rows belong to the right owner and are the first three under
	// the fixed (updated_at DESC, id DESC) ordering.
	for _, cg := range got {
		require.Equal(t, ownerID, string(cg.OwnerID))
	}
	expected := sortCardgroupsByUpdatedAt(cgs)
	for i, cg := range got {
		require.Equal(t, expected[i].ID, cg.ID, "rows must be in updated_at DESC, id DESC order")
	}
}

// ---------------------------------------------------------------------------
// FindPageByOwner — fixed (updated_at DESC, id DESC) ordering
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

// TestCardgroupRepo_FindPageByOwner_UpdatedAt_Desc verifies the fixed ordering
// returns the most recently updated row first.
func TestCardgroupRepo_FindPageByOwner_UpdatedAt_Desc(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ownerID := insertAuthUser(t, ctx)
	repo := repository.NewCardgroupRepository(testDB.GORM)

	cgs := insertNamedCardgroups(t, ctx, ownerID, []string{"first", "second", "third"})

	// Wait then bump second so it has the latest updated_at.
	time.Sleep(5 * time.Millisecond)
	updated := "second updated"
	updatedCG, err := repo.Update(ctx, string(cgs[1].ID), repository.CardgroupUpdate{Name: &updated})
	require.NoError(t, err)
	cgs[1] = updatedCG

	want := map[string]struct{}{string(cgs[0].ID): {}, string(cgs[1].ID): {}, string(cgs[2].ID): {}}
	got, _, err := repo.FindPageByOwner(ctx, ownerID, nil, 100, nil)
	require.NoError(t, err)
	mine := pickOwnerCardgroups(got, want)
	require.Len(t, mine, 3)
	expected := sortCardgroupsByUpdatedAt(cgs)
	require.Equal(t, cgs[1].ID, expected[0].ID, "sanity: the bumped row has the latest updated_at")
	for i := range mine {
		require.Equal(t, expected[i].ID, mine[i].ID, "rows must be in updated_at DESC, id DESC order")
	}
}

// TestCardgroupRepo_FindPageByOwner_Cursor_UpdatedAtTie_TupleComparison verifies
// that two cardgroups sharing an updated_at fall back to id DESC, and that a
// cursor on the first tied row still reaches the second — `updated_at < ?` alone
// would skip it.
func TestCardgroupRepo_FindPageByOwner_Cursor_UpdatedAtTie_TupleComparison(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ownerID := insertAuthUser(t, ctx)
	repo := repository.NewCardgroupRepository(testDB.GORM)

	cgs := insertNamedCardgroups(t, ctx, ownerID, []string{"older", "tie-a", "tie-b"})
	// One statement runs in one transaction, so the trigger stamps both rows with
	// the same now() — a deterministic updated_at tie that is newer than "older".
	time.Sleep(5 * time.Millisecond)
	require.NoError(t, testDB.GORM.WithContext(ctx).Exec(
		"UPDATE cardgroups SET name = name WHERE id IN (?, ?)", string(cgs[1].ID), string(cgs[2].ID),
	).Error)
	want := map[string]struct{}{string(cgs[0].ID): {}, string(cgs[1].ID): {}, string(cgs[2].ID): {}}

	all, _, err := repo.FindPageByOwner(ctx, ownerID, nil, 100, nil)
	require.NoError(t, err)
	mine := pickOwnerCardgroups(all, want)
	require.Len(t, mine, 3)
	require.True(t, mine[0].UpdatedAt.Equal(mine[1].UpdatedAt), "sanity: the two tied rows share updated_at")
	require.Greater(t, mine[0].ID, mine[1].ID, "an updated_at tie falls back to id DESC")
	require.Equal(t, cgs[0].ID, mine[2].ID, "the untouched row has the oldest updated_at")

	ua := mine[0].UpdatedAt
	cursor := &repository.CardgroupCursor{ID: string(mine[0].ID), UpdatedAt: &ua}
	pageAfter, _, err := repo.FindPageByOwner(ctx, ownerID, cursor, 100, nil)
	require.NoError(t, err)
	myAfter := pickOwnerCardgroups(pageAfter, want)
	require.Len(t, myAfter, 2, "tuple compare must not skip the second tie row")
	require.Equal(t, mine[1].ID, myAfter[0].ID, "second tie row must come next")
	require.Equal(t, cgs[0].ID, myAfter[1].ID)
}

// ---------------------------------------------------------------------------
// CountByOwner
// ---------------------------------------------------------------------------

// TestCardgroupRepo_CountByOwner verifies that CountByOwner returns the
// owner's total cardgroup count, scoped to ownerID.
func TestCardgroupRepo_CountByOwner(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ownerID := insertAuthUser(t, ctx)
	repo := repository.NewCardgroupRepository(testDB.GORM)

	insertNamedCardgroups(t, ctx, ownerID, []string{"x", "y", "z"})

	total, err := repo.CountByOwner(ctx, ownerID)
	require.NoError(t, err)
	require.Equal(t, int64(3), total)
}

// TestCardgroupRepo_FindPageByOwner_EmptySearchTreatedAsNil verifies that a
// whitespace-only search does not filter FindPageByOwner: all rows are
// returned, because searchLikePattern returns ok=false for trimmed-empty input.
func TestCardgroupRepo_FindPageByOwner_EmptySearchTreatedAsNil(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ownerID := insertAuthUser(t, ctx)
	repo := repository.NewCardgroupRepository(testDB.GORM)

	cgs := insertNamedCardgroups(t, ctx, ownerID, []string{"x", "y"})
	want := map[string]struct{}{string(cgs[0].ID): {}, string(cgs[1].ID): {}}

	whitespace := "   "
	got, _, err := repo.FindPageByOwner(
		ctx, ownerID, nil, 100, &whitespace,
	)
	require.NoError(t, err)
	mine := pickOwnerCardgroups(got, want)
	require.Len(t, mine, 2,
		"all-whitespace search must NOT filter results (treated as no search)")
}

// TestCardgroupRepo_FindPageByOwner_FirstZero verifies the early
// short-circuit when first is zero — the repo returns an empty page slice
// after the count, without running the page query.
func TestCardgroupRepo_FindPageByOwner_FirstZero(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ownerID := insertAuthUser(t, ctx)
	repo := repository.NewCardgroupRepository(testDB.GORM)

	insertNamedCardgroups(t, ctx, ownerID, []string{"x", "y"})

	got, _, err := repo.FindPageByOwner(
		ctx, ownerID, nil, 0, nil,
	)
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Empty(t, got, "first=0 must short-circuit to an empty slice")
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

	got, _, err := repo.FindPageByOwner(
		ctx, ownerID, nil, -10, nil,
	)
	require.NoError(t, err)
	require.Empty(t, got, "negative first must clamp to 0 and short-circuit")
}

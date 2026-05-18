package repository_test

import (
	"context"
	"errors"
	"sort"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"backend/internal/domain"
	"backend/internal/repository"
)

// insertUserWithName inserts a fresh auth user (the handle_new_user trigger
// then creates the corresponding public.users row), backdates created_at via
// direct SQL so test cases can control ordering, and stamps the supplied
// display_name. Returns the seeded user id.
//
// The created_at update fires the BEFORE UPDATE trigger which refreshes
// updated_at to now(); that is harmless for these tests because they never
// assert on updated_at directly.
func insertUserWithName(t *testing.T, ctx context.Context, displayName string, createdAt time.Time) string {
	t.Helper()
	id := insertAuthUser(t, ctx)
	sqlDB := sqlDBHandle(t)
	_, err := sqlDB.ExecContext(ctx,
		`UPDATE public.users SET display_name = $1, created_at = $2 WHERE id = $3`,
		displayName, createdAt, id,
	)
	require.NoError(t, err, "stamp display_name + created_at")
	return id
}

// insertUserAt inserts an auth user and backdates created_at without touching
// display_name. Use when display_name does not matter for the assertion.
func insertUserAt(t *testing.T, ctx context.Context, createdAt time.Time) string {
	t.Helper()
	id := insertAuthUser(t, ctx)
	sqlDB := sqlDBHandle(t)
	_, err := sqlDB.ExecContext(ctx,
		`UPDATE public.users SET created_at = $1 WHERE id = $2`,
		createdAt, id,
	)
	require.NoError(t, err, "stamp created_at")
	return id
}

// fetchUserCreatedAt re-fetches a user so the test observes the DB-rounded
// timestamp; tuple comparisons must use that exact value.
func fetchUser(t *testing.T, ctx context.Context, repo repository.UserRepository, id string) *domain.User {
	t.Helper()
	u, err := repo.FindByID(ctx, id)
	require.NoError(t, err)
	return u
}

// expectedListOrder sorts the supplied users by (created_at DESC, id ASC) and
// returns their ids. Mirrors the SQL ORDER BY emitted by ListPage.
func expectedListOrder(users []*domain.User) []string {
	cp := make([]*domain.User, len(users))
	copy(cp, users)
	sort.SliceStable(cp, func(i, j int) bool {
		if !cp[i].CreatedAt.Equal(cp[j].CreatedAt) {
			return cp[i].CreatedAt.After(cp[j].CreatedAt)
		}
		return cp[i].ID < cp[j].ID
	})
	out := make([]string, len(cp))
	for i, u := range cp {
		out[i] = u.ID
	}
	return out
}

// TestUserPagination_EmptyTable verifies that ListPage on an empty result set
// returns total=0 and an empty (but non-nil) slice. The empty-table semantics
// also imply hasNextPage=false at the usecase layer because the +1 fetch
// returns 0 rows.
func TestUserPagination_EmptyTable(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := repository.NewUserRepository(testDB.GORM)

	// A unique nonsense search term ensures no other test's user matches.
	q := "no-such-user-xyz-" + uuid.NewString()
	users, total, err := repo.ListPage(ctx, nil, nil, 10, 0, &q)
	require.NoError(t, err)
	require.Equal(t, int64(0), total)
	require.NotNil(t, users)
	require.Empty(t, users)
}

// TestUserPagination_ForwardPageOne verifies the no-cursor forward path
// returns rows in (created_at DESC, id ASC) order.
func TestUserPagination_ForwardPageOne(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := repository.NewUserRepository(testDB.GORM)

	tag := "fwd1-" + uuid.NewString()
	now := time.Now().UTC().Truncate(time.Microsecond)
	// 5 users: tag-0 oldest, tag-4 newest. Stagger by 1 hour.
	ids := make([]string, 5)
	for i := 0; i < 5; i++ {
		ids[i] = insertUserWithName(t, ctx, tag+"-"+uuidShort(),
			now.Add(time.Duration(i)*time.Hour))
	}

	// Re-fetch so we observe DB-rounded timestamps.
	users := make([]*domain.User, 5)
	for i, id := range ids {
		users[i] = fetchUser(t, ctx, repo, id)
	}
	want := expectedListOrder(users)

	got, total, err := repo.ListPage(ctx, nil, nil, 3, 0, ptrStr(tag))
	require.NoError(t, err)
	require.Equal(t, int64(5), total)
	require.Len(t, got, 3)
	require.Equal(t, want[0], got[0].ID)
	require.Equal(t, want[1], got[1].ID)
	require.Equal(t, want[2], got[2].ID)
}

// TestUserPagination_ForwardPageTwo verifies that the cursor is exclusive:
// the cursor row itself does not appear in the next page.
func TestUserPagination_ForwardPageTwo(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := repository.NewUserRepository(testDB.GORM)

	tag := "fwd2-" + uuid.NewString()
	now := time.Now().UTC().Truncate(time.Microsecond)
	ids := make([]string, 5)
	for i := 0; i < 5; i++ {
		ids[i] = insertUserWithName(t, ctx, tag+"-"+uuidShort(),
			now.Add(time.Duration(i)*time.Hour))
	}

	users := make([]*domain.User, 5)
	for i, id := range ids {
		users[i] = fetchUser(t, ctx, repo, id)
	}
	want := expectedListOrder(users)

	// Page 1: first=2 → want[0..1].
	page1, _, err := repo.ListPage(ctx, nil, nil, 2, 0, ptrStr(tag))
	require.NoError(t, err)
	require.Len(t, page1, 2)
	require.Equal(t, want[0], page1[0].ID)
	require.Equal(t, want[1], page1[1].ID)

	// Page 2: after = last cursor of page 1, first=2 → want[2..3].
	cursor := page1[1].ID
	page2, _, err := repo.ListPage(ctx, &cursor, nil, 2, 0, ptrStr(tag))
	require.NoError(t, err)
	require.Len(t, page2, 2)
	require.Equal(t, want[2], page2[0].ID)
	require.Equal(t, want[3], page2[1].ID)
}

// TestUserPagination_BackwardBeforeCursor verifies that backward pagination
// returns the rows immediately preceding the cursor in the same display order
// as forward pagination.
func TestUserPagination_BackwardBeforeCursor(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := repository.NewUserRepository(testDB.GORM)

	tag := "bwd-" + uuid.NewString()
	now := time.Now().UTC().Truncate(time.Microsecond)
	ids := make([]string, 5)
	for i := 0; i < 5; i++ {
		ids[i] = insertUserWithName(t, ctx, tag+"-"+uuidShort(),
			now.Add(time.Duration(i)*time.Hour))
	}

	users := make([]*domain.User, 5)
	for i, id := range ids {
		users[i] = fetchUser(t, ctx, repo, id)
	}
	want := expectedListOrder(users)
	// want order: newest → oldest. want[0]=ids[4], want[4]=ids[0].

	// last=2 before=want[3] should return want[1..2] — the page immediately
	// before the cursor — in the same display order as forward.
	cursor := want[3]
	got, total, err := repo.ListPage(ctx, nil, &cursor, 0, 2, ptrStr(tag))
	require.NoError(t, err)
	require.Equal(t, int64(5), total)
	require.Len(t, got, 2)
	require.Equal(t, want[1], got[0].ID)
	require.Equal(t, want[2], got[1].ID)
}

// TestUserPagination_SearchSubstring verifies that ILIKE substring match is
// case-insensitive and that total reflects the filtered count.
//
// Layout: every row carries a unique per-test token chosen so it cannot
// appear in any other test's display_names (a uuid-derived hex string). The
// matching rows additionally contain a per-test "needle" letter sequence
// inside the body — in mixed case, to exercise the case-insensitive ILIKE.
// Rows that should not match leave the needle out.
//
// The search query is just the needle (no marker), so rows seeded by other
// tests cannot satisfy the match. The total count therefore reflects exactly
// the rows seeded here.
func TestUserPagination_SearchSubstring(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := repository.NewUserRepository(testDB.GORM)

	// `needle` is the per-test substring to search for. Use only letters
	// (no LIKE meta-chars) and append a uuid-hex tail so the substring is
	// globally unique across tests.
	needle := "Needle" + onlyHex(uuid.NewString())
	// `tag` is the suffix appended to every row in this test, used solely
	// to defend the assertions below if a future test happens to embed
	// `needle` by accident.
	tag := "Tag" + onlyHex(uuid.NewString())

	type row struct {
		body  string
		match bool
	}
	rows := []row{
		{"row1-" + needle + "-alice", true},             // contains needle
		{"row2-" + flipCase(needle) + "-bob", true},     // case-insensitive match
		{"row3-prefix" + needle + "suffix-carol", true}, // mid-string match
		{"row4-bob", false},                             // no needle
		{"row5-eve", false},                             // no needle
	}
	now := time.Now().UTC().Truncate(time.Microsecond)
	for i, r := range rows {
		insertUserWithName(t, ctx, r.body+tag, now.Add(time.Duration(i)*time.Hour))
	}

	got, total, err := repo.ListPage(ctx, nil, nil, 10, 0, &needle)
	require.NoError(t, err)
	require.Equal(t, int64(3), total)
	require.Len(t, got, 3)

	gotNames := map[string]bool{}
	for _, u := range got {
		require.NotNil(t, u.DisplayName)
		gotNames[string(*u.DisplayName)] = true
	}
	for _, r := range rows {
		full := r.body + tag
		if r.match {
			require.True(t, gotNames[full],
				"expected %q in result", full)
		} else {
			require.False(t, gotNames[full],
				"%q should be excluded", full)
		}
	}
}

// TestUserPagination_SearchEscapesPercentLiteral verifies that `%` in the
// search input is treated as a literal character, not as an ILIKE wildcard.
// Without escaping, the search "<marker>100%" would match every row whose
// display_name starts with "<marker>100".
func TestUserPagination_SearchEscapesPercentLiteral(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := repository.NewUserRepository(testDB.GORM)

	// Per-test marker has no LIKE meta-characters so the row prefix is
	// matched verbatim; only the suffix differs across rows.
	marker := "pctesc" + onlyHex(uuid.NewString())
	now := time.Now().UTC().Truncate(time.Microsecond)
	insertUserWithName(t, ctx, marker+"100%legit", now)
	insertUserWithName(t, ctx, marker+"99reasons", now.Add(time.Hour))
	insertUserWithName(t, ctx, marker+"100reasons", now.Add(2*time.Hour))

	// Search "<marker>100%": must match only "<marker>100%legit". If `%`
	// were treated as a wildcard, "<marker>100reasons" would also match.
	q := marker + "100%"
	got, total, err := repo.ListPage(ctx, nil, nil, 10, 0, &q)
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Len(t, got, 1)
	require.NotNil(t, got[0].DisplayName)
	require.Equal(t, marker+"100%legit", string(*got[0].DisplayName))
}

// TestUserPagination_SearchEscapesUnderscoreLiteral verifies that `_` in the
// search input is treated as a literal character, not as an ILIKE
// single-character wildcard. Mirrors the percent-escape test so a future
// refactor that drops `_` from the escape replacer is caught here.
func TestUserPagination_SearchEscapesUnderscoreLiteral(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := repository.NewUserRepository(testDB.GORM)

	// Per-test marker has no LIKE meta-characters so the row prefix is
	// matched verbatim; only the suffix (with/without underscore) differs.
	marker := "uscesc" + onlyHex(uuid.NewString())
	now := time.Now().UTC().Truncate(time.Microsecond)
	insertUserWithName(t, ctx, marker+"a_min", now)
	insertUserWithName(t, ctx, marker+"admin", now.Add(time.Hour))

	// Search "<marker>a_min": must match only "<marker>a_min". If `_` were
	// treated as a single-character wildcard, "<marker>admin" would also
	// match (the underscore covers any single character between "a" and "m").
	q := marker + "a_min"
	got, total, err := repo.ListPage(ctx, nil, nil, 10, 0, &q)
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Len(t, got, 1)
	require.NotNil(t, got[0].DisplayName)
	require.Equal(t, marker+"a_min", string(*got[0].DisplayName))
}

// TestUserPagination_CursorNotFound verifies that a cursor pointing at a uuid
// that does not exist in the users table surfaces ErrCursorNotFound — not a
// silent empty page or a wrapped DB error.
func TestUserPagination_CursorNotFound(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := repository.NewUserRepository(testDB.GORM)

	missing := uuid.NewString()
	_, _, err := repo.ListPage(ctx, &missing, nil, 5, 0, nil)
	require.Error(t, err)
	require.True(t, errors.Is(err, repository.ErrCursorNotFound),
		"expected ErrCursorNotFound, got %v", err)
}

// TestUserPagination_PageCapAllowsMaxPlusOne mirrors the cards-side test:
// userPageCap == maxUserPageSize+1 (101) so first=101 is accepted, while
// first=100 stays at the documented user-facing maximum.
func TestUserPagination_PageCapAllowsMaxPlusOne(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := repository.NewUserRepository(testDB.GORM)

	tag := "_cap_" + uuid.NewString()
	now := time.Now().UTC().Truncate(time.Microsecond)
	for i := 0; i < 101; i++ {
		insertUserWithName(t, ctx, "u"+tag, now.Add(time.Duration(i)*time.Second))
	}

	// first=101 must return all 101 rows because userPageCap == 101.
	q := tag
	cards101, total101, err := repo.ListPage(ctx, nil, nil, 101, 0, &q)
	require.NoError(t, err)
	require.Equal(t, int64(101), total101)
	require.Len(t, cards101, 101)

	// first=100 must be limited to 100 rows.
	cards100, total100, err := repo.ListPage(ctx, nil, nil, 100, 0, &q)
	require.NoError(t, err)
	require.Equal(t, int64(101), total100)
	require.Len(t, cards100, 100)
}

// TestUserPagination_ZeroPageReturnsTotal verifies that first==0 && last==0
// short-circuits the row fetch but still returns a real totalCount from the
// separate COUNT(*) query.
func TestUserPagination_ZeroPageReturnsTotal(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := repository.NewUserRepository(testDB.GORM)

	tag := "_zero_" + uuid.NewString()
	now := time.Now().UTC().Truncate(time.Microsecond)
	for i := 0; i < 5; i++ {
		insertUserWithName(t, ctx, "u"+tag, now.Add(time.Duration(i)*time.Second))
	}

	q := tag
	users, total, err := repo.ListPage(ctx, nil, nil, 0, 0, &q)
	require.NoError(t, err)
	require.NotNil(t, users)
	require.Empty(t, users)
	require.Equal(t, int64(5), total)
}

// TestUserPagination_NegativeFirstOrLast verifies that negative page sizes are
// rejected with an error rather than silently coerced to 0.
func TestUserPagination_NegativeFirstOrLast(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := repository.NewUserRepository(testDB.GORM)

	_, _, err := repo.ListPage(ctx, nil, nil, -1, 0, nil)
	require.Error(t, err)

	_, _, err = repo.ListPage(ctx, nil, nil, 0, -1, nil)
	require.Error(t, err)
}

// TestUserPagination_BlankSearchTreatedAsNoFilter verifies that whitespace-
// only search input is treated the same as nil — total reflects the entire
// users table, not zero.
func TestUserPagination_BlankSearchTreatedAsNoFilter(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := repository.NewUserRepository(testDB.GORM)

	// Seed at least one user so total > 0 even if the table was empty.
	insertUserAt(t, ctx, time.Now().UTC().Truncate(time.Microsecond))

	blank := "   "
	_, total, err := repo.ListPage(ctx, nil, nil, 1, 0, &blank)
	require.NoError(t, err)
	require.GreaterOrEqual(t, total, int64(1),
		"blank search must not zero out totalCount")
}

// uuidShort returns the first 8 hex characters of a fresh uuid — used to
// disambiguate display_name suffixes inside a single test without bloating
// log output.
func uuidShort() string {
	return uuid.NewString()[:8]
}

// onlyHex strips dashes from a uuid string so the result contains no
// LIKE/ILIKE meta-characters and no separators that could split a search
// substring across non-matching characters.
func onlyHex(s string) string {
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c != '-' {
			out = append(out, c)
		}
	}
	return string(out)
}

// flipCase swaps the case of every ASCII letter in s. Used to exercise the
// case-insensitive ILIKE: passing flipCase(needle) produces a stored row in
// the opposite case from the search term, so a successful match proves the
// comparison ignores case.
func flipCase(s string) string {
	out := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= 'a' && c <= 'z':
			out[i] = c - 32
		case c >= 'A' && c <= 'Z':
			out[i] = c + 32
		default:
			out[i] = c
		}
	}
	return string(out)
}

// ptrStr is a one-line helper for &literal in test call sites.
func ptrStr(s string) *string { return &s }

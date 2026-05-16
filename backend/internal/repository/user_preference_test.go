package repository_test

// TestMain, testDB, insertAuthUser, and sqlDBHandle are defined in user_test.go
// and shared across this package.

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"backend/internal/repository"
)

// insertCardgroupForUser inserts a cardgroup owned by ownerID and returns its
// ID. Uses the raw SQL helper to stay independent of CardgroupRepository
// semantics.
func insertCardgroupForUser(t *testing.T, ctx context.Context, ownerID, name string) string {
	t.Helper()
	sqlDB := sqlDBHandle(t)
	id := uuid.NewString()
	now := time.Now().UTC()
	if _, err := sqlDB.ExecContext(ctx,
		`INSERT INTO public.cardgroups (id, owner_id, name, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $4)`,
		id, ownerID, name, now); err != nil {
		t.Fatalf("insertCardgroupForUser %q: %v", name, err)
	}
	return id
}

// prefUpdatedAt reads user_preferences.updated_at directly via SQL.
func prefUpdatedAt(t *testing.T, ctx context.Context, userID string) time.Time {
	t.Helper()
	sqlDB := sqlDBHandle(t)
	var ts time.Time
	if err := sqlDB.QueryRowContext(ctx,
		`SELECT updated_at FROM public.user_preferences WHERE user_id = $1`, userID).
		Scan(&ts); err != nil {
		t.Fatalf("read user_preferences.updated_at for user %q: %v", userID, err)
	}
	return ts
}

// prefRowExists returns true when a user_preferences row exists for userID.
func prefRowExists(t *testing.T, ctx context.Context, userID string) bool {
	t.Helper()
	sqlDB := sqlDBHandle(t)
	var count int
	if err := sqlDB.QueryRowContext(ctx,
		`SELECT count(*) FROM public.user_preferences WHERE user_id = $1`, userID).
		Scan(&count); err != nil {
		t.Fatalf("count user_preferences for user %q: %v", userID, err)
	}
	return count > 0
}

// ---------------------------------------------------------------------------
// UpsertLastViewedCardgroup
// ---------------------------------------------------------------------------

// TestUserPreferenceRepository_Upsert_CreateRow verifies that the first UPSERT
// with an owned cardgroup inserts a new user_preferences row.
func TestUserPreferenceRepository_Upsert_CreateRow(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	userID := insertAuthUser(t, ctx)
	cgID := insertCardgroupForUser(t, ctx, userID, "owned-cg")

	repo := repository.NewUserPreferenceRepository(testDB.GORM)
	if err := repo.UpsertLastViewedCardgroup(ctx, userID, cgID); err != nil {
		t.Fatalf("UpsertLastViewedCardgroup: %v", err)
	}

	if !prefRowExists(t, ctx, userID) {
		t.Fatal("expected user_preferences row to be created")
	}
}

// TestUserPreferenceRepository_Upsert_UpdateRow verifies that a second UPSERT
// with an owned cardgroup updates the existing row and advances updated_at.
func TestUserPreferenceRepository_Upsert_UpdateRow(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	userID := insertAuthUser(t, ctx)
	cg1 := insertCardgroupForUser(t, ctx, userID, "first-cg")
	cg2 := insertCardgroupForUser(t, ctx, userID, "second-cg")

	repo := repository.NewUserPreferenceRepository(testDB.GORM)
	if err := repo.UpsertLastViewedCardgroup(ctx, userID, cg1); err != nil {
		t.Fatalf("UpsertLastViewedCardgroup (first): %v", err)
	}
	ts1 := prefUpdatedAt(t, ctx, userID)

	// Sleep so Postgres now() advances.
	time.Sleep(5 * time.Millisecond)

	if err := repo.UpsertLastViewedCardgroup(ctx, userID, cg2); err != nil {
		t.Fatalf("UpsertLastViewedCardgroup (second): %v", err)
	}
	ts2 := prefUpdatedAt(t, ctx, userID)

	if !ts2.After(ts1) {
		t.Fatalf("updated_at should advance on second UPSERT: first=%v second=%v", ts1, ts2)
	}

	// Confirm the row was updated, not duplicated.
	sqlDB := sqlDBHandle(t)
	var lastViewedID string
	if err := sqlDB.QueryRowContext(ctx,
		`SELECT last_viewed_cardgroup_id FROM public.user_preferences WHERE user_id = $1`, userID).
		Scan(&lastViewedID); err != nil {
		t.Fatalf("read last_viewed_cardgroup_id: %v", err)
	}
	if lastViewedID != cg2 {
		t.Fatalf("last_viewed_cardgroup_id = %q, want %q", lastViewedID, cg2)
	}
}

// TestUserPreferenceRepository_Upsert_UnownedCardgroup verifies that a UPSERT
// targeting a cardgroup owned by a different user returns ErrCardgroupNotFound
// and does not create a row.
func TestUserPreferenceRepository_Upsert_UnownedCardgroup(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	userA := insertAuthUser(t, ctx)
	userB := insertAuthUser(t, ctx)
	cgOwnedByB := insertCardgroupForUser(t, ctx, userB, "b-cg")

	repo := repository.NewUserPreferenceRepository(testDB.GORM)
	err := repo.UpsertLastViewedCardgroup(ctx, userA, cgOwnedByB)
	if !errors.Is(err, repository.ErrCardgroupNotFound) {
		t.Fatalf("want ErrCardgroupNotFound for unowned cardgroup, got %v", err)
	}
	if !errors.Is(err, repository.ErrNotFound) {
		t.Fatalf("want errors.Is(_, ErrNotFound) true (joined sentinel), got %v", err)
	}
	if prefRowExists(t, ctx, userA) {
		t.Fatal("unowned UPSERT must not write a user_preferences row")
	}
}

// TestUserPreferenceRepository_Upsert_NonExistentCardgroup verifies that a
// UPSERT with a cardgroup ID that does not exist at all returns the same
// ErrCardgroupNotFound sentinel as the ownership case, preserving the
// existence-oracle guard.
func TestUserPreferenceRepository_Upsert_NonExistentCardgroup(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	userID := insertAuthUser(t, ctx)

	repo := repository.NewUserPreferenceRepository(testDB.GORM)
	err := repo.UpsertLastViewedCardgroup(ctx, userID, uuid.NewString())
	if !errors.Is(err, repository.ErrCardgroupNotFound) {
		t.Fatalf("want ErrCardgroupNotFound for non-existent cardgroup, got %v", err)
	}
	if !errors.Is(err, repository.ErrNotFound) {
		t.Fatalf("want errors.Is(_, ErrNotFound) true, got %v", err)
	}
	if prefRowExists(t, ctx, userID) {
		t.Fatal("missing cardgroup UPSERT must not write a user_preferences row")
	}
}

// TestUserPreferenceRepository_CrossTenant_UpsertBlocked verifies that User A
// cannot record User B's cardgroup as their last-viewed cardgroup. The
// ownership guard in the EXISTS clause must prevent it, and no row is written.
func TestUserPreferenceRepository_CrossTenant_UpsertBlocked(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	userA := insertAuthUser(t, ctx)
	userB := insertAuthUser(t, ctx)
	cgB := insertCardgroupForUser(t, ctx, userB, "tenant-b-cg")

	// Also create a cardgroup owned by A so the test asserts "A's own CG works
	// but B's does not" — removes any doubt about setup correctness.
	cgA := insertCardgroupForUser(t, ctx, userA, "tenant-a-cg")

	repo := repository.NewUserPreferenceRepository(testDB.GORM)

	// A can UPSERT their own cardgroup.
	if err := repo.UpsertLastViewedCardgroup(ctx, userA, cgA); err != nil {
		t.Fatalf("A upsert own cardgroup: %v", err)
	}

	// A must NOT be able to UPSERT B's cardgroup.
	err := repo.UpsertLastViewedCardgroup(ctx, userA, cgB)
	if !errors.Is(err, repository.ErrCardgroupNotFound) {
		t.Fatalf("cross-tenant: want ErrCardgroupNotFound, got %v", err)
	}

	// The row for A must still point to A's own cardgroup, not B's.
	sqlDB := sqlDBHandle(t)
	var lastViewedID string
	if err := sqlDB.QueryRowContext(ctx,
		`SELECT last_viewed_cardgroup_id FROM public.user_preferences WHERE user_id = $1`, userA).
		Scan(&lastViewedID); err != nil {
		t.Fatalf("read last_viewed_cardgroup_id for userA: %v", err)
	}
	if lastViewedID != cgA {
		t.Fatalf("cross-tenant UPSERT must not overwrite existing row: got %q, want %q", lastViewedID, cgA)
	}

	// B must have no row in user_preferences (was never written by this test).
	if prefRowExists(t, ctx, userB) {
		t.Fatal("no row should exist for userB — only userA upserted")
	}
}

// ---------------------------------------------------------------------------
// FindByUserID
// ---------------------------------------------------------------------------

// TestUserPreferenceRepository_FindByUserID_Found verifies that a row created
// by UpsertLastViewedCardgroup is returned correctly by FindByUserID.
func TestUserPreferenceRepository_FindByUserID_Found(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	userID := insertAuthUser(t, ctx)
	cgID := insertCardgroupForUser(t, ctx, userID, "lookup-cg")

	repo := repository.NewUserPreferenceRepository(testDB.GORM)
	if err := repo.UpsertLastViewedCardgroup(ctx, userID, cgID); err != nil {
		t.Fatalf("UpsertLastViewedCardgroup: %v", err)
	}

	got, err := repo.FindByUserID(ctx, userID)
	if err != nil {
		t.Fatalf("FindByUserID: %v", err)
	}
	if got.UserID != userID {
		t.Fatalf("UserID = %q, want %q", got.UserID, userID)
	}
	if got.LastViewedCardgroupID == nil || *got.LastViewedCardgroupID != cgID {
		t.Fatalf("LastViewedCardgroupID = %v, want %q", got.LastViewedCardgroupID, cgID)
	}
	if got.UpdatedAt.IsZero() {
		t.Fatal("UpdatedAt must be non-zero")
	}
}

// TestUserPreferenceRepository_FindByUserID_NotFound verifies that looking up
// a user with no preference row returns ErrNotFound.
func TestUserPreferenceRepository_FindByUserID_NotFound(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	// Use a random UUID that has no corresponding row.
	repo := repository.NewUserPreferenceRepository(testDB.GORM)
	_, err := repo.FindByUserID(ctx, uuid.NewString())
	if !errors.Is(err, repository.ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// FindByUserIDs
// ---------------------------------------------------------------------------

// TestUserPreferenceRepository_FindByUserIDs_Found verifies that multiple
// user IDs are returned together in one call.
func TestUserPreferenceRepository_FindByUserIDs_Found(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	userA := insertAuthUser(t, ctx)
	userB := insertAuthUser(t, ctx)
	cgA := insertCardgroupForUser(t, ctx, userA, "ids-cg-a")
	cgB := insertCardgroupForUser(t, ctx, userB, "ids-cg-b")

	repo := repository.NewUserPreferenceRepository(testDB.GORM)
	if err := repo.UpsertLastViewedCardgroup(ctx, userA, cgA); err != nil {
		t.Fatalf("upsert A: %v", err)
	}
	if err := repo.UpsertLastViewedCardgroup(ctx, userB, cgB); err != nil {
		t.Fatalf("upsert B: %v", err)
	}

	prefs, err := repo.FindByUserIDs(ctx, []string{userA, userB})
	if err != nil {
		t.Fatalf("FindByUserIDs: %v", err)
	}
	if len(prefs) != 2 {
		t.Fatalf("expected 2 results, got %d", len(prefs))
	}
	byUser := make(map[string]*struct{}, 2)
	for _, p := range prefs {
		byUser[p.UserID] = &struct{}{}
	}
	for _, uid := range []string{userA, userB} {
		if _, ok := byUser[uid]; !ok {
			t.Fatalf("user %q missing from FindByUserIDs result", uid)
		}
	}
}

// TestUserPreferenceRepository_FindByUserIDs_MissingUser verifies that a user
// ID with no preference row is simply absent from the result (not an error).
func TestUserPreferenceRepository_FindByUserIDs_MissingUser(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	userID := insertAuthUser(t, ctx)
	cgID := insertCardgroupForUser(t, ctx, userID, "partial-cg")

	repo := repository.NewUserPreferenceRepository(testDB.GORM)
	if err := repo.UpsertLastViewedCardgroup(ctx, userID, cgID); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	missing := uuid.NewString()
	prefs, err := repo.FindByUserIDs(ctx, []string{userID, missing})
	if err != nil {
		t.Fatalf("FindByUserIDs: %v", err)
	}
	// Only the existing row should appear.
	if len(prefs) != 1 {
		t.Fatalf("expected 1 result, got %d", len(prefs))
	}
	if prefs[0].UserID != userID {
		t.Fatalf("result UserID = %q, want %q", prefs[0].UserID, userID)
	}
}

// TestUserPreferenceRepository_FindByUserIDs_EmptyInput verifies that an
// empty IDs slice returns an empty slice without error and without hitting the
// database. This guards against the GORM WHERE IN () full-table scan bug
// documented in go-library-gotchas.md.
func TestUserPreferenceRepository_FindByUserIDs_EmptyInput(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	repo := repository.NewUserPreferenceRepository(testDB.GORM)
	prefs, err := repo.FindByUserIDs(ctx, []string{})
	if err != nil {
		t.Fatalf("FindByUserIDs(empty): %v", err)
	}
	if len(prefs) != 0 {
		t.Fatalf("expected empty slice, got %d entries", len(prefs))
	}
}

// ---------------------------------------------------------------------------
// FK ON DELETE SET NULL — cardgroup deletion
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// FK ON DELETE CASCADE — user deletion
// ---------------------------------------------------------------------------

// TestUserPreferenceRepository_OnDeleteUser_CascadesPreferenceRow verifies the
// FK action declared in 20260516120000_extract_user_preferences.up.sql —
// guards against accidental RESTRICT or SET NULL on
// user_preferences.user_id REFERENCES public.users(id) ON DELETE CASCADE.
//
// When the owning user is deleted, the user_preferences row must be removed
// automatically by the database:
//   - RESTRICT would cause the auth.users DELETE to fail, breaking user
//     account deletion and causing a service outage.
//   - SET NULL would violate the PRIMARY KEY constraint on user_preferences.user_id
//     (NOT NULL is implied by PRIMARY KEY), crashing the database operation.
func TestUserPreferenceRepository_OnDeleteUser_CascadesPreferenceRow(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	userID := insertAuthUser(t, ctx)
	cgID := insertCardgroupForUser(t, ctx, userID, "fk-cascade-user-cg")

	repo := repository.NewUserPreferenceRepository(testDB.GORM)
	if err := repo.UpsertLastViewedCardgroup(ctx, userID, cgID); err != nil {
		t.Fatalf("UpsertLastViewedCardgroup: %v", err)
	}

	// Confirm the user_preferences row was written before triggering the cascade.
	if !prefRowExists(t, ctx, userID) {
		t.Fatal("expected user_preferences row to exist before user deletion")
	}

	// Delete from auth.users; the trigger cascades to public.users, which then
	// cascades to user_preferences via the ON DELETE CASCADE FK.
	sqlDB := sqlDBHandle(t)
	if _, err := sqlDB.ExecContext(ctx,
		`DELETE FROM auth.users WHERE id = $1`, userID); err != nil {
		t.Fatalf("delete auth.users %q: %v", userID, err)
	}

	// The user_preferences row must be gone — RESTRICT would have made the
	// DELETE above fail, and SET NULL is impossible on a PRIMARY KEY column.
	if prefRowExists(t, ctx, userID) {
		t.Fatal("user_preferences row still exists after user deletion: FK is not ON DELETE CASCADE")
	}
}

// TestUserPreferenceRepository_OnDeleteCardgroup_SetsNull verifies the FK
// action declared in 20260516120000_extract_user_preferences.up.sql —
// guards against accidental CASCADE.
//
// When a cardgroup referenced by user_preferences.last_viewed_cardgroup_id is
// deleted, the DB must set that column to NULL (ON DELETE SET NULL) rather than
// deleting the user_preferences row (ON DELETE CASCADE). A CASCADE would silently
// destroy user preference data every time a cardgroup is removed.
func TestUserPreferenceRepository_OnDeleteCardgroup_SetsNull(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	userID := insertAuthUser(t, ctx)
	cgID := insertCardgroupForUser(t, ctx, userID, "fk-set-null-cg")

	repo := repository.NewUserPreferenceRepository(testDB.GORM)
	if err := repo.UpsertLastViewedCardgroup(ctx, userID, cgID); err != nil {
		t.Fatalf("UpsertLastViewedCardgroup: %v", err)
	}

	// Confirm the row was written with the expected cardgroup ID before deletion.
	if !prefRowExists(t, ctx, userID) {
		t.Fatal("expected user_preferences row to exist before cardgroup deletion")
	}

	// Delete the cardgroup directly via SQL to trigger the FK ON DELETE action.
	sqlDB := sqlDBHandle(t)
	if _, err := sqlDB.ExecContext(ctx,
		`DELETE FROM public.cardgroups WHERE id = $1`, cgID); err != nil {
		t.Fatalf("delete cardgroup %q: %v", cgID, err)
	}

	// The user_preferences row must still exist — CASCADE would have removed it.
	if !prefRowExists(t, ctx, userID) {
		t.Fatal("user_preferences row was deleted after cardgroup deletion: FK is CASCADE, not SET NULL")
	}

	// last_viewed_cardgroup_id must now be NULL — that is the SET NULL action.
	var lastViewedID *string
	if err := sqlDB.QueryRowContext(ctx,
		`SELECT last_viewed_cardgroup_id FROM public.user_preferences WHERE user_id = $1`, userID).
		Scan(&lastViewedID); err != nil {
		t.Fatalf("read last_viewed_cardgroup_id after cardgroup deletion: %v", err)
	}
	if lastViewedID != nil {
		t.Fatalf("last_viewed_cardgroup_id = %q after cardgroup deletion, want NULL", *lastViewedID)
	}
}

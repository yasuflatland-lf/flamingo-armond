package repository_test

// TestMain, testDB, insertAuthUser, and sqlDBHandle are shared from user_test.go.

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"backend/internal/domain"
	"backend/internal/repository"
)

// insertCardgroupForUser inserts a cardgroup owned by ownerID and returns its ID.
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

// prefUpdatedAt reads user_preferences.updated_at for the given user.
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

// prefRowExists reports whether a user_preferences row exists for userID.
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
// missing cardgroup returns the same ErrCardgroupNotFound as an unowned one,
// preventing an existence oracle.
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

	if err := repo.UpsertLastViewedCardgroup(ctx, userA, cgA); err != nil {
		t.Fatalf("A upsert own cardgroup: %v", err)
	}

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

	if prefRowExists(t, ctx, userB) {
		t.Fatal("no row should exist for userB — only userA upserted")
	}
}

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
	if string(got.UserID) != userID {
		t.Fatalf("UserID = %q, want %q", got.UserID, userID)
	}
	if got.LastViewedCardgroupID == nil || *got.LastViewedCardgroupID != cgID {
		t.Fatalf("LastViewedCardgroupID = %v, want %q", got.LastViewedCardgroupID, cgID)
	}
	if got.UpdatedAt.IsZero() {
		t.Fatal("UpdatedAt must be non-zero")
	}
}

func TestUserPreferenceRepository_FindByUserID_NotFound(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := repository.NewUserPreferenceRepository(testDB.GORM)
	_, err := repo.FindByUserID(ctx, uuid.NewString())
	if !errors.Is(err, repository.ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

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
		byUser[string(p.UserID)] = &struct{}{}
	}
	for _, uid := range []string{userA, userB} {
		if _, ok := byUser[uid]; !ok {
			t.Fatalf("user %q missing from FindByUserIDs result", uid)
		}
	}
}

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
	if len(prefs) != 1 {
		t.Fatalf("expected 1 result, got %d", len(prefs))
	}
	if string(prefs[0].UserID) != userID {
		t.Fatalf("result UserID = %q, want %q", prefs[0].UserID, userID)
	}
}

// TestUserPreferenceRepository_FindByUserIDs_EmptyInput guards against the GORM
// WHERE IN () full-table scan: an empty slice must return empty without error.
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

// TestUserPreferenceRepository_OnDeleteUser_CascadesPreferenceRow guards
// against accidental RESTRICT or SET NULL on user_preferences.user_id:
//   - RESTRICT would make auth.users DELETE fail, breaking account deletion.
//   - SET NULL is impossible on a PRIMARY KEY column.
func TestUserPreferenceRepository_OnDeleteUser_CascadesPreferenceRow(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	userID := insertAuthUser(t, ctx)
	cgID := insertCardgroupForUser(t, ctx, userID, "fk-cascade-user-cg")

	repo := repository.NewUserPreferenceRepository(testDB.GORM)
	if err := repo.UpsertLastViewedCardgroup(ctx, userID, cgID); err != nil {
		t.Fatalf("UpsertLastViewedCardgroup: %v", err)
	}

	if !prefRowExists(t, ctx, userID) {
		t.Fatal("expected user_preferences row to exist before user deletion")
	}

	sqlDB := sqlDBHandle(t)
	if _, err := sqlDB.ExecContext(ctx,
		`DELETE FROM auth.users WHERE id = $1`, userID); err != nil {
		t.Fatalf("delete auth.users %q: %v", userID, err)
	}

	if prefRowExists(t, ctx, userID) {
		t.Fatal("user_preferences row still exists after user deletion: FK is not ON DELETE CASCADE")
	}
}

// TestUserPreferenceRepository_UpsertLearnDisplayMode_CreateRow verifies that
// UpsertLearnDisplayMode creates a new user_preferences row when none exists,
// and that FindByUserID reflects the stored mode.
func TestUserPreferenceRepository_UpsertLearnDisplayMode_CreateRow(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	userID := insertAuthUser(t, ctx)

	repo := repository.NewUserPreferenceRepository(testDB.GORM)
	if err := repo.UpsertLearnDisplayMode(ctx, userID, "always_visible"); err != nil {
		t.Fatalf("UpsertLearnDisplayMode (create): %v", err)
	}

	got, err := repo.FindByUserID(ctx, userID)
	if err != nil {
		t.Fatalf("FindByUserID after create: %v", err)
	}
	if got.LearnDisplayMode != domain.LearnDisplayAlwaysVisible {
		t.Fatalf("after create: LearnDisplayMode = %q, want %q", got.LearnDisplayMode, domain.LearnDisplayAlwaysVisible)
	}
}

// TestUserPreferenceRepository_UpsertLearnDisplayMode_UpdateRow verifies that
// calling UpsertLearnDisplayMode a second time updates the column without
// creating a duplicate row.
func TestUserPreferenceRepository_UpsertLearnDisplayMode_UpdateRow(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	userID := insertAuthUser(t, ctx)

	repo := repository.NewUserPreferenceRepository(testDB.GORM)
	if err := repo.UpsertLearnDisplayMode(ctx, userID, "always_visible"); err != nil {
		t.Fatalf("UpsertLearnDisplayMode (first): %v", err)
	}
	if err := repo.UpsertLearnDisplayMode(ctx, userID, "flip_to_reveal"); err != nil {
		t.Fatalf("UpsertLearnDisplayMode (second): %v", err)
	}

	got, err := repo.FindByUserID(ctx, userID)
	if err != nil {
		t.Fatalf("FindByUserID after update: %v", err)
	}
	if got.LearnDisplayMode != domain.LearnDisplayFlipToReveal {
		t.Fatalf("after update: LearnDisplayMode = %q, want %q", got.LearnDisplayMode, domain.LearnDisplayFlipToReveal)
	}
}

// TestUserPreferenceRepository_UpsertNewCardRatio_CreateRow verifies that
// UpsertNewCardRatio creates a new user_preferences row when none exists, and
// that FindByUserID reflects the stored fraction (proving the raw INSERT ...
// ON CONFLICT SQL and the new_card_ratio CHECK constraint against a real
// Postgres instance).
func TestUserPreferenceRepository_UpsertNewCardRatio_CreateRow(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	userID := insertAuthUser(t, ctx)

	repo := repository.NewUserPreferenceRepository(testDB.GORM)
	if err := repo.UpsertNewCardRatio(ctx, userID, 3, 7); err != nil {
		t.Fatalf("UpsertNewCardRatio (create): %v", err)
	}

	got, err := repo.FindByUserID(ctx, userID)
	if err != nil {
		t.Fatalf("FindByUserID after create: %v", err)
	}
	if got.NewCardRatio.Numerator() != 3 || got.NewCardRatio.Denominator() != 7 {
		t.Fatalf("after create: NewCardRatio = %d/%d, want 3/7",
			got.NewCardRatio.Numerator(), got.NewCardRatio.Denominator())
	}
}

// TestUserPreferenceRepository_UpsertNewCardRatio_UpdateRow verifies that
// calling UpsertNewCardRatio a second time updates the columns without creating
// a duplicate row.
func TestUserPreferenceRepository_UpsertNewCardRatio_UpdateRow(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	userID := insertAuthUser(t, ctx)

	repo := repository.NewUserPreferenceRepository(testDB.GORM)
	if err := repo.UpsertNewCardRatio(ctx, userID, 3, 7); err != nil {
		t.Fatalf("UpsertNewCardRatio (first): %v", err)
	}
	if err := repo.UpsertNewCardRatio(ctx, userID, 2, 9); err != nil {
		t.Fatalf("UpsertNewCardRatio (second): %v", err)
	}

	got, err := repo.FindByUserID(ctx, userID)
	if err != nil {
		t.Fatalf("FindByUserID after update: %v", err)
	}
	if got.NewCardRatio.Numerator() != 2 || got.NewCardRatio.Denominator() != 9 {
		t.Fatalf("after update: NewCardRatio = %d/%d, want 2/9",
			got.NewCardRatio.Numerator(), got.NewCardRatio.Denominator())
	}

	// Confirm the row was updated, not duplicated.
	sqlDB := sqlDBHandle(t)
	var count int
	if err := sqlDB.QueryRowContext(ctx,
		`SELECT count(*) FROM public.user_preferences WHERE user_id = $1`, userID).
		Scan(&count); err != nil {
		t.Fatalf("count user_preferences for user %q: %v", userID, err)
	}
	if count != 1 {
		t.Fatalf("expected exactly 1 user_preferences row, got %d", count)
	}
}

// TestUserPreferenceRepository_OnDeleteCardgroup_SetsNull guards against
// accidental CASCADE on last_viewed_cardgroup_id: deleting a cardgroup must
// set that column to NULL rather than removing the user_preferences row.
func TestUserPreferenceRepository_OnDeleteCardgroup_SetsNull(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	userID := insertAuthUser(t, ctx)
	cgID := insertCardgroupForUser(t, ctx, userID, "fk-set-null-cg")

	repo := repository.NewUserPreferenceRepository(testDB.GORM)
	if err := repo.UpsertLastViewedCardgroup(ctx, userID, cgID); err != nil {
		t.Fatalf("UpsertLastViewedCardgroup: %v", err)
	}

	if !prefRowExists(t, ctx, userID) {
		t.Fatal("expected user_preferences row to exist before cardgroup deletion")
	}

	sqlDB := sqlDBHandle(t)
	if _, err := sqlDB.ExecContext(ctx,
		`DELETE FROM public.cardgroups WHERE id = $1`, cgID); err != nil {
		t.Fatalf("delete cardgroup %q: %v", cgID, err)
	}

	if !prefRowExists(t, ctx, userID) {
		t.Fatal("user_preferences row was deleted after cardgroup deletion: FK is CASCADE, not SET NULL")
	}

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

package repository_test

// TestMain, testDB, insertAuthUser, sqlDBHandle, and insertRole live in the
// shared test files in this package; this file reuses them.

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"backend/internal/repository"
)

// insertCardgroupSQL inserts a cardgroup directly via SQL and returns its ID.
// Bypasses the GORM repository so the test fixture stays minimal and
// independent of CardgroupRepository semantics. Named distinctly from
// insertCardgroup (defined in card_test.go) to avoid a redeclaration.
func insertCardgroupSQL(t *testing.T, ctx context.Context, ownerID, name string) string {
	t.Helper()
	sqlDB := sqlDBHandle(t)
	id := uuid.NewString()
	now := time.Now().UTC()
	if _, err := sqlDB.ExecContext(ctx,
		`INSERT INTO public.cardgroups (id, owner_id, name, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $4)`,
		id, ownerID, name, now); err != nil {
		t.Fatalf("insert cardgroup %q: %v", name, err)
	}
	return id
}

// userLastViewed reads users.last_viewed_cardgroup_id directly via SQL. Returns
// (nil, nil) when the column is NULL; (&id, nil) when set; (_, err) on DB error.
func userLastViewed(t *testing.T, ctx context.Context, userID string) *string {
	t.Helper()
	sqlDB := sqlDBHandle(t)
	var lvid *string
	if err := sqlDB.QueryRowContext(ctx,
		`SELECT last_viewed_cardgroup_id FROM public.users WHERE id = $1`, userID).
		Scan(&lvid); err != nil {
		t.Fatalf("read users.last_viewed_cardgroup_id: %v", err)
	}
	return lvid
}

// TestUserRepository_SetLastViewedCardgroup_HappyPath verifies that an
// owned cardgroup can be recorded as last-viewed and the column is updated.
func TestUserRepository_SetLastViewedCardgroup_HappyPath(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	userID := insertAuthUser(t, ctx)
	cgID := insertCardgroupSQL(t, ctx, userID, "owned cardgroup")

	repo := repository.NewUserRepository(testDB.GORM)
	if err := repo.SetLastViewedCardgroup(ctx, userID, cgID); err != nil {
		t.Fatalf("SetLastViewedCardgroup: %v", err)
	}

	lvid := userLastViewed(t, ctx, userID)
	if lvid == nil || *lvid != cgID {
		t.Fatalf("last_viewed_cardgroup_id = %v, want %q", lvid, cgID)
	}

	got, err := repo.FindByID(ctx, userID)
	if err != nil {
		t.Fatalf("FindByID after Set: %v", err)
	}
	if got.LastViewedCardgroupID == nil || *got.LastViewedCardgroupID != cgID {
		t.Fatalf("domain LastViewedCardgroupID = %v, want %q", got.LastViewedCardgroupID, cgID)
	}
}

// TestUserRepository_SetLastViewedCardgroup_OwnershipViolation verifies that
// attempting to set a cardgroup owned by another user returns the joined
// ErrCardgroupNotFound sentinel and does NOT touch the column.
func TestUserRepository_SetLastViewedCardgroup_OwnershipViolation(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	caller := insertAuthUser(t, ctx)
	otherUser := insertAuthUser(t, ctx)
	foreignCG := insertCardgroupSQL(t, ctx, otherUser, "foreign cardgroup")

	repo := repository.NewUserRepository(testDB.GORM)
	err := repo.SetLastViewedCardgroup(ctx, caller, foreignCG)
	if !errors.Is(err, repository.ErrCardgroupNotFound) {
		t.Fatalf("want ErrCardgroupNotFound, got %v", err)
	}
	if !errors.Is(err, repository.ErrNotFound) {
		t.Fatalf("want errors.Is(_, ErrNotFound) true (joined sentinel), got %v", err)
	}

	if lvid := userLastViewed(t, ctx, caller); lvid != nil {
		t.Fatalf("ownership violation must not write last_viewed_cardgroup_id; got %q", *lvid)
	}
}

// TestUserRepository_SetLastViewedCardgroup_MissingCardgroup verifies that a
// non-existent cardgroup ID returns the same sentinel as the ownership
// violation. This is the existence-oracle guard: callers cannot distinguish
// "doesn't exist" from "owned by someone else".
func TestUserRepository_SetLastViewedCardgroup_MissingCardgroup(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	userID := insertAuthUser(t, ctx)

	repo := repository.NewUserRepository(testDB.GORM)
	err := repo.SetLastViewedCardgroup(ctx, userID, uuid.NewString())
	if !errors.Is(err, repository.ErrCardgroupNotFound) {
		t.Fatalf("want ErrCardgroupNotFound for missing cardgroup, got %v", err)
	}
	if !errors.Is(err, repository.ErrNotFound) {
		t.Fatalf("want errors.Is(_, ErrNotFound) true, got %v", err)
	}
}

// TestUserRepository_SetLastViewedCardgroup_MissingUser verifies that a
// non-existent userID also returns ErrCardgroupNotFound. The EXISTS subquery
// scopes to the user_id, so a missing user produces RowsAffected == 0 and
// the same sentinel is returned. This is acceptable: the calling usecase
// always passes the authenticated caller's sub, so a missing user is not a
// reachable production state.
func TestUserRepository_SetLastViewedCardgroup_MissingUser(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	owner := insertAuthUser(t, ctx)
	cgID := insertCardgroupSQL(t, ctx, owner, "cg")

	repo := repository.NewUserRepository(testDB.GORM)
	err := repo.SetLastViewedCardgroup(ctx, uuid.NewString(), cgID)
	if !errors.Is(err, repository.ErrCardgroupNotFound) {
		t.Fatalf("want ErrCardgroupNotFound for missing user, got %v", err)
	}
}

// TestUserRepository_SetLastViewedCardgroup_Idempotent verifies that
// repeating the same Set call leaves the column at the same value without
// erroring. RowsAffected may be 0 or 1 depending on Postgres' optimisation
// for "no actual change" — both must be treated as success since the
// statement is logically a no-op.
//
// NOTE: GORM's Updates() with a map sets RowsAffected to the number of
// matched rows, not the number of physically changed rows; so the second
// call should succeed because the WHERE+EXISTS still matches.
func TestUserRepository_SetLastViewedCardgroup_Idempotent(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	userID := insertAuthUser(t, ctx)
	cgID := insertCardgroupSQL(t, ctx, userID, "cg")

	repo := repository.NewUserRepository(testDB.GORM)
	if err := repo.SetLastViewedCardgroup(ctx, userID, cgID); err != nil {
		t.Fatalf("SetLastViewedCardgroup (first): %v", err)
	}
	if err := repo.SetLastViewedCardgroup(ctx, userID, cgID); err != nil {
		t.Fatalf("SetLastViewedCardgroup (second, idempotent): %v", err)
	}

	lvid := userLastViewed(t, ctx, userID)
	if lvid == nil || *lvid != cgID {
		t.Fatalf("after idempotent set, last_viewed_cardgroup_id = %v, want %q", lvid, cgID)
	}
}

// TestUserRepository_SetLastViewedCardgroup_OnDeleteSetNull verifies the
// FK cascade behaviour: deleting the referenced cardgroup nulls out the
// column rather than cascading the delete back to the user row.
func TestUserRepository_SetLastViewedCardgroup_OnDeleteSetNull(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	userID := insertAuthUser(t, ctx)
	cgID := insertCardgroupSQL(t, ctx, userID, "cg")

	repo := repository.NewUserRepository(testDB.GORM)
	if err := repo.SetLastViewedCardgroup(ctx, userID, cgID); err != nil {
		t.Fatalf("SetLastViewedCardgroup: %v", err)
	}

	// Delete the cardgroup directly; ON DELETE SET NULL must null the column.
	sqlDB := sqlDBHandle(t)
	if _, err := sqlDB.ExecContext(ctx, `DELETE FROM public.cardgroups WHERE id = $1`, cgID); err != nil {
		t.Fatalf("DELETE cardgroup: %v", err)
	}

	// User row must still exist.
	got, err := repo.FindByID(ctx, userID)
	if err != nil {
		t.Fatalf("FindByID after cardgroup delete: %v", err)
	}
	if got.LastViewedCardgroupID != nil {
		t.Fatalf("expected last_viewed_cardgroup_id to be NULL after parent delete, got %q",
			*got.LastViewedCardgroupID)
	}
}

// TestUserRepository_SetLastViewedCardgroup_DomainRoundTrip verifies that the
// LastViewedCardgroupID field flows correctly through the domain conversion:
// nil before any Set call, non-nil and matching after Set.
func TestUserRepository_SetLastViewedCardgroup_DomainRoundTrip(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	userID := insertAuthUser(t, ctx)
	cgID := insertCardgroupSQL(t, ctx, userID, "cg")

	repo := repository.NewUserRepository(testDB.GORM)
	before, err := repo.FindByID(ctx, userID)
	if err != nil {
		t.Fatalf("FindByID before: %v", err)
	}
	if before.LastViewedCardgroupID != nil {
		t.Fatalf("LastViewedCardgroupID before Set should be nil, got %q",
			*before.LastViewedCardgroupID)
	}

	if err := repo.SetLastViewedCardgroup(ctx, userID, cgID); err != nil {
		t.Fatalf("SetLastViewedCardgroup: %v", err)
	}

	after, err := repo.FindByID(ctx, userID)
	if err != nil {
		t.Fatalf("FindByID after: %v", err)
	}
	if after.LastViewedCardgroupID == nil {
		t.Fatalf("LastViewedCardgroupID after Set should be non-nil")
	}
	if *after.LastViewedCardgroupID != cgID {
		t.Fatalf("LastViewedCardgroupID = %q, want %q", *after.LastViewedCardgroupID, cgID)
	}

	// Confirm the rest of the domain shape is untouched by the new field.
	if after.ID != userID {
		t.Fatalf("ID changed: got %q, want %q", after.ID, userID)
	}
}

// TestRLS_SetLastViewedCardgroup_AuthenticatedRoleBlocksCrossUserUpdate verifies
// that the users_update_own_or_admin RLS policy (from migration
// 20260502130000_add_rls_policies.up.sql) blocks an UPDATE on
// users.last_viewed_cardgroup_id when the Postgres session is running as the
// authenticated role but request.jwt.claim.sub identifies a DIFFERENT user from
// the row being updated.
//
// Architecture note: the Go repository layer (SetLastViewedCardgroup) connects
// as the migration owner and bypasses RLS by definition. The RLS policies exist
// to protect direct authenticated-role access (Supabase PostgREST / Edge
// callers). This test exercises that second line of defence directly via raw SQL
// inside a transaction that impersonates the authenticated role.
//
// Test shape: victim owns a cardgroup; attacker has a valid session (JWT claim
// points to attacker). The attacker issues a raw UPDATE against victim's row.
// RLS must block the UPDATE — RowsAffected == 0 is the expected outcome.
func TestRLS_SetLastViewedCardgroup_AuthenticatedRoleBlocksCrossUserUpdate(t *testing.T) {
	// Not parallel: SET LOCAL ROLE / SET LOCAL config changes are
	// transaction-scoped and do not leak, but running in parallel with other
	// tests that also manipulate the same table rows is fine only if test data
	// is independent. Using t.Parallel() is safe here because victim/attacker
	// are freshly inserted rows, but to keep the intent explicit and avoid any
	// accidental interaction with the shared sqlDB handle, we omit t.Parallel().
	ctx := context.Background()

	victimID := insertAuthUser(t, ctx)
	attackerID := insertAuthUser(t, ctx)
	victimCGID := insertCardgroupSQL(t, ctx, victimID, "victim-cg")

	// Set victim's last_viewed_cardgroup_id using the superuser path so there is
	// a non-NULL value to attempt overwriting.
	repo := repository.NewUserRepository(testDB.GORM)
	if err := repo.SetLastViewedCardgroup(ctx, victimID, victimCGID); err != nil {
		t.Fatalf("setup SetLastViewedCardgroup for victim: %v", err)
	}

	// Open a raw *sql.DB from testDSN (the same superuser DSN used by the
	// container). SET LOCAL ROLE is available to superusers without needing a
	// separate connection string for the authenticated role.
	rawDB, err := sql.Open("pgx", testDSN)
	if err != nil {
		t.Fatalf("sql.Open testDSN: %v", err)
	}
	defer rawDB.Close()

	// All SET LOCAL statements must be inside a transaction.
	tx, err := rawDB.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	defer tx.Rollback() //nolint:errcheck // cleanup-only rollback

	// Impersonate the authenticated role. SET LOCAL is transaction-scoped and
	// resets on COMMIT/ROLLBACK, so this does not affect other connections or
	// tests.
	if _, err := tx.ExecContext(ctx, "SET LOCAL ROLE authenticated"); err != nil {
		t.Fatalf("SET LOCAL ROLE authenticated: %v", err)
	}

	// Claim to be the attacker, not the victim.
	if _, err := tx.ExecContext(ctx,
		"SELECT set_config('request.jwt.claim.sub', $1, true)", attackerID,
	); err != nil {
		t.Fatalf("set_config request.jwt.claim.sub: %v", err)
	}

	// Issue the UPDATE that the attacker should NOT be able to perform: updating
	// the victim's row. The RLS policy (id = auth.uid() OR is_admin(auth.uid()))
	// evaluates auth.uid() as attackerID, so the WHERE id = victimID row is
	// invisible to the attacker and RowsAffected must be 0.
	result, err := tx.ExecContext(ctx,
		"UPDATE public.users SET last_viewed_cardgroup_id = $1 WHERE id = $2",
		victimCGID, victimID,
	)
	if err != nil {
		// A permission-denied error is also an acceptable RLS enforcement signal,
		// but the policy uses USING (not WITH CHECK alone), so Postgres silently
		// filters the row rather than erroring. Either way we treat it as blocked.
		t.Logf("UPDATE returned error (also an RLS block signal): %v", err)
		return
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		t.Fatalf("RowsAffected: %v", err)
	}
	if rowsAffected != 0 {
		t.Fatalf(
			"RLS policy users_update_own_or_admin did not block cross-user UPDATE: "+
				"attacker %q updated victim %q row, RowsAffected = %d (want 0)",
			attackerID, victimID, rowsAffected,
		)
	}
}

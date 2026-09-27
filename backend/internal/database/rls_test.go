package database_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type rlsFixture struct {
	userA     string
	userB     string
	userC     string // fresh user with no user_preferences row; used for INSERT-own tests
	adminUser string
	groupA    string
	groupB    string
	cardA     string
	cardB     string
}

func TestRLSPolicies_AuthenticatedRole(t *testing.T) {
	ctx := context.Background()
	db := openMigratedDB(t)
	defer db.Close()

	fx := createRLSFixture(t, ctx, sqlDBForTest(t, db))

	authPool := openAuthenticatedPool(t, ctx)
	defer authPool.Close()

	probePool := openPolicyProbePool(t, ctx)
	defer probePool.Close()

	t.Run("users", func(t *testing.T) {
		assertCount(t, queryCountAs(t, ctx, authPool, fx.userA, `SELECT count(*) FROM public.users WHERE id = $1`, fx.userA), 1)
		assertCount(t, queryCountAs(t, ctx, authPool, fx.userA, `SELECT count(*) FROM public.users WHERE id = $1`, fx.userB), 0)
		assertCount(t, queryCountAs(t, ctx, authPool, fx.adminUser, `SELECT count(*) FROM public.users WHERE id = $1`, fx.userB), 1)

		assertRows(t, execOKAs(t, ctx, probePool, fx.userA,
			`UPDATE public.users SET display_name = 'self update' WHERE id = $1`, fx.userA), 1)
		assertRows(t, execOKAs(t, ctx, probePool, fx.userA,
			`UPDATE public.users SET display_name = 'blocked update' WHERE id = $1`, fx.userB), 0)
	})

	t.Run("cardgroups", func(t *testing.T) {
		assertCount(t, queryCountAs(t, ctx, authPool, fx.userA, `SELECT count(*) FROM public.cardgroups WHERE id = $1`, fx.groupA), 1)
		assertCount(t, queryCountAs(t, ctx, authPool, fx.userA, `SELECT count(*) FROM public.cardgroups WHERE id = $1`, fx.groupB), 0)
		assertCount(t, queryCountAs(t, ctx, authPool, fx.adminUser, `SELECT count(*) FROM public.cardgroups WHERE id = $1`, fx.groupB), 1)

		assertRows(t, execOKAs(t, ctx, probePool, fx.userA,
			`INSERT INTO public.cardgroups (owner_id, name) VALUES ($1, $2)`,
			fx.userA, "RLS own group"), 1)
		execDeniedAs(t, ctx, probePool, fx.userA,
			`INSERT INTO public.cardgroups (owner_id, name) VALUES ($1, $2)`,
			fx.userB, "RLS blocked group")
	})

	t.Run("cards", func(t *testing.T) {
		assertCount(t, queryCountAs(t, ctx, authPool, fx.userA, `SELECT count(*) FROM public.cards WHERE id = $1`, fx.cardA), 1)
		assertCount(t, queryCountAs(t, ctx, authPool, fx.userA, `SELECT count(*) FROM public.cards WHERE id = $1`, fx.cardB), 0)
		assertCount(t, queryCountAs(t, ctx, authPool, fx.adminUser, `SELECT count(*) FROM public.cards WHERE id = $1`, fx.cardB), 1)

		assertRows(t, execOKAs(t, ctx, probePool, fx.userA, insertCardSQL(),
			uuid.NewString(), fx.groupA, "RLS own card", "Back"), 1)
		execDeniedAs(t, ctx, probePool, fx.userA, insertCardSQL(),
			uuid.NewString(), fx.groupB, "RLS blocked card", "Back")
	})

	t.Run("swipe_records", func(t *testing.T) {
		assertCount(t, queryCountAs(t, ctx, authPool, fx.userA, `SELECT count(*) FROM public.swipe_records WHERE user_id = $1`, fx.userA), 1)
		assertCount(t, queryCountAs(t, ctx, authPool, fx.userA, `SELECT count(*) FROM public.swipe_records WHERE user_id = $1`, fx.userB), 0)
		assertCount(t, queryCountAs(t, ctx, authPool, fx.adminUser, `SELECT count(*) FROM public.swipe_records WHERE user_id = $1`, fx.userB), 1)

		assertRows(t, execOKAs(t, ctx, probePool, fx.userA, insertSwipeSQL(),
			fx.userA, fx.cardA, fx.groupA, time.Now().UTC()), 1)
		execDeniedAs(t, ctx, probePool, fx.userA, insertSwipeSQL(),
			fx.userB, fx.cardA, fx.groupA, time.Now().UTC())
		execDeniedAs(t, ctx, probePool, fx.adminUser, insertSwipeSQL(),
			fx.userB, fx.cardB, fx.groupB, time.Now().UTC())

		// The swipe_records_insert_own policy constrains user_id only, so a
		// caller inserting their own user_id can still name any deck id. The
		// cardgroup_id foreign key is what rejects a planted value: this insert
		// passes the RLS WITH CHECK and fails on SQLSTATE 23503 instead.
		execDeniedAs(t, ctx, probePool, fx.userA, insertSwipeSQL(),
			fx.userA, fx.cardA, uuid.NewString(), time.Now().UTC())
	})

	t.Run("user_card_fsrs", func(t *testing.T) {
		insertRLSUserCardFSRS(t, ctx, sqlDBForTest(t, db), fx.userA, fx.cardA)
		insertRLSUserCardFSRS(t, ctx, sqlDBForTest(t, db), fx.userB, fx.cardB)

		// User A can read their own rows.
		assertCount(t, queryCountAs(t, ctx, authPool, fx.userA, `SELECT count(*) FROM public.user_card_fsrs WHERE user_id = $1`, fx.userA), 1)
		// User A cannot read User B's rows.
		assertCount(t, queryCountAs(t, ctx, authPool, fx.userA, `SELECT count(*) FROM public.user_card_fsrs WHERE user_id = $1`, fx.userB), 0)
		// Admin can read any user's rows.
		assertCount(t, queryCountAs(t, ctx, authPool, fx.adminUser, `SELECT count(*) FROM public.user_card_fsrs WHERE user_id = $1`, fx.userB), 1)

		// User A can insert a row for themselves.
		assertRows(t, execOKAs(t, ctx, probePool, fx.userA, insertUserCardFSRSSQL(),
			fx.userA, fx.cardB, time.Now().UTC()), 1)
		// User A cannot insert a row with a different user_id.
		execDeniedAs(t, ctx, probePool, fx.userA, insertUserCardFSRSSQL(),
			fx.userB, fx.cardA, time.Now().UTC())
	})

	t.Run("user_preferences", func(t *testing.T) {
		insertRLSUserPreferences(t, ctx, sqlDBForTest(t, db), fx.userA)
		insertRLSUserPreferences(t, ctx, sqlDBForTest(t, db), fx.userB)

		// SELECT-own: User A can read their own row.
		assertCount(t, queryCountAs(t, ctx, authPool, fx.userA, `SELECT count(*) FROM public.user_preferences WHERE user_id = $1`, fx.userA), 1)
		// SELECT-other denied: User A cannot read User B's row.
		assertCount(t, queryCountAs(t, ctx, authPool, fx.userA, `SELECT count(*) FROM public.user_preferences WHERE user_id = $1`, fx.userB), 0)
		// SELECT-admin: admin can read any user's row.
		assertCount(t, queryCountAs(t, ctx, authPool, fx.adminUser, `SELECT count(*) FROM public.user_preferences WHERE user_id = $1`, fx.userB), 1)

		// INSERT-own: User C (no pre-existing row) can insert a row for themselves.
		// userA/userB already have rows from the fixture setup above, so we use userC
		// to avoid the ON CONFLICT DO NOTHING returning 0 rows affected.
		assertRows(t, execOKAs(t, ctx, probePool, fx.userC, insertUserPreferencesSQL(), fx.userC), 1)
		// INSERT-other denied: User C cannot insert a row with User A's user_id.
		execDeniedAs(t, ctx, probePool, fx.userC, insertUserPreferencesSQL(), fx.userA)

		// UPDATE-own: User A can update their own row.
		assertRows(t, execOKAs(t, ctx, probePool, fx.userA,
			`UPDATE public.user_preferences SET updated_at = now() WHERE user_id = $1`, fx.userA), 1)
		// UPDATE-other denied: User A cannot update User B's row (0 rows affected).
		assertRows(t, execOKAs(t, ctx, probePool, fx.userA,
			`UPDATE public.user_preferences SET updated_at = now() WHERE user_id = $1`, fx.userB), 0)

		// DELETE-own: User A can delete their own row.
		assertRows(t, execOKAs(t, ctx, probePool, fx.userA,
			`DELETE FROM public.user_preferences WHERE user_id = $1`, fx.userA), 1)
		// DELETE-other denied: User A cannot delete User B's row (0 rows affected).
		assertRows(t, execOKAs(t, ctx, probePool, fx.userA,
			`DELETE FROM public.user_preferences WHERE user_id = $1`, fx.userB), 0)
	})

	t.Run("roles", func(t *testing.T) {
		assertCount(t, queryCountAs(t, ctx, authPool, fx.userA, `SELECT count(*) FROM public.roles WHERE name = 'admin'`), 1)
		execDeniedAs(t, ctx, probePool, fx.userA,
			`INSERT INTO public.roles (name) VALUES ($1)`, uniqueName("blocked-role"))

		adminCreatedRoleID := insertRoleAs(t, ctx, probePool, fx.adminUser, uniqueName("admin-created-role"))
		if adminCreatedRoleID == "" {
			t.Fatal("admin role insert returned empty id")
		}

		roleID := insertRLSRole(t, ctx, sqlDBForTest(t, db), uniqueName("user-roles-probe"))

		t.Run("user_roles", func(t *testing.T) {
			assertCount(t, queryCountAs(t, ctx, authPool, fx.userA, `SELECT count(*) FROM public.user_roles WHERE user_id = $1`, fx.userA), 1)
			assertCount(t, queryCountAs(t, ctx, authPool, fx.userA, `SELECT count(*) FROM public.user_roles WHERE user_id = $1`, fx.userB), 0)
			assertCount(t, queryCountAs(t, ctx, authPool, fx.adminUser, `SELECT count(*) FROM public.user_roles WHERE user_id = $1`, fx.userB), 1)

			execDeniedAs(t, ctx, probePool, fx.userA,
				`INSERT INTO public.user_roles (user_id, role_id) VALUES ($1, $2)`,
				fx.userA, roleID)
			assertRows(t, execOKAs(t, ctx, probePool, fx.adminUser,
				`INSERT INTO public.user_roles (user_id, role_id) VALUES ($1, $2)`,
				fx.userB, roleID), 1)
		})
	})

	t.Run("schema_migrations", func(t *testing.T) {
		// golang-migrate's bookkeeping table carries a deny-all posture: RLS is
		// enabled with no policy and the API-role GRANTs are revoked, so an
		// authenticated PostgREST caller is denied outright. The owner connection
		// used by golang-migrate and the backend bypasses RLS and is unaffected.
		execPrivilegeDeniedAs(t, ctx, authPool, fx.userA,
			`SELECT version FROM public.schema_migrations`)
	})
}

func createRLSFixture(t *testing.T, ctx context.Context, sqlDB *sql.DB) rlsFixture {
	t.Helper()
	userA := insertRLSAuthUser(t, ctx, sqlDB)
	userB := insertRLSAuthUser(t, ctx, sqlDB)
	userC := insertRLSAuthUser(t, ctx, sqlDB) // reserved for INSERT-own tests; no pre-inserted rows
	adminUser := insertRLSAuthUser(t, ctx, sqlDB)

	var adminRoleID string
	if err := sqlDB.QueryRowContext(ctx, `SELECT id FROM public.roles WHERE name = 'admin'`).Scan(&adminRoleID); err != nil {
		t.Fatalf("query admin role: %v", err)
	}
	if _, err := sqlDB.ExecContext(ctx,
		`INSERT INTO public.user_roles (user_id, role_id) VALUES ($1, $2)`,
		adminUser, adminRoleID); err != nil {
		t.Fatalf("grant admin role: %v", err)
	}

	roleID := uuid.NewString()
	if _, err := sqlDB.ExecContext(ctx,
		`INSERT INTO public.roles (id, name) VALUES ($1, $2)`,
		roleID, uniqueName("fixture-role")); err != nil {
		t.Fatalf("insert fixture role: %v", err)
	}
	for _, userID := range []string{userA, userB} {
		if _, err := sqlDB.ExecContext(ctx,
			`INSERT INTO public.user_roles (user_id, role_id) VALUES ($1, $2)`,
			userID, roleID); err != nil {
			t.Fatalf("insert fixture user role: %v", err)
		}
	}

	groupA := insertRLSCardgroup(t, ctx, sqlDB, userA, "RLS group A")
	groupB := insertRLSCardgroup(t, ctx, sqlDB, userB, "RLS group B")
	cardA := insertRLSCard(t, ctx, sqlDB, groupA, "Card A")
	cardB := insertRLSCard(t, ctx, sqlDB, groupB, "Card B")
	insertRLSSwipe(t, ctx, sqlDB, userA, cardA, groupA)
	insertRLSSwipe(t, ctx, sqlDB, userB, cardB, groupB)

	return rlsFixture{
		userA:     userA,
		userB:     userB,
		userC:     userC,
		adminUser: adminUser,
		groupA:    groupA,
		groupB:    groupB,
		cardA:     cardA,
		cardB:     cardB,
	}
}

func insertRLSAuthUser(t *testing.T, ctx context.Context, sqlDB *sql.DB) string {
	t.Helper()
	id := uuid.NewString()
	if _, err := sqlDB.ExecContext(ctx,
		`INSERT INTO auth.users (id, email) VALUES ($1, $2)`,
		id, fmt.Sprintf("%s@test", id)); err != nil {
		t.Fatalf("insert auth user: %v", err)
	}
	return id
}

func insertRLSRole(t *testing.T, ctx context.Context, sqlDB *sql.DB, name string) string {
	t.Helper()
	id := uuid.NewString()
	if _, err := sqlDB.ExecContext(ctx,
		`INSERT INTO public.roles (id, name) VALUES ($1, $2)`,
		id, name); err != nil {
		t.Fatalf("insert role: %v", err)
	}
	return id
}

func insertRLSCardgroup(t *testing.T, ctx context.Context, sqlDB *sql.DB, ownerID, name string) string {
	t.Helper()
	id := uuid.NewString()
	if _, err := sqlDB.ExecContext(ctx,
		`INSERT INTO public.cardgroups (id, owner_id, name) VALUES ($1, $2, $3)`,
		id, ownerID, name); err != nil {
		t.Fatalf("insert cardgroup: %v", err)
	}
	return id
}

func insertRLSCard(t *testing.T, ctx context.Context, sqlDB *sql.DB, cardgroupID, front string) string {
	t.Helper()
	id := uuid.NewString()
	if _, err := sqlDB.ExecContext(ctx, insertCardSQL(), id, cardgroupID, front, "Back"); err != nil {
		t.Fatalf("insert card: %v", err)
	}
	return id
}

func insertRLSSwipe(t *testing.T, ctx context.Context, sqlDB *sql.DB, userID, cardID, cardgroupID string) {
	t.Helper()
	if _, err := sqlDB.ExecContext(ctx, insertSwipeSQL(), userID, cardID, cardgroupID, time.Now().UTC()); err != nil {
		t.Fatalf("insert swipe: %v", err)
	}
}

func openAuthenticatedPool(t *testing.T, ctx context.Context) *pgxpool.Pool {
	t.Helper()
	cfg, err := pgxpool.ParseConfig(testDSN)
	if err != nil {
		t.Fatalf("parse authenticated dsn: %v", err)
	}
	cfg.ConnConfig.User = "authenticated"
	cfg.ConnConfig.Password = "test"
	cfg.MaxConns = 2
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatalf("open authenticated pool: %v", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		t.Fatalf("ping authenticated pool: %v", err)
	}
	return pool
}

// openPolicyProbePool opens an owner-role pool for beginPolicyProbe.
func openPolicyProbePool(t *testing.T, ctx context.Context) *pgxpool.Pool {
	t.Helper()
	pool, err := pgxpool.New(ctx, testDSN)
	if err != nil {
		t.Fatalf("open policy probe pool: %v", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		t.Fatalf("ping policy probe pool: %v", err)
	}
	return pool
}

// beginPolicyProbe re-grants INSERT/UPDATE/DELETE (withheld from the API roles
// since 20260927000001_revoke_client_writes) and switches to authenticated inside
// the returned tx. Callers must roll back: committing persists the GRANT for the
// whole shared test DB.
func beginPolicyProbe(t *testing.T, ctx context.Context, pool *pgxpool.Pool, userID string) pgx.Tx {
	t.Helper()
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin policy probe tx: %v", err)
	}
	ready := false
	defer func() {
		if !ready {
			tx.Rollback(ctx)
		}
	}()
	if _, err := tx.Exec(ctx, `GRANT INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO authenticated`); err != nil {
		t.Fatalf("grant policy probe privileges: %v", err)
	}
	if _, err := tx.Exec(ctx, `SET LOCAL ROLE authenticated`); err != nil {
		t.Fatalf("set policy probe role: %v", err)
	}
	setClaims(t, ctx, tx, userID)
	ready = true
	return tx
}

func queryCountAs(t *testing.T, ctx context.Context, pool *pgxpool.Pool, userID, query string, args ...any) int {
	t.Helper()
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin authenticated tx: %v", err)
	}
	defer tx.Rollback(ctx)
	setClaims(t, ctx, tx, userID)

	var count int
	if err := tx.QueryRow(ctx, query, args...).Scan(&count); err != nil {
		t.Fatalf("query count as %s: %v", userID, err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit authenticated tx: %v", err)
	}
	return count
}

func execOKAs(t *testing.T, ctx context.Context, pool *pgxpool.Pool, userID, query string, args ...any) int64 {
	t.Helper()
	tx := beginPolicyProbe(t, ctx, pool, userID)
	defer tx.Rollback(ctx)

	tag, err := tx.Exec(ctx, query, args...)
	if err != nil {
		t.Fatalf("exec as %s: %v", userID, err)
	}
	return tag.RowsAffected()
}

func execDeniedAs(t *testing.T, ctx context.Context, pool *pgxpool.Pool, userID, query string, args ...any) {
	t.Helper()
	tx := beginPolicyProbe(t, ctx, pool, userID)
	defer tx.Rollback(ctx)

	_, err := tx.Exec(ctx, query, args...)
	if err == nil {
		t.Fatalf("exec as %s unexpectedly succeeded", userID)
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && strings.Contains(pgErr.Message, "permission denied for table") {
		t.Fatalf("exec as %s: denied by table privilege, not by policy: %v", userID, err)
	}
}

// execPrivilegeDeniedAs asserts the statement fails on the table-privilege check
// (SQLSTATE 42501, permission denied for table), before any RLS policy runs.
func execPrivilegeDeniedAs(t *testing.T, ctx context.Context, pool *pgxpool.Pool, userID, query string, args ...any) {
	t.Helper()
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin authenticated tx: %v", err)
	}
	defer tx.Rollback(ctx)
	setClaims(t, ctx, tx, userID)

	_, err = tx.Exec(ctx, query, args...)
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "42501" || !strings.Contains(pgErr.Message, "permission denied for table") {
		t.Fatalf("exec as %s: expected table privilege denial (42501), got: %v", userID, err)
	}
}

// insertRoleAs probes the roles INSERT policy as userID. The probe transaction is
// rolled back, so the returned id does not persist; seed FK targets with insertRLSRole.
func insertRoleAs(t *testing.T, ctx context.Context, pool *pgxpool.Pool, userID, name string) string {
	t.Helper()
	tx := beginPolicyProbe(t, ctx, pool, userID)
	defer tx.Rollback(ctx)

	var roleID string
	if err := tx.QueryRow(ctx,
		`INSERT INTO public.roles (name) VALUES ($1) RETURNING id`, name).Scan(&roleID); err != nil {
		t.Fatalf("insert role as %s: %v", userID, err)
	}
	return roleID
}

func setClaims(t *testing.T, ctx context.Context, tx pgx.Tx, userID string) {
	t.Helper()
	claims := fmt.Sprintf(`{"sub":%q}`, userID)
	if _, err := tx.Exec(ctx, `SELECT set_config('request.jwt.claims', $1, true)`, claims); err != nil {
		t.Fatalf("set jwt claims: %v", err)
	}
}

func assertCount(t *testing.T, got, want int) {
	t.Helper()
	if got != want {
		t.Fatalf("count: got %d, want %d", got, want)
	}
}

func assertRows(t *testing.T, got, want int64) {
	t.Helper()
	if got != want {
		t.Fatalf("rows affected: got %d, want %d", got, want)
	}
}

func uniqueName(prefix string) string {
	return fmt.Sprintf("%s-%s", prefix, uuid.NewString())
}

func insertCardSQL() string {
	return `
        INSERT INTO public.cards (id, cardgroup_id, front, back)
        VALUES ($1, $2, $3, $4)
    `
}

func insertSwipeSQL() string {
	return `
        INSERT INTO public.swipe_records (
            user_id, card_id, cardgroup_id, rating, reviewed_at, due, stability, difficulty,
            scheduled_days, reps, lapses, state, last_review,
            due_before, phase_before, stability_before
        )
        VALUES ($1, $2, $3, 3, $4, $4, 2.5, 5.0, 0, 0, 0, 0, $4, $4, 0, 2.5)
    `
}

func insertRLSUserCardFSRS(t *testing.T, ctx context.Context, sqlDB *sql.DB, userID, cardID string) {
	t.Helper()
	if _, err := sqlDB.ExecContext(ctx, insertUserCardFSRSSQL(),
		userID, cardID, time.Now().UTC()); err != nil {
		t.Fatalf("insert user_card_fsrs: %v", err)
	}
}

func insertUserCardFSRSSQL() string {
	return `
        INSERT INTO public.user_card_fsrs (
            user_id, card_id, state, due, stability, difficulty,
            reps, lapses, last_review, scheduled_days
        )
        VALUES ($1, $2, 0, $3, 2.5, 5.0, 0, 0, $3, 0)
        ON CONFLICT (user_id, card_id) DO NOTHING
    `
}

func insertRLSUserPreferences(t *testing.T, ctx context.Context, sqlDB *sql.DB, userID string) {
	t.Helper()
	if _, err := sqlDB.ExecContext(ctx, insertUserPreferencesSQL(), userID); err != nil {
		t.Fatalf("insert user_preferences: %v", err)
	}
}

func insertUserPreferencesSQL() string {
	return `
        INSERT INTO public.user_preferences (user_id)
        VALUES ($1)
        ON CONFLICT (user_id) DO NOTHING
    `
}

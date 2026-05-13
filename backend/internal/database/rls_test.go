package database_test

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type rlsFixture struct {
	userA     string
	userB     string
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

	t.Run("users", func(t *testing.T) {
		assertCount(t, queryCountAs(t, ctx, authPool, fx.userA, `SELECT count(*) FROM public.users WHERE id = $1`, fx.userA), 1)
		assertCount(t, queryCountAs(t, ctx, authPool, fx.userA, `SELECT count(*) FROM public.users WHERE id = $1`, fx.userB), 0)
		assertCount(t, queryCountAs(t, ctx, authPool, fx.adminUser, `SELECT count(*) FROM public.users WHERE id = $1`, fx.userB), 1)

		assertRows(t, execOKAs(t, ctx, authPool, fx.userA,
			`UPDATE public.users SET display_name = 'self update' WHERE id = $1`, fx.userA), 1)
		assertRows(t, execOKAs(t, ctx, authPool, fx.userA,
			`UPDATE public.users SET display_name = 'blocked update' WHERE id = $1`, fx.userB), 0)
	})

	t.Run("cardgroups", func(t *testing.T) {
		assertCount(t, queryCountAs(t, ctx, authPool, fx.userA, `SELECT count(*) FROM public.cardgroups WHERE id = $1`, fx.groupA), 1)
		assertCount(t, queryCountAs(t, ctx, authPool, fx.userA, `SELECT count(*) FROM public.cardgroups WHERE id = $1`, fx.groupB), 0)
		assertCount(t, queryCountAs(t, ctx, authPool, fx.adminUser, `SELECT count(*) FROM public.cardgroups WHERE id = $1`, fx.groupB), 1)

		assertRows(t, execOKAs(t, ctx, authPool, fx.userA,
			`INSERT INTO public.cardgroups (owner_id, name) VALUES ($1, $2)`,
			fx.userA, "RLS own group"), 1)
		execDeniedAs(t, ctx, authPool, fx.userA,
			`INSERT INTO public.cardgroups (owner_id, name) VALUES ($1, $2)`,
			fx.userB, "RLS blocked group")
	})

	t.Run("cards", func(t *testing.T) {
		assertCount(t, queryCountAs(t, ctx, authPool, fx.userA, `SELECT count(*) FROM public.cards WHERE id = $1`, fx.cardA), 1)
		assertCount(t, queryCountAs(t, ctx, authPool, fx.userA, `SELECT count(*) FROM public.cards WHERE id = $1`, fx.cardB), 0)
		assertCount(t, queryCountAs(t, ctx, authPool, fx.adminUser, `SELECT count(*) FROM public.cards WHERE id = $1`, fx.cardB), 1)

		assertRows(t, execOKAs(t, ctx, authPool, fx.userA, insertCardSQL(),
			uuid.NewString(), fx.groupA, "RLS own card", "Back"), 1)
		execDeniedAs(t, ctx, authPool, fx.userA, insertCardSQL(),
			uuid.NewString(), fx.groupB, "RLS blocked card", "Back")
	})

	t.Run("swipe_records", func(t *testing.T) {
		assertCount(t, queryCountAs(t, ctx, authPool, fx.userA, `SELECT count(*) FROM public.swipe_records WHERE user_id = $1`, fx.userA), 1)
		assertCount(t, queryCountAs(t, ctx, authPool, fx.userA, `SELECT count(*) FROM public.swipe_records WHERE user_id = $1`, fx.userB), 0)
		assertCount(t, queryCountAs(t, ctx, authPool, fx.adminUser, `SELECT count(*) FROM public.swipe_records WHERE user_id = $1`, fx.userB), 1)

		assertRows(t, execOKAs(t, ctx, authPool, fx.userA, insertSwipeSQL(),
			fx.userA, fx.cardA, time.Now().UTC()), 1)
		execDeniedAs(t, ctx, authPool, fx.userA, insertSwipeSQL(),
			fx.userB, fx.cardA, time.Now().UTC())
		execDeniedAs(t, ctx, authPool, fx.adminUser, insertSwipeSQL(),
			fx.userB, fx.cardB, time.Now().UTC())
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
		assertRows(t, execOKAs(t, ctx, authPool, fx.userA, insertUserCardFSRSSQL(),
			fx.userA, fx.cardB, time.Now().UTC()), 1)
		// User A cannot insert a row with a different user_id.
		execDeniedAs(t, ctx, authPool, fx.userA, insertUserCardFSRSSQL(),
			fx.userB, fx.cardA, time.Now().UTC())
	})

	t.Run("roles", func(t *testing.T) {
		assertCount(t, queryCountAs(t, ctx, authPool, fx.userA, `SELECT count(*) FROM public.roles WHERE name = 'admin'`), 1)
		execDeniedAs(t, ctx, authPool, fx.userA,
			`INSERT INTO public.roles (name) VALUES ($1)`, uniqueName("blocked-role"))

		roleID := insertRoleAs(t, ctx, authPool, fx.adminUser, uniqueName("admin-created-role"))
		if roleID == "" {
			t.Fatal("admin role insert returned empty id")
		}

		t.Run("user_roles", func(t *testing.T) {
			assertCount(t, queryCountAs(t, ctx, authPool, fx.userA, `SELECT count(*) FROM public.user_roles WHERE user_id = $1`, fx.userA), 1)
			assertCount(t, queryCountAs(t, ctx, authPool, fx.userA, `SELECT count(*) FROM public.user_roles WHERE user_id = $1`, fx.userB), 0)
			assertCount(t, queryCountAs(t, ctx, authPool, fx.adminUser, `SELECT count(*) FROM public.user_roles WHERE user_id = $1`, fx.userB), 1)

			execDeniedAs(t, ctx, authPool, fx.userA,
				`INSERT INTO public.user_roles (user_id, role_id) VALUES ($1, $2)`,
				fx.userA, roleID)
			assertRows(t, execOKAs(t, ctx, authPool, fx.adminUser,
				`INSERT INTO public.user_roles (user_id, role_id) VALUES ($1, $2)`,
				fx.userB, roleID), 1)
		})
	})
}

func createRLSFixture(t *testing.T, ctx context.Context, sqlDB *sql.DB) rlsFixture {
	t.Helper()
	userA := insertRLSAuthUser(t, ctx, sqlDB)
	userB := insertRLSAuthUser(t, ctx, sqlDB)
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
	insertRLSSwipe(t, ctx, sqlDB, userA, cardA)
	insertRLSSwipe(t, ctx, sqlDB, userB, cardB)

	return rlsFixture{
		userA:     userA,
		userB:     userB,
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

func insertRLSSwipe(t *testing.T, ctx context.Context, sqlDB *sql.DB, userID, cardID string) {
	t.Helper()
	if _, err := sqlDB.ExecContext(ctx, insertSwipeSQL(), userID, cardID, time.Now().UTC()); err != nil {
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
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin authenticated tx: %v", err)
	}
	defer tx.Rollback(ctx)
	setClaims(t, ctx, tx, userID)

	tag, err := tx.Exec(ctx, query, args...)
	if err != nil {
		t.Fatalf("exec as %s: %v", userID, err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit authenticated tx: %v", err)
	}
	return tag.RowsAffected()
}

func execDeniedAs(t *testing.T, ctx context.Context, pool *pgxpool.Pool, userID, query string, args ...any) {
	t.Helper()
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin authenticated tx: %v", err)
	}
	defer tx.Rollback(ctx)
	setClaims(t, ctx, tx, userID)

	if _, err := tx.Exec(ctx, query, args...); err == nil {
		t.Fatalf("exec as %s unexpectedly succeeded", userID)
	}
}

func insertRoleAs(t *testing.T, ctx context.Context, pool *pgxpool.Pool, userID, name string) string {
	t.Helper()
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin authenticated tx: %v", err)
	}
	defer tx.Rollback(ctx)
	setClaims(t, ctx, tx, userID)

	var roleID string
	if err := tx.QueryRow(ctx,
		`INSERT INTO public.roles (name) VALUES ($1) RETURNING id`, name).Scan(&roleID); err != nil {
		t.Fatalf("insert role as %s: %v", userID, err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit authenticated tx: %v", err)
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
            user_id, card_id, rating, reviewed_at, due, stability, difficulty,
            elapsed_days, scheduled_days, reps, lapses, state, last_review
        )
        VALUES ($1, $2, 3, $3, $3, 2.5, 5.0, 0, 0, 0, 0, 0, $3)
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
            reps, lapses, last_review, elapsed_days, scheduled_days
        )
        VALUES ($1, $2, 0, $3, 2.5, 5.0, 0, 0, $3, 0, 0)
        ON CONFLICT (user_id, card_id) DO NOTHING
    `
}

package database_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestMigrations_APIRolesHaveNoWritePrivilegeOnPublicTables(t *testing.T) {
	ctx := context.Background()
	db := openMigratedDB(t)
	defer db.Close()
	sqlDB := sqlDBForTest(t, db)

	rows, err := sqlDB.QueryContext(ctx, `
		SELECT c.relname
		FROM pg_class c
		JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE n.nspname = 'public' AND c.relkind IN ('r', 'p')
		ORDER BY c.relname
	`)
	if err != nil {
		t.Fatalf("query public tables: %v", err)
	}
	defer rows.Close()
	var tables []string
	for rows.Next() {
		var table string
		if err := rows.Scan(&table); err != nil {
			t.Fatalf("scan public table: %v", err)
		}
		tables = append(tables, table)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate public tables: %v", err)
	}
	if err := rows.Close(); err != nil {
		t.Fatalf("close public tables: %v", err)
	}
	if len(tables) < 12 {
		t.Fatalf("found %d public tables, want at least 12", len(tables))
	}

	var offenders []string
	for _, table := range tables {
		for _, role := range []string{"anon", "authenticated"} {
			for _, privilege := range []string{"INSERT", "UPDATE", "DELETE", "TRUNCATE", "SELECT"} {
				var granted bool
				if err := sqlDB.QueryRowContext(ctx,
					`SELECT has_table_privilege($1, 'public.' || quote_ident($2), $3)`,
					role, table, privilege).Scan(&granted); err != nil {
					t.Fatalf("query %s:%s:%s: %v", role, table, privilege, err)
				}
				want := privilege == "SELECT" && table != "schema_migrations"
				if granted != want {
					offenders = append(offenders, role+":"+table+":"+privilege)
				}
			}
		}
	}
	if len(offenders) > 0 {
		t.Fatalf("unexpected API table privileges: %s", strings.Join(offenders, ", "))
	}
}

func TestMigrations_DefaultPrivilegesWithholdWritesFromAPIRoles(t *testing.T) {
	ctx := context.Background()
	db := openMigratedDB(t)
	defer db.Close()
	sqlDB := sqlDBForTest(t, db)
	tx, err := sqlDB.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin default privilege probe: %v", err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `CREATE TABLE public.default_privilege_probe (id int)`); err != nil {
		t.Fatalf("create default privilege probe: %v", err)
	}
	for _, role := range []string{"anon", "authenticated"} {
		for _, privilege := range []string{"INSERT", "UPDATE", "DELETE", "TRUNCATE", "SELECT"} {
			var granted bool
			if err := tx.QueryRowContext(ctx,
				`SELECT has_table_privilege($1, 'public.default_privilege_probe', $2)`,
				role, privilege).Scan(&granted); err != nil {
				t.Fatalf("query default privilege %s:%s: %v", role, privilege, err)
			}
			if want := privilege == "SELECT"; granted != want {
				t.Errorf("default privilege %s:%s: got %t, want %t", role, privilege, granted, want)
			}
		}
	}
}

func TestAPIRoles_OwnRowWritesArePermissionDenied(t *testing.T) {
	ctx := context.Background()
	db := openMigratedDB(t)
	defer db.Close()
	fx := createRLSFixture(t, ctx, sqlDBForTest(t, db))
	authPool := openAuthenticatedPool(t, ctx)
	defer authPool.Close()

	cases := []struct {
		name   string
		userID string
		query  string
		args   []any
	}{
		{"users_update", fx.userA, `UPDATE public.users SET display_name = 'bypass' WHERE id = $1`, []any{fx.userA}},
		{"cardgroups_insert", fx.userA, `INSERT INTO public.cardgroups (owner_id, name) VALUES ($1, $2)`, []any{fx.userA, "sixth deck"}},
		{"cards_insert", fx.userA, insertCardSQL(), []any{uuid.NewString(), fx.groupA, "front", "Back"}},
		{"cards_delete", fx.userA, `DELETE FROM public.cards WHERE id = $1`, []any{fx.cardA}},
		{"swipe_records_insert", fx.userA, insertSwipeSQL(), []any{fx.userA, fx.cardA, fx.groupA, time.Now().UTC()}},
		{"user_card_fsrs_insert", fx.userA, insertUserCardFSRSSQL(), []any{fx.userA, fx.cardB, time.Now().UTC()}},
		{"user_preferences_insert", fx.userC, insertUserPreferencesSQL(), []any{fx.userC}},
		{"roles_admin_insert", fx.adminUser, `INSERT INTO public.roles (name) VALUES ($1)`, []any{uniqueName("bypass-role")}},
		{"master_cardgroups_admin_insert", fx.adminUser, `INSERT INTO public.master_cardgroups (name) VALUES ($1)`, []any{"bypass master"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			execPrivilegeDeniedAs(t, ctx, authPool, tc.userID, tc.query, tc.args...)
		})
	}
}

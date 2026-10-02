package database_test

import (
	"context"
	"database/sql"
	"testing"

	"github.com/google/uuid"
)

// masterRLSFixture holds the privileged-seeded rows and the two caller
// identities the admin-only RLS policies on master_cardgroups / master_cards
// must distinguish: a plain authenticated user (denied) and an admin (allowed).
type masterRLSFixture struct {
	nonAdminUser string
	adminUser    string
	groupID      string // master_cardgroups row seeded via the privileged connection
	cardID       string // master_cards row seeded via the privileged connection
}

// TestMasterTablesRLS_AdminOnly checks admin-only reads and write policies.
// openAuthenticatedPool opens the non-privileged "authenticated" login for SELECT
// visibility; execOKAs / execDeniedAs run on the policy-probe pool, which re-grants
// writes inside a rolled-back transaction because the API roles hold none since
// 20260927000001_revoke_client_writes. Owner-seeded rows prevent empty-table passes.
func TestMasterTablesRLS_AdminOnly(t *testing.T) {
	ctx := context.Background()
	db := openMigratedDB(t)
	defer db.Close()

	fx := createMasterRLSFixture(t, ctx, sqlDBForTest(t, db))

	authPool := openAuthenticatedPool(t, ctx)
	defer authPool.Close()

	probePool := openPolicyProbePool(t, ctx)
	defer probePool.Close()

	t.Run("master_cardgroups", func(t *testing.T) {
		// Anti-false-green: the privileged connection seeded fx.groupID, so a
		// count of 0 here proves RLS denied the read, not that the table is empty.
		assertCount(t, queryCountAs(t, ctx, authPool, fx.nonAdminUser,
			`SELECT count(*) FROM public.master_cardgroups WHERE id = $1`, fx.groupID), 0)
		// Admin sees the same seeded row.
		assertCount(t, queryCountAs(t, ctx, authPool, fx.adminUser,
			`SELECT count(*) FROM public.master_cardgroups WHERE id = $1`, fx.groupID), 1)

		// Non-admin INSERT is denied by the WITH CHECK clause.
		execDeniedAs(t, ctx, probePool, fx.nonAdminUser,
			`INSERT INTO public.master_cardgroups (name) VALUES ($1)`, "non-admin blocked group")
		// Admin INSERT is allowed.
		assertRows(t, execOKAs(t, ctx, probePool, fx.adminUser,
			`INSERT INTO public.master_cardgroups (name) VALUES ($1)`, "admin created group"), 1)
	})

	t.Run("master_cards", func(t *testing.T) {
		// Anti-false-green: fx.cardID was seeded via the privileged connection.
		assertCount(t, queryCountAs(t, ctx, authPool, fx.nonAdminUser,
			`SELECT count(*) FROM public.master_cards WHERE id = $1`, fx.cardID), 0)
		// Admin sees the same seeded row.
		assertCount(t, queryCountAs(t, ctx, authPool, fx.adminUser,
			`SELECT count(*) FROM public.master_cards WHERE id = $1`, fx.cardID), 1)

		// Non-admin INSERT is denied by the WITH CHECK clause.
		execDeniedAs(t, ctx, probePool, fx.nonAdminUser, insertMasterCardSQL(),
			uuid.NewString(), fx.groupID, "non-admin blocked card front", "Back")
		// Admin INSERT is allowed.
		assertRows(t, execOKAs(t, ctx, probePool, fx.adminUser, insertMasterCardSQL(),
			uuid.NewString(), fx.groupID, "admin created card front", "Back"), 1)
	})
}

func createMasterRLSFixture(t *testing.T, ctx context.Context, sqlDB *sql.DB) masterRLSFixture {
	t.Helper()

	// Two authenticated identities; only adminUser is granted the admin role.
	nonAdminUser := insertRLSAuthUser(t, ctx, sqlDB)
	adminUser := insertRLSAuthUser(t, ctx, sqlDB)

	var adminRoleID string
	if err := sqlDB.QueryRowContext(ctx,
		`SELECT id FROM public.roles WHERE name = 'admin'`).Scan(&adminRoleID); err != nil {
		t.Fatalf("query admin role: %v", err)
	}
	if _, err := sqlDB.ExecContext(ctx,
		`INSERT INTO public.user_roles (user_id, role_id) VALUES ($1, $2)`,
		adminUser, adminRoleID); err != nil {
		t.Fatalf("grant admin role: %v", err)
	}

	// Seed one master_cardgroups + one master_cards row via the PRIVILEGED
	// (migrate/owner) connection, which bypasses RLS. These rows exist so the
	// non-admin SELECT-denial assertions count 0 because RLS denied the read,
	// not because the table is empty (the anti-false-green guarantee).
	groupID := uuid.NewString()
	if _, err := sqlDB.ExecContext(ctx,
		`INSERT INTO public.master_cardgroups (id, name) VALUES ($1, $2)`,
		groupID, "seeded master group"); err != nil {
		t.Fatalf("seed master_cardgroups: %v", err)
	}

	cardID := uuid.NewString()
	if _, err := sqlDB.ExecContext(ctx, insertMasterCardSQL(),
		cardID, groupID, "seeded master card front", "Back"); err != nil {
		t.Fatalf("seed master_cards: %v", err)
	}

	return masterRLSFixture{
		nonAdminUser: nonAdminUser,
		adminUser:    adminUser,
		groupID:      groupID,
		cardID:       cardID,
	}
}

func insertMasterCardSQL() string {
	return `
        INSERT INTO public.master_cards (id, master_cardgroup_id, front, back)
        VALUES ($1, $2, $3, $4)
    `
}

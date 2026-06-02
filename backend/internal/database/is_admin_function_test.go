package database_test

import (
	"context"
	"database/sql"
	"fmt"
	"testing"

	"github.com/google/uuid"

	"backend/internal/database"
)

// sqlDBForTest unwraps the underlying *sql.DB from db.GORM, failing the test on
// error.
func sqlDBForTest(t *testing.T, db *database.DB) *sql.DB {
	t.Helper()
	sqlDB, err := db.GORM.DB()
	if err != nil {
		t.Fatalf("gorm.DB(): %v", err)
	}
	return sqlDB
}

// openMigratedDB migrates testDSN and returns an open *database.DB.
// The caller is responsible for calling db.Close().
func openMigratedDB(t *testing.T) *database.DB {
	t.Helper()
	if err := database.Migrate(testDSN); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	db, err := database.Open(context.Background(), database.Config{URL: testDSN, MaxConns: 4})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	return db
}

// insertAuthUserForAdmin inserts a row in auth.users and returns the uuid string.
// The corresponding public.users row is created automatically by the trigger.
func insertAuthUserForAdmin(t *testing.T, ctx context.Context, db *database.DB) string {
	t.Helper()
	id := uuid.NewString()
	sqlDB := sqlDBForTest(t, db)
	email := fmt.Sprintf("%s@test", id)
	if _, err := sqlDB.ExecContext(ctx,
		`INSERT INTO auth.users (id, email) VALUES ($1, $2)`, id, email); err != nil {
		t.Fatalf("insert auth.users: %v", err)
	}
	return id
}

// TestIsAdminFunction_True verifies that a user assigned the admin role is
// reported as admin by private.is_admin.
func TestIsAdminFunction_True(t *testing.T) {
	ctx := context.Background()
	db := openMigratedDB(t)
	defer db.Close()

	userID := insertAuthUserForAdmin(t, ctx, db)
	sqlDB := sqlDBForTest(t, db)

	// Look up the seeded admin role id.
	var adminRoleID string
	if err := sqlDB.QueryRowContext(ctx,
		`SELECT id FROM public.roles WHERE name = 'admin'`).Scan(&adminRoleID); err != nil {
		t.Fatalf("query admin role id: %v", err)
	}

	// Grant admin role to user.
	if _, err := sqlDB.ExecContext(ctx,
		`INSERT INTO public.user_roles (user_id, role_id) VALUES ($1, $2)`, userID, adminRoleID); err != nil {
		t.Fatalf("insert user_roles: %v", err)
	}

	var got bool
	if err := sqlDB.QueryRowContext(ctx,
		`SELECT private.is_admin($1::uuid)`, userID).Scan(&got); err != nil {
		t.Fatalf("query is_admin: %v", err)
	}
	if !got {
		t.Fatal("is_admin: got false, want true for user with admin role")
	}
}

// TestIsAdminFunction_FalseWhenNoRole verifies that a user without any role
// assignment is not reported as admin.
func TestIsAdminFunction_FalseWhenNoRole(t *testing.T) {
	ctx := context.Background()
	db := openMigratedDB(t)
	defer db.Close()

	userID := insertAuthUserForAdmin(t, ctx, db)
	sqlDB := sqlDBForTest(t, db)

	var got bool
	if err := sqlDB.QueryRowContext(ctx,
		`SELECT private.is_admin($1::uuid)`, userID).Scan(&got); err != nil {
		t.Fatalf("query is_admin: %v", err)
	}
	if got {
		t.Fatal("is_admin: got true, want false for user with no role")
	}
}

// TestIsAdminFunction_FalseWhenUnknownUser verifies that a uuid that does not
// correspond to any user returns false (not an error).
func TestIsAdminFunction_FalseWhenUnknownUser(t *testing.T) {
	ctx := context.Background()
	db := openMigratedDB(t)
	defer db.Close()

	sqlDB := sqlDBForTest(t, db)

	const unknownUID = "00000000-0000-0000-0000-000000000000"
	var got bool
	if err := sqlDB.QueryRowContext(ctx,
		`SELECT private.is_admin($1::uuid)`, unknownUID).Scan(&got); err != nil {
		t.Fatalf("query is_admin: %v", err)
	}
	if got {
		t.Fatal("is_admin: got true, want false for unknown user")
	}
}

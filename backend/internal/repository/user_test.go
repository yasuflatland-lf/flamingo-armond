package repository_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"gorm.io/gorm"

	"backend/internal/database"
	"backend/internal/repository"
)

var (
	testDSN string
	testDB  *database.DB
)

func TestMain(m *testing.M) {
	os.Exit(runTests(m))
}

func runTests(m *testing.M) int {
	ctx := context.Background()
	container, err := tcpostgres.Run(ctx,
		"postgres:15-alpine",
		tcpostgres.WithDatabase("flamingo_test"),
		tcpostgres.WithUsername("test"),
		tcpostgres.WithPassword("test"),
		// Required on macOS/Windows; the default wait is racy against postgres's
		// init-time restart and causes "connection reset by peer" on first connect.
		tcpostgres.BasicWaitStrategies(),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "run container: %v\n", err)
		return 1
	}
	defer func() {
		if err := testcontainers.TerminateContainer(container); err != nil {
			fmt.Fprintf(os.Stderr, "terminate container: %v\n", err)
		}
	}()

	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		fmt.Fprintf(os.Stderr, "conn string: %v\n", err)
		return 1
	}
	if err := bootstrapAuthSchema(ctx, dsn); err != nil {
		fmt.Fprintf(os.Stderr, "bootstrap auth schema: %v\n", err)
		return 1
	}
	if err := database.Migrate(dsn); err != nil {
		fmt.Fprintf(os.Stderr, "migrate: %v\n", err)
		return 1
	}

	db, err := database.Open(ctx, database.Config{URL: dsn, MaxConns: 4})
	if err != nil {
		fmt.Fprintf(os.Stderr, "open db: %v\n", err)
		return 1
	}
	defer db.Close()

	testDSN = dsn
	testDB = db
	return m.Run()
}

// bootstrapAuthSchema mimics the Supabase-managed auth schema and roles just
// enough for FK, trigger, and RLS policy references in migrations to resolve.
func bootstrapAuthSchema(ctx context.Context, dsn string) error {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return err
	}
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return err
	}
	defer pool.Close()
	_, err = pool.Exec(ctx, `
        DO $$
        BEGIN
            IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'authenticated') THEN
                CREATE ROLE authenticated LOGIN PASSWORD 'test';
            END IF;
        END
        $$;
        CREATE SCHEMA IF NOT EXISTS auth;
        CREATE TABLE IF NOT EXISTS auth.users (
            id uuid PRIMARY KEY,
            email text
        );
        CREATE OR REPLACE FUNCTION auth.uid()
        RETURNS uuid
        LANGUAGE sql
        STABLE
        AS $$
            SELECT COALESCE(
                NULLIF(current_setting('request.jwt.claim.sub', true), ''),
                NULLIF(current_setting('request.jwt.claims', true), '')::jsonb ->> 'sub'
            )::uuid
        $$;
        GRANT USAGE ON SCHEMA auth TO authenticated;
        GRANT EXECUTE ON FUNCTION auth.uid() TO authenticated;
        GRANT USAGE ON SCHEMA public TO authenticated;
        ALTER DEFAULT PRIVILEGES IN SCHEMA public
            GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO authenticated;
        ALTER DEFAULT PRIVILEGES IN SCHEMA public
            GRANT USAGE, SELECT ON SEQUENCES TO authenticated;
    `)
	return err
}

// insertAuthUser inserts a fresh row in auth.users so the trigger creates a
// user. Returns the new user id (string uuid).
func insertAuthUser(t *testing.T, ctx context.Context) string {
	t.Helper()
	id := uuid.NewString()
	sqlDB, err := testDB.GORM.DB()
	if err != nil {
		t.Fatalf("gorm.DB(): %v", err)
	}
	if _, err := sqlDB.ExecContext(ctx, `INSERT INTO auth.users (id, email) VALUES ($1, $2)`, id, fmt.Sprintf("%s@test", id)); err != nil {
		t.Fatalf("insert auth.users: %v", err)
	}
	return id
}

func sqlDBHandle(t *testing.T) *sql.DB {
	t.Helper()
	sqlDB, err := testDB.GORM.DB()
	if err != nil {
		t.Fatalf("gorm.DB(): %v", err)
	}
	return sqlDB
}

func TestFindByID_Success(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	id := insertAuthUser(t, ctx)

	repo := repository.NewUserRepository(testDB.GORM)
	got, err := repo.FindByID(ctx, id)
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if got.ID != id {
		t.Fatalf("ID: got %q, want %q", got.ID, id)
	}
	if got.DisplayName != nil {
		t.Fatalf("DisplayName: want nil, got %v", *got.DisplayName)
	}
	if got.Bio.IsSet() {
		t.Fatalf("Bio: want IsSet=false, got Ptr=%v", got.Bio.Ptr())
	}
	if got.AvatarURL != nil {
		t.Fatalf("AvatarURL: want nil, got %v", *got.AvatarURL)
	}
	if got.CreatedAt.IsZero() || got.UpdatedAt.IsZero() {
		t.Fatalf("timestamps should be populated: created=%v updated=%v", got.CreatedAt, got.UpdatedAt)
	}
}

func TestAuthUserTriggerCreatesPublicUser(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	id := insertAuthUser(t, ctx)

	sqlDB := sqlDBHandle(t)
	var count int
	if err := sqlDB.QueryRowContext(ctx,
		`SELECT count(*) FROM public.users WHERE id = $1`, id).Scan(&count); err != nil {
		t.Fatalf("count public.users: %v", err)
	}
	if count != 1 {
		t.Fatalf("handle_new_user created %d public.users rows, want 1", count)
	}
}

func TestMigrationsExposeUsersRolesAndNoProfilesTable(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	sqlDB := sqlDBHandle(t)
	rows, err := sqlDB.QueryContext(ctx, `
        SELECT table_name
        FROM information_schema.tables
        WHERE table_schema = 'public' AND table_type = 'BASE TABLE'
    `)
	if err != nil {
		t.Fatalf("query public tables: %v", err)
	}
	defer rows.Close()

	got := map[string]bool{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatalf("scan table name: %v", err)
		}
		got[name] = true
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate table names: %v", err)
	}

	for _, name := range []string{"users", "roles", "user_roles", "schema_migrations"} {
		if !got[name] {
			t.Fatalf("expected public.%s table after migrations; got %v", name, got)
		}
	}
	if got["profiles"] {
		t.Fatalf("public.profiles should not exist after migrations; got %v", got)
	}
}

func TestFindByID_NotFound(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := repository.NewUserRepository(testDB.GORM)
	_, err := repo.FindByID(ctx, uuid.NewString())
	if !errors.Is(err, repository.ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

func TestUpdate_Success_DisplayNameOnly(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	id := insertAuthUser(t, ctx)
	repo := repository.NewUserRepository(testDB.GORM)

	before, err := repo.FindByID(ctx, id)
	if err != nil {
		t.Fatalf("FindByID (before): %v", err)
	}
	// Ensure now() advances past the created_at default when the trigger fires.
	time.Sleep(5 * time.Millisecond)

	name := "Alice"
	got, err := repo.Update(ctx, id, repository.UserUpdate{DisplayName: &name})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if got.DisplayName == nil || string(*got.DisplayName) != name {
		t.Fatalf("DisplayName: got %v, want %q", got.DisplayName, name)
	}
	if got.Bio.IsSet() {
		t.Fatalf("Bio should remain IsSet=false, got Ptr=%v", got.Bio.Ptr())
	}
	if got.AvatarURL != nil {
		t.Fatalf("AvatarURL should remain nil, got %v", *got.AvatarURL)
	}
	if !got.UpdatedAt.After(before.UpdatedAt) {
		t.Fatalf("UpdatedAt should advance: before=%v after=%v", before.UpdatedAt, got.UpdatedAt)
	}
	if !got.CreatedAt.Equal(before.CreatedAt) {
		t.Fatalf("CreatedAt should be stable: before=%v after=%v", before.CreatedAt, got.CreatedAt)
	}
}

func TestUpdate_PartialBioOnly(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	id := insertAuthUser(t, ctx)
	repo := repository.NewUserRepository(testDB.GORM)

	name := "Bob"
	if _, err := repo.Update(ctx, id, repository.UserUpdate{DisplayName: &name}); err != nil {
		t.Fatalf("seed Update: %v", err)
	}

	bio := "hello world"
	got, err := repo.Update(ctx, id, repository.UserUpdate{Bio: &bio})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if !got.Bio.IsSet() || got.Bio.Ptr() == nil || *got.Bio.Ptr() != bio {
		t.Fatalf("Bio: got Ptr=%v, want %q", got.Bio.Ptr(), bio)
	}
	if got.DisplayName == nil || string(*got.DisplayName) != name {
		t.Fatalf("DisplayName should remain %q, got %v", name, got.DisplayName)
	}
}

func TestUpdate_NotFound(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := repository.NewUserRepository(testDB.GORM)
	name := "nobody"
	_, err := repo.Update(ctx, uuid.NewString(), repository.UserUpdate{DisplayName: &name})
	if !errors.Is(err, repository.ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

func TestUpdate_EmptyStringClearsField(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	id := insertAuthUser(t, ctx)
	repo := repository.NewUserRepository(testDB.GORM)

	name := "x"
	if _, err := repo.Update(ctx, id, repository.UserUpdate{DisplayName: &name}); err != nil {
		t.Fatalf("seed Update: %v", err)
	}

	empty := ""
	got, err := repo.Update(ctx, id, repository.UserUpdate{DisplayName: &empty})
	if err != nil {
		t.Fatalf("Update (clear): %v", err)
	}
	// Cleared field persists as empty string, not NULL — the pointer stays
	// non-nil and dereferences to "".
	if got.DisplayName == nil {
		t.Fatalf("DisplayName should be non-nil empty string, got nil")
	}
	if string(*got.DisplayName) != "" {
		t.Fatalf("DisplayName: got %q, want empty", *got.DisplayName)
	}
}

func TestUser_OnDeleteCascade(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	id := insertAuthUser(t, ctx)
	repo := repository.NewUserRepository(testDB.GORM)

	if _, err := repo.FindByID(ctx, id); err != nil {
		t.Fatalf("precondition FindByID: %v", err)
	}

	sqlDB := sqlDBHandle(t)
	if _, err := sqlDB.ExecContext(ctx, `DELETE FROM auth.users WHERE id = $1`, id); err != nil {
		t.Fatalf("delete auth.users: %v", err)
	}

	if _, err := repo.FindByID(ctx, id); !errors.Is(err, repository.ErrNotFound) {
		t.Fatalf("want ErrNotFound after cascade, got %v", err)
	}
}

func TestUpdate_EmptyPatchReturnsCurrentRow(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	id := insertAuthUser(t, ctx)

	repo := repository.NewUserRepository(testDB.GORM)

	// 1) Fetch the row and record the baseline.
	before, err := repo.FindByID(ctx, id)
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	beforeUpdated := before.UpdatedAt

	// 2) Empty patch must not touch the DB, so updated_at stays the same.
	after, err := repo.Update(ctx, id, repository.UserUpdate{})
	if err != nil {
		t.Fatalf("Update with empty patch: %v", err)
	}
	if !after.UpdatedAt.Equal(beforeUpdated) {
		t.Errorf("empty patch should not bump updated_at; before=%v after=%v", beforeUpdated, after.UpdatedAt)
	}
	if after.ID != before.ID {
		t.Errorf("ID mismatch")
	}
}

// TestUpdateTx_Success_DisplayNameOnly exercises the happy path of the
// transactional variant. Unlike Update, UpdateTx does not re-fetch and
// returns no value — callers refetch after the transaction commits.
func TestUpdateTx_Success_DisplayNameOnly(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	id := insertAuthUser(t, ctx)
	repo := repository.NewUserRepository(testDB.GORM)

	name := "AliceTx"
	err := testDB.GORM.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return repo.UpdateTx(ctx, tx, id, repository.UserUpdate{DisplayName: &name})
	})
	if err != nil {
		t.Fatalf("UpdateTx: %v", err)
	}

	got, err := repo.FindByID(ctx, id)
	if err != nil {
		t.Fatalf("FindByID after UpdateTx: %v", err)
	}
	if got.DisplayName == nil || string(*got.DisplayName) != name {
		t.Fatalf("DisplayName: got %v, want %q", got.DisplayName, name)
	}
}

// TestUpdateTx_NotFound asserts that targeting a missing row returns
// ErrNotFound so the surrounding transaction can roll back atomically.
// This is load-bearing for adminUserUsecase.EditUser which classifies the
// sentinel into an InputValidationError on field=id.
func TestUpdateTx_NotFound(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := repository.NewUserRepository(testDB.GORM)

	name := "nobody"
	missing := uuid.NewString()
	err := testDB.GORM.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return repo.UpdateTx(ctx, tx, missing, repository.UserUpdate{DisplayName: &name})
	})
	if !errors.Is(err, repository.ErrNotFound) {
		t.Fatalf("UpdateTx(missing): want ErrNotFound, got %v", err)
	}
}

// TestUpdateTx_EmptyPatchNoOp verifies that a patch with no non-nil fields
// returns nil without touching the database (no UPDATE issued). This matches
// adminUserUsecase.EditUser's profile-omitted branch where UpdateTx must not
// be called or, if called defensively, must be a no-op.
func TestUpdateTx_EmptyPatchNoOp(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	id := insertAuthUser(t, ctx)
	repo := repository.NewUserRepository(testDB.GORM)

	before, err := repo.FindByID(ctx, id)
	if err != nil {
		t.Fatalf("FindByID (before): %v", err)
	}
	beforeUpdated := before.UpdatedAt

	err = testDB.GORM.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return repo.UpdateTx(ctx, tx, id, repository.UserUpdate{})
	})
	if err != nil {
		t.Fatalf("UpdateTx (empty patch): %v", err)
	}

	after, err := repo.FindByID(ctx, id)
	if err != nil {
		t.Fatalf("FindByID (after): %v", err)
	}
	if !after.UpdatedAt.Equal(beforeUpdated) {
		t.Errorf("empty patch must not bump updated_at; before=%v after=%v", beforeUpdated, after.UpdatedAt)
	}
}

// insertNAuthUsers inserts n auth users and returns their ids.
func insertNAuthUsers(t *testing.T, ctx context.Context, n int) []string {
	t.Helper()
	ids := make([]string, n)
	for i := range ids {
		ids[i] = insertAuthUser(t, ctx)
	}
	return ids
}

func TestFindByIDs_AllFound(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	ids := insertNAuthUsers(t, ctx, 4)
	query := ids[:3]

	repo := repository.NewUserRepository(testDB.GORM)
	got, err := repo.FindByIDs(ctx, query)
	if err != nil {
		t.Fatalf("FindByIDs: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("map size: got %d, want 3", len(got))
	}
	for _, id := range query {
		p, ok := got[id]
		if !ok {
			t.Fatalf("missing id %q in result", id)
		}
		if p.ID != id {
			t.Fatalf("user ID mismatch: got %q, want %q", p.ID, id)
		}
	}
	if _, ok := got[ids[3]]; ok {
		t.Fatalf("unexpected id %q present in result", ids[3])
	}
}

func TestFindByIDs_PartialMissing(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	ids := insertNAuthUsers(t, ctx, 3)
	missing := "00000000-0000-0000-0000-000000000001"
	query := append(ids, missing)

	repo := repository.NewUserRepository(testDB.GORM)
	got, err := repo.FindByIDs(ctx, query)
	if err != nil {
		t.Fatalf("FindByIDs: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("map size: got %d, want 3", len(got))
	}
	for _, id := range ids {
		if _, ok := got[id]; !ok {
			t.Fatalf("missing expected id %q in result", id)
		}
	}
	if _, ok := got[missing]; ok {
		t.Fatalf("missing id %q should not appear in result", missing)
	}
}

func TestFindByIDs_EmptySlice(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	repo := repository.NewUserRepository(testDB.GORM)
	got, err := repo.FindByIDs(ctx, []string{})
	if err != nil {
		t.Fatalf("FindByIDs(empty): %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected empty map, got %d entries", len(got))
	}
}

func TestFindByIDs_DBError(t *testing.T) {
	t.Parallel()

	// A pre-cancelled context forces the underlying driver to fail.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	repo := repository.NewUserRepository(testDB.GORM)
	_, err := repo.FindByIDs(ctx, []string{"00000000-0000-0000-0000-000000000002"})
	if err == nil {
		t.Fatal("expected error for cancelled context, got nil")
	}
}

func TestRoleRepository_FindByName_SeededAdmin(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	repo := repository.NewRoleRepository(testDB.GORM)
	got, err := repo.FindByName(ctx, "admin")
	if err != nil {
		t.Fatalf("FindByName(admin): %v", err)
	}
	if got.ID == "" {
		t.Fatal("admin role ID is empty")
	}
	if got.Name != "admin" {
		t.Fatalf("role name = %q, want admin", got.Name)
	}
}

func TestRoleRepository_FindByIDs(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	repo := repository.NewRoleRepository(testDB.GORM)
	admin, err := repo.FindByName(ctx, "admin")
	if err != nil {
		t.Fatalf("FindByName(admin): %v", err)
	}

	got, err := repo.FindByIDs(ctx, []string{admin.ID})
	if err != nil {
		t.Fatalf("FindByIDs: %v", err)
	}
	if got[admin.ID] == nil || got[admin.ID].Name != "admin" {
		t.Fatalf("FindByIDs missing admin role: %+v", got)
	}
}

func TestRoleRepository_NotFound(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	repo := repository.NewRoleRepository(testDB.GORM)
	_, err := repo.FindByName(ctx, "missing")
	if !errors.Is(err, repository.ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

func TestUserRoleRepository_HasRole(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	userID := insertAuthUser(t, ctx)

	roleRepo := repository.NewRoleRepository(testDB.GORM)
	admin, err := roleRepo.FindByName(ctx, "admin")
	if err != nil {
		t.Fatalf("FindByName(admin): %v", err)
	}

	userRoleRepo := repository.NewUserRoleRepository(testDB.GORM)
	hasRole, err := userRoleRepo.HasRole(ctx, userID, "admin")
	if err != nil {
		t.Fatalf("HasRole before insert: %v", err)
	}
	if hasRole {
		t.Fatal("new user should not have admin role")
	}

	sqlDB := sqlDBHandle(t)
	if _, err := sqlDB.ExecContext(ctx,
		`INSERT INTO public.user_roles (user_id, role_id) VALUES ($1, $2)`, userID, admin.ID); err != nil {
		t.Fatalf("insert user_roles: %v", err)
	}

	hasRole, err = userRoleRepo.HasRole(ctx, userID, "admin")
	if err != nil {
		t.Fatalf("HasRole after insert: %v", err)
	}
	if !hasRole {
		t.Fatal("expected user to have admin role")
	}
}

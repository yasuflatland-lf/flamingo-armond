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

	"backend/internal/database"
	"backend/internal/repository"
)

var (
	testDSN string
	testDB  *database.DB
)

func TestMain(m *testing.M) {
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
		fail("testcontainer postgres run: %v", err)
	}
	defer testcontainers.CleanupContainer(nil, container)

	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		fail("connection string: %v", err)
	}
	if err := bootstrapAuthSchema(ctx, dsn); err != nil {
		fail("bootstrap auth schema: %v", err)
	}
	if err := database.Migrate(dsn); err != nil {
		fail("migrate: %v", err)
	}

	db, err := database.Open(ctx, database.Config{URL: dsn, MaxConns: 4})
	if err != nil {
		fail("open db: %v", err)
	}
	defer db.Close()

	testDSN = dsn
	testDB = db
	os.Exit(m.Run())
}

func fail(format string, a ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", a...)
	os.Exit(1)
}

// bootstrapAuthSchema mimics the Supabase-managed auth.users table just enough
// for FK and trigger references in our migrations to resolve.
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
        CREATE SCHEMA IF NOT EXISTS auth;
        CREATE TABLE IF NOT EXISTS auth.users (
            id uuid PRIMARY KEY,
            email text
        );
    `)
	return err
}

// insertAuthUser inserts a fresh row in auth.users so the trigger creates a
// profile. Returns the new user id (string uuid).
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

	repo := repository.NewProfileRepository(testDB.GORM)
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
	if got.Bio != nil {
		t.Fatalf("Bio: want nil, got %v", *got.Bio)
	}
	if got.AvatarURL != nil {
		t.Fatalf("AvatarURL: want nil, got %v", *got.AvatarURL)
	}
	if got.CreatedAt.IsZero() || got.UpdatedAt.IsZero() {
		t.Fatalf("timestamps should be populated: created=%v updated=%v", got.CreatedAt, got.UpdatedAt)
	}
}

func TestFindByID_NotFound(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := repository.NewProfileRepository(testDB.GORM)
	_, err := repo.FindByID(ctx, uuid.NewString())
	if !errors.Is(err, repository.ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

func TestUpdate_Success_DisplayNameOnly(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	id := insertAuthUser(t, ctx)
	repo := repository.NewProfileRepository(testDB.GORM)

	before, err := repo.FindByID(ctx, id)
	if err != nil {
		t.Fatalf("FindByID (before): %v", err)
	}
	// Ensure now() advances past the created_at default when the trigger fires.
	time.Sleep(5 * time.Millisecond)

	name := "Alice"
	got, err := repo.Update(ctx, id, repository.ProfileUpdate{DisplayName: &name})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if got.DisplayName == nil || *got.DisplayName != name {
		t.Fatalf("DisplayName: got %v, want %q", got.DisplayName, name)
	}
	if got.Bio != nil {
		t.Fatalf("Bio should remain nil, got %v", *got.Bio)
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
	repo := repository.NewProfileRepository(testDB.GORM)

	name := "Bob"
	if _, err := repo.Update(ctx, id, repository.ProfileUpdate{DisplayName: &name}); err != nil {
		t.Fatalf("seed Update: %v", err)
	}

	bio := "hello world"
	got, err := repo.Update(ctx, id, repository.ProfileUpdate{Bio: &bio})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if got.Bio == nil || *got.Bio != bio {
		t.Fatalf("Bio: got %v, want %q", got.Bio, bio)
	}
	if got.DisplayName == nil || *got.DisplayName != name {
		t.Fatalf("DisplayName should remain %q, got %v", name, got.DisplayName)
	}
}

func TestUpdate_NotFound(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := repository.NewProfileRepository(testDB.GORM)
	name := "nobody"
	_, err := repo.Update(ctx, uuid.NewString(), repository.ProfileUpdate{DisplayName: &name})
	if !errors.Is(err, repository.ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

func TestUpdate_EmptyStringClearsField(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	id := insertAuthUser(t, ctx)
	repo := repository.NewProfileRepository(testDB.GORM)

	name := "x"
	if _, err := repo.Update(ctx, id, repository.ProfileUpdate{DisplayName: &name}); err != nil {
		t.Fatalf("seed Update: %v", err)
	}

	empty := ""
	got, err := repo.Update(ctx, id, repository.ProfileUpdate{DisplayName: &empty})
	if err != nil {
		t.Fatalf("Update (clear): %v", err)
	}
	// Cleared field persists as empty string, not NULL — the pointer stays
	// non-nil and dereferences to "".
	if got.DisplayName == nil {
		t.Fatalf("DisplayName should be non-nil empty string, got nil")
	}
	if *got.DisplayName != "" {
		t.Fatalf("DisplayName: got %q, want empty", *got.DisplayName)
	}
}

func TestProfile_OnDeleteCascade(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	id := insertAuthUser(t, ctx)
	repo := repository.NewProfileRepository(testDB.GORM)

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

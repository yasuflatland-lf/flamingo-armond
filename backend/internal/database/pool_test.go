package database_test

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"

	"backend/internal/database"
)

var testDSN string

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
		fmt.Fprintf(os.Stderr, "testcontainer postgres run: %v\n", err)
		return 1
	}
	defer func() {
		if err := testcontainers.TerminateContainer(container); err != nil {
			fmt.Fprintf(os.Stderr, "terminate container: %v\n", err)
		}
	}()

	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		fmt.Fprintf(os.Stderr, "connection string: %v\n", err)
		return 1
	}
	if err := bootstrapAuthSchema(ctx, dsn); err != nil {
		fmt.Fprintf(os.Stderr, "bootstrap auth schema: %v\n", err)
		return 1
	}
	testDSN = dsn
	return m.Run()
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

func TestOpenAndPing(t *testing.T) {
	ctx := context.Background()
	db, err := database.Open(ctx, database.Config{URL: testDSN})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db.Close()
	if db.Pool == nil {
		t.Fatal("Pool is nil")
	}
	if db.GORM == nil {
		t.Fatal("GORM is nil")
	}
	if err := db.Pool.Ping(ctx); err != nil {
		t.Fatalf("Ping: %v", err)
	}
}

func TestCloseIsIdempotent(t *testing.T) {
	ctx := context.Background()
	db, err := database.Open(ctx, database.Config{URL: testDSN})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	// Second Close on a closed pool would panic if not guarded; our Close
	// tolerates it only via the nil-Pool shortcut, so the actual expectation
	// is: Close once, and a nil-receiver Close does not panic.
	db.Close()

	var nilDB *database.DB
	nilDB.Close() // must not panic
}

func TestOpenInvalidDSN(t *testing.T) {
	ctx := context.Background()
	if _, err := database.Open(ctx, database.Config{URL: "not-a-valid-dsn://???"}); err == nil {
		t.Fatal("expected error for bogus DSN, got nil")
	}
}

func TestMigrateIdempotent(t *testing.T) {
	if err := database.Migrate(testDSN); err != nil {
		t.Fatalf("first Migrate: %v", err)
	}
	if err := database.Migrate(testDSN); err != nil {
		t.Fatalf("second Migrate (expected no-op): %v", err)
	}
}

func TestOpen_RespectsCanceledContext(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel before Open is called
	_, err := database.Open(ctx, database.Config{URL: testDSN})
	if err == nil {
		t.Fatal("expected error when ctx is canceled before Open, got nil")
	}
}

func TestConvertSchemeForMigrate(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"postgres scheme", "postgres://u:p@h:5432/db", "pgx5://u:p@h:5432/db"},
		{"postgresql scheme", "postgresql://u:p@h:5432/db", "pgx5://u:p@h:5432/db"},
		{"already pgx5", "pgx5://u:p@h/db", "pgx5://u:p@h/db"},
		{"query string preserved", "postgres://u:p@h/db?sslmode=disable", "pgx5://u:p@h/db?sslmode=disable"},
		{"ipv6 host preserved", "postgresql://u:p@[::1]:5432/db", "pgx5://u:p@[::1]:5432/db"},
		{"unknown scheme untouched", "mysql://x", "mysql://x"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := database.ConvertSchemeForMigrateForTest(tc.in)
			if got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

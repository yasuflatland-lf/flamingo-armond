package database_test

import (
	"context"
	"database/sql"
	"strings"
	"testing"

	"backend/internal/database"
)

// textLengthCheckClause returns the rendered CHECK expression of a named
// constraint, or false when the constraint does not exist.
func textLengthCheckClause(t *testing.T, ctx context.Context, sqlDB *sql.DB, table, constraint string) (string, bool) {
	t.Helper()
	var clause string
	err := sqlDB.QueryRowContext(ctx, `
		SELECT pg_get_constraintdef(c.oid)
		FROM pg_constraint c
		JOIN pg_class t ON t.oid = c.conrelid
		JOIN pg_namespace n ON n.oid = t.relnamespace
		WHERE n.nspname = 'public'
		  AND t.relname = $1
		  AND c.conname = $2
	`, table, constraint).Scan(&clause)
	if err == sql.ErrNoRows {
		return "", false
	}
	if err != nil {
		t.Fatalf("query constraint %s.%s: %v", table, constraint, err)
	}
	return clause, true
}

func requireCheckUpperBound(t *testing.T, ctx context.Context, sqlDB *sql.DB, table, constraint, want string) {
	t.Helper()
	clause, ok := textLengthCheckClause(t, ctx, sqlDB, table, constraint)
	if !ok {
		t.Fatalf("constraint %s.%s does not exist", table, constraint)
	}
	if !strings.Contains(clause, want) {
		t.Fatalf("constraint %s.%s = %q, want it to contain %q", table, constraint, clause, want)
	}
}

// widenedTextLengthChecks pins the six constraints the migration rewrites, with
// the code-point upper bound each carries at HEAD (20x the domain grapheme cap:
// 500 -> 10000 for card text, 100 -> 2000 for deck names).
var widenedTextLengthChecks = []struct {
	table      string
	constraint string
	wantUpper  string
	wantLegacy string
}{
	{"cardgroups", "cardgroups_name_length", "2000", "100"},
	{"cards", "cards_front_length", "10000", "500"},
	{"cards", "cards_back_length", "10000", "500"},
	{"master_cardgroups", "master_cardgroups_name_length", "2000", "100"},
	{"master_cards", "master_cards_front_length", "10000", "500"},
	{"master_cards", "master_cards_back_length", "10000", "500"},
}

// TestWidenTextLengthChecksDownUpRoundtrip proves the newest migration restores
// the original narrow bounds on the way down and the widened bounds on the way
// back up, for all six text-length constraints, without leaving the schema
// behind the latest migration version.
//
// t.Parallel() is intentionally absent: the test runs a global migration
// down/up that would race other tests in the package.
func TestWidenTextLengthChecksDownUpRoundtrip(t *testing.T) {
	ctx := context.Background()
	db := openMigratedDB(t)
	defer db.Close()
	t.Cleanup(func() {
		if err := database.Migrate(testDSN); err != nil {
			t.Errorf("restore latest migration: %v", err)
		}
	})

	sqlDB := sqlDBForTest(t, db)
	for _, c := range widenedTextLengthChecks {
		requireCheckUpperBound(t, ctx, sqlDB, c.table, c.constraint, c.wantUpper)
	}

	m, err := database.NewMigrateInstanceForTest(testDSN)
	if err != nil {
		t.Fatalf("NewMigrateInstanceForTest: %v", err)
	}
	defer func() {
		if srcErr, dbErr := m.Close(); srcErr != nil || dbErr != nil {
			t.Logf("migrate close: src_err=%v db_err=%v", srcErr, dbErr)
		}
	}()

	// widen_updated_at_triggers_to_insert sits above widen_text_length_checks, so
	// two steps reach the target. Bump this count when adding later migrations.
	if err := m.Steps(-2); err != nil {
		t.Fatalf("migrate down widen_text_length_checks: %v", err)
	}
	for _, c := range widenedTextLengthChecks {
		requireCheckUpperBound(t, ctx, sqlDB, c.table, c.constraint, c.wantLegacy)
	}

	if err := m.Steps(1); err != nil {
		t.Fatalf("migrate up widen_text_length_checks: %v", err)
	}
	for _, c := range widenedTextLengthChecks {
		requireCheckUpperBound(t, ctx, sqlDB, c.table, c.constraint, c.wantUpper)
	}
}

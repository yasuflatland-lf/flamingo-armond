package database_test

import (
	"context"
	"database/sql"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/rotisserie/eris"

	"backend/internal/database"
)

// indexExists returns true if a relation named indexName exists in
// public's pg_indexes. Used by both directions of the migration test.
func indexExists(t *testing.T, ctx context.Context, sqlDB *sql.DB, indexName string) bool {
	t.Helper()
	var count int
	if err := sqlDB.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM pg_indexes WHERE schemaname = 'public' AND indexname = $1`,
		indexName,
	).Scan(&count); err != nil {
		t.Fatalf("query pg_indexes for %q: %v", indexName, err)
	}
	return count == 1
}

// insertCardForUpsertTest inserts a minimal valid card row directly via SQL,
// bypassing any application-layer constraint logic. The returned id lets
// the caller clean up afterwards.
func insertCardForUpsertTest(ctx context.Context, sqlDB *sql.DB, cardgroupID, front string) (string, error) {
	id := uuid.NewString()
	if _, err := sqlDB.ExecContext(ctx, insertCardSQL(),
		id, cardgroupID, front, "Back", time.Now().UTC()); err != nil {
		return "", eris.Wrap(err, "test: insert card")
	}
	return id, nil
}

// TestCardsUpsertIndex_CleanDBSucceeds exercises the happy path: with no
// duplicates present, the migration creates uq_cards_cardgroup_front, and
// the down migration removes it. The test leaves the DB in the migrated-up
// state so subsequent tests in the package see a consistent schema.
func TestCardsUpsertIndex_CleanDBSucceeds(t *testing.T) {
	ctx := context.Background()
	db := openMigratedDB(t)
	defer db.Close()

	sqlDB := sqlDBForTest(t, db)
	const indexName = "uq_cards_cardgroup_front"

	if !indexExists(t, ctx, sqlDB, indexName) {
		t.Fatalf("expected %q to exist after migrate up, but it does not", indexName)
	}

	// Roll the index migration back and confirm the index is gone.
	if err := database.MigrateStepsForTest(testDSN, -1); err != nil {
		t.Fatalf("migrate down 1 step: %v", err)
	}
	if indexExists(t, ctx, sqlDB, indexName) {
		t.Fatalf("expected %q to be dropped after migrate down, but it still exists", indexName)
	}

	// Re-apply so the DB ends in the migrated-up state for the rest of the suite.
	if err := database.MigrateStepsForTest(testDSN, 1); err != nil {
		t.Fatalf("migrate up 1 step (restore): %v", err)
	}
	if !indexExists(t, ctx, sqlDB, indexName) {
		t.Fatalf("expected %q to be re-created after migrate up, but it is missing", indexName)
	}
}

// TestCardsUpsertIndex_DuplicatesBlockMigration verifies the pre-check in
// the up migration: with a (cardgroup_id, front) duplicate present, the
// migration must abort with a message that calls out the duplication and
// the offending column names.
func TestCardsUpsertIndex_DuplicatesBlockMigration(t *testing.T) {
	ctx := context.Background()
	db := openMigratedDB(t)
	defer db.Close()

	sqlDB := sqlDBForTest(t, db)
	const indexName = "uq_cards_cardgroup_front"

	// Step 1: roll the index migration back so we can plant a duplicate
	// the unique index would otherwise reject. The index must be absent
	// for the INSERT below to succeed.
	if err := database.MigrateStepsForTest(testDSN, -1); err != nil {
		t.Fatalf("migrate down 1 step: %v", err)
	}
	if indexExists(t, ctx, sqlDB, indexName) {
		t.Fatalf("expected %q to be absent after migrate down, but it still exists", indexName)
	}

	// Step 2: create an owner + cardgroup, then insert two cards with
	// identical (cardgroup_id, front).
	ownerID := insertAuthUserForAdmin(t, ctx, db)
	groupID := uuid.NewString()
	if _, err := sqlDB.ExecContext(ctx,
		`INSERT INTO public.cardgroups (id, owner_id, name) VALUES ($1, $2, $3)`,
		groupID, ownerID, "upsert-index-test-group"); err != nil {
		t.Fatalf("insert cardgroup: %v", err)
	}

	const dupFront = "duplicate-front-text"
	cardA, err := insertCardForUpsertTest(ctx, sqlDB, groupID, dupFront)
	if err != nil {
		t.Fatalf("insert first card: %v", err)
	}
	cardB, err := insertCardForUpsertTest(ctx, sqlDB, groupID, dupFront)
	if err != nil {
		t.Fatalf("insert duplicate card: %v", err)
	}

	// Step 3: re-apply the migration. The pre-check should abort it with
	// a message that names the problem and the columns.
	upErr := database.MigrateStepsForTest(testDSN, 1)
	if upErr == nil {
		t.Fatal("expected migrate up to fail due to duplicate rows, got nil")
	}
	msg := strings.ToLower(upErr.Error())
	if !strings.Contains(msg, "duplicate") {
		t.Errorf("error %q does not mention 'duplicate'", upErr.Error())
	}
	if !strings.Contains(msg, "cardgroup_id") || !strings.Contains(msg, "front") {
		t.Errorf("error %q does not mention column names cardgroup_id and front", upErr.Error())
	}
	if indexExists(t, ctx, sqlDB, indexName) {
		t.Fatalf("expected %q to remain absent after a failed migration, but it exists", indexName)
	}

	// Step 4: clean up the duplicates, then re-apply so the DB ends in the
	// migrated-up state for the rest of the suite. golang-migrate marks the
	// failed version dirty; force the version back to the previous one
	// before re-running.
	for _, id := range []string{cardA, cardB} {
		if _, err := sqlDB.ExecContext(ctx, `DELETE FROM public.cards WHERE id = $1`, id); err != nil {
			t.Fatalf("cleanup card %s: %v", id, err)
		}
	}
	if _, err := sqlDB.ExecContext(ctx,
		`UPDATE public.schema_migrations SET dirty = false WHERE dirty = true`); err != nil {
		t.Fatalf("reset dirty flag: %v", err)
	}
	if err := database.MigrateStepsForTest(testDSN, 1); err != nil {
		t.Fatalf("migrate up 1 step (restore): %v", err)
	}
	if !indexExists(t, ctx, sqlDB, indexName) {
		t.Fatalf("expected %q to exist after restore migrate up, but it is missing", indexName)
	}
}

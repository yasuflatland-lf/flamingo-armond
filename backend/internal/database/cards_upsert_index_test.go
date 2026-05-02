package database_test

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/rotisserie/eris"

	"backend/internal/database"
)

// cardsUpsertIndexVersion is the integer timestamp embedded in the
// 20260503000000_add_cards_upsert_index migration file. New migrations added
// to this repo land at higher version numbers, so any migration "above"
// this one needs rolling off before the index migration itself can be
// stepped down.
const cardsUpsertIndexVersion int = 20260503000000

// migrationsAboveCardsUpsertIndex counts how many up-migrations sit strictly
// above the cards_upsert_index migration in the embedded source. Returning
// 0 keeps the historical "single -1 step toggles the index" behaviour;
// returning N > 0 lets the test peel off the newer migrations before
// touching the index pair.
//
// The function inspects the on-disk migrations directory (relative to the
// _test package) rather than parsing the embedded FS, because the embed.FS
// var is package-private. This is acceptable for test code: the migrations
// directory is right next to this file in the same package.
func migrationsAboveCardsUpsertIndex(t *testing.T) int {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(".", "migrations"))
	if err != nil {
		t.Fatalf("read migrations dir: %v", err)
	}
	seen := map[string]struct{}{}
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".up.sql") {
			continue
		}
		// Filename pattern: <version>_<name>.up.sql; the version is everything
		// up to the first underscore and parses as an int.
		us := strings.IndexByte(name, '_')
		if us <= 0 {
			continue
		}
		versionStr := name[:us]
		// Track unique versions; .down.sql / .up.sql pairs share a version.
		seen[versionStr] = struct{}{}
	}
	count := 0
	for v := range seen {
		// Strip leading zeros via direct numeric compare on the string after
		// pad to the same width as cardsUpsertIndexVersion. The timestamps in
		// this repo are all 14 digits already, so a lexicographic compare
		// matches a numeric compare.
		if len(v) == 14 && v > "20260503000000" {
			count++
		}
	}
	_ = cardsUpsertIndexVersion // kept for documentation parity with the constant
	return count
}

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

	// Roll forward off the index migration so we can roll the index migration
	// itself back. New migrations added after 20260503000000 (e.g.
	// 20260503000001) sit between the index migration and HEAD; rolling them
	// off first keeps this test pinned to the cards_upsert_index step pair.
	stepsAboveIndex := migrationsAboveCardsUpsertIndex(t)
	if stepsAboveIndex > 0 {
		if err := database.MigrateStepsForTest(testDSN, -stepsAboveIndex); err != nil {
			t.Fatalf("migrate down %d steps to top of cards_upsert_index: %v", stepsAboveIndex, err)
		}
	}

	// Roll the index migration back and confirm the index is gone.
	if err := database.MigrateStepsForTest(testDSN, -1); err != nil {
		t.Fatalf("migrate down 1 step: %v", err)
	}
	if indexExists(t, ctx, sqlDB, indexName) {
		t.Fatalf("expected %q to be dropped after migrate down, but it still exists", indexName)
	}

	// Re-apply so the DB ends in the migrated-up state for the rest of the suite.
	if err := database.MigrateStepsForTest(testDSN, 1+stepsAboveIndex); err != nil {
		t.Fatalf("migrate up %d step(s) (restore): %v", 1+stepsAboveIndex, err)
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

	// Roll forward off any post-index migrations first so the next -1 step
	// targets cards_upsert_index itself. See TestCardsUpsertIndex_CleanDBSucceeds
	// for the rationale; this test mirrors the same step accounting.
	stepsAboveIndex := migrationsAboveCardsUpsertIndex(t)
	if stepsAboveIndex > 0 {
		if err := database.MigrateStepsForTest(testDSN, -stepsAboveIndex); err != nil {
			t.Fatalf("migrate down %d steps to top of cards_upsert_index: %v", stepsAboveIndex, err)
		}
	}

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
	// failed migration dirty and records its version number. Clearing the
	// dirty flag alone leaves the version pointing at the failed migration,
	// so the subsequent Steps(1) call finds no successor file and errors.
	// Force the version back to the predecessor (20260502130000) first —
	// that is the idiomatic golang-migrate recovery path.
	for _, id := range []string{cardA, cardB} {
		if _, err := sqlDB.ExecContext(ctx, `DELETE FROM public.cards WHERE id = $1`, id); err != nil {
			t.Fatalf("cleanup card %s: %v", id, err)
		}
	}
	// 20260502130000 is the timestamp of the last successfully applied
	// migration before 20260503000000_add_cards_upsert_index.
	if err := database.MigrateForceForTest(testDSN, 20260502130000); err != nil {
		t.Fatalf("force version to predecessor: %v", err)
	}
	if err := database.MigrateStepsForTest(testDSN, 1+stepsAboveIndex); err != nil {
		t.Fatalf("migrate up %d step(s) (restore): %v", 1+stepsAboveIndex, err)
	}
	if !indexExists(t, ctx, sqlDB, indexName) {
		t.Fatalf("expected %q to exist after restore migrate up, but it is missing", indexName)
	}
}

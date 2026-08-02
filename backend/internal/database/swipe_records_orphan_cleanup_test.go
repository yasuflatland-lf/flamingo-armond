package database_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/google/uuid"

	"backend/internal/database"
)

// insertSwipeRecordWithID inserts one public.swipe_records row with a caller-chosen
// primary key so the roundtrip test can follow that exact row across a migration
// down/up cycle. The shared insertSwipeSQL() helper lets the database generate the
// id, which is fine for RLS assertions but useless when the row must be identified
// again after the schema has moved.
func insertSwipeRecordWithID(t *testing.T, ctx context.Context, sqlDB *sql.DB, id, userID, cardID, cardgroupID string) {
	t.Helper()
	now := time.Now().UTC()
	if _, err := sqlDB.ExecContext(ctx, `
        INSERT INTO public.swipe_records (
            id, user_id, card_id, cardgroup_id, rating, reviewed_at, due, stability, difficulty,
            scheduled_days, reps, lapses, state, last_review,
            due_before, phase_before, stability_before
        )
        VALUES ($1, $2, $3, $4, 3, $5, $5, 2.5, 5.0, 0, 0, 0, 0, $5, $5, 0, 2.5)
    `, id, userID, cardID, cardgroupID, now); err != nil {
		t.Fatalf("insert swipe record %s: %v", id, err)
	}
}

func insertLegacySwipeRecordWithID(t *testing.T, ctx context.Context, sqlDB *sql.DB, id, userID, cardID, cardgroupID string) {
	t.Helper()
	now := time.Now().UTC()
	if _, err := sqlDB.ExecContext(ctx, `
        INSERT INTO public.swipe_records (
            id, user_id, card_id, cardgroup_id, rating, reviewed_at, due, stability, difficulty,
            elapsed_days, scheduled_days, reps, lapses, state, last_review
        )
        VALUES ($1, $2, $3, $4, 3, $5, $5, 2.5, 5.0, 0, 0, 0, 0, 0, $5)
    `, id, userID, cardID, cardgroupID, now); err != nil {
		t.Fatalf("insert legacy swipe record %s: %v", id, err)
	}
}

// countSwipeRecordsByID returns how many public.swipe_records rows carry the id.
func countSwipeRecordsByID(t *testing.T, ctx context.Context, sqlDB *sql.DB, id string) int {
	t.Helper()
	var n int
	if err := sqlDB.QueryRowContext(ctx,
		`SELECT count(*) FROM public.swipe_records WHERE id = $1`, id).Scan(&n); err != nil {
		t.Fatalf("count swipe record %s: %v", id, err)
	}
	return n
}

// TestSwipeRecordsCardgroupOrphanCleanupRoundtrip pins the destructive statement in
// 20260721000000_add_cardgroup_fk_to_swipe_records.up.sql:
//
//	DELETE FROM public.swipe_records
//	WHERE cardgroup_id NOT IN (SELECT id FROM public.cardgroups);
//
// The down migration drops only the constraint and the index, so an operator
// down/up cycle re-runs that DELETE against live data. Both directions of the
// predicate are therefore asserted:
//
//  1. A legitimate row whose cardgroup_id references a real deck must survive the
//     roundtrip (assertion family "data survives the roundtrip"). A mistyped
//     predicate -- for example the adjacent FK column, cards instead of cardgroups
//     -- deletes every review row and would otherwise leave the suite green,
//     because ADD CONSTRAINT then trivially succeeds on an empty table.
//  2. A genuine orphan, insertable only while the foreign key is absent, must be
//     gone after the up step. That half fails if the DELETE is dropped or inverted.
//
// t.Parallel() is intentionally absent: the test runs a global migration down/up
// that would race other tests in the package.
func TestSwipeRecordsCardgroupOrphanCleanupRoundtrip(t *testing.T) {
	ctx := context.Background()
	db := openMigratedDB(t)
	// Cleanups run last-in-first-out, so registering the restore first makes it
	// run after the pool is closed; database.Migrate opens its own connection.
	t.Cleanup(func() {
		if err := database.Migrate(testDSN); err != nil {
			t.Errorf("restore latest migration: %v", err)
		}
	})
	t.Cleanup(db.Close)

	sqlDB := sqlDBForTest(t, db)

	ownerID := insertRLSAuthUser(t, ctx, sqlDB)
	// Deleting the auth user cascades through public.users to the cardgroup, the
	// card, and every swipe record seeded below. Registered after t.Cleanup(db.Close)
	// so it runs while the pool is still open.
	t.Cleanup(func() {
		if _, err := sqlDB.ExecContext(context.Background(),
			`DELETE FROM auth.users WHERE id = $1`, ownerID); err != nil {
			t.Errorf("cleanup seeded auth user: %v", err)
		}
	})
	cardgroupID := insertRLSCardgroup(t, ctx, sqlDB, ownerID, uniqueName("orphan-cleanup"))
	cardID := insertRLSCard(t, ctx, sqlDB, cardgroupID, uniqueName("orphan-cleanup-front"))

	keepID := uuid.NewString()
	insertSwipeRecordWithID(t, ctx, sqlDB, keepID, ownerID, cardID, cardgroupID)

	m, err := database.NewMigrateInstanceForTest(testDSN)
	if err != nil {
		t.Fatalf("NewMigrateInstanceForTest: %v", err)
	}
	defer func() {
		if srcErr, dbErr := m.Close(); srcErr != nil || dbErr != nil {
			t.Logf("migrate close: src_err=%v db_err=%v", srcErr, dbErr)
		}
	}()

	// Five steps reach add_cardgroup_fk_to_swipe_records:
	// tighten_new_card_ratio_check, realign_fsrs_snapshot_columns_to_v4,
	// widen_updated_at_triggers_to_insert,
	// and widen_text_length_checks sit above it. Bump this count when adding
	// migrations after any of them.
	if err := m.Steps(-5); err != nil {
		t.Fatalf("migrate down cardgroup fk migration: %v", err)
	}
	if got := countSwipeRecordsByID(t, ctx, sqlDB, keepID); got != 1 {
		t.Fatalf("legitimate swipe record count after down migration = %d, want 1", got)
	}

	// The foreign key is gone, so a dangling cardgroup_id is insertable here and
	// only here -- this is the window that manufactures the orphan the up
	// migration must clean up.
	orphanID := uuid.NewString()
	orphanCardgroupID := uuid.NewString()
	insertLegacySwipeRecordWithID(t, ctx, sqlDB, orphanID, ownerID, cardID, orphanCardgroupID)
	if got := countSwipeRecordsByID(t, ctx, sqlDB, orphanID); got != 1 {
		t.Fatalf("orphan swipe record count before up migration = %d, want 1", got)
	}

	if err := m.Steps(3); err != nil {
		t.Fatalf("migrate up cardgroup fk migration: %v", err)
	}
	if got := countSwipeRecordsByID(t, ctx, sqlDB, keepID); got != 1 {
		t.Fatalf("legitimate swipe record count after up migration = %d, want 1 (orphan cleanup deleted a non-orphan row)", got)
	}
	if got := countSwipeRecordsByID(t, ctx, sqlDB, orphanID); got != 0 {
		t.Fatalf("orphan swipe record count after up migration = %d, want 0 (orphan cleanup did not run)", got)
	}
}

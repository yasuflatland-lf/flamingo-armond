package database_test

import (
	"context"
	"database/sql"
	"strings"
	"testing"

	"github.com/google/uuid"
)

// indexIsUnique returns true if an index with the given name exists on
// public.cards AND pg_indexes reports it as a UNIQUE INDEX.
func indexIsUnique(t *testing.T, ctx context.Context, sqlDB *sql.DB, indexName string) bool {
	t.Helper()
	var indexdef string
	err := sqlDB.QueryRowContext(ctx,
		`SELECT indexdef FROM pg_indexes
		 WHERE schemaname = 'public' AND tablename = 'cards' AND indexname = $1`,
		indexName,
	).Scan(&indexdef)
	if err == sql.ErrNoRows {
		return false
	}
	if err != nil {
		t.Fatalf("query pg_indexes for %q: %v", indexName, err)
	}
	// pg_indexes.indexdef for a unique index contains "UNIQUE INDEX".
	// Example: "CREATE UNIQUE INDEX uq_cards_cardgroup_front ON public.cards USING btree (cardgroup_id, front)"
	return strings.Contains(indexdef, "UNIQUE INDEX")
}

// setupUpsertFixture inserts an auth.users row and a cardgroup owned by that
// user, returning the cardgroup UUID. The public.users row is created by the
// handle_new_user trigger.
func setupUpsertFixture(t *testing.T, ctx context.Context, sqlDB *sql.DB) string {
	t.Helper()
	ownerID := uuid.NewString()
	if _, err := sqlDB.ExecContext(ctx,
		`INSERT INTO auth.users (id, email) VALUES ($1, $2)`,
		ownerID, ownerID+"@upsert-spec"); err != nil {
		t.Fatalf("insert auth.users: %v", err)
	}
	groupID := uuid.NewString()
	if _, err := sqlDB.ExecContext(ctx,
		`INSERT INTO public.cardgroups (id, owner_id, name) VALUES ($1, $2, $3)`,
		groupID, ownerID, "upsert-spec-group"); err != nil {
		t.Fatalf("insert cardgroup: %v", err)
	}
	return groupID
}

// TestCardsUpsertIndex_UniqueIndexExists asserts that after baseline migration
// the index uq_cards_cardgroup_front exists on public.cards and is unique.
func TestCardsUpsertIndex_UniqueIndexExists(t *testing.T) {
	ctx := context.Background()
	db := openMigratedDB(t)
	defer db.Close()

	sqlDB := sqlDBForTest(t, db)
	const indexName = "uq_cards_cardgroup_front"

	if !indexIsUnique(t, ctx, sqlDB, indexName) {
		t.Fatalf("expected unique index %q on public.cards after migration, but it is absent or not unique", indexName)
	}
}

// TestCardsUpsertIndex_OnConflictUpdatesRow verifies that an INSERT with
// ON CONFLICT (cardgroup_id, front) DO UPDATE updates the existing row
// in place rather than inserting a duplicate or returning an error.
func TestCardsUpsertIndex_OnConflictUpdatesRow(t *testing.T) {
	ctx := context.Background()
	db := openMigratedDB(t)
	defer db.Close()

	sqlDB := sqlDBForTest(t, db)
	groupID := setupUpsertFixture(t, ctx, sqlDB)

	const front = "upsert-spec-front"
	const backFirst = "back-v1"
	const backSecond = "back-v2"

	// First INSERT — creates the row.
	if _, err := sqlDB.ExecContext(ctx, `
		INSERT INTO public.cards (id, cardgroup_id, front, back)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (cardgroup_id, front) DO UPDATE SET back = EXCLUDED.back
	`, uuid.NewString(), groupID, front, backFirst); err != nil {
		t.Fatalf("first insert: %v", err)
	}

	// Second INSERT with the same (cardgroup_id, front) key but a different back.
	// Must update the existing row, not insert a new one.
	if _, err := sqlDB.ExecContext(ctx, `
		INSERT INTO public.cards (id, cardgroup_id, front, back)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (cardgroup_id, front) DO UPDATE SET back = EXCLUDED.back
	`, uuid.NewString(), groupID, front, backSecond); err != nil {
		t.Fatalf("second insert (on conflict): %v", err)
	}

	// Assert exactly one row exists for the key.
	var count int
	if err := sqlDB.QueryRowContext(ctx,
		`SELECT count(*) FROM public.cards WHERE cardgroup_id = $1 AND front = $2`,
		groupID, front,
	).Scan(&count); err != nil {
		t.Fatalf("count cards: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 card row after upsert, got %d", count)
	}

	// Assert the back column reflects the second value.
	var gotBack string
	if err := sqlDB.QueryRowContext(ctx,
		`SELECT back FROM public.cards WHERE cardgroup_id = $1 AND front = $2`,
		groupID, front,
	).Scan(&gotBack); err != nil {
		t.Fatalf("select back: %v", err)
	}
	if gotBack != backSecond {
		t.Fatalf("back = %q, want %q", gotBack, backSecond)
	}
}

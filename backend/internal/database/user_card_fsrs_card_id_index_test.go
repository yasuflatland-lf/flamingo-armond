package database_test

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"
)

// TestUserCardFSRSCardIDIndex_Exists asserts the migration created a btree index
// backing the user_card_fsrs.card_id foreign key. The composite PK
// (user_id, card_id) cannot serve a card_id-only lookup, so cascade deletes of a
// card rely on this dedicated index.
func TestUserCardFSRSCardIDIndex_Exists(t *testing.T) {
	ctx := context.Background()
	db := openMigratedDB(t)
	defer db.Close()

	sqlDB := sqlDBForTest(t, db)
	const indexName = "idx_user_card_fsrs_card_id"

	var indexdef string
	err := sqlDB.QueryRowContext(ctx,
		`SELECT indexdef FROM pg_indexes
		 WHERE schemaname = 'public' AND tablename = 'user_card_fsrs' AND indexname = $1`,
		indexName,
	).Scan(&indexdef)
	if errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("expected index %q on public.user_card_fsrs after migration, but it is absent", indexName)
	}
	if err != nil {
		t.Fatalf("query pg_indexes for %q: %v", indexName, err)
	}
	// Assert the shape (btree keyed on card_id), not merely that the substring
	// "card_id" appears — that rules out a future regression to a different
	// method or a composite that no longer serves the card_id-only cascade lookup.
	if !strings.Contains(indexdef, "btree (card_id)") {
		t.Fatalf("index %q is not a btree keyed on card_id: %q", indexName, indexdef)
	}
}

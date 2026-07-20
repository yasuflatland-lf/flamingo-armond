package database_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
)

// swipeRecordsCardgroupFKName is the foreign key added by
// 20260721000000_add_cardgroup_fk_to_swipe_records; swipeRecordsCardgroupIndexName
// is the single-column btree index that backs it.
const (
	swipeRecordsCardgroupFKName    = "swipe_records_cardgroup_id_fkey"
	swipeRecordsCardgroupIndexName = "idx_swipe_records_cardgroup_id"
)

// swipeRecordsCardgroupFKDeleteRule returns pg_constraint.confdeltype for the
// cardgroup_id foreign key ("c" = ON DELETE CASCADE) and whether the constraint
// exists at all.
func swipeRecordsCardgroupFKDeleteRule(t *testing.T, ctx context.Context, sqlDB *sql.DB) (string, bool) {
	t.Helper()
	var rule string
	err := sqlDB.QueryRowContext(ctx, `
		SELECT confdeltype
		FROM pg_constraint
		WHERE conrelid = 'public.swipe_records'::regclass
		  AND contype = 'f'
		  AND conname = $1
	`, swipeRecordsCardgroupFKName).Scan(&rule)
	if err == sql.ErrNoRows {
		return "", false
	}
	if err != nil {
		t.Fatalf("query %s: %v", swipeRecordsCardgroupFKName, err)
	}
	return rule, true
}

// swipeRecordsIndexDef returns pg_get_indexdef for the named public.swipe_records
// index and whether the index exists.
func swipeRecordsIndexDef(t *testing.T, ctx context.Context, sqlDB *sql.DB, indexName string) (string, bool) {
	t.Helper()
	var indexdef string
	err := sqlDB.QueryRowContext(ctx, `
		SELECT pg_get_indexdef(i.indexrelid)
		FROM pg_index i
		JOIN pg_class c ON c.oid = i.indexrelid
		JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE i.indrelid = 'public.swipe_records'::regclass
		  AND n.nspname = 'public'
		  AND c.relname = $1
	`, indexName).Scan(&indexdef)
	if err == sql.ErrNoRows {
		return "", false
	}
	if err != nil {
		t.Fatalf("query index %s definition: %v", indexName, err)
	}
	return indexdef, true
}

func TestSwipeRecordsCardgroupIDSchema(t *testing.T) {
	ctx := context.Background()
	db := openMigratedDB(t)
	defer db.Close()

	sqlDB := sqlDBForTest(t, db)

	var dataType, isNullable string
	if err := sqlDB.QueryRowContext(ctx, `
		SELECT data_type, is_nullable
		FROM information_schema.columns
		WHERE table_schema = 'public'
		  AND table_name = 'swipe_records'
		  AND column_name = 'cardgroup_id'
	`).Scan(&dataType, &isNullable); err != nil {
		t.Fatalf("query swipe_records.cardgroup_id: %v", err)
	}
	if dataType != "uuid" {
		t.Fatalf("swipe_records.cardgroup_id data_type=%q, want uuid", dataType)
	}
	if isNullable != "NO" {
		t.Fatalf("swipe_records.cardgroup_id is_nullable=%q, want NO", isNullable)
	}

	var indexdef string
	if err := sqlDB.QueryRowContext(ctx, `
		SELECT pg_get_indexdef(indexrelid)
		FROM pg_index
		WHERE indexrelid = 'public.idx_swipe_records_user_cardgroup'::regclass
	`).Scan(&indexdef); err != nil {
		t.Fatalf("query idx_swipe_records_user_cardgroup definition: %v", err)
	}
	const wantIndexDef = "CREATE INDEX idx_swipe_records_user_cardgroup ON public.swipe_records USING btree (user_id, cardgroup_id, reviewed_at DESC)"
	if indexdef != wantIndexDef {
		t.Fatalf("idx_swipe_records_user_cardgroup definition=%q, want %q", indexdef, wantIndexDef)
	}

	var indexedTable string
	if err := sqlDB.QueryRowContext(ctx, `
		SELECT tablename
		FROM pg_indexes
		WHERE schemaname = 'public'
		  AND tablename = 'swipe_records'
		  AND indexname = 'idx_swipe_records_user_cardgroup'
	`).Scan(&indexedTable); err != nil {
		t.Fatalf("query pg_indexes for idx_swipe_records_user_cardgroup: %v", err)
	}
}

// TestSwipeRecordsCardgroupIDForeignKey pins the foreign key and its backing
// index. The composite idx_swipe_records_user_cardgroup leads with user_id, so it
// cannot serve a lookup or cascade keyed on cardgroup_id alone; the single-column
// index asserted here is what the foreign key actually uses.
func TestSwipeRecordsCardgroupIDForeignKey(t *testing.T) {
	ctx := context.Background()
	db := openMigratedDB(t)
	defer db.Close()

	sqlDB := sqlDBForTest(t, db)

	rule, ok := swipeRecordsCardgroupFKDeleteRule(t, ctx, sqlDB)
	if !ok {
		t.Fatalf("foreign key %s is missing from public.swipe_records", swipeRecordsCardgroupFKName)
	}
	// "c" is ON DELETE CASCADE. "a" (NO ACTION) or "r" (RESTRICT) would block
	// cardgroup deletion; "n" (SET NULL) would violate the column's NOT NULL.
	if rule != "c" {
		t.Fatalf("%s confdeltype=%q, want %q (ON DELETE CASCADE)", swipeRecordsCardgroupFKName, rule, "c")
	}

	indexdef, ok := swipeRecordsIndexDef(t, ctx, sqlDB, swipeRecordsCardgroupIndexName)
	if !ok {
		t.Fatalf("index %s is missing from public.swipe_records", swipeRecordsCardgroupIndexName)
	}
	const wantIndexDef = "CREATE INDEX idx_swipe_records_cardgroup_id ON public.swipe_records USING btree (cardgroup_id)"
	if indexdef != wantIndexDef {
		t.Fatalf("%s definition=%q, want %q", swipeRecordsCardgroupIndexName, indexdef, wantIndexDef)
	}
}

// TestSwipeRecordsCardgroupIDForeignKey_RejectsUnknownCardgroup proves the
// constraint is enforced at write time, not merely declared: an insert naming a
// cardgroup that does not exist fails with SQLSTATE 23503 (foreign_key_violation).
func TestSwipeRecordsCardgroupIDForeignKey_RejectsUnknownCardgroup(t *testing.T) {
	ctx := context.Background()
	db := openMigratedDB(t)
	defer db.Close()

	sqlDB := sqlDBForTest(t, db)
	userID := insertRLSAuthUser(t, ctx, sqlDB)
	cardgroupID := insertRLSCardgroup(t, ctx, sqlDB, userID, "FK unknown cardgroup")
	cardID := insertRLSCard(t, ctx, sqlDB, cardgroupID, "FK unknown cardgroup card")

	const unknownCardgroupID = "ffffffff-ffff-4fff-bfff-fffffffffffe"
	_, err := sqlDB.ExecContext(ctx, insertSwipeSQL(),
		userID, cardID, unknownCardgroupID, time.Now().UTC())
	if err == nil {
		t.Fatal("insert with an unknown cardgroup_id unexpectedly succeeded")
	}
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		t.Fatalf("insert error is not a *pgconn.PgError: %v", err)
	}
	if pgErr.Code != "23503" {
		t.Fatalf("insert SQLSTATE=%s, want 23503 (foreign_key_violation): %v", pgErr.Code, err)
	}
}

package database_test

import (
	"context"
	"testing"
)

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

package database_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/google/uuid"

	"backend/internal/database"
)

// redundantFKIndexes are the single-column FK indexes that
// 20260927000005_drop_redundant_fk_indexes drops and its down restores.
var redundantFKIndexes = []struct {
	table, name, shape string
}{
	{table: "cards", name: "idx_cards_cardgroup_id", shape: "USING btree (cardgroup_id)"},
	{table: "master_cards", name: "idx_master_cards_master_cardgroup_id", shape: "USING btree (master_cardgroup_id)"},
}

// coveringFKIndexes are the unique composites whose leading column serves the
// FK lookups (FK checks, ON DELETE CASCADE) the dropped indexes used to serve.
var coveringFKIndexes = []struct {
	table, fkColumn, name, shape string
}{
	{table: "cards", fkColumn: "cardgroup_id", name: "uq_cards_cardgroup_front", shape: "USING btree (cardgroup_id, front)"},
	{table: "master_cards", fkColumn: "master_cardgroup_id", name: "uq_master_cards_cg_front", shape: "USING btree (master_cardgroup_id, front)"},
}

// TestDropRedundantFKIndexes_DownUpRoundtrip pins the rollback shape of the two
// dropped FK indexes, and that each covering composite leads with its FK column
// and serves an equality lookup on it, at the drop version and at the latest schema.
//
// t.Parallel() is intentionally absent: migrations change global suite state.
func TestDropRedundantFKIndexes_DownUpRoundtrip(t *testing.T) {
	// Named versions avoid relative step counts that drift after new migrations.
	const (
		beforeDrop uint = 20260927000004
		afterDrop  uint = 20260927000005
	)

	ctx := context.Background()
	db := openMigratedDB(t)
	defer db.Close()
	t.Cleanup(func() {
		if err := database.Migrate(testDSN); err != nil {
			t.Errorf("restore latest migration: %v", err)
		}
	})

	sqlDB := sqlDBForTest(t, db)
	requireRedundantFKIndexes(t, ctx, sqlDB, false)
	requireCoveringFKIndexes(t, ctx, sqlDB)

	m, err := database.NewMigrateInstanceForTest(testDSN)
	if err != nil {
		t.Fatalf("NewMigrateInstanceForTest: %v", err)
	}
	defer func() {
		if srcErr, dbErr := m.Close(); srcErr != nil || dbErr != nil {
			t.Logf("migrate close: src_err=%v db_err=%v", srcErr, dbErr)
		}
	}()

	if err := m.Migrate(beforeDrop); err != nil {
		t.Fatalf("migrate to version %d: %v", beforeDrop, err)
	}
	requireRedundantFKIndexes(t, ctx, sqlDB, true)

	if err := m.Migrate(afterDrop); err != nil {
		t.Fatalf("migrate to version %d: %v", afterDrop, err)
	}
	requireRedundantFKIndexes(t, ctx, sqlDB, false)
	requireCoveringFKIndexes(t, ctx, sqlDB)
}

// requireRedundantFKIndexes asserts that every entry of redundantFKIndexes is
// present with its single-column btree shape, or absent, per want.
func requireRedundantFKIndexes(t *testing.T, ctx context.Context, sqlDB *sql.DB, want bool) {
	t.Helper()
	for _, idx := range redundantFKIndexes {
		var indexdef string
		err := sqlDB.QueryRowContext(ctx,
			`SELECT indexdef FROM pg_indexes
			 WHERE schemaname = 'public' AND tablename = $1 AND indexname = $2`,
			idx.table, idx.name,
		).Scan(&indexdef)
		if errors.Is(err, sql.ErrNoRows) {
			if want {
				t.Fatalf("index %q on public.%s is absent, want present", idx.name, idx.table)
			}
			continue
		}
		if err != nil {
			t.Fatalf("query pg_indexes for %q: %v", idx.name, err)
		}
		if !want {
			t.Fatalf("index %q on public.%s is present (%q), want absent", idx.name, idx.table, indexdef)
		}
		if !strings.HasSuffix(indexdef, idx.shape) {
			t.Fatalf("index %q on public.%s = %q, want shape %q", idx.name, idx.table, indexdef, idx.shape)
		}
	}
}

// requireCoveringFKIndexes asserts that every entry of coveringFKIndexes is
// present, leads with its FK column, and is planned for an equality lookup on it.
func requireCoveringFKIndexes(t *testing.T, ctx context.Context, sqlDB *sql.DB) {
	t.Helper()
	for _, idx := range coveringFKIndexes {
		var indexdef string
		err := sqlDB.QueryRowContext(ctx,
			`SELECT indexdef FROM pg_indexes
			 WHERE schemaname = 'public' AND tablename = $1 AND indexname = $2`,
			idx.table, idx.name,
		).Scan(&indexdef)
		if errors.Is(err, sql.ErrNoRows) {
			t.Fatalf("covering index %q on public.%s is absent, want present", idx.name, idx.table)
		}
		if err != nil {
			t.Fatalf("query pg_indexes for %q: %v", idx.name, err)
		}
		// Not the EXPLAIN check alone: with seq scans off the planner also
		// full-scans a btree whose leading column is not the FK column.
		if !strings.HasSuffix(indexdef, idx.shape) {
			t.Fatalf("covering index %q on public.%s = %q, want shape %q (must lead with %s)",
				idx.name, idx.table, indexdef, idx.shape, idx.fkColumn)
		}
		if got := fkLookupIndexScans(t, ctx, sqlDB, idx.table, idx.fkColumn); !slices.Contains(got, idx.name) {
			t.Fatalf("public.%s.%s lookup index scans = %q, want one on %q", idx.table, idx.fkColumn, got, idx.name)
		}
	}
}

// explainNode is the subset of an EXPLAIN (FORMAT JSON) plan node the
// index-path assertion reads.
type explainNode struct {
	NodeType  string        `json:"Node Type"`
	IndexName string        `json:"Index Name"`
	Plans     []explainNode `json:"Plans"`
}

// indexScans returns the index names of every index-scan node in the subtree.
func (n explainNode) indexScans() []string {
	var names []string
	switch n.NodeType {
	case "Index Scan", "Index Only Scan", "Bitmap Index Scan":
		names = append(names, n.IndexName)
	}
	for _, child := range n.Plans {
		names = append(names, child.indexScans()...)
	}
	return names
}

// fkLookupIndexScans returns the indexes the planner scans for an equality
// lookup on public.<table>.<fkColumn> with sequential scans disabled.
func fkLookupIndexScans(t *testing.T, ctx context.Context, sqlDB *sql.DB, table, fkColumn string) []string {
	t.Helper()
	// A pinned connection, not the pool: SET is session-scoped, and the pool may
	// run the EXPLAIN on a connection that never saw it.
	conn, err := sqlDB.Conn(ctx)
	if err != nil {
		t.Fatalf("pin connection: %v", err)
	}
	defer conn.Close()

	// With seq scans enabled the planner may pick one on the near-empty table and
	// hide whether an index path exists at all.
	if _, err := conn.ExecContext(ctx, `SET enable_seqscan = off`); err != nil {
		t.Fatalf("disable seq scan: %v", err)
	}
	// Not bind parameters: identifiers cannot be bound, and both come from the
	// constant coveringFKIndexes table.
	query := fmt.Sprintf(`EXPLAIN (FORMAT JSON) SELECT id FROM public.%s WHERE %s = $1`, table, fkColumn)
	var raw string
	if err := conn.QueryRowContext(ctx, query, uuid.NewString()).Scan(&raw); err != nil {
		t.Fatalf("explain public.%s.%s lookup: %v", table, fkColumn, err)
	}

	var plans []struct {
		Plan explainNode `json:"Plan"`
	}
	if err := json.Unmarshal([]byte(raw), &plans); err != nil {
		t.Fatalf("decode explain output %q: %v", raw, err)
	}
	if len(plans) != 1 {
		t.Fatalf("explain returned %d plans, want 1: %q", len(plans), raw)
	}
	return plans[0].Plan.indexScans()
}

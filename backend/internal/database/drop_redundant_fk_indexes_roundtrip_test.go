package database_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
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

// TestDropRedundantFKIndexes_DownUpRoundtrip pins the rollback shape of the two
// dropped FK indexes, and that a cards.cardgroup_id equality lookup still has an
// index path through uq_cards_cardgroup_front once they are gone.
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

	const coveringIndex = "uq_cards_cardgroup_front"
	if got := cardgroupLookupIndexScans(t, ctx, sqlDB); !slices.Contains(got, coveringIndex) {
		t.Fatalf("cards.cardgroup_id lookup index scans at version %d = %q, want one on %q", afterDrop, got, coveringIndex)
	}
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

// cardgroupLookupIndexScans returns the indexes the planner scans for a
// cards.cardgroup_id equality lookup with sequential scans disabled.
func cardgroupLookupIndexScans(t *testing.T, ctx context.Context, sqlDB *sql.DB) []string {
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
	var raw string
	if err := conn.QueryRowContext(ctx,
		`EXPLAIN (FORMAT JSON) SELECT id FROM public.cards WHERE cardgroup_id = $1`,
		uuid.NewString(),
	).Scan(&raw); err != nil {
		t.Fatalf("explain cards.cardgroup_id lookup: %v", err)
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

package database_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"testing"

	"github.com/google/uuid"

	"backend/internal/database"
)

const cardgroupsOwnerUpdatedAtIndex = "idx_cardgroups_owner_updated_at_id"

// cardgroupsComposite is the index set 20260927000006_cardgroups_owner_updated_at_index
// leaves on public.cardgroups; cardgroupsSingleColumn is the set its down restores.
var (
	cardgroupsComposite = map[string]string{
		cardgroupsOwnerUpdatedAtIndex: "CREATE INDEX idx_cardgroups_owner_updated_at_id ON public.cardgroups USING btree (owner_id, updated_at DESC, id DESC)",
	}
	cardgroupsSingleColumn = map[string]string{
		"idx_cardgroups_owner_id":   "CREATE INDEX idx_cardgroups_owner_id ON public.cardgroups USING btree (owner_id)",
		"idx_cardgroups_updated_at": "CREATE INDEX idx_cardgroups_updated_at ON public.cardgroups USING btree (updated_at)",
	}
)

// TestCardgroupsOwnerUpdatedAtIndex_DownUpRoundtrip pins the composite's exact
// definition and the absence of the two indexes it replaces, the rollback that
// restores them, and that a replica of the default myCardgroupsConnection page
// query plans without a Sort node.
//
// t.Parallel() is intentionally absent: migrations change global suite state.
func TestCardgroupsOwnerUpdatedAtIndex_DownUpRoundtrip(t *testing.T) {
	// Named versions avoid relative step counts that drift after new migrations.
	const (
		beforeSwap uint = 20260927000005
		afterSwap  uint = 20260927000006
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
	requireCardgroupsIndexDefs(t, ctx, sqlDB, "HEAD", cardgroupsComposite)
	requireCardgroupsPageIsIndexOrdered(t, ctx, sqlDB)

	m, err := database.NewMigrateInstanceForTest(testDSN)
	if err != nil {
		t.Fatalf("NewMigrateInstanceForTest: %v", err)
	}
	defer func() {
		if srcErr, dbErr := m.Close(); srcErr != nil || dbErr != nil {
			t.Logf("migrate close: src_err=%v db_err=%v", srcErr, dbErr)
		}
	}()

	if err := m.Migrate(beforeSwap); err != nil {
		t.Fatalf("migrate to version %d: %v", beforeSwap, err)
	}
	requireCardgroupsIndexDefs(t, ctx, sqlDB, fmt.Sprint(beforeSwap), cardgroupsSingleColumn)

	if err := m.Migrate(afterSwap); err != nil {
		t.Fatalf("migrate to version %d: %v", afterSwap, err)
	}
	requireCardgroupsIndexDefs(t, ctx, sqlDB, fmt.Sprint(afterSwap), cardgroupsComposite)
	requireCardgroupsPageIsIndexOrdered(t, ctx, sqlDB)
}

// requireCardgroupsIndexDefs asserts that, of the three indexes the migration
// swaps, exactly the entries of want exist on public.cardgroups with those defs.
func requireCardgroupsIndexDefs(t *testing.T, ctx context.Context, sqlDB *sql.DB, at string, want map[string]string) {
	t.Helper()
	rows, err := sqlDB.QueryContext(ctx,
		`SELECT indexname, indexdef FROM pg_indexes
		 WHERE schemaname = 'public' AND tablename = 'cardgroups'
		   AND indexname IN ($1, $2, $3)`,
		cardgroupsOwnerUpdatedAtIndex, "idx_cardgroups_owner_id", "idx_cardgroups_updated_at",
	)
	if err != nil {
		t.Fatalf("query cardgroups indexes at %s: %v", at, err)
	}
	defer rows.Close()

	got := map[string]string{}
	for rows.Next() {
		var name, def string
		if err := rows.Scan(&name, &def); err != nil {
			t.Fatalf("scan cardgroups index at %s: %v", at, err)
		}
		got[name] = def
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate cardgroups indexes at %s: %v", at, err)
	}
	if !maps.Equal(got, want) {
		t.Fatalf("cardgroups indexes at %s = %q, want %q", at, got, want)
	}
}

// requireCardgroupsPageIsIndexOrdered asserts that an owner-scoped, cursorless
// replica of FindPageByOwner's UPDATED_AT page, in both directions, scans the
// composite without a Sort node. It does not run FindPageByOwner: keep the SQL
// in cardgroupsPagePlan in sync with repository.cardgroupOrderClause.
func requireCardgroupsPageIsIndexOrdered(t *testing.T, ctx context.Context, sqlDB *sql.DB) {
	t.Helper()
	for _, dir := range []string{"DESC", "ASC"} {
		plan := cardgroupsPagePlan(t, ctx, sqlDB, dir)
		if got := plan.indexScans(); !slices.Contains(got, cardgroupsOwnerUpdatedAtIndex) {
			t.Fatalf("%s page index scans = %q, want one on %q", dir, got, cardgroupsOwnerUpdatedAtIndex)
		}
		if types := plan.nodeTypes(); slices.Contains(types, "Sort") || slices.Contains(types, "Incremental Sort") {
			t.Fatalf("%s page plan node types = %q, want no sort", dir, types)
		}
	}
}

// nodeTypes returns the node type of every node in the subtree.
func (n explainNode) nodeTypes() []string {
	types := []string{n.NodeType}
	for _, child := range n.Plans {
		types = append(types, child.nodeTypes()...)
	}
	return types
}

// cardgroupsPagePlan returns the plan of an owner-scoped first page ordered by
// (updated_at, id) in dir, with sequential scans and Sort nodes cost-penalized
// (enable_seqscan/enable_sort = off).
func cardgroupsPagePlan(t *testing.T, ctx context.Context, sqlDB *sql.DB, dir string) explainNode {
	t.Helper()
	// A pinned connection, not the pool: SET is session-scoped.
	conn, err := sqlDB.Conn(ctx)
	if err != nil {
		t.Fatalf("pin connection: %v", err)
	}
	defer conn.Close()

	// Not the default costs: on the near-empty table the planner prefers a seq or
	// bitmap scan plus Sort and hides whether an index-ordered path exists at all.
	// enable_sort = off only penalizes a Sort node, so one still appears when no
	// sort-free path exists.
	for _, stmt := range []string{`SET enable_seqscan = off`, `SET enable_sort = off`} {
		if _, err := conn.ExecContext(ctx, stmt); err != nil {
			t.Fatalf("%s: %v", stmt, err)
		}
	}
	// Not a bind parameter: ORDER BY directions cannot be bound, and dir comes
	// from the constant list in requireCardgroupsPageIsIndexOrdered.
	query := fmt.Sprintf(`EXPLAIN (FORMAT JSON)
		SELECT * FROM public.cardgroups WHERE owner_id = $1
		ORDER BY updated_at %[1]s, id %[1]s LIMIT 21`, dir)
	var raw string
	if err := conn.QueryRowContext(ctx, query, uuid.NewString()).Scan(&raw); err != nil {
		t.Fatalf("explain %s cardgroups page: %v", dir, err)
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
	return plans[0].Plan
}

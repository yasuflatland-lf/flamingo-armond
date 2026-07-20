package usecase

import (
	"strings"
	"testing"
	"time"
)

// testSyncNow is the fixed timestamp injected into the plan computation in place
// of a clock. Every card in one plan must be stamped with this single value.
var testSyncNow = time.Date(2026, 3, 4, 5, 6, 7, 0, time.UTC)

const testPlanCardgroupID = "mcg-plan"

// hasKeepFront reports whether front is in the plan's keep-set, applying the
// same case-insensitive key the diff-prune step uses.
func hasKeepFront(plan notionSyncPlan, front string) bool {
	_, ok := plan.KeepFronts[frontMatchKey(front)]
	return ok
}

// TestComputeSyncPlan_AllValid pins the happy path: every row survives domain
// construction, so the plan carries one card per row in document order, a
// keep-set covering all of them, no skips, and nothing that would block
// persistence. No repository, transaction runner, logger or clock is involved.
func TestComputeSyncPlan_AllValid(t *testing.T) {
	t.Parallel()

	rows := []ParsedRow{
		{Front: "apple", Back: "fruit", SourcePageID: "page-1", Line: 1},
		{Front: "banana", Back: "fruit", SourcePageID: "page-1", Line: 2},
		{Front: "carrot", Back: "vegetable", SourcePageID: "page-2", Line: 1},
	}

	plan := computeSyncPlan(rows, nil, testPlanCardgroupID, testSyncNow)

	if len(plan.Rows) != 3 {
		t.Fatalf("len(plan.Rows) = %d, want 3", len(plan.Rows))
	}
	if len(plan.Cards) != 3 {
		t.Fatalf("len(plan.Cards) = %d, want 3", len(plan.Cards))
	}
	if len(plan.DomainSkips) != 0 {
		t.Fatalf("plan.DomainSkips = %+v, want none", plan.DomainSkips)
	}
	if len(plan.ParseErrors) != 0 {
		t.Fatalf("plan.ParseErrors = %+v, want none", plan.ParseErrors)
	}
	if plan.NothingValidToPersist() {
		t.Error("NothingValidToPersist() = true, want false (every row produced a card)")
	}
	for i, card := range plan.Cards {
		if card.Position != i {
			t.Errorf("plan.Cards[%d].Position = %d, want %d", i, card.Position, i)
		}
		if card.MasterCardgroupID != testPlanCardgroupID {
			t.Errorf("plan.Cards[%d].MasterCardgroupID = %q, want %q",
				i, card.MasterCardgroupID, testPlanCardgroupID)
		}
		// The whole batch is pinned to the single injected created_at. updated_at
		// is database-owned (the BEFORE INSERT OR UPDATE trigger sets it), so the
		// plan deliberately leaves whatever the constructor stamped and nothing
		// here pins or asserts it.
		if !card.CreatedAt.Equal(testSyncNow) {
			t.Errorf("plan.Cards[%d].CreatedAt = %s, want %s", i, card.CreatedAt, testSyncNow)
		}
	}
	for _, front := range []string{"apple", "banana", "carrot"} {
		if !hasKeepFront(plan, front) {
			t.Errorf("plan.KeepFronts is missing %q", front)
		}
	}
	if len(plan.KeepFronts) != 3 {
		t.Errorf("len(plan.KeepFronts) = %d, want 3", len(plan.KeepFronts))
	}
}

// TestComputeSyncPlan_PartiallyInvalid pins the per-row skip semantics: a row
// rejected by domain.NewMasterCard is absent from both the cards and the
// keep-set (so the diff-prune deletes its stale row), while the surviving rows
// keep their deduped-slice index as Position. The plan is still persistable.
func TestComputeSyncPlan_PartiallyInvalid(t *testing.T) {
	t.Parallel()

	rows := []ParsedRow{
		{Front: "apple", Back: "fruit", SourcePageID: "page-1", Line: 1},
		{Front: "banana", Back: strings.Repeat("x", 501), SourcePageID: "page-1", Line: 2},
		{Front: "carrot", Back: "vegetable", SourcePageID: "page-1", Line: 3},
	}

	plan := computeSyncPlan(rows, nil, testPlanCardgroupID, testSyncNow)

	if plan.NothingValidToPersist() {
		t.Error("NothingValidToPersist() = true, want false (two rows produced cards)")
	}
	if len(plan.Cards) != 2 {
		t.Fatalf("len(plan.Cards) = %d, want 2", len(plan.Cards))
	}
	if string(plan.Cards[0].Front) != "apple" || plan.Cards[0].Position != 0 {
		t.Errorf("plan.Cards[0] = {Front:%q, Position:%d}, want {apple, 0}",
			plan.Cards[0].Front, plan.Cards[0].Position)
	}
	if string(plan.Cards[1].Front) != "carrot" || plan.Cards[1].Position != 2 {
		t.Errorf("plan.Cards[1] = {Front:%q, Position:%d}, want {carrot, 2}",
			plan.Cards[1].Front, plan.Cards[1].Position)
	}
	if len(plan.DomainSkips) != 1 {
		t.Fatalf("len(plan.DomainSkips) = %d, want 1", len(plan.DomainSkips))
	}
	skip := plan.DomainSkips[0]
	if skip.Position != 1 || skip.Diagnostic.Line != 2 || skip.Diagnostic.Front != "banana" {
		t.Errorf("skip = {Position:%d, Line:%d, Front:%q}, want {1, 2, banana}",
			skip.Position, skip.Diagnostic.Line, skip.Diagnostic.Front)
	}
	if skip.Diagnostic.Kind != CardImportErrKindHard {
		t.Errorf("skip.Diagnostic.Kind = %q, want %q", skip.Diagnostic.Kind, CardImportErrKindHard)
	}
	if skip.Reason == nil {
		t.Error("skip.Reason = nil, want the constructor error")
	}
	// The dropped row is not in the keep-set, so its stale card is pruned.
	if hasKeepFront(plan, "banana") {
		t.Error("plan.KeepFronts contains banana, want it absent so the stale row is pruned")
	}
}

// TestComputeSyncPlan_AllInvalid pins the wipe guard as a property of the plan:
// when every row fails domain construction the keep-set is empty, so persisting
// would delete every card in the target master cardgroup. NothingValidToPersist
// is the single checkable condition that expresses the rule.
func TestComputeSyncPlan_AllInvalid(t *testing.T) {
	t.Parallel()

	rows := []ParsedRow{
		{Front: "apple", Back: strings.Repeat("x", 501), SourcePageID: "page-1", Line: 1},
		{Front: "banana", Back: "   ", SourcePageID: "page-1", Line: 2},
	}

	plan := computeSyncPlan(rows, nil, testPlanCardgroupID, testSyncNow)

	if !plan.NothingValidToPersist() {
		t.Fatal("NothingValidToPersist() = false, want true (no row produced a card)")
	}
	if len(plan.Cards) != 0 {
		t.Fatalf("len(plan.Cards) = %d, want 0", len(plan.Cards))
	}
	if len(plan.KeepFronts) != 0 {
		t.Fatalf("len(plan.KeepFronts) = %d, want 0", len(plan.KeepFronts))
	}
	// The guard's error message and warn are built from the first skip, so the
	// slice must be non-empty and ordered by row.
	if len(plan.DomainSkips) != 2 {
		t.Fatalf("len(plan.DomainSkips) = %d, want 2", len(plan.DomainSkips))
	}
	if plan.DomainSkips[0].Diagnostic.Line != 1 || plan.DomainSkips[1].Diagnostic.Line != 2 {
		t.Errorf("skip lines = (%d, %d), want (1, 2)",
			plan.DomainSkips[0].Diagnostic.Line, plan.DomainSkips[1].Diagnostic.Line)
	}
}

// TestComputeSyncPlan_CaseVariantDuplicateFronts pins the citext-matching
// behaviour the plan inherits from dedupeParsedRows and frontMatchKey: two rows
// whose fronts differ only in case collapse to one row (the later occurrence
// wins and keeps its own case), a DUPLICATE diagnostic is appended for the
// discarded row, and the keep-set holds a single case-insensitive key.
func TestComputeSyncPlan_CaseVariantDuplicateFronts(t *testing.T) {
	t.Parallel()

	rows := []ParsedRow{
		{Front: "Drive", Back: "to operate a vehicle", SourcePageID: "page-1", Line: 1},
		{Front: "drive", Back: "to propel forward", SourcePageID: "page-1", Line: 2},
	}

	plan := computeSyncPlan(rows, nil, testPlanCardgroupID, testSyncNow)

	if len(plan.Rows) != 1 || plan.Rows[0].Front != "drive" {
		t.Fatalf("plan.Rows = %+v, want only the later 'drive' row", plan.Rows)
	}
	if len(plan.Cards) != 1 || string(plan.Cards[0].Front) != "drive" {
		t.Fatalf("plan.Cards = %+v, want a single 'drive' card", plan.Cards)
	}
	if len(plan.KeepFronts) != 1 || !hasKeepFront(plan, "DRIVE") {
		t.Fatalf("plan.KeepFronts = %v, want a single case-insensitive drive key", plan.KeepFronts)
	}
	if len(plan.ParseErrors) != 1 {
		t.Fatalf("len(plan.ParseErrors) = %d, want 1 duplicate diagnostic", len(plan.ParseErrors))
	}
	dup := plan.ParseErrors[0]
	if dup.Kind != CardImportErrKindDuplicate {
		t.Errorf("ParseErrors[0].Kind = %q, want %q", dup.Kind, CardImportErrKindDuplicate)
	}
	// The diagnostic is anchored to the discarded row's line, not the survivor's.
	if dup.Line != 1 || dup.Front != "Drive" {
		t.Errorf("ParseErrors[0] = {Line:%d, Front:%q}, want {1, Drive}", dup.Line, dup.Front)
	}
	if plan.NothingValidToPersist() {
		t.Error("NothingValidToPersist() = true, want false (the surviving row produced a card)")
	}
}

// TestComputeSyncPlan_EmptyInput pins the empty plan: no rows means nothing to
// upsert and an empty keep-set, but NothingValidToPersist must stay false — the
// empty payload is the grammar-skip short-circuit's business, and reporting it
// as a domain-validation wipe would attribute the wrong cause.
func TestComputeSyncPlan_EmptyInput(t *testing.T) {
	t.Parallel()

	plan := computeSyncPlan(nil, nil, testPlanCardgroupID, testSyncNow)

	if len(plan.Rows) != 0 || len(plan.Cards) != 0 || len(plan.DomainSkips) != 0 {
		t.Fatalf("plan = %+v, want empty rows, cards and skips", plan)
	}
	if len(plan.KeepFronts) != 0 {
		t.Fatalf("len(plan.KeepFronts) = %d, want 0", len(plan.KeepFronts))
	}
	if plan.NothingValidToPersist() {
		t.Error("NothingValidToPersist() = true, want false (an empty payload is not a wipe)")
	}
}

package usecase

import (
	"strings"
	"testing"
	"time"
)

// testSyncNow is the fixed timestamp injected in place of a clock.
var testSyncNow = time.Date(2026, 3, 4, 5, 6, 7, 0, time.UTC)

const testPlanCardgroupID = "mcg-plan"

// hasKeepFront reports whether front is in the plan's keep-set, applying the
// same case-insensitive key the diff-prune step uses.
func hasKeepFront(plan notionSyncPlan, front string) bool {
	_, ok := plan.KeepFronts[frontMatchKey(front)]
	return ok
}

// TestComputeSyncPlan_AllValid pins the happy path: one card per row in
// document order, a keep-set covering all of them, no skips, nothing blocking
// persistence.
func TestComputeSyncPlan_AllValid(t *testing.T) {
	t.Parallel()

	rows := []ParsedRow{
		{Front: "apple", Back: "fruit", SourcePageID: "page-1", Line: 1},
		{Front: "banana", Back: "fruit", SourcePageID: "page-1", Line: 2},
		{Front: "carrot", Back: "vegetable", SourcePageID: "page-2", Line: 1},
	}

	plan := computeSyncPlan(rows, nil, 0, testPlanCardgroupID, testSyncNow)

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
		// updated_at is not asserted: the DB trigger owns it, not the plan.
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

// TestComputeSyncPlan_PartiallyInvalid pins the per-row skip semantics: a
// rejected row is absent from the cards AND the keep-set, while survivors keep
// their deduped-slice index as Position. Such a plan is still persistable.
func TestComputeSyncPlan_PartiallyInvalid(t *testing.T) {
	t.Parallel()

	rows := []ParsedRow{
		{Front: "apple", Back: "fruit", SourcePageID: "page-1", Line: 1},
		{Front: "banana", Back: strings.Repeat("x", 501), SourcePageID: "page-1", Line: 2},
		{Front: "carrot", Back: "vegetable", SourcePageID: "page-1", Line: 3},
	}

	plan := computeSyncPlan(rows, nil, 0, testPlanCardgroupID, testSyncNow)

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
	if hasKeepFront(plan, "banana") {
		t.Error("plan.KeepFronts contains banana, want it absent so the stale row is pruned")
	}
}

// TestComputeSyncPlan_AllInvalid pins the wipe guard: an all-invalid batch
// leaves the keep-set empty, so persisting it would delete every card in the
// target master cardgroup.
func TestComputeSyncPlan_AllInvalid(t *testing.T) {
	t.Parallel()

	rows := []ParsedRow{
		{Front: "apple", Back: strings.Repeat("x", 501), SourcePageID: "page-1", Line: 1},
		{Front: "banana", Back: "   ", SourcePageID: "page-1", Line: 2},
	}

	plan := computeSyncPlan(rows, nil, 0, testPlanCardgroupID, testSyncNow)

	if !plan.NothingValidToPersist() {
		t.Fatal("NothingValidToPersist() = false, want true (no row produced a card)")
	}
	if len(plan.Cards) != 0 {
		t.Fatalf("len(plan.Cards) = %d, want 0", len(plan.Cards))
	}
	if len(plan.KeepFronts) != 0 {
		t.Fatalf("len(plan.KeepFronts) = %d, want 0", len(plan.KeepFronts))
	}
	// Order is not incidental: the guard reads DomainSkips[0] for its message.
	if len(plan.DomainSkips) != 2 {
		t.Fatalf("len(plan.DomainSkips) = %d, want 2", len(plan.DomainSkips))
	}
	if plan.DomainSkips[0].Diagnostic.Line != 1 || plan.DomainSkips[1].Diagnostic.Line != 2 {
		t.Errorf("skip lines = (%d, %d), want (1, 2)",
			plan.DomainSkips[0].Diagnostic.Line, plan.DomainSkips[1].Diagnostic.Line)
	}
}

// TestComputeSyncPlan_CaseVariantDuplicateFronts pins the citext matching the
// plan inherits from its dedupeByKey step and frontMatchKey: fronts differing only
// in case collapse to one row (later wins, keeping its own case) with a
// DUPLICATE diagnostic and a single keep-set key.
func TestComputeSyncPlan_CaseVariantDuplicateFronts(t *testing.T) {
	t.Parallel()

	rows := []ParsedRow{
		{Front: "Drive", Back: "to operate a vehicle", SourcePageID: "page-1", Line: 1},
		{Front: "drive", Back: "to propel forward", SourcePageID: "page-1", Line: 2},
	}

	plan := computeSyncPlan(rows, nil, 0, testPlanCardgroupID, testSyncNow)

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

	plan := computeSyncPlan(nil, nil, 0, testPlanCardgroupID, testSyncNow)

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

// TestComputeSyncPlan_SkippedPagesDeferPrune pins prune safety as a property of
// the plan value: a batch with no skipped pages may prune, while any skipped
// page makes the keep-set incomplete and blocks the diff-prune.
func TestComputeSyncPlan_SkippedPagesDeferPrune(t *testing.T) {
	t.Parallel()

	rows := []ParsedRow{
		{Front: "apple", Back: "fruit", SourcePageID: "page-1", Line: 1},
	}

	cases := []struct {
		name         string
		skippedPages int
		wantSafe     bool
	}{
		{name: "no skipped pages", skippedPages: 0, wantSafe: true},
		{name: "one skipped page", skippedPages: 1, wantSafe: false},
		{name: "two skipped pages", skippedPages: 2, wantSafe: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			plan := computeSyncPlan(rows, nil, tc.skippedPages, testPlanCardgroupID, testSyncNow)
			if plan.SkippedPages != tc.skippedPages {
				t.Fatalf("plan.SkippedPages = %d, want %d", plan.SkippedPages, tc.skippedPages)
			}
			if got := plan.PruneSafe(); got != tc.wantSafe {
				t.Fatalf("PruneSafe() = %v, want %v", got, tc.wantSafe)
			}
		})
	}
}

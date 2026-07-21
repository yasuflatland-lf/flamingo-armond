package usecase

import (
	"context"
	"sort"
	"testing"
	"time"

	"github.com/rotisserie/eris"

	"backend/internal/cursor"
	"backend/internal/domain"
	"backend/internal/repository"
)

// ---------------------------------------------------------------------------
// Cursor stability across edits of the ordering key.
//
// The cardgroup listing defaults to UPDATED_AT DESC — a mutable column. A
// cursor that carries only a row id has to re-read that row at serve time to
// recover its ordering value, so editing the row between two page fetches moves
// the bookmark: rows already returned come back a second time (S1) or rows the
// caller has not seen yet are skipped (S2). A v2 cursor carries the ordering
// value captured when the page was served, so the bookmark stays put.
//
// These walks use cursorWalkRepo, an in-memory repository implementing the same
// (updated_at, id) tuple comparison the SQL repository emits, so the scenarios
// are reproduced end-to-end through ListCardgroupsByOwnerConnection without a
// database.
// ---------------------------------------------------------------------------

// cursorWalkRepo is an in-memory CardgroupRepository supporting exactly the
// slice of the interface these walks need: forward AND backward paging over one
// owner's rows ordered by (updated_at DESC, id DESC). Any other ordering is
// rejected loudly so a future test cannot silently exercise an unimplemented
// branch. FindByID returns the CURRENT row, which is what makes the v1
// re-hydration path observe a mutation made between two page fetches.
//
// The backward branch models the repository's direction-flip + reverse: it
// keeps the rows that sort strictly BEFORE the cursor and returns the ones
// closest to it, still in DESC order, so the usecase's leading-edge trim
// (TrimAndDetectBackward) sees the same slice shape SQL would hand it.
type cursorWalkRepo struct {
	rows []*domain.Cardgroup
}

func (r *cursorWalkRepo) FindByID(_ context.Context, id string) (*domain.Cardgroup, error) {
	for _, cg := range r.rows {
		if string(cg.ID) == id {
			return cg, nil
		}
	}
	return nil, repository.ErrNotFound
}

// afterInDescTuple reports whether row sorts strictly after cur under the
// (updated_at DESC, id DESC) total order the repository emits.
func afterInDescTuple(row *domain.Cardgroup, curKey time.Time, curID string) bool {
	if row.UpdatedAt.Equal(curKey) {
		return string(row.ID) < curID
	}
	return row.UpdatedAt.Before(curKey)
}

// beforeInDescTuple is the mirror predicate for backward paging: it reports
// whether row sorts strictly before cur under the same total order. It is not
// !afterInDescTuple — the cursor row itself sorts neither after nor before
// itself, and both edges must exclude it.
func beforeInDescTuple(row *domain.Cardgroup, curKey time.Time, curID string) bool {
	if row.UpdatedAt.Equal(curKey) {
		return string(row.ID) > curID
	}
	return row.UpdatedAt.After(curKey)
}

func (r *cursorWalkRepo) FindPageByOwner(
	_ context.Context,
	ownerID string,
	after, before *repository.CardgroupCursor,
	first, last int,
	orderBy repository.CardgroupOrderBy,
	dir repository.SortOrder,
	_ *string,
) ([]*domain.Cardgroup, int64, error) {
	if orderBy != repository.CardgroupOrderByUpdatedAt || dir != repository.SortDesc {
		return nil, 0, eris.Errorf(
			"cursorWalkRepo: only the (updated_at, DESC) ordering is implemented; got orderBy=%q dir=%q",
			orderBy, dir,
		)
	}

	owned := make([]*domain.Cardgroup, 0, len(r.rows))
	for _, cg := range r.rows {
		if cg.IsOwnedBy(domain.UserID(ownerID)) {
			owned = append(owned, cg)
		}
	}
	sort.SliceStable(owned, func(i, j int) bool {
		if !owned[i].UpdatedAt.Equal(owned[j].UpdatedAt) {
			return owned[i].UpdatedAt.After(owned[j].UpdatedAt)
		}
		return owned[i].ID > owned[j].ID
	})
	total := int64(len(owned))

	if after != nil {
		if after.UpdatedAt == nil {
			return nil, 0, eris.New("cursorWalkRepo: after cursor is missing the updated_at column")
		}
		rest := make([]*domain.Cardgroup, 0, len(owned))
		for _, cg := range owned {
			if afterInDescTuple(cg, *after.UpdatedAt, after.ID) {
				rest = append(rest, cg)
			}
		}
		owned = rest
	}
	if before != nil {
		if before.UpdatedAt == nil {
			return nil, 0, eris.New("cursorWalkRepo: before cursor is missing the updated_at column")
		}
		rest := make([]*domain.Cardgroup, 0, len(owned))
		for _, cg := range owned {
			if beforeInDescTuple(cg, *before.UpdatedAt, before.ID) {
				rest = append(rest, cg)
			}
		}
		owned = rest
	}
	if first > 0 && len(owned) > first {
		owned = owned[:first]
	}
	// Backward paging takes the rows CLOSEST to the before cursor, which are the
	// trailing ones in DESC order — the in-memory equivalent of the repository's
	// "invert ORDER BY, LIMIT last+1, reverse" path.
	if last > 0 && len(owned) > last {
		owned = owned[len(owned)-last:]
	}
	return owned, total, nil
}

func (r *cursorWalkRepo) CountByOwner(_ context.Context, _ string, _ *string) (int64, error) {
	return int64(len(r.rows)), nil
}

func (r *cursorWalkRepo) Create(_ context.Context, _ *domain.Cardgroup) error { return nil }

func (r *cursorWalkRepo) Update(_ context.Context, _ string, _ repository.CardgroupUpdate) (*domain.Cardgroup, error) {
	return nil, nil
}

func (r *cursorWalkRepo) Delete(_ context.Context, _ string) error { return nil }

// walkBase is a fixed instant so the fixture timestamps are deterministic.
var walkBase = time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC)

// newCursorWalkFixture builds five owned rows whose UPDATED_AT DESC order is
// cg-a, cg-b, cg-c, cg-d, cg-e, plus one row owned by a different user so the
// owner filter and the cross-tenant cursor guard have something to reject.
func newCursorWalkFixture() *cursorWalkRepo {
	mk := func(id string, minutesAgo int) *domain.Cardgroup {
		return &domain.Cardgroup{
			ID:        domain.CardgroupID(id),
			OwnerID:   "u1",
			Name:      domain.CardgroupName("Deck " + id),
			CreatedAt: walkBase,
			UpdatedAt: walkBase.Add(-time.Duration(minutesAgo) * time.Minute),
		}
	}
	foreign := mk("cg-foreign", 3)
	foreign.OwnerID = "u2"
	return &cursorWalkRepo{rows: []*domain.Cardgroup{
		mk("cg-a", 1), mk("cg-b", 2), mk("cg-c", 3), mk("cg-d", 4), mk("cg-e", 5), foreign,
	}}
}

// newTiedCursorWalkFixture builds a fixture where cg-b and cg-c share the SAME
// updated_at, so the id tie-break is the only thing separating them. Under
// (updated_at DESC, id DESC) the order is cg-a, cg-c, cg-b, cg-d, cg-e.
//
// The tie matters because the ordering key alone cannot anchor a bookmark
// between two rows that carry the same value: a cursor holding only the
// timestamp would either re-serve the sibling or swallow it, depending on which
// side of the comparison the implementation puts the equal case on.
func newTiedCursorWalkFixture() *cursorWalkRepo {
	mk := func(id string, minutesAgo int) *domain.Cardgroup {
		return &domain.Cardgroup{
			ID:        domain.CardgroupID(id),
			OwnerID:   "u1",
			Name:      domain.CardgroupName("Deck " + id),
			CreatedAt: walkBase,
			UpdatedAt: walkBase.Add(-time.Duration(minutesAgo) * time.Minute),
		}
	}
	return &cursorWalkRepo{rows: []*domain.Cardgroup{
		mk("cg-a", 1), mk("cg-b", 2), mk("cg-c", 2), mk("cg-d", 4), mk("cg-e", 5),
	}}
}

// encodeWalkCursor rebuilds the opaque cursor the resolver emits for a boundary
// row of the given page: the v2 envelope carrying the ordering the page was
// served under plus that row's ordering-key value as captured at serve time.
// It mirrors orderedCursorEncoder in graph/resolver/connection.go — the usecase
// itself never encodes cursors.
func encodeWalkCursor(out *CardgroupConnectionOutput, id string) string {
	return cursor.EncodeV2(cursor.Payload{
		ID:        id,
		OrderBy:   out.Ordering.OrderBy,
		Direction: out.Ordering.Direction,
		OrderKey:  out.OrderKeys[id],
	})
}

// walkIDs lists the cardgroup ids of a page in order.
func walkIDs(out *CardgroupConnectionOutput) []string {
	ids := make([]string, 0, len(out.Cardgroups))
	for _, cg := range out.Cardgroups {
		ids = append(ids, string(cg.ID))
	}
	return ids
}

// fetchWalkPage runs one forward page of size two, optionally after a cursor.
func fetchWalkPage(t *testing.T, uc CardgroupUsecase, after *string) *CardgroupConnectionOutput {
	t.Helper()
	first := 2
	out, err := uc.ListCardgroupsByOwnerConnection(cgAuthedCtx("u1"), CardgroupConnectionInput{
		First: &first,
		After: after,
	})
	if err != nil {
		t.Fatalf("unexpected error paging: %v", err)
	}
	return out
}

// fetchWalkPageBackward runs one backward page of the given size, optionally
// before a cursor. Passing a nil cursor asks for the tail of the listing.
func fetchWalkPageBackward(t *testing.T, uc CardgroupUsecase, before *string, last int) *CardgroupConnectionOutput {
	t.Helper()
	out, err := uc.ListCardgroupsByOwnerConnection(cgAuthedCtx("u1"), CardgroupConnectionInput{
		Last:   &last,
		Before: before,
	})
	if err != nil {
		t.Fatalf("unexpected error paging backward: %v", err)
	}
	return out
}

func newWalkUsecase(repo *cursorWalkRepo) CardgroupUsecase {
	return NewCardgroupUsecase(repo, cgDefaultAdmin(), newTestLogger())
}

// setUpdatedAt moves a row's ordering key, simulating an edit made from another
// device between two page fetches.
func setUpdatedAt(t *testing.T, repo *cursorWalkRepo, id string, at time.Time) {
	t.Helper()
	for _, cg := range repo.rows {
		if string(cg.ID) == id {
			cg.UpdatedAt = at
			return
		}
	}
	t.Fatalf("fixture has no row %q", id)
}

// TestCardgroupCursorWalk_S1_BoundaryRowEditedUp_NoDuplicate reproduces the
// machine-found duplicate scenario: the boundary row of page 1 is edited so its
// ordering key moves to the top of the list before page 2 is fetched. With the
// captured ordering key the second page starts exactly where the first ended,
// so it repeats no row the caller has already seen.
func TestCardgroupCursorWalk_S1_BoundaryRowEditedUp_NoDuplicate(t *testing.T) {
	t.Parallel()

	repo := newCursorWalkFixture()
	uc := newWalkUsecase(repo)

	page1 := fetchWalkPage(t, uc, nil)
	if got := walkIDs(page1); len(got) != 2 || got[0] != "cg-a" || got[1] != "cg-b" {
		t.Fatalf("page 1 = %v, want [cg-a cg-b]", got)
	}
	next := encodeWalkCursor(page1, page1.EndCur)

	// The caller renames cg-b on another device: updated_at jumps to the newest
	// value, moving the row to the head of the DESC listing.
	setUpdatedAt(t, repo, "cg-b", walkBase.Add(time.Minute))

	page2 := fetchWalkPage(t, uc, &next)
	got := walkIDs(page2)
	if len(got) != 2 || got[0] != "cg-c" || got[1] != "cg-d" {
		t.Fatalf("page 2 = %v, want [cg-c cg-d]", got)
	}
	for _, id := range got {
		if id == "cg-a" || id == "cg-b" {
			t.Fatalf("page 2 repeated %q from page 1: %v", id, got)
		}
	}
}

// TestCardgroupCursorWalk_S1_V1Cursor_StillDuplicates pins the defect the v2
// envelope exists to fix, and simultaneously proves the v1 backward-compat path
// is still wired: an id-only cursor re-reads the edited row and therefore hands
// back a row page 1 already returned.
func TestCardgroupCursorWalk_S1_V1Cursor_StillDuplicates(t *testing.T) {
	t.Parallel()

	repo := newCursorWalkFixture()
	uc := newWalkUsecase(repo)

	page1 := fetchWalkPage(t, uc, nil)
	legacy := cursor.Encode(page1.EndCur)

	setUpdatedAt(t, repo, "cg-b", walkBase.Add(time.Minute))

	page2 := fetchWalkPage(t, uc, &legacy)
	got := walkIDs(page2)
	if len(got) == 0 || got[0] != "cg-a" {
		t.Fatalf("v1 cursor should re-serve cg-a after the boundary row moves up; page 2 = %v", got)
	}
}

// TestCardgroupCursorWalk_S2_BoundaryRowEditedDown_NoSkip reproduces the
// machine-found skip scenario: the boundary row of page 1 is edited so its
// ordering key drops below every remaining row. With the captured ordering key
// the walk continues from where page 1 ended, so every row the caller had not
// yet seen is still returned exactly once.
//
// The edited row itself is the documented exception: its ordering key moved
// into the not-yet-visited region, so the walk legitimately meets it again. No
// cursor scheme can avoid that — the edit moved the row across the bookmark,
// not the bookmark across the rows. What v2 fixes is that the UNSEEN rows are
// no longer swallowed along with it.
func TestCardgroupCursorWalk_S2_BoundaryRowEditedDown_NoSkip(t *testing.T) {
	t.Parallel()

	repo := newCursorWalkFixture()
	uc := newWalkUsecase(repo)

	page1 := fetchWalkPage(t, uc, nil)
	next := encodeWalkCursor(page1, page1.EndCur)

	// cg-b is edited so it now sorts last.
	setUpdatedAt(t, repo, "cg-b", walkBase.Add(-99*time.Minute))

	seen := append([]string{}, walkIDs(page1)...)
	cur := next
	for page := 2; page <= 5; page++ {
		out := fetchWalkPage(t, uc, &cur)
		ids := walkIDs(out)
		if len(ids) == 0 {
			break
		}
		seen = append(seen, ids...)
		cur = encodeWalkCursor(out, out.EndCur)
	}

	counts := map[string]int{}
	for _, id := range seen {
		counts[id]++
	}
	// cg-a was already served on page 1 and was not edited; cg-c / cg-d / cg-e
	// were unseen when the edit landed. All four must appear exactly once.
	for _, id := range []string{"cg-a", "cg-c", "cg-d", "cg-e"} {
		if counts[id] != 1 {
			t.Fatalf("row %q appeared %d times across the walk (want exactly 1); walk = %v", id, counts[id], seen)
		}
	}
	if counts["cg-foreign"] != 0 {
		t.Fatal("walk leaked a row owned by another user")
	}
}

// TestCardgroupCursorWalk_S2_V1Cursor_StillSkips pins the defect the v2
// envelope exists to fix on the skip side: an id-only cursor re-reads the
// edited boundary row, finds it now sorts last, and reports an empty second
// page — the "rows silently vanish" symptom.
func TestCardgroupCursorWalk_S2_V1Cursor_StillSkips(t *testing.T) {
	t.Parallel()

	repo := newCursorWalkFixture()
	uc := newWalkUsecase(repo)

	page1 := fetchWalkPage(t, uc, nil)
	legacy := cursor.Encode(page1.EndCur)

	setUpdatedAt(t, repo, "cg-b", walkBase.Add(-99*time.Minute))

	page2 := fetchWalkPage(t, uc, &legacy)
	if got := walkIDs(page2); len(got) != 0 {
		t.Fatalf("v1 cursor should return an empty page after the boundary row moves to the bottom, got %v", got)
	}
}

// TestCardgroupCursorWalk_LegacyBareIDStillPages verifies the oldest cursor
// form — a bare UUID with no envelope at all — still decodes and pages.
func TestCardgroupCursorWalk_LegacyBareIDStillPages(t *testing.T) {
	t.Parallel()

	repo := newCursorWalkFixture()
	uc := newWalkUsecase(repo)

	page1 := fetchWalkPage(t, uc, nil)
	bare := page1.EndCur

	page2 := fetchWalkPage(t, uc, &bare)
	if got := walkIDs(page2); len(got) != 2 || got[0] != "cg-c" || got[1] != "cg-d" {
		t.Fatalf("legacy bare-id cursor page 2 = %v, want [cg-c cg-d]", got)
	}
}

// TestCardgroupCursorWalk_OrderKeysCoverEveryReturnedRow verifies the usecase
// output carries the ordering metadata the resolver needs to emit a v2 cursor
// for every edge, not just the page boundaries.
func TestCardgroupCursorWalk_OrderKeysCoverEveryReturnedRow(t *testing.T) {
	t.Parallel()

	repo := newCursorWalkFixture()
	out := fetchWalkPage(t, newWalkUsecase(repo), nil)

	if out.Ordering != (PageOrdering{
		OrderBy:   string(repository.CardgroupOrderByUpdatedAt),
		Direction: string(repository.SortDesc),
	}) {
		t.Fatalf("Ordering = %+v, want the schema default (updated_at, DESC)", out.Ordering)
	}
	for _, cg := range out.Cardgroups {
		got, ok := out.OrderKeys[string(cg.ID)]
		if !ok {
			t.Fatalf("OrderKeys is missing row %q", cg.ID)
		}
		if want := cg.UpdatedAt.UTC().Format(time.RFC3339Nano); got != want {
			t.Fatalf("OrderKeys[%q] = %q, want %q", cg.ID, got, want)
		}
	}
}

// ---------------------------------------------------------------------------
// Backward paging and tied ordering keys.
//
// The forward walks above exercise the trailing-edge trim. Backward paging runs
// the opposite path — the repository inverts the ORDER BY, takes last+1 rows and
// reverses them, and the usecase trims the LEADING row — so the v2 decode, the
// boundary comparison and the trim have to compose correctly on that edge too.
// ---------------------------------------------------------------------------

// TestCardgroupCursorWalk_BackwardV2_BoundaryRowEditedDown_NoDuplicate is the
// backward mirror of S1. Paging backwards from the tail, the boundary row of the
// first backward page is edited so its ordering key drops to the bottom of the
// listing. With the captured key the previous page still starts exactly where
// the first ended and repeats nothing the caller has already seen.
func TestCardgroupCursorWalk_BackwardV2_BoundaryRowEditedDown_NoDuplicate(t *testing.T) {
	t.Parallel()

	repo := newCursorWalkFixture()
	uc := newWalkUsecase(repo)

	// Tail of the listing under (updated_at DESC, id DESC): cg-d, cg-e.
	page1 := fetchWalkPageBackward(t, uc, nil, 2)
	if got := walkIDs(page1); len(got) != 2 || got[0] != "cg-d" || got[1] != "cg-e" {
		t.Fatalf("backward page 1 = %v, want [cg-d cg-e]", got)
	}
	if !page1.HasPrev {
		t.Fatal("backward page 1 must report hasPreviousPage: cg-a/cg-b/cg-c are still ahead")
	}
	if page1.HasNext {
		t.Fatal("backward page 1 without a before cursor is the tail; hasNextPage must be false")
	}

	prev := encodeWalkCursor(page1, page1.StartCur)

	// cg-d — the leading boundary row of the page just served — is edited so it
	// now sorts last. A v1 cursor would re-read it and walk from the bottom.
	setUpdatedAt(t, repo, "cg-d", walkBase.Add(-99*time.Minute))

	page2 := fetchWalkPageBackward(t, uc, &prev, 2)
	got := walkIDs(page2)
	if len(got) != 2 || got[0] != "cg-b" || got[1] != "cg-c" {
		t.Fatalf("backward page 2 = %v, want [cg-b cg-c]", got)
	}
	for _, id := range got {
		if id == "cg-d" || id == "cg-e" {
			t.Fatalf("backward page 2 repeated %q from page 1: %v", id, got)
		}
	}
	if !page2.HasNext {
		t.Fatal("a backward page taken before a cursor must report hasNextPage")
	}
	if !page2.HasPrev {
		t.Fatal("cg-a is still ahead of backward page 2; hasPreviousPage must be true")
	}
}

// TestCardgroupCursorWalk_BackwardV1Cursor_StillDuplicates pins the defect on
// the backward edge, the mirror of the forward S1/S2 v1 regressions: an id-only
// cursor re-reads the edited boundary row, finds it now sorts last, and hands
// back rows the caller already saw.
func TestCardgroupCursorWalk_BackwardV1Cursor_StillDuplicates(t *testing.T) {
	t.Parallel()

	repo := newCursorWalkFixture()
	uc := newWalkUsecase(repo)

	page1 := fetchWalkPageBackward(t, uc, nil, 2)
	legacy := cursor.Encode(page1.StartCur)

	setUpdatedAt(t, repo, "cg-d", walkBase.Add(-99*time.Minute))

	page2 := fetchWalkPageBackward(t, uc, &legacy, 2)
	got := walkIDs(page2)
	if len(got) == 0 || got[len(got)-1] != "cg-e" {
		t.Fatalf("v1 cursor should re-serve cg-e after the boundary row moves to the bottom; backward page 2 = %v", got)
	}
}

// TestCardgroupCursorWalk_TiedOrderKeys_ForwardAndBackward verifies the id
// tie-break survives the v2 round trip on both edges. cg-b and cg-c carry the
// same updated_at, so a bookmark taken at cg-c must still separate it from
// cg-b — forward must serve cg-b next, and paging back before cg-b must land on
// cg-c, not skip past the whole tied pair or re-serve it.
func TestCardgroupCursorWalk_TiedOrderKeys_ForwardAndBackward(t *testing.T) {
	t.Parallel()

	uc := newWalkUsecase(newTiedCursorWalkFixture())

	// Forward: page 1 ends on cg-c, the first of the tied pair under id DESC.
	page1 := fetchWalkPage(t, uc, nil)
	if got := walkIDs(page1); len(got) != 2 || got[0] != "cg-a" || got[1] != "cg-c" {
		t.Fatalf("tied page 1 = %v, want [cg-a cg-c]", got)
	}
	next := encodeWalkCursor(page1, page1.EndCur)
	page2 := fetchWalkPage(t, uc, &next)
	if got := walkIDs(page2); len(got) != 2 || got[0] != "cg-b" || got[1] != "cg-d" {
		t.Fatalf("tied page 2 = %v, want [cg-b cg-d]: the tie-break must not swallow cg-b", got)
	}

	// Guard the premise: if the two rows stopped sharing an ordering key the
	// walk above would pass for the wrong reason — it would no longer be
	// exercising the tie at all.
	keyC, keyB := page1.OrderKeys["cg-c"], page2.OrderKeys["cg-b"]
	if keyC == "" || keyB == "" {
		t.Fatalf("OrderKeys must cover both tied rows; got cg-c=%q cg-b=%q", keyC, keyB)
	}
	if keyC != keyB {
		t.Fatalf("fixture no longer ties the rows: cg-c=%q, cg-b=%q", keyC, keyB)
	}

	// Backward: paging before cg-b must return the rows immediately ahead of it,
	// which means stopping at its tied sibling cg-c rather than jumping the pair.
	beforeB := cursor.EncodeV2(cursor.Payload{
		ID:        "cg-b",
		OrderBy:   page2.Ordering.OrderBy,
		Direction: page2.Ordering.Direction,
		OrderKey:  page2.OrderKeys["cg-b"],
	})
	back := fetchWalkPageBackward(t, uc, &beforeB, 2)
	if got := walkIDs(back); len(got) != 2 || got[0] != "cg-a" || got[1] != "cg-c" {
		t.Fatalf("backward page before cg-b = %v, want [cg-a cg-c]", got)
	}
	if back.HasPrev {
		t.Fatal("cg-a is the head of the listing; hasPreviousPage must be false")
	}
	if !back.HasNext {
		t.Fatal("a backward page taken before a cursor must report hasNextPage")
	}
}

// TestCardgroupCursorWalk_BackwardV2_ForeignOwnerStillRejected verifies the
// cross-tenant guard runs on the backward cursor field too. The scope lookup is
// what keeps the endpoint from becoming an existence oracle, and the v2 path
// skips the ordering re-read — so the guard has to be reachable independently of
// hydration on both edges.
func TestCardgroupCursorWalk_BackwardV2_ForeignOwnerStillRejected(t *testing.T) {
	t.Parallel()

	uc := newWalkUsecase(newCursorWalkFixture())
	cur := cursor.EncodeV2(cursor.Payload{
		ID:        "cg-foreign",
		OrderBy:   string(repository.CardgroupOrderByUpdatedAt),
		Direction: string(repository.SortDesc),
		OrderKey:  walkBase.Add(-3 * time.Minute).Format(time.RFC3339Nano),
	})
	last := 2
	_, err := uc.ListCardgroupsByOwnerConnection(cgAuthedCtx("u1"), CardgroupConnectionInput{
		Last:   &last,
		Before: &cur,
	})
	assertValidationError(t, err, "before", "cursor not found")
}

// ---------------------------------------------------------------------------
// v2 cursor rejection paths
// ---------------------------------------------------------------------------

// TestResolveCardgroupCursor_V2OrderingMismatch_ReturnsBadUserInput verifies a
// cursor taken under a different column or direction is rejected with the
// existing BAD_USER_INPUT shape rather than silently mis-paging against a
// value that belongs to another column.
func TestResolveCardgroupCursor_V2OrderingMismatch_ReturnsBadUserInput(t *testing.T) {
	t.Parallel()

	cases := map[string]cursor.Payload{
		"different column": {
			ID: "cg-a", OrderBy: string(repository.CardgroupOrderByName),
			Direction: string(repository.SortDesc), OrderKey: "Deck cg-a",
		},
		"different direction": {
			ID: "cg-a", OrderBy: string(repository.CardgroupOrderByUpdatedAt),
			Direction: string(repository.SortAsc), OrderKey: walkBase.Format(time.RFC3339Nano),
		},
	}
	for name, p := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			uc := newWalkUsecase(newCursorWalkFixture())
			cur := cursor.EncodeV2(p)
			first := 2
			_, err := uc.ListCardgroupsByOwnerConnection(cgAuthedCtx("u1"), CardgroupConnectionInput{
				First: &first,
				After: &cur,
			})
			assertValidationError(t, err, "after", "")
		})
	}
}

// TestResolveCardgroupCursor_V2MalformedOrderKey_ReturnsBadUserInput verifies a
// v2 cursor whose ordering-key value does not parse into the active column's
// type is a client error, not an INTERNAL fault.
func TestResolveCardgroupCursor_V2MalformedOrderKey_ReturnsBadUserInput(t *testing.T) {
	t.Parallel()

	uc := newWalkUsecase(newCursorWalkFixture())
	cur := cursor.EncodeV2(cursor.Payload{
		ID:        "cg-a",
		OrderBy:   string(repository.CardgroupOrderByUpdatedAt),
		Direction: string(repository.SortDesc),
		OrderKey:  "not-a-timestamp",
	})
	first := 2
	_, err := uc.ListCardgroupsByOwnerConnection(cgAuthedCtx("u1"), CardgroupConnectionInput{
		First: &first,
		After: &cur,
	})
	assertValidationError(t, err, "after", "")
}

// TestResolveCardgroupCursor_V2ForeignOwner_ReturnsCursorNotFound verifies the
// cross-tenant guard is not bypassed by a v2 cursor. A v2 cursor can hydrate
// its ordering column without the repository, but the ownership lookup must
// still run or the endpoint becomes an existence oracle over other users'
// cardgroup ids.
func TestResolveCardgroupCursor_V2ForeignOwner_ReturnsCursorNotFound(t *testing.T) {
	t.Parallel()

	repo := newCursorWalkFixture()
	uc := newWalkUsecase(repo)

	cur := cursor.EncodeV2(cursor.Payload{
		ID:        "cg-foreign",
		OrderBy:   string(repository.CardgroupOrderByUpdatedAt),
		Direction: string(repository.SortDesc),
		OrderKey:  walkBase.Add(-3 * time.Minute).Format(time.RFC3339Nano),
	})
	first := 2
	_, err := uc.ListCardgroupsByOwnerConnection(cgAuthedCtx("u1"), CardgroupConnectionInput{
		First: &first,
		After: &cur,
	})
	assertValidationError(t, err, "after", "cursor not found")
}

// TestResolveCardgroupCursor_V2UnknownID_ReturnsCursorNotFound verifies a v2
// cursor for a row that no longer exists is rejected exactly like a v1 one —
// the hydration lookup still runs even though the ordering key is embedded.
func TestResolveCardgroupCursor_V2UnknownID_ReturnsCursorNotFound(t *testing.T) {
	t.Parallel()

	uc := newWalkUsecase(newCursorWalkFixture())
	cur := cursor.EncodeV2(cursor.Payload{
		ID:        "cg-deleted",
		OrderBy:   string(repository.CardgroupOrderByUpdatedAt),
		Direction: string(repository.SortDesc),
		OrderKey:  walkBase.Format(time.RFC3339Nano),
	})
	first := 2
	_, err := uc.ListCardgroupsByOwnerConnection(cgAuthedCtx("u1"), CardgroupConnectionInput{
		First: &first,
		After: &cur,
	})
	assertValidationError(t, err, "after", "cursor not found")
}

// TestApplyCardgroupOrderKey_PerColumn covers the decode half of the ordering
// key for every column in the allowlist, including the ID column (whose key is
// empty because the id is already carried) and the impossible default arm,
// which must stay INTERNAL rather than degrade to BAD_USER_INPUT.
func TestApplyCardgroupOrderKey_PerColumn(t *testing.T) {
	t.Parallel()

	when := walkBase.Format(time.RFC3339Nano)

	c := &repository.CardgroupCursor{ID: "cg-a"}
	if err := applyCardgroupOrderKey(c, repository.CardgroupOrderByID, ""); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.Name != nil || c.CreatedAt != nil || c.UpdatedAt != nil {
		t.Fatalf("orderBy=id must hydrate no extra column, got %+v", c)
	}

	c = &repository.CardgroupCursor{ID: "cg-a"}
	if err := applyCardgroupOrderKey(c, repository.CardgroupOrderByName, "Deck cg-a"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.Name == nil || *c.Name != "Deck cg-a" {
		t.Fatalf("orderBy=name must hydrate Name, got %+v", c)
	}

	c = &repository.CardgroupCursor{ID: "cg-a"}
	if err := applyCardgroupOrderKey(c, repository.CardgroupOrderByCreatedAt, when); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.CreatedAt == nil || !c.CreatedAt.Equal(walkBase) {
		t.Fatalf("orderBy=created_at must hydrate CreatedAt, got %+v", c)
	}

	c = &repository.CardgroupCursor{ID: "cg-a"}
	if err := applyCardgroupOrderKey(c, repository.CardgroupOrderByUpdatedAt, when); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.UpdatedAt == nil || !c.UpdatedAt.Equal(walkBase) {
		t.Fatalf("orderBy=updated_at must hydrate UpdatedAt, got %+v", c)
	}

	err := applyCardgroupOrderKey(&repository.CardgroupCursor{}, repository.CardgroupOrderBy("not_a_real_column"), "")
	assertInternalChain(t, err, "usecase: cardgroup: unhandled orderBy")
}

// TestCardgroupOrderKey_UnknownOrderBy covers the encode half's impossible
// default arm: an unmapped column is a caller bug, surfaced as INTERNAL rather
// than an empty key that would silently produce an unanchored cursor.
func TestCardgroupOrderKey_UnknownOrderBy(t *testing.T) {
	t.Parallel()

	_, err := cardgroupOrderKeys(repository.CardgroupOrderBy("not_a_real_column"), []*domain.Cardgroup{
		{ID: domain.CardgroupID("cg-a"), OwnerID: "u1"},
	})
	assertInternalChain(t, err, "usecase: cardgroup: unhandled orderBy")
}

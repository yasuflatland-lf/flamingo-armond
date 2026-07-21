package usecase

import (
	"context"
	"errors"
	"sort"
	"strconv"
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
// The master-card listing defaults to POSITION ASC — a column an admin batch
// import rewrites for every conflicting row. It also offers UPDATED_AT, which
// the same import touches on every row it overwrites. A cursor that carries only
// a row id has to re-read that row at serve time to recover its ordering value,
// so changing the row between two page fetches moves the bookmark: rows already
// returned come back a second time (S1) or rows the caller has not seen yet are
// skipped (S2). A v2 cursor carries the ordering value captured when the page was
// served, so the bookmark stays put.
//
// These walks use masterCardWalkRepo, an in-memory repository implementing the
// same (orderKey, id) tuple comparison the SQL repository emits, so the
// scenarios are reproduced end-to-end through ListMasterCards /
// ListPublicMasterCards without a database.
//
// "Moved to the head" / "moved to the tail" are used instead of "raised" /
// "lowered" throughout: under the ASC default a SMALLER position sorts EARLIER,
// so promoting a row to the head means decreasing its position number.
// ---------------------------------------------------------------------------

// mcWalkDeckID is the master deck every walk pages through.
const mcWalkDeckID = "deck-1"

// mcWalkForeignDeckID owns the one fixture row that must never surface: it backs
// the cross-deck cursor guard.
const mcWalkForeignDeckID = "deck-2"

// masterCardWalkRepo is an in-memory master-card repository supporting exactly
// the slice of the interface these walks need: forward AND backward paging over
// one deck's rows ordered by (orderKey ASC, id ASC), where orderKey is position
// or updated_at. Any other ordering is rejected loudly so a future test cannot
// silently exercise an unimplemented branch. FindByID returns the CURRENT row,
// which is what makes the v1 re-hydration path observe a mutation made between
// two page fetches.
//
// The backward branch models the repository's direction-flip + reverse: it keeps
// the rows that sort strictly BEFORE the cursor and returns the ones closest to
// it, still in ASC order, so the usecase's leading-edge trim
// (TrimAndDetectBackward) sees the same slice shape SQL would hand it.
type masterCardWalkRepo struct {
	panicMasterCardRepo

	rows []*domain.MasterCard
}

func (r *masterCardWalkRepo) FindByID(_ context.Context, id string) (*domain.MasterCard, error) {
	for _, c := range r.rows {
		if c.ID == id {
			return c, nil
		}
	}
	return nil, repository.ErrNotFound
}

// mcWalkKey is the fake's model of the SQL sort expression, normalised to one
// comparable scalar so a single tuple predicate serves both orderings:
// master_cards.position for POSITION, and the nanosecond instant of
// master_cards.updated_at for UPDATED_AT. The fixture timestamps are fixed UTC
// values, so the nanosecond flattening is lossless here.
func mcWalkKey(orderBy repository.MasterCardOrderBy, c *domain.MasterCard) (int64, error) {
	switch orderBy {
	case repository.MasterCardOrderByPosition:
		return int64(c.Position), nil
	case repository.MasterCardOrderByUpdatedAt:
		return c.UpdatedAt.UnixNano(), nil
	default:
		return 0, eris.Errorf("masterCardWalkRepo: unhandled orderBy %q", orderBy)
	}
}

// mcWalkCursorKey extracts the boundary value the active ordering compares
// against. A cursor that reaches the repository without its column populated is
// a usecase bug, so it is surfaced rather than defaulted to a zero key.
func mcWalkCursorKey(orderBy repository.MasterCardOrderBy, c *repository.MasterCardCursor) (int64, error) {
	switch orderBy {
	case repository.MasterCardOrderByPosition:
		if c.Position == nil {
			return 0, eris.New("masterCardWalkRepo: cursor is missing the position column")
		}
		return int64(*c.Position), nil
	case repository.MasterCardOrderByUpdatedAt:
		if c.UpdatedAt == nil {
			return 0, eris.New("masterCardWalkRepo: cursor is missing the updated_at column")
		}
		return c.UpdatedAt.UnixNano(), nil
	default:
		return 0, eris.Errorf("masterCardWalkRepo: unhandled orderBy %q", orderBy)
	}
}

// mcAfterInAscTuple reports whether a row sorts strictly after the cursor under
// the (orderKey ASC, id ASC) total order the repository emits.
func mcAfterInAscTuple(rowKey int64, rowID string, curKey int64, curID string) bool {
	if rowKey == curKey {
		return rowID > curID
	}
	return rowKey > curKey
}

// mcBeforeInAscTuple is the mirror predicate for backward paging: it reports
// whether a row sorts strictly before the cursor under the same total order. It
// is not !mcAfterInAscTuple — the cursor row itself sorts neither after nor
// before itself, and both edges must exclude it.
func mcBeforeInAscTuple(rowKey int64, rowID string, curKey int64, curID string) bool {
	if rowKey == curKey {
		return rowID < curID
	}
	return rowKey < curKey
}

func (r *masterCardWalkRepo) FindPageByMasterCardgroup(
	_ context.Context,
	masterCardgroupID string,
	after, before *repository.MasterCardCursor,
	first, last int,
	orderBy repository.MasterCardOrderBy,
	dir repository.SortOrder,
	_ *string,
) ([]*domain.MasterCard, int64, error) {
	if dir != repository.SortAsc {
		return nil, 0, eris.Errorf("masterCardWalkRepo: only the ASC direction is implemented; got dir=%q", dir)
	}

	// Keys are computed for EVERY fixture row, not just the scoped ones, so an
	// ordering this fake does not implement is rejected even when the deck filter
	// would have emptied the page first.
	keys := make(map[string]int64, len(r.rows))
	for _, c := range r.rows {
		k, err := mcWalkKey(orderBy, c)
		if err != nil {
			return nil, 0, err
		}
		keys[c.ID] = k
	}

	scoped := make([]*domain.MasterCard, 0, len(r.rows))
	for _, c := range r.rows {
		if c.BelongsToMasterCardgroup(masterCardgroupID) {
			scoped = append(scoped, c)
		}
	}
	sort.SliceStable(scoped, func(i, j int) bool {
		if keys[scoped[i].ID] != keys[scoped[j].ID] {
			return keys[scoped[i].ID] < keys[scoped[j].ID]
		}
		return scoped[i].ID < scoped[j].ID
	})
	total := int64(len(scoped))

	if after != nil {
		key, err := mcWalkCursorKey(orderBy, after)
		if err != nil {
			return nil, 0, err
		}
		rest := make([]*domain.MasterCard, 0, len(scoped))
		for _, c := range scoped {
			if mcAfterInAscTuple(keys[c.ID], c.ID, key, after.ID) {
				rest = append(rest, c)
			}
		}
		scoped = rest
	}
	if before != nil {
		key, err := mcWalkCursorKey(orderBy, before)
		if err != nil {
			return nil, 0, err
		}
		rest := make([]*domain.MasterCard, 0, len(scoped))
		for _, c := range scoped {
			if mcBeforeInAscTuple(keys[c.ID], c.ID, key, before.ID) {
				rest = append(rest, c)
			}
		}
		scoped = rest
	}
	if first > 0 && len(scoped) > first {
		scoped = scoped[:first]
	}
	// Backward paging takes the rows CLOSEST to the before cursor, which are the
	// trailing ones in ASC order — the in-memory equivalent of the repository's
	// "invert ORDER BY, LIMIT last+1, reverse" path.
	if last > 0 && len(scoped) > last {
		scoped = scoped[len(scoped)-last:]
	}
	return scoped, total, nil
}

// mcWalkBase is a fixed instant so the fixture timestamps are deterministic.
var mcWalkBase = time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC)

// mcWalkAt returns the fixture instant N hours after the base.
func mcWalkAt(hours int) time.Time {
	return mcWalkBase.Add(time.Duration(hours) * time.Hour)
}

// newMasterCardWalkFixture builds five rows in mcWalkDeckID whose POSITION ASC
// order is mc-a, mc-b, mc-c, mc-d, mc-e, plus one row in a different deck so the
// deck filter and the cross-deck cursor guard have something to reject.
//
// Each row's updated_at is derived from its position, so the UPDATED_AT walks
// start from the same row order as the POSITION ones and the two orderings can
// share every fixture assertion.
func newMasterCardWalkFixture() *masterCardWalkRepo {
	mk := func(id string, pos int) *domain.MasterCard {
		return &domain.MasterCard{
			ID:                id,
			MasterCardgroupID: mcWalkDeckID,
			Front:             domain.CardText("front-" + id),
			Back:              domain.CardText("back-" + id),
			Position:          pos,
			CreatedAt:         mcWalkBase,
			UpdatedAt:         mcWalkAt(pos),
		}
	}
	foreign := mk("mc-foreign", 3)
	foreign.MasterCardgroupID = mcWalkForeignDeckID
	return &masterCardWalkRepo{rows: []*domain.MasterCard{
		mk("mc-a", 1), mk("mc-b", 2), mk("mc-c", 3), mk("mc-d", 4), mk("mc-e", 5), foreign,
	}}
}

// newTiedMasterCardWalkFixture builds a fixture where mc-b and mc-c share the
// SAME position, so the id tie-break is the only thing separating them. Under
// (position ASC, id ASC) the order is mc-a, mc-b, mc-c, mc-d, mc-e.
//
// The tie matters because the ordering key alone cannot anchor a bookmark
// between two rows that carry the same value: a cursor holding only the position
// would either re-serve the sibling or swallow it, depending on which side of
// the comparison the implementation puts the equal case on.
func newTiedMasterCardWalkFixture() *masterCardWalkRepo {
	mk := func(id string, pos int) *domain.MasterCard {
		return &domain.MasterCard{
			ID:                id,
			MasterCardgroupID: mcWalkDeckID,
			Front:             domain.CardText("front-" + id),
			Back:              domain.CardText("back-" + id),
			Position:          pos,
			CreatedAt:         mcWalkBase,
			UpdatedAt:         mcWalkBase,
		}
	}
	return &masterCardWalkRepo{rows: []*domain.MasterCard{
		mk("mc-a", 1), mk("mc-b", 2), mk("mc-c", 2), mk("mc-d", 4), mk("mc-e", 5),
	}}
}

// newMasterCardWalkUsecase wires the walk repo behind an admin-gated usecase.
// The deck repo answers FindPublishedByID so the public listing's published-only
// gate passes; the admin listing never touches it.
func newMasterCardWalkUsecase(repo *masterCardWalkRepo) MasterCardUsecase {
	deckRepo := &mockMasterCardgroupReadRepo{
		findPublishedByIDFn: func(id string) (*domain.MasterCardgroup, error) {
			return masterCardgroup(id), nil
		},
	}
	return NewMasterCardUsecase(nil, repo, deckRepo, newTestAdminGate(true), newTestLogger())
}

// mcEncodeWalkCursor rebuilds the opaque cursor the resolver emits for a
// boundary row of the given page: the v2 envelope carrying the ordering the page
// was served under plus that row's ordering-key value as captured at serve time.
// It mirrors orderedCursorEncoder in graph/resolver/connection.go — the usecase
// itself never encodes cursors.
func mcEncodeWalkCursor(out *MasterCardConnectionOutput, id string) string {
	return cursor.EncodeV2(cursor.Payload{
		ID:        id,
		OrderBy:   out.Ordering.OrderBy,
		Direction: out.Ordering.Direction,
		OrderKey:  out.OrderKeys[id],
	})
}

// mcWalkIDs lists the master card ids of a page in order.
func mcWalkIDs(out *MasterCardConnectionOutput) []string {
	ids := make([]string, 0, len(out.Cards))
	for _, c := range out.Cards {
		ids = append(ids, c.ID)
	}
	return ids
}

// mcFetchWalkPage runs one forward page of size two, optionally after a cursor.
// It sends no orderBy, so the walk also covers the schema default (POSITION ASC)
// resolution rather than pinning the column from the caller side.
func mcFetchWalkPage(t *testing.T, uc MasterCardUsecase, after *string) *MasterCardConnectionOutput {
	t.Helper()
	return mcFetchWalkPageOrdered(t, uc, nil, after)
}

// mcFetchWalkPageOrdered runs one forward page of size two under the given
// ordering — nil asks for the schema default — optionally after a cursor.
func mcFetchWalkPageOrdered(
	t *testing.T, uc MasterCardUsecase, orderBy *MasterCardOrderBy, after *string,
) *MasterCardConnectionOutput {
	t.Helper()
	first := 2
	out, err := uc.ListMasterCards(authedCtx("admin1"), MasterCardConnectionInput{
		MasterCardgroupID: mcWalkDeckID,
		First:             &first,
		After:             after,
		OrderBy:           orderBy,
	})
	if err != nil {
		t.Fatalf("unexpected error paging: %v", err)
	}
	return out
}

// mcFetchWalkPageBackward runs one backward page of the given size, optionally
// before a cursor. Passing a nil cursor asks for the tail of the listing.
func mcFetchWalkPageBackward(t *testing.T, uc MasterCardUsecase, before *string, last int) *MasterCardConnectionOutput {
	t.Helper()
	out, err := uc.ListMasterCards(authedCtx("admin1"), MasterCardConnectionInput{
		MasterCardgroupID: mcWalkDeckID,
		Last:              &last,
		Before:            before,
	})
	if err != nil {
		t.Fatalf("unexpected error paging backward: %v", err)
	}
	return out
}

// mcOrderByPtr lifts an ordering into the pointer the connection input takes.
func mcOrderByPtr(v MasterCardOrderBy) *MasterCardOrderBy { return &v }

// mcSetPosition moves a row's POSITION ordering key, simulating the
// repositioning a batch import performs on every conflicting row between two
// page fetches.
func mcSetPosition(t *testing.T, repo *masterCardWalkRepo, id string, pos int) {
	t.Helper()
	mcMutateWalkRow(t, repo, id, func(c *domain.MasterCard) { c.Position = pos })
}

// mcSetUpdatedAt moves a row's UPDATED_AT ordering key, simulating the touch a
// batch import applies to every row it overwrites between two page fetches.
func mcSetUpdatedAt(t *testing.T, repo *masterCardWalkRepo, id string, at time.Time) {
	t.Helper()
	mcMutateWalkRow(t, repo, id, func(c *domain.MasterCard) { c.UpdatedAt = at })
}

// mcMutateWalkRow applies an edit to one fixture row in place, failing loudly
// when the id does not exist so a renamed fixture row cannot silently turn a
// stability walk into a no-op.
func mcMutateWalkRow(t *testing.T, repo *masterCardWalkRepo, id string, edit func(*domain.MasterCard)) {
	t.Helper()
	for _, c := range repo.rows {
		if c.ID == id {
			edit(c)
			return
		}
	}
	t.Fatalf("fixture has no row %q", id)
}

// TestMasterCardCursorWalk_S1_BoundaryRowMovedToHead_NoDuplicate reproduces the
// duplicate scenario: the boundary row of page 1 is repositioned so its ordering
// key moves to the head of the listing before page 2 is fetched. With the
// captured ordering key the second page starts exactly where the first ended, so
// it repeats no row the caller has already seen.
func TestMasterCardCursorWalk_S1_BoundaryRowMovedToHead_NoDuplicate(t *testing.T) {
	t.Parallel()

	repo := newMasterCardWalkFixture()
	uc := newMasterCardWalkUsecase(repo)

	page1 := mcFetchWalkPage(t, uc, nil)
	if got := mcWalkIDs(page1); len(got) != 2 || got[0] != "mc-a" || got[1] != "mc-b" {
		t.Fatalf("page 1 = %v, want [mc-a mc-b]", got)
	}
	next := mcEncodeWalkCursor(page1, page1.EndCur)

	// A batch import rewrites mc-b's position to the head of the deck.
	mcSetPosition(t, repo, "mc-b", 0)

	page2 := mcFetchWalkPage(t, uc, &next)
	got := mcWalkIDs(page2)
	if len(got) != 2 || got[0] != "mc-c" || got[1] != "mc-d" {
		t.Fatalf("page 2 = %v, want [mc-c mc-d]", got)
	}
	for _, id := range got {
		if id == "mc-a" || id == "mc-b" {
			t.Fatalf("page 2 repeated %q from page 1: %v", id, got)
		}
	}
}

// TestMasterCardCursorWalk_S1_V1Cursor_StillDuplicates pins the defect the v2
// envelope exists to fix, and simultaneously proves the v1 backward-compat path
// is still wired: an id-only cursor re-reads the repositioned row and therefore
// hands back a row page 1 already returned.
func TestMasterCardCursorWalk_S1_V1Cursor_StillDuplicates(t *testing.T) {
	t.Parallel()

	repo := newMasterCardWalkFixture()
	uc := newMasterCardWalkUsecase(repo)

	page1 := mcFetchWalkPage(t, uc, nil)
	legacy := cursor.Encode(page1.EndCur)

	mcSetPosition(t, repo, "mc-b", 0)

	page2 := mcFetchWalkPage(t, uc, &legacy)
	got := mcWalkIDs(page2)
	if len(got) == 0 || got[0] != "mc-a" {
		t.Fatalf("v1 cursor should re-serve mc-a after the boundary row moves to the head; page 2 = %v", got)
	}
}

// TestMasterCardCursorWalk_S2_BoundaryRowMovedToTail_NoSkip reproduces the skip
// scenario: the boundary row of page 1 is repositioned so its ordering key drops
// below every remaining row. With the captured ordering key the walk continues
// from where page 1 ended, so every row the caller had not yet seen is still
// returned exactly once.
//
// The repositioned row itself is the documented exception: its ordering key
// moved into the not-yet-visited region, so the walk legitimately meets it
// again. No cursor scheme can avoid that — the edit moved the row across the
// bookmark, not the bookmark across the rows. What v2 fixes is that the UNSEEN
// rows are no longer swallowed along with it.
func TestMasterCardCursorWalk_S2_BoundaryRowMovedToTail_NoSkip(t *testing.T) {
	t.Parallel()

	repo := newMasterCardWalkFixture()
	uc := newMasterCardWalkUsecase(repo)

	page1 := mcFetchWalkPage(t, uc, nil)
	next := mcEncodeWalkCursor(page1, page1.EndCur)

	// mc-b is repositioned so it now sorts last.
	mcSetPosition(t, repo, "mc-b", 99)

	seen := append([]string{}, mcWalkIDs(page1)...)
	cur := next
	for page := 2; page <= 5; page++ {
		out := mcFetchWalkPage(t, uc, &cur)
		ids := mcWalkIDs(out)
		if len(ids) == 0 {
			break
		}
		seen = append(seen, ids...)
		cur = mcEncodeWalkCursor(out, out.EndCur)
	}

	counts := map[string]int{}
	for _, id := range seen {
		counts[id]++
	}
	// mc-a was already served on page 1 and was not moved; mc-c / mc-d / mc-e
	// were unseen when the edit landed. All four must appear exactly once.
	for _, id := range []string{"mc-a", "mc-c", "mc-d", "mc-e"} {
		if counts[id] != 1 {
			t.Fatalf("row %q appeared %d times across the walk (want exactly 1); walk = %v", id, counts[id], seen)
		}
	}
	if counts["mc-foreign"] != 0 {
		t.Fatal("walk leaked a row from another deck")
	}
}

// TestMasterCardCursorWalk_S2_V1Cursor_StillSkips pins the defect the v2
// envelope exists to fix on the skip side: an id-only cursor re-reads the
// repositioned boundary row, finds it now sorts last, and reports an empty
// second page — the "rows silently vanish" symptom.
func TestMasterCardCursorWalk_S2_V1Cursor_StillSkips(t *testing.T) {
	t.Parallel()

	repo := newMasterCardWalkFixture()
	uc := newMasterCardWalkUsecase(repo)

	page1 := mcFetchWalkPage(t, uc, nil)
	legacy := cursor.Encode(page1.EndCur)

	mcSetPosition(t, repo, "mc-b", 99)

	page2 := mcFetchWalkPage(t, uc, &legacy)
	if got := mcWalkIDs(page2); len(got) != 0 {
		t.Fatalf("v1 cursor should return an empty page after the boundary row moves to the tail, got %v", got)
	}
}

// ---------------------------------------------------------------------------
// The UPDATED_AT ordering.
//
// POSITION is the schema default and the one an admin reorders by hand, but
// UPDATED_AT is mutable too and moves under the SAME operation: a batch import
// overwrites the row and touches its updated_at. The per-column encode/decode
// unit tests above prove the key serializes and hydrates; only a walk proves the
// bookmark survives an edit landing between two page fetches, so each scenario
// is paired with a v1 control that still exhibits the defect.
// ---------------------------------------------------------------------------

// TestMasterCardCursorWalk_UpdatedAt_S1_BoundaryRowMovedToHead_NoDuplicate is
// the S1 scenario on the UPDATED_AT ordering: the boundary row of page 1 is
// touched so its updated_at moves to the head of the listing before page 2 is
// fetched. With the captured ordering key the second page starts exactly where
// the first ended, so it repeats no row the caller has already seen.
func TestMasterCardCursorWalk_UpdatedAt_S1_BoundaryRowMovedToHead_NoDuplicate(t *testing.T) {
	t.Parallel()

	repo := newMasterCardWalkFixture()
	uc := newMasterCardWalkUsecase(repo)
	updatedAt := mcOrderByPtr(MasterCardOrderByUpdatedAt)

	page1 := mcFetchWalkPageOrdered(t, uc, updatedAt, nil)
	if got := mcWalkIDs(page1); len(got) != 2 || got[0] != "mc-a" || got[1] != "mc-b" {
		t.Fatalf("page 1 = %v, want [mc-a mc-b]", got)
	}
	if page1.Ordering.OrderBy != string(repository.MasterCardOrderByUpdatedAt) {
		t.Fatalf("Ordering.OrderBy = %q, want updated_at: the walk must exercise the requested column", page1.Ordering.OrderBy)
	}
	next := mcEncodeWalkCursor(page1, page1.EndCur)

	// A batch import overwrites mc-b, touching its updated_at to before mc-a's.
	mcSetUpdatedAt(t, repo, "mc-b", mcWalkAt(0))

	page2 := mcFetchWalkPageOrdered(t, uc, updatedAt, &next)
	got := mcWalkIDs(page2)
	if len(got) != 2 || got[0] != "mc-c" || got[1] != "mc-d" {
		t.Fatalf("page 2 = %v, want [mc-c mc-d]", got)
	}
	for _, id := range got {
		if id == "mc-a" || id == "mc-b" {
			t.Fatalf("page 2 repeated %q from page 1: %v", id, got)
		}
	}
}

// TestMasterCardCursorWalk_UpdatedAt_S1_V1Cursor_StillDuplicates is the v1
// control for the walk above: an id-only cursor re-reads the touched row and
// therefore hands back a row page 1 already returned.
func TestMasterCardCursorWalk_UpdatedAt_S1_V1Cursor_StillDuplicates(t *testing.T) {
	t.Parallel()

	repo := newMasterCardWalkFixture()
	uc := newMasterCardWalkUsecase(repo)
	updatedAt := mcOrderByPtr(MasterCardOrderByUpdatedAt)

	page1 := mcFetchWalkPageOrdered(t, uc, updatedAt, nil)
	legacy := cursor.Encode(page1.EndCur)

	mcSetUpdatedAt(t, repo, "mc-b", mcWalkAt(0))

	page2 := mcFetchWalkPageOrdered(t, uc, updatedAt, &legacy)
	got := mcWalkIDs(page2)
	if len(got) == 0 || got[0] != "mc-a" {
		t.Fatalf("v1 cursor should re-serve mc-a after the boundary row moves to the head; page 2 = %v", got)
	}
}

// TestMasterCardCursorWalk_UpdatedAt_S2_BoundaryRowMovedToTail_NoSkip is the S2
// scenario on the UPDATED_AT ordering: the boundary row of page 1 is touched so
// its updated_at drops below every remaining row. With the captured ordering key
// the walk continues from where page 1 ended, so every row the caller had not
// yet seen is still returned exactly once.
//
// The touched row itself is the documented exception: its ordering key moved
// into the not-yet-visited region, so the walk legitimately meets it again. What
// v2 fixes is that the UNSEEN rows are no longer swallowed along with it.
func TestMasterCardCursorWalk_UpdatedAt_S2_BoundaryRowMovedToTail_NoSkip(t *testing.T) {
	t.Parallel()

	repo := newMasterCardWalkFixture()
	uc := newMasterCardWalkUsecase(repo)
	updatedAt := mcOrderByPtr(MasterCardOrderByUpdatedAt)

	page1 := mcFetchWalkPageOrdered(t, uc, updatedAt, nil)
	next := mcEncodeWalkCursor(page1, page1.EndCur)

	// mc-b is touched so it now sorts last.
	mcSetUpdatedAt(t, repo, "mc-b", mcWalkAt(99))

	seen := append([]string{}, mcWalkIDs(page1)...)
	cur := next
	for page := 2; page <= 5; page++ {
		out := mcFetchWalkPageOrdered(t, uc, updatedAt, &cur)
		ids := mcWalkIDs(out)
		if len(ids) == 0 {
			break
		}
		seen = append(seen, ids...)
		cur = mcEncodeWalkCursor(out, out.EndCur)
	}

	counts := map[string]int{}
	for _, id := range seen {
		counts[id]++
	}
	// mc-a was already served on page 1 and was not touched; mc-c / mc-d / mc-e
	// were unseen when the edit landed. All four must appear exactly once.
	for _, id := range []string{"mc-a", "mc-c", "mc-d", "mc-e"} {
		if counts[id] != 1 {
			t.Fatalf("row %q appeared %d times across the walk (want exactly 1); walk = %v", id, counts[id], seen)
		}
	}
	if counts["mc-foreign"] != 0 {
		t.Fatal("walk leaked a row from another deck")
	}
}

// TestMasterCardCursorWalk_UpdatedAt_S2_V1Cursor_StillSkips is the v1 control
// for the walk above: an id-only cursor re-reads the touched boundary row, finds
// it now sorts last, and reports an empty second page — the "rows silently
// vanish" symptom.
func TestMasterCardCursorWalk_UpdatedAt_S2_V1Cursor_StillSkips(t *testing.T) {
	t.Parallel()

	repo := newMasterCardWalkFixture()
	uc := newMasterCardWalkUsecase(repo)
	updatedAt := mcOrderByPtr(MasterCardOrderByUpdatedAt)

	page1 := mcFetchWalkPageOrdered(t, uc, updatedAt, nil)
	legacy := cursor.Encode(page1.EndCur)

	mcSetUpdatedAt(t, repo, "mc-b", mcWalkAt(99))

	page2 := mcFetchWalkPageOrdered(t, uc, updatedAt, &legacy)
	if got := mcWalkIDs(page2); len(got) != 0 {
		t.Fatalf("v1 cursor should return an empty page after the boundary row moves to the tail, got %v", got)
	}
}

// TestMasterCardCursorWalk_LegacyBareIDStillPages verifies the oldest cursor
// form — a bare UUID with no envelope at all — still decodes and pages.
func TestMasterCardCursorWalk_LegacyBareIDStillPages(t *testing.T) {
	t.Parallel()

	repo := newMasterCardWalkFixture()
	uc := newMasterCardWalkUsecase(repo)

	page1 := mcFetchWalkPage(t, uc, nil)
	bare := page1.EndCur

	page2 := mcFetchWalkPage(t, uc, &bare)
	if got := mcWalkIDs(page2); len(got) != 2 || got[0] != "mc-c" || got[1] != "mc-d" {
		t.Fatalf("legacy bare-id cursor page 2 = %v, want [mc-c mc-d]", got)
	}
}

// TestMasterCardCursorWalk_OrderKeysCoverEveryReturnedRow verifies the usecase
// output carries the ordering metadata the resolver needs to emit a v2 cursor
// for every edge, not just the page boundaries.
func TestMasterCardCursorWalk_OrderKeysCoverEveryReturnedRow(t *testing.T) {
	t.Parallel()

	out := mcFetchWalkPage(t, newMasterCardWalkUsecase(newMasterCardWalkFixture()), nil)

	if out.Ordering != (PageOrdering{
		OrderBy:   string(repository.MasterCardOrderByPosition),
		Direction: string(repository.SortAsc),
	}) {
		t.Fatalf("Ordering = %+v, want the schema default (position, ASC)", out.Ordering)
	}
	for _, c := range out.Cards {
		got, ok := out.OrderKeys[c.ID]
		if !ok {
			t.Fatalf("OrderKeys is missing row %q", c.ID)
		}
		if want := strconv.Itoa(c.Position); got != want {
			t.Fatalf("OrderKeys[%q] = %q, want %q", c.ID, got, want)
		}
	}
}

// TestMasterCardCursorWalk_PublicListing_V2CursorSurvivesReposition runs the
// same S1 scenario through ListPublicMasterCards. The public listing is the one
// /catalog/[id] infinite-scrolls, and it shares listMasterCardsCore with the
// admin path — this pins that the published-only gate does not bypass the v2
// wiring.
func TestMasterCardCursorWalk_PublicListing_V2CursorSurvivesReposition(t *testing.T) {
	t.Parallel()

	repo := newMasterCardWalkFixture()
	uc := newMasterCardWalkUsecase(repo)

	first := 2
	page1, err := uc.ListPublicMasterCards(authedCtx("u1"), MasterCardConnectionInput{
		MasterCardgroupID: mcWalkDeckID,
		First:             &first,
	})
	if err != nil {
		t.Fatalf("unexpected error paging the public listing: %v", err)
	}
	if got := mcWalkIDs(page1); len(got) != 2 || got[0] != "mc-a" || got[1] != "mc-b" {
		t.Fatalf("public page 1 = %v, want [mc-a mc-b]", got)
	}
	next := mcEncodeWalkCursor(page1, page1.EndCur)

	mcSetPosition(t, repo, "mc-b", 0)

	page2, err := uc.ListPublicMasterCards(authedCtx("u1"), MasterCardConnectionInput{
		MasterCardgroupID: mcWalkDeckID,
		First:             &first,
		After:             &next,
	})
	if err != nil {
		t.Fatalf("unexpected error paging the public listing: %v", err)
	}
	if got := mcWalkIDs(page2); len(got) != 2 || got[0] != "mc-c" || got[1] != "mc-d" {
		t.Fatalf("public page 2 = %v, want [mc-c mc-d]", got)
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

// TestMasterCardCursorWalk_BackwardV2_BoundaryRowMovedToTail_NoDuplicate is the
// backward mirror of S1. Paging backwards from the tail, the leading boundary row
// of the first backward page is repositioned so its ordering key drops to the
// bottom of the listing. With the captured key the previous page still starts
// exactly where the first ended and repeats nothing the caller has already seen.
func TestMasterCardCursorWalk_BackwardV2_BoundaryRowMovedToTail_NoDuplicate(t *testing.T) {
	t.Parallel()

	repo := newMasterCardWalkFixture()
	uc := newMasterCardWalkUsecase(repo)

	// Tail of the listing under (position ASC, id ASC): mc-d, mc-e.
	page1 := mcFetchWalkPageBackward(t, uc, nil, 2)
	if got := mcWalkIDs(page1); len(got) != 2 || got[0] != "mc-d" || got[1] != "mc-e" {
		t.Fatalf("backward page 1 = %v, want [mc-d mc-e]", got)
	}
	if !page1.HasPrev {
		t.Fatal("backward page 1 must report hasPreviousPage: mc-a/mc-b/mc-c are still ahead")
	}
	if page1.HasNext {
		t.Fatal("backward page 1 without a before cursor is the tail; hasNextPage must be false")
	}

	prev := mcEncodeWalkCursor(page1, page1.StartCur)

	// mc-d — the leading boundary row of the page just served — is repositioned
	// so it now sorts last. A v1 cursor would re-read it and walk from the bottom.
	mcSetPosition(t, repo, "mc-d", 99)

	page2 := mcFetchWalkPageBackward(t, uc, &prev, 2)
	got := mcWalkIDs(page2)
	if len(got) != 2 || got[0] != "mc-b" || got[1] != "mc-c" {
		t.Fatalf("backward page 2 = %v, want [mc-b mc-c]", got)
	}
	for _, id := range got {
		if id == "mc-d" || id == "mc-e" {
			t.Fatalf("backward page 2 repeated %q from page 1: %v", id, got)
		}
	}
	if !page2.HasNext {
		t.Fatal("a backward page taken before a cursor must report hasNextPage")
	}
	if !page2.HasPrev {
		t.Fatal("mc-a is still ahead of backward page 2; hasPreviousPage must be true")
	}
}

// TestMasterCardCursorWalk_BackwardV1Cursor_StillDuplicates pins the defect on
// the backward edge, the mirror of the forward S1/S2 v1 regressions: an id-only
// cursor re-reads the repositioned boundary row, finds it now sorts last, and
// hands back a row the caller already saw.
func TestMasterCardCursorWalk_BackwardV1Cursor_StillDuplicates(t *testing.T) {
	t.Parallel()

	repo := newMasterCardWalkFixture()
	uc := newMasterCardWalkUsecase(repo)

	page1 := mcFetchWalkPageBackward(t, uc, nil, 2)
	legacy := cursor.Encode(page1.StartCur)

	mcSetPosition(t, repo, "mc-d", 99)

	page2 := mcFetchWalkPageBackward(t, uc, &legacy, 2)
	got := mcWalkIDs(page2)
	if len(got) == 0 || got[len(got)-1] != "mc-e" {
		t.Fatalf("v1 cursor should re-serve mc-e after the boundary row moves to the tail; backward page 2 = %v", got)
	}
}

// TestMasterCardCursorWalk_TiedOrderKeys_ForwardAndBackward verifies the id
// tie-break survives the v2 round trip on both edges. mc-b and mc-c carry the
// same position, so a bookmark taken at mc-b must still separate it from mc-c —
// forward must serve mc-c next, and paging back before mc-c must land on mc-b,
// not skip past the whole tied pair or re-serve it.
func TestMasterCardCursorWalk_TiedOrderKeys_ForwardAndBackward(t *testing.T) {
	t.Parallel()

	uc := newMasterCardWalkUsecase(newTiedMasterCardWalkFixture())

	// Forward: page 1 ends on mc-b, the first of the tied pair under id ASC.
	page1 := mcFetchWalkPage(t, uc, nil)
	if got := mcWalkIDs(page1); len(got) != 2 || got[0] != "mc-a" || got[1] != "mc-b" {
		t.Fatalf("tied page 1 = %v, want [mc-a mc-b]", got)
	}
	next := mcEncodeWalkCursor(page1, page1.EndCur)
	page2 := mcFetchWalkPage(t, uc, &next)
	if got := mcWalkIDs(page2); len(got) != 2 || got[0] != "mc-c" || got[1] != "mc-d" {
		t.Fatalf("tied page 2 = %v, want [mc-c mc-d]: the tie-break must not swallow mc-c", got)
	}

	// Guard the premise: if the two rows stopped sharing an ordering key the walk
	// above would pass for the wrong reason — it would no longer be exercising
	// the tie at all.
	keyB, keyC := page1.OrderKeys["mc-b"], page2.OrderKeys["mc-c"]
	if keyB == "" || keyC == "" {
		t.Fatalf("OrderKeys must cover both tied rows; got mc-b=%q mc-c=%q", keyB, keyC)
	}
	if keyB != keyC {
		t.Fatalf("fixture no longer ties the rows: mc-b=%q, mc-c=%q", keyB, keyC)
	}

	// Backward: paging before mc-c must return the rows immediately ahead of it,
	// which means stopping at its tied sibling mc-b rather than jumping the pair.
	beforeC := cursor.EncodeV2(cursor.Payload{
		ID:        "mc-c",
		OrderBy:   page2.Ordering.OrderBy,
		Direction: page2.Ordering.Direction,
		OrderKey:  page2.OrderKeys["mc-c"],
	})
	back := mcFetchWalkPageBackward(t, uc, &beforeC, 2)
	if got := mcWalkIDs(back); len(got) != 2 || got[0] != "mc-a" || got[1] != "mc-b" {
		t.Fatalf("backward page before mc-c = %v, want [mc-a mc-b]", got)
	}
	if back.HasPrev {
		t.Fatal("mc-a is the head of the listing; hasPreviousPage must be false")
	}
	if !back.HasNext {
		t.Fatal("a backward page taken before a cursor must report hasNextPage")
	}
}

// ---------------------------------------------------------------------------
// v2 cursor rejection paths
// ---------------------------------------------------------------------------

// TestResolveMasterCardCursor_V2OrderingMismatch_ReturnsBadUserInput verifies a
// cursor taken under a different column or direction is rejected with the
// existing BAD_USER_INPUT shape rather than silently mis-paging against a value
// that belongs to another column.
func TestResolveMasterCardCursor_V2OrderingMismatch_ReturnsBadUserInput(t *testing.T) {
	t.Parallel()

	cases := map[string]cursor.Payload{
		"different column": {
			ID: "mc-a", OrderBy: string(repository.MasterCardOrderByCreatedAt),
			Direction: string(repository.SortAsc), OrderKey: mcWalkBase.Format(time.RFC3339Nano),
		},
		"different direction": {
			ID: "mc-a", OrderBy: string(repository.MasterCardOrderByPosition),
			Direction: string(repository.SortDesc), OrderKey: "1",
		},
	}
	for name, p := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			uc := newMasterCardWalkUsecase(newMasterCardWalkFixture())
			cur := cursor.EncodeV2(p)
			first := 2
			_, err := uc.ListMasterCards(authedCtx("admin1"), MasterCardConnectionInput{
				MasterCardgroupID: mcWalkDeckID,
				First:             &first,
				After:             &cur,
			})
			assertValidationError(t, err, "after", "cursor does not match the requested ordering")
		})
	}
}

// TestResolveMasterCardCursor_V2MalformedOrderKey_ReturnsBadUserInput verifies a
// v2 cursor whose position value does not parse as an integer is a client error,
// not an INTERNAL fault.
func TestResolveMasterCardCursor_V2MalformedOrderKey_ReturnsBadUserInput(t *testing.T) {
	t.Parallel()

	uc := newMasterCardWalkUsecase(newMasterCardWalkFixture())
	cur := cursor.EncodeV2(cursor.Payload{
		ID:        "mc-a",
		OrderBy:   string(repository.MasterCardOrderByPosition),
		Direction: string(repository.SortAsc),
		OrderKey:  "not-an-integer",
	})
	first := 2
	_, err := uc.ListMasterCards(authedCtx("admin1"), MasterCardConnectionInput{
		MasterCardgroupID: mcWalkDeckID,
		First:             &first,
		After:             &cur,
	})
	assertValidationError(t, err, "after", "invalid cursor")
}

// TestResolveMasterCardCursor_V2ForeignDeck_ReturnsCursorNotFound verifies the
// cross-deck guard is not bypassed by a v2 cursor. A v2 cursor can hydrate its
// ordering column without the repository, but the deck lookup must still run or
// the endpoint becomes an existence oracle over other decks' card ids.
func TestResolveMasterCardCursor_V2ForeignDeck_ReturnsCursorNotFound(t *testing.T) {
	t.Parallel()

	uc := newMasterCardWalkUsecase(newMasterCardWalkFixture())
	cur := cursor.EncodeV2(cursor.Payload{
		ID:        "mc-foreign",
		OrderBy:   string(repository.MasterCardOrderByPosition),
		Direction: string(repository.SortAsc),
		OrderKey:  "3",
	})
	first := 2
	_, err := uc.ListMasterCards(authedCtx("admin1"), MasterCardConnectionInput{
		MasterCardgroupID: mcWalkDeckID,
		First:             &first,
		After:             &cur,
	})
	assertValidationError(t, err, "after", "cursor not found")
}

// TestResolveMasterCardCursor_V2UnknownID_ReturnsCursorNotFound verifies a v2
// cursor for a row that no longer exists is rejected exactly like a v1 one — the
// hydration lookup still runs even though the ordering key is embedded.
func TestResolveMasterCardCursor_V2UnknownID_ReturnsCursorNotFound(t *testing.T) {
	t.Parallel()

	uc := newMasterCardWalkUsecase(newMasterCardWalkFixture())
	cur := cursor.EncodeV2(cursor.Payload{
		ID:        "mc-deleted",
		OrderBy:   string(repository.MasterCardOrderByPosition),
		Direction: string(repository.SortAsc),
		OrderKey:  "2",
	})
	first := 2
	_, err := uc.ListMasterCards(authedCtx("admin1"), MasterCardConnectionInput{
		MasterCardgroupID: mcWalkDeckID,
		First:             &first,
		After:             &cur,
	})
	assertValidationError(t, err, "after", "cursor not found")
}

// TestApplyMasterCardOrderKey_PerColumn covers the decode half of the ordering
// key for every column in the allowlist, including the ID column (whose key is
// empty because the id is already carried) and the impossible default arm, which
// must stay INTERNAL rather than degrade to BAD_USER_INPUT.
//
// The two time columns are also fed a malformed key: their arms must return
// errCursorKeyMalformed (so resolveMasterCardCursor maps them to BAD_USER_INPUT)
// AND leave the column unhydrated. The unhydrated half is what pins the failure
// mode a discarded decode error would open — a zero time.Time written into
// CreatedAt/UpdatedAt passes the repository's nil-gate and becomes a bound that
// matches every row, so the caller is handed page 1 forever instead of an error.
func TestApplyMasterCardOrderKey_PerColumn(t *testing.T) {
	t.Parallel()

	when := mcWalkBase.Format(time.RFC3339Nano)

	c := &repository.MasterCardCursor{ID: "mc-a"}
	if err := applyMasterCardOrderKey(c, repository.MasterCardOrderByID, ""); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.Position != nil || c.CreatedAt != nil || c.UpdatedAt != nil {
		t.Fatalf("orderBy=id must hydrate no extra column, got %+v", c)
	}

	c = &repository.MasterCardCursor{ID: "mc-a"}
	if err := applyMasterCardOrderKey(c, repository.MasterCardOrderByPosition, "7"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.Position == nil || *c.Position != 7 {
		t.Fatalf("orderBy=position must hydrate Position, got %+v", c)
	}

	c = &repository.MasterCardCursor{ID: "mc-a"}
	if err := applyMasterCardOrderKey(c, repository.MasterCardOrderByCreatedAt, when); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.CreatedAt == nil || !c.CreatedAt.Equal(mcWalkBase) {
		t.Fatalf("orderBy=created_at must hydrate CreatedAt, got %+v", c)
	}

	c = &repository.MasterCardCursor{ID: "mc-a"}
	if err := applyMasterCardOrderKey(c, repository.MasterCardOrderByUpdatedAt, when); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.UpdatedAt == nil || !c.UpdatedAt.Equal(mcWalkBase) {
		t.Fatalf("orderBy=updated_at must hydrate UpdatedAt, got %+v", c)
	}

	c = &repository.MasterCardCursor{ID: "mc-a"}
	if err := applyMasterCardOrderKey(c, repository.MasterCardOrderByCreatedAt, "not-a-timestamp"); !errors.Is(err, errCursorKeyMalformed) {
		t.Fatalf("orderBy=created_at must reject a malformed key with errCursorKeyMalformed, got %v", err)
	}
	if c.CreatedAt != nil {
		t.Fatalf("a rejected created_at key must leave CreatedAt unhydrated, got %+v", c)
	}

	c = &repository.MasterCardCursor{ID: "mc-a"}
	if err := applyMasterCardOrderKey(c, repository.MasterCardOrderByUpdatedAt, "not-a-timestamp"); !errors.Is(err, errCursorKeyMalformed) {
		t.Fatalf("orderBy=updated_at must reject a malformed key with errCursorKeyMalformed, got %v", err)
	}
	if c.UpdatedAt != nil {
		t.Fatalf("a rejected updated_at key must leave UpdatedAt unhydrated, got %+v", c)
	}

	err := applyMasterCardOrderKey(&repository.MasterCardCursor{}, repository.MasterCardOrderBy("not_a_real_column"), "")
	assertInternalChain(t, err, "usecase: master card: unhandled orderBy")
}

// TestMasterCardOrderKeys_PerColumn covers the encode half: every column in the
// allowlist serializes to the value the decode half above consumes, and the ID
// column yields an empty key because the id is already carried by the cursor.
func TestMasterCardOrderKeys_PerColumn(t *testing.T) {
	t.Parallel()

	card := &domain.MasterCard{
		ID:                "mc-a",
		MasterCardgroupID: mcWalkDeckID,
		Position:          7,
		CreatedAt:         mcWalkBase,
		UpdatedAt:         mcWalkBase.Add(time.Hour),
	}
	cards := []*domain.MasterCard{card}

	cases := map[repository.MasterCardOrderBy]string{
		repository.MasterCardOrderByID:        "",
		repository.MasterCardOrderByPosition:  "7",
		repository.MasterCardOrderByCreatedAt: mcWalkBase.Format(time.RFC3339Nano),
		repository.MasterCardOrderByUpdatedAt: mcWalkBase.Add(time.Hour).Format(time.RFC3339Nano),
	}
	for orderBy, want := range cases {
		keys, err := masterCardOrderKeys(orderBy, cards)
		if err != nil {
			t.Fatalf("orderBy=%q: unexpected error: %v", orderBy, err)
		}
		if got := keys["mc-a"]; got != want {
			t.Fatalf("orderBy=%q: OrderKeys[mc-a] = %q, want %q", orderBy, got, want)
		}
	}
}

// TestMasterCardOrderKeys_UnknownOrderBy covers the encode half's impossible
// default arm: an unmapped column is a caller bug, surfaced as INTERNAL rather
// than an empty key that would silently produce an unanchored cursor.
func TestMasterCardOrderKeys_UnknownOrderBy(t *testing.T) {
	t.Parallel()

	_, err := masterCardOrderKeys(repository.MasterCardOrderBy("not_a_real_column"), []*domain.MasterCard{
		{ID: "mc-a", MasterCardgroupID: mcWalkDeckID},
	})
	assertInternalChain(t, err, "usecase: master card: unhandled orderBy")
}

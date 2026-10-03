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
// The master-card listing orders by position ASC — a column an admin batch
// import rewrites for every conflicting row. A cursor that carries only a row id has to re-read that row at serve time to recover its ordering value,
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
// "lowered" throughout: under the ASC ordering a SMALLER position sorts EARLIER,
// so promoting a row to the head means decreasing its position number.
// ---------------------------------------------------------------------------

// mcWalkDeckID is the master deck every walk pages through.
const mcWalkDeckID = "deck-1"

// mcWalkForeignDeckID owns the one fixture row that must never surface: it backs
// the cross-deck cursor guard.
const mcWalkForeignDeckID = "deck-2"

// masterCardWalkRepo is an in-memory master-card repository supporting exactly
// the slice of the interface these walks need: forward paging over one deck's
// rows ordered by (position ASC, id ASC). FindByID returns the CURRENT row,
// which is what makes the v1 re-hydration path observe a mutation made between
// two page fetches.
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

// mcWalkCursorKey extracts the position boundary the ordering compares against.
// A cursor that reaches the repository without its position populated is a
// usecase bug, so it is surfaced rather than defaulted to a zero key.
func mcWalkCursorKey(c *repository.MasterCardCursor) (int, error) {
	if c.Position == nil {
		return 0, eris.New("masterCardWalkRepo: cursor is missing the position column")
	}
	return *c.Position, nil
}

// mcAfterInAscTuple reports whether a row sorts strictly after the cursor under
// the (orderKey ASC, id ASC) total order the repository emits.
func mcAfterInAscTuple(rowKey int, rowID string, curKey int, curID string) bool {
	if rowKey == curKey {
		return rowID > curID
	}
	return rowKey > curKey
}

func (r *masterCardWalkRepo) FindPageByMasterCardgroup(
	_ context.Context,
	masterCardgroupID string,
	after *repository.MasterCardCursor,
	first int,
	_ *string,
) ([]*domain.MasterCard, int64, error) {
	scoped := make([]*domain.MasterCard, 0, len(r.rows))
	for _, c := range r.rows {
		if c.BelongsToMasterCardgroup(masterCardgroupID) {
			scoped = append(scoped, c)
		}
	}
	sort.SliceStable(scoped, func(i, j int) bool {
		if scoped[i].Position != scoped[j].Position {
			return scoped[i].Position < scoped[j].Position
		}
		return scoped[i].ID < scoped[j].ID
	})
	total := int64(len(scoped))

	if after != nil {
		key, err := mcWalkCursorKey(after)
		if err != nil {
			return nil, 0, err
		}
		rest := make([]*domain.MasterCard, 0, len(scoped))
		for _, c := range scoped {
			if mcAfterInAscTuple(c.Position, c.ID, key, after.ID) {
				rest = append(rest, c)
			}
		}
		scoped = rest
	}
	if first > 0 && len(scoped) > first {
		scoped = scoped[:first]
	}
	return scoped, total, nil
}

// mcWalkBase is a fixed instant so the fixture timestamps are deterministic.
var mcWalkBase = time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC)

// newMasterCardWalkFixture builds five rows in mcWalkDeckID whose position ASC
// order is mc-a, mc-b, mc-c, mc-d, mc-e, plus one row in a different deck so the
// deck filter and the cross-deck cursor guard have something to reject.
func newMasterCardWalkFixture() *masterCardWalkRepo {
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
func mcFetchWalkPage(t *testing.T, uc MasterCardUsecase, after *string) *MasterCardConnectionOutput {
	t.Helper()
	first := 2
	out, err := uc.ListMasterCards(authedCtx("admin1"), MasterCardConnectionInput{
		MasterCardgroupID: mcWalkDeckID,
		First:             &first,
		After:             after,
	})
	if err != nil {
		t.Fatalf("unexpected error paging: %v", err)
	}
	return out
}

// mcSetPosition moves a row's position ordering key, simulating the
// repositioning a batch import performs on every conflicting row between two
// page fetches.
func mcSetPosition(t *testing.T, repo *masterCardWalkRepo, id string, pos int) {
	t.Helper()
	mcMutateWalkRow(t, repo, id, func(c *domain.MasterCard) { c.Position = pos })
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

	if out.Ordering != (PageOrdering{OrderBy: "position", Direction: "ASC"}) {
		t.Fatalf("Ordering = %+v, want the fixed (position, ASC)", out.Ordering)
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
// Tied ordering keys.
// ---------------------------------------------------------------------------

// TestMasterCardCursorWalk_TiedOrderKeys_Forward verifies the id tie-break
// survives the v2 round trip. mc-b and mc-c carry the same position, so a
// bookmark taken at mc-b must still separate it from mc-c — the next page must
// serve mc-c, not skip past the whole tied pair or re-serve it.
func TestMasterCardCursorWalk_TiedOrderKeys_Forward(t *testing.T) {
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
			ID: "mc-a", OrderBy: "created_at",
			Direction: "ASC", OrderKey: mcWalkBase.Format(time.RFC3339Nano),
		},
		"different direction": {
			ID: "mc-a", OrderBy: "position",
			Direction: "DESC", OrderKey: "1",
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
		OrderBy:   "position",
		Direction: "ASC",
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
		OrderBy:   "position",
		Direction: "ASC",
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
		OrderBy:   "position",
		Direction: "ASC",
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

// TestApplyMasterCardOrderKey_Position covers the decode half of the ordering
// key. A malformed key must return errCursorKeyMalformed (so
// resolveMasterCardCursor maps it to BAD_USER_INPUT) AND leave Position
// unhydrated: a zero position written into the cursor would pass the
// repository's nil-gate and become a bound that silently mis-pages.
func TestApplyMasterCardOrderKey_Position(t *testing.T) {
	t.Parallel()

	c := &repository.MasterCardCursor{ID: "mc-a"}
	if err := applyMasterCardOrderKey(c, "7"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.Position == nil || *c.Position != 7 {
		t.Fatalf("applyMasterCardOrderKey must hydrate Position, got %+v", c)
	}

	c = &repository.MasterCardCursor{ID: "mc-a"}
	if err := applyMasterCardOrderKey(c, "not-an-integer"); !errors.Is(err, errCursorKeyMalformed) {
		t.Fatalf("a malformed key must be rejected with errCursorKeyMalformed, got %v", err)
	}
	if c.Position != nil {
		t.Fatalf("a rejected key must leave Position unhydrated, got %+v", c)
	}
}

// TestMasterCardOrderKeys_Position covers the encode half: the position
// serializes to the value the decode half above consumes.
func TestMasterCardOrderKeys_Position(t *testing.T) {
	t.Parallel()

	card := &domain.MasterCard{
		ID:                "mc-a",
		MasterCardgroupID: mcWalkDeckID,
		Position:          7,
		CreatedAt:         mcWalkBase,
		UpdatedAt:         mcWalkBase.Add(time.Hour),
	}
	keys := masterCardOrderKeys([]*domain.MasterCard{card})
	if got := keys["mc-a"]; got != "7" {
		t.Fatalf("OrderKeys[mc-a] = %q, want %q", got, "7")
	}
}

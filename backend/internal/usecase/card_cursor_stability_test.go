package usecase

import (
	"context"
	"sort"
	"testing"
	"time"

	"github.com/rotisserie/eris"
	"gorm.io/gorm"

	"backend/internal/cursor"
	"backend/internal/domain"
	"backend/internal/repository"
)

// ---------------------------------------------------------------------------
// Cursor stability across edits of the ordering key.
//
// The card listing defaults to the immutable ID ordering, so its default path
// was never exposed. But CardOrderBy also offers DUE and UPDATED_AT, and both
// move under ordinary use — every FSRS review rewrites a card's due date. A
// cursor that carries only a row id has to re-read that row at serve time to
// recover its ordering value, so a change between two page fetches moves the
// bookmark: rows already returned come back a second time (S1) or rows the
// caller has not seen yet are skipped (S2). A v2 cursor carries the ordering
// value captured when the page was served, so the bookmark stays put.
//
// DUE is the subtle one: it is not a column on cards at all. The page query
// orders by COALESCE(user_card_fsrs.due, cards.created_at) for the requesting
// user, so the value captured into a cursor and the value the ordering means
// have to come from the same fallback. cardWalkRepo models exactly that
// expression, so a divergence between emit and compare shows up as a wrong page
// rather than passing silently.
//
// These walks drive ListCardsByCardgroupConnection end-to-end against an
// in-memory repository implementing the same (orderKey, id) tuple comparison
// the SQL emits, so the scenarios are reproduced without a database.
// ---------------------------------------------------------------------------

// cardWalkFSRSRepo is an in-memory UserCardFSRSRepositoryForCard. It counts its
// calls and records the id batch of each one so the tests can pin the batching
// contract: the page-emit path must resolve every row's due value in a SINGLE
// lookup, never one per row.
type cardWalkFSRSRepo struct {
	states  map[string]*domain.UserCardFSRS
	calls   int
	batches [][]string
}

func (r *cardWalkFSRSRepo) FindByUserAndCardIDs(
	_ context.Context, _ string, ids []string,
) (map[string]*domain.UserCardFSRS, error) {
	r.calls++
	r.batches = append(r.batches, append([]string(nil), ids...))
	out := make(map[string]*domain.UserCardFSRS, len(ids))
	for _, id := range ids {
		if st := r.states[id]; st != nil {
			out[id] = st
		}
	}
	return out, nil
}

// cardWalkRepo is an in-memory CardRepository supporting exactly the slice of
// the interface these walks need: forward AND backward paging over one
// cardgroup's cards ordered by (orderKey ASC, id ASC), where orderKey is
// updated_at or the DUE COALESCE. Any other ordering is rejected loudly so a
// future test cannot silently exercise an unimplemented branch. FindByID
// returns the CURRENT row, which is what makes the v1 re-hydration path observe
// a mutation made between two page fetches.
//
// The backward branch models the repository's direction-flip + reverse: it
// keeps the rows that sort strictly BEFORE the cursor and returns the ones
// closest to it, still in ASC order, so the usecase's leading-edge trim
// (TrimAndDetectBackward) sees the same slice shape SQL would hand it.
type cardWalkRepo struct {
	cards []*domain.Card
	// fsrs supplies the due values the DUE ordering coalesces over. A nil fsrs
	// means "no state rows exist", i.e. every card orders by its created_at.
	fsrs *cardWalkFSRSRepo
}

func (r *cardWalkRepo) FindByID(_ context.Context, id string) (*domain.Card, error) {
	for _, c := range r.cards {
		if c.ID == id {
			return c, nil
		}
	}
	return nil, repository.ErrNotFound
}

// cardWalkKey is the fake's model of the SQL sort expression: cards.updated_at
// for UPDATED_AT, and COALESCE(ucs.due, cards.created_at) for DUE.
func (r *cardWalkRepo) cardWalkKey(orderBy repository.CardOrderBy, c *domain.Card) time.Time {
	if orderBy == repository.CardOrderByDue {
		if r.fsrs != nil {
			if st := r.fsrs.states[c.ID]; st != nil {
				return st.State.Due
			}
		}
		return c.CreatedAt
	}
	return c.UpdatedAt
}

// cardAfterInAscTuple reports whether row sorts strictly after cur under the
// (orderKey ASC, id ASC) total order the repository emits.
func cardAfterInAscTuple(rowKey time.Time, rowID string, curKey time.Time, curID string) bool {
	if rowKey.Equal(curKey) {
		return rowID > curID
	}
	return rowKey.After(curKey)
}

// cardBeforeInAscTuple is the mirror predicate for backward paging. It is not
// !cardAfterInAscTuple — the cursor row itself sorts neither after nor before
// itself, and both edges must exclude it.
func cardBeforeInAscTuple(rowKey time.Time, rowID string, curKey time.Time, curID string) bool {
	if rowKey.Equal(curKey) {
		return rowID < curID
	}
	return rowKey.Before(curKey)
}

// cardWalkCursorKey extracts the boundary value the active ordering compares
// against. A cursor that reaches the repository without its column populated is
// a usecase bug, so it is surfaced rather than defaulted to the zero time.
func cardWalkCursorKey(orderBy repository.CardOrderBy, c *repository.CardCursor) (time.Time, error) {
	switch orderBy {
	case repository.CardOrderByUpdatedAt:
		if c.UpdatedAt == nil {
			return time.Time{}, eris.New("cardWalkRepo: cursor is missing the updated_at column")
		}
		return *c.UpdatedAt, nil
	case repository.CardOrderByDue:
		if c.Due == nil {
			return time.Time{}, eris.New("cardWalkRepo: cursor is missing the due column")
		}
		return *c.Due, nil
	default:
		return time.Time{}, eris.Errorf("cardWalkRepo: unhandled orderBy %q", orderBy)
	}
}

func (r *cardWalkRepo) FindPageByCardgroupForUser(
	_ context.Context,
	_, cardgroupID string,
	after, before *repository.CardCursor,
	first, last int,
	orderBy repository.CardOrderBy,
	dir repository.SortOrder,
	_ *string,
) ([]*domain.Card, int64, error) {
	if orderBy != repository.CardOrderByUpdatedAt && orderBy != repository.CardOrderByDue {
		return nil, 0, eris.Errorf(
			"cardWalkRepo: only the updated_at and due orderings are implemented; got orderBy=%q", orderBy,
		)
	}
	if dir != repository.SortAsc {
		return nil, 0, eris.Errorf("cardWalkRepo: only the ASC direction is implemented; got dir=%q", dir)
	}

	scoped := make([]*domain.Card, 0, len(r.cards))
	for _, c := range r.cards {
		if c.BelongsToCardgroup(domain.CardgroupID(cardgroupID)) {
			scoped = append(scoped, c)
		}
	}
	sort.SliceStable(scoped, func(i, j int) bool {
		ki, kj := r.cardWalkKey(orderBy, scoped[i]), r.cardWalkKey(orderBy, scoped[j])
		if !ki.Equal(kj) {
			return ki.Before(kj)
		}
		return scoped[i].ID < scoped[j].ID
	})
	total := int64(len(scoped))

	if after != nil {
		key, err := cardWalkCursorKey(orderBy, after)
		if err != nil {
			return nil, 0, err
		}
		rest := make([]*domain.Card, 0, len(scoped))
		for _, c := range scoped {
			if cardAfterInAscTuple(r.cardWalkKey(orderBy, c), c.ID, key, after.ID) {
				rest = append(rest, c)
			}
		}
		scoped = rest
	}
	if before != nil {
		key, err := cardWalkCursorKey(orderBy, before)
		if err != nil {
			return nil, 0, err
		}
		rest := make([]*domain.Card, 0, len(scoped))
		for _, c := range scoped {
			if cardBeforeInAscTuple(r.cardWalkKey(orderBy, c), c.ID, key, before.ID) {
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

func (r *cardWalkRepo) Create(_ context.Context, _ *domain.Card) error { return nil }

func (r *cardWalkRepo) FindByCardgroupAndFront(_ context.Context, _, _ string) (*domain.Card, error) {
	return nil, repository.ErrNotFound
}

func (r *cardWalkRepo) Update(_ context.Context, _ string, _ repository.CardUpdate) (*domain.Card, error) {
	return nil, nil
}

func (r *cardWalkRepo) Delete(_ context.Context, _ string) error { return nil }

func (r *cardWalkRepo) DeleteByIDsTx(_ context.Context, _ *gorm.DB, _ string, _ []string) (int64, error) {
	return 0, nil
}

// cdWalkBase is a fixed instant so the fixture timestamps are deterministic.
var cdWalkBase = time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC)

// cdWalkAt returns the fixture instant N minutes after the base.
func cdWalkAt(minutes int) time.Time {
	return cdWalkBase.Add(time.Duration(minutes) * time.Minute)
}

// newCardWalkFixture builds five cards in cg1 whose ascending (created_at,
// updated_at, due) order is c-a, c-b, c-c, c-d, c-e, plus one card in a
// different cardgroup so the cross-cardgroup cursor guard has something to
// reject. Every card in cg1 gets an FSRS state row whose due mirrors its
// created_at, so the UPDATED_AT and DUE walks start from the same row order.
//
// statelessID, when non-empty, names a cg1 card that gets NO state row. Its DUE
// key then falls through the COALESCE to created_at, which is the case the
// emit-side and compare-side fallbacks must agree on.
func newCardWalkFixture(statelessID string) *cardWalkRepo {
	mk := func(id string, minutes int, cardgroupID string) *domain.Card {
		return &domain.Card{
			ID:          id,
			CardgroupID: domain.CardgroupID(cardgroupID),
			Front:       domain.CardText("Q " + id),
			Back:        domain.CardText("A " + id),
			CreatedAt:   cdWalkAt(minutes),
			UpdatedAt:   cdWalkAt(minutes),
		}
	}
	cards := []*domain.Card{
		mk("c-a", 1, "cg1"),
		mk("c-b", 2, "cg1"),
		mk("c-c", 3, "cg1"),
		mk("c-d", 4, "cg1"),
		mk("c-e", 5, "cg1"),
		mk("c-foreign", 3, "cg2"),
	}
	fsrs := &cardWalkFSRSRepo{states: map[string]*domain.UserCardFSRS{}}
	for _, c := range cards {
		if c.ID == statelessID || !c.BelongsToCardgroup(domain.CardgroupID("cg1")) {
			continue
		}
		fsrs.states[c.ID] = &domain.UserCardFSRS{
			UserID: domain.UserID("u1"),
			CardID: c.ID,
			State:  domain.FSRSState{Due: c.CreatedAt},
		}
	}
	return &cardWalkRepo{cards: cards, fsrs: fsrs}
}

// newCardWalkUsecase wires the walk repository behind a cardgroup repo that
// reports cg1 as owned by u1, so authorizeCardgroupOrBadInput lets the listing
// through. Passing fsrs=nil exercises the unconfigured-FSRS-repository fallback.
func newCardWalkUsecase(repo *cardWalkRepo, fsrs UserCardFSRSRepositoryForCard) CardUsecase {
	return NewCardUsecase(nil, repo,
		&mockCardgroupRepoForCard{findResult: &domain.Cardgroup{ID: domain.CardgroupID("cg1"), OwnerID: "u1"}},
		fsrs, nil, newTestLogger(),
	)
}

// cardWalkIDs lists the card ids of a page in order.
func cardWalkIDs(out *CardConnectionOutput) []string {
	ids := make([]string, 0, len(out.Cards))
	for _, c := range out.Cards {
		ids = append(ids, c.ID)
	}
	return ids
}

// encodeCardWalkCursor rebuilds the opaque cursor the resolver emits for a
// boundary row of the given page: the v2 envelope carrying the ordering the
// page was served under plus that row's ordering-key value as captured at serve
// time. It mirrors orderedCursorEncoder in graph/resolver/connection.go — the
// usecase itself never encodes cursors.
func encodeCardWalkCursor(out *CardConnectionOutput, id string) string {
	return cursor.EncodeV2(cursor.Payload{
		ID:        id,
		OrderBy:   out.Ordering.OrderBy,
		Direction: out.Ordering.Direction,
		OrderKey:  out.OrderKeys[id],
	})
}

// fetchCardWalkPage runs one forward page of size two under the given ordering,
// optionally after a cursor.
func fetchCardWalkPage(t *testing.T, uc CardUsecase, orderBy CardOrderBy, after *string) *CardConnectionOutput {
	t.Helper()
	out, err := uc.ListCardsByCardgroupConnection(authedCtx("u1"), CardConnectionInput{
		CardgroupID: "cg1",
		First:       intPtr(2),
		After:       after,
		OrderBy:     orderByPtr(orderBy),
	})
	if err != nil {
		t.Fatalf("unexpected error paging: %v", err)
	}
	return out
}

// fetchCardWalkPageBackward runs one backward page of the given size, optionally
// before a cursor. Passing a nil cursor asks for the tail of the listing.
func fetchCardWalkPageBackward(
	t *testing.T, uc CardUsecase, orderBy CardOrderBy, before *string, last int,
) *CardConnectionOutput {
	t.Helper()
	out, err := uc.ListCardsByCardgroupConnection(authedCtx("u1"), CardConnectionInput{
		CardgroupID: "cg1",
		Last:        intPtr(last),
		Before:      before,
		OrderBy:     orderByPtr(orderBy),
	})
	if err != nil {
		t.Fatalf("unexpected error paging backward: %v", err)
	}
	return out
}

// setCardWalkUpdatedAt moves a row's UPDATED_AT ordering key, simulating an edit
// made from another device between two page fetches.
func setCardWalkUpdatedAt(t *testing.T, repo *cardWalkRepo, id string, at time.Time) {
	t.Helper()
	for _, c := range repo.cards {
		if c.ID == id {
			c.UpdatedAt = at
			return
		}
	}
	t.Fatalf("fixture has no card %q", id)
}

// reviewCardWalkDue moves a row's DUE ordering key the way a real review does —
// by rewriting the caller's user_card_fsrs state, not by touching the card. A
// card with no state row acquires one, which is how the COALESCE fallback stops
// applying to it mid-walk.
func reviewCardWalkDue(t *testing.T, repo *cardWalkRepo, id string, due time.Time) {
	t.Helper()
	if repo.fsrs == nil {
		t.Fatal("fixture has no FSRS repository to review through")
	}
	if st := repo.fsrs.states[id]; st != nil {
		st.State.Due = due
		return
	}
	repo.fsrs.states[id] = &domain.UserCardFSRS{
		UserID: domain.UserID("u1"),
		CardID: id,
		State:  domain.FSRSState{Due: due},
	}
}

// collectCardWalk pages forward from cur until the listing runs out, returning
// every id served after the supplied first page.
func collectCardWalk(t *testing.T, uc CardUsecase, orderBy CardOrderBy, page1 *CardConnectionOutput, cur string) []string {
	t.Helper()
	seen := append([]string{}, cardWalkIDs(page1)...)
	for page := 2; page <= 6; page++ {
		out := fetchCardWalkPage(t, uc, orderBy, &cur)
		ids := cardWalkIDs(out)
		if len(ids) == 0 {
			break
		}
		seen = append(seen, ids...)
		cur = encodeCardWalkCursor(out, out.EndCur)
	}
	return seen
}

// assertCardWalkServedOnce fails when any of the supplied ids appears a number
// of times other than once across a full walk.
func assertCardWalkServedOnce(t *testing.T, seen []string, ids ...string) {
	t.Helper()
	counts := map[string]int{}
	for _, id := range seen {
		counts[id]++
	}
	for _, id := range ids {
		if counts[id] != 1 {
			t.Fatalf("row %q appeared %d times across the walk (want exactly 1); walk = %v", id, counts[id], seen)
		}
	}
	if counts["c-foreign"] != 0 {
		t.Fatal("walk leaked a card from another cardgroup")
	}
}

// ---------------------------------------------------------------------------
// UPDATED_AT — the opt-in ordering whose key an ordinary card edit moves.
// ---------------------------------------------------------------------------

// TestCardCursorWalk_S1_UpdatedAtMovedToHead_NoDuplicate reproduces the
// duplicate scenario: the boundary row of page 1 is edited so its updated_at
// moves to the head of the ASC listing before page 2 is fetched. With the
// captured ordering key the second page starts exactly where the first ended,
// so it repeats no row the caller has already seen.
func TestCardCursorWalk_S1_UpdatedAtMovedToHead_NoDuplicate(t *testing.T) {
	t.Parallel()

	repo := newCardWalkFixture("")
	uc := newCardWalkUsecase(repo, repo.fsrs)

	page1 := fetchCardWalkPage(t, uc, CardOrderByUpdatedAt, nil)
	if got := cardWalkIDs(page1); len(got) != 2 || got[0] != "c-a" || got[1] != "c-b" {
		t.Fatalf("page 1 = %v, want [c-a c-b]", got)
	}
	next := encodeCardWalkCursor(page1, page1.EndCur)

	// The caller edits c-b on another device: updated_at drops below every other
	// row, moving it to the head of the ASC listing.
	setCardWalkUpdatedAt(t, repo, "c-b", cdWalkAt(-99))

	page2 := fetchCardWalkPage(t, uc, CardOrderByUpdatedAt, &next)
	got := cardWalkIDs(page2)
	if len(got) != 2 || got[0] != "c-c" || got[1] != "c-d" {
		t.Fatalf("page 2 = %v, want [c-c c-d]", got)
	}
	for _, id := range got {
		if id == "c-a" || id == "c-b" {
			t.Fatalf("page 2 repeated %q from page 1: %v", id, got)
		}
	}
}

// TestCardCursorWalk_S1_UpdatedAt_V1CursorStillDuplicates pins the defect the v2
// envelope exists to fix, and simultaneously proves the v1 backward-compat path
// is still wired: an id-only cursor re-reads the edited row and therefore hands
// back a row page 1 already returned.
func TestCardCursorWalk_S1_UpdatedAt_V1CursorStillDuplicates(t *testing.T) {
	t.Parallel()

	repo := newCardWalkFixture("")
	uc := newCardWalkUsecase(repo, repo.fsrs)

	page1 := fetchCardWalkPage(t, uc, CardOrderByUpdatedAt, nil)
	legacy := cursor.Encode(page1.EndCur)

	setCardWalkUpdatedAt(t, repo, "c-b", cdWalkAt(-99))

	page2 := fetchCardWalkPage(t, uc, CardOrderByUpdatedAt, &legacy)
	got := cardWalkIDs(page2)
	if len(got) == 0 || got[0] != "c-a" {
		t.Fatalf("v1 cursor should re-serve c-a after the boundary row moves to the head; page 2 = %v", got)
	}
}

// TestCardCursorWalk_S2_UpdatedAtMovedToTail_NoSkip reproduces the skip
// scenario: the boundary row of page 1 is edited so its updated_at rises above
// every remaining row. With the captured ordering key the walk continues from
// where page 1 ended, so every row the caller had not yet seen is still returned
// exactly once.
//
// The edited row itself is the documented exception: its ordering key moved into
// the not-yet-visited region, so the walk legitimately meets it again. No cursor
// scheme can avoid that — the edit moved the row across the bookmark, not the
// bookmark across the rows. What v2 fixes is that the UNSEEN rows are no longer
// swallowed along with it.
func TestCardCursorWalk_S2_UpdatedAtMovedToTail_NoSkip(t *testing.T) {
	t.Parallel()

	repo := newCardWalkFixture("")
	uc := newCardWalkUsecase(repo, repo.fsrs)

	page1 := fetchCardWalkPage(t, uc, CardOrderByUpdatedAt, nil)
	next := encodeCardWalkCursor(page1, page1.EndCur)

	setCardWalkUpdatedAt(t, repo, "c-b", cdWalkAt(99))

	seen := collectCardWalk(t, uc, CardOrderByUpdatedAt, page1, next)
	assertCardWalkServedOnce(t, seen, "c-a", "c-c", "c-d", "c-e")
}

// TestCardCursorWalk_S2_UpdatedAt_V1CursorStillSkips pins the defect on the skip
// side: an id-only cursor re-reads the edited boundary row, finds it now sorts
// last, and reports an empty second page — the "rows silently vanish" symptom.
func TestCardCursorWalk_S2_UpdatedAt_V1CursorStillSkips(t *testing.T) {
	t.Parallel()

	repo := newCardWalkFixture("")
	uc := newCardWalkUsecase(repo, repo.fsrs)

	page1 := fetchCardWalkPage(t, uc, CardOrderByUpdatedAt, nil)
	legacy := cursor.Encode(page1.EndCur)

	setCardWalkUpdatedAt(t, repo, "c-b", cdWalkAt(99))

	page2 := fetchCardWalkPage(t, uc, CardOrderByUpdatedAt, &legacy)
	if got := cardWalkIDs(page2); len(got) != 0 {
		t.Fatalf("v1 cursor should return an empty page after the boundary row moves to the tail, got %v", got)
	}
}

// ---------------------------------------------------------------------------
// DUE — the most volatile ordering key in the repository. Its value is not a
// card column at all: every scenario below moves it by rewriting the caller's
// user_card_fsrs row, exactly as a review does.
// ---------------------------------------------------------------------------

// TestCardCursorWalk_S1_DueMovedToHeadByReview_NoDuplicate is the DUE mirror of
// S1, driven by a review that pulls the boundary card's due date in front of
// every other row.
func TestCardCursorWalk_S1_DueMovedToHeadByReview_NoDuplicate(t *testing.T) {
	t.Parallel()

	repo := newCardWalkFixture("")
	uc := newCardWalkUsecase(repo, repo.fsrs)

	page1 := fetchCardWalkPage(t, uc, CardOrderByDue, nil)
	if got := cardWalkIDs(page1); len(got) != 2 || got[0] != "c-a" || got[1] != "c-b" {
		t.Fatalf("page 1 = %v, want [c-a c-b]", got)
	}
	next := encodeCardWalkCursor(page1, page1.EndCur)

	reviewCardWalkDue(t, repo, "c-b", cdWalkAt(-99))

	page2 := fetchCardWalkPage(t, uc, CardOrderByDue, &next)
	got := cardWalkIDs(page2)
	if len(got) != 2 || got[0] != "c-c" || got[1] != "c-d" {
		t.Fatalf("page 2 = %v, want [c-c c-d]", got)
	}
	for _, id := range got {
		if id == "c-a" || id == "c-b" {
			t.Fatalf("page 2 repeated %q from page 1: %v", id, got)
		}
	}
}

// TestCardCursorWalk_S1_Due_V1CursorStillDuplicates pins the DUE side of the
// defect: an id-only cursor re-reads the reviewed card's CURRENT due value and
// therefore hands back a row page 1 already returned.
func TestCardCursorWalk_S1_Due_V1CursorStillDuplicates(t *testing.T) {
	t.Parallel()

	repo := newCardWalkFixture("")
	uc := newCardWalkUsecase(repo, repo.fsrs)

	page1 := fetchCardWalkPage(t, uc, CardOrderByDue, nil)
	legacy := cursor.Encode(page1.EndCur)

	reviewCardWalkDue(t, repo, "c-b", cdWalkAt(-99))

	page2 := fetchCardWalkPage(t, uc, CardOrderByDue, &legacy)
	got := cardWalkIDs(page2)
	if len(got) == 0 || got[0] != "c-a" {
		t.Fatalf("v1 cursor should re-serve c-a after the boundary card is reviewed forward; page 2 = %v", got)
	}
}

// TestCardCursorWalk_S2_DueMovedToTailByReview_NoSkip is the DUE mirror of S2:
// the boundary card is reviewed so its due date is scheduled past every
// remaining row. Every still-unseen card must be served exactly once.
func TestCardCursorWalk_S2_DueMovedToTailByReview_NoSkip(t *testing.T) {
	t.Parallel()

	repo := newCardWalkFixture("")
	uc := newCardWalkUsecase(repo, repo.fsrs)

	page1 := fetchCardWalkPage(t, uc, CardOrderByDue, nil)
	next := encodeCardWalkCursor(page1, page1.EndCur)

	reviewCardWalkDue(t, repo, "c-b", cdWalkAt(99))

	seen := collectCardWalk(t, uc, CardOrderByDue, page1, next)
	assertCardWalkServedOnce(t, seen, "c-a", "c-c", "c-d", "c-e")
}

// TestCardCursorWalk_S2_Due_V1CursorStillSkips pins the DUE side of the skip
// defect: an id-only cursor re-reads the reviewed card, finds it now sorts last,
// and reports an empty second page.
func TestCardCursorWalk_S2_Due_V1CursorStillSkips(t *testing.T) {
	t.Parallel()

	repo := newCardWalkFixture("")
	uc := newCardWalkUsecase(repo, repo.fsrs)

	page1 := fetchCardWalkPage(t, uc, CardOrderByDue, nil)
	legacy := cursor.Encode(page1.EndCur)

	reviewCardWalkDue(t, repo, "c-b", cdWalkAt(99))

	page2 := fetchCardWalkPage(t, uc, CardOrderByDue, &legacy)
	if got := cardWalkIDs(page2); len(got) != 0 {
		t.Fatalf("v1 cursor should return an empty page after the boundary card is scheduled last, got %v", got)
	}
}

// TestCardCursorWalk_Due_StatelessCardPagesThenGainsState covers the COALESCE
// fallback end-to-end. c-b starts with NO user_card_fsrs row, so its DUE key is
// its created_at — page 1 must still order it second. It is then reviewed for
// the first time, acquiring a state row scheduled past every remaining card; the
// captured key must keep the rest of the walk anchored, so no still-unseen row
// is served twice.
//
// This is the case where an emit/compare disagreement about the fallback would
// be silent: capture created_at at emit time but compare against a zero due (or
// vice versa) and the bookmark lands on the wrong row rather than erroring.
func TestCardCursorWalk_Due_StatelessCardPagesThenGainsState(t *testing.T) {
	t.Parallel()

	repo := newCardWalkFixture("c-b")
	uc := newCardWalkUsecase(repo, repo.fsrs)

	page1 := fetchCardWalkPage(t, uc, CardOrderByDue, nil)
	if got := cardWalkIDs(page1); len(got) != 2 || got[0] != "c-a" || got[1] != "c-b" {
		t.Fatalf("page 1 = %v, want [c-a c-b]: a card with no FSRS state orders by its created_at", got)
	}
	if want := encodeTimeOrderKey(cdWalkAt(2)); page1.OrderKeys["c-b"] != want {
		t.Fatalf("OrderKeys[c-b] = %q, want the created_at fallback %q", page1.OrderKeys["c-b"], want)
	}
	next := encodeCardWalkCursor(page1, page1.EndCur)

	// First review of c-b: it gains a state row scheduled after every other card.
	reviewCardWalkDue(t, repo, "c-b", cdWalkAt(99))

	seen := collectCardWalk(t, uc, CardOrderByDue, page1, next)
	assertCardWalkServedOnce(t, seen, "c-a", "c-c", "c-d", "c-e")
}

// TestCardCursorWalk_Due_NilFSRSRepo_EmitAndApplyAgreeOnCreatedAt walks a full
// page set under DUE with the usecase wired WITHOUT an FSRS repository. Both the
// emit path (cardOrderKeys) and the v1 re-hydration path go through
// dueOrderValues, so both must fall back to created_at; a divergence would
// anchor the second page against a value the ordering never produced and drop
// or repeat rows. The fixture carries no state rows either, so the repository's
// COALESCE agrees.
func TestCardCursorWalk_Due_NilFSRSRepo_EmitAndApplyAgreeOnCreatedAt(t *testing.T) {
	t.Parallel()

	repo := &cardWalkRepo{cards: newCardWalkFixture("").cards}
	uc := newCardWalkUsecase(repo, nil)

	page1 := fetchCardWalkPage(t, uc, CardOrderByDue, nil)
	if got := cardWalkIDs(page1); len(got) != 2 || got[0] != "c-a" || got[1] != "c-b" {
		t.Fatalf("page 1 = %v, want [c-a c-b]", got)
	}
	if want := encodeTimeOrderKey(cdWalkAt(2)); page1.OrderKeys["c-b"] != want {
		t.Fatalf("OrderKeys[c-b] = %q, want the created_at fallback %q", page1.OrderKeys["c-b"], want)
	}

	seen := collectCardWalk(t, uc, CardOrderByDue, page1, encodeCardWalkCursor(page1, page1.EndCur))
	assertCardWalkServedOnce(t, seen, "c-a", "c-b", "c-c", "c-d", "c-e")
}

// TestCardCursorWalk_Due_BatchesFSRSLookupOncePerPage pins the batching
// contract. Resolving the DUE key needs the caller's FSRS rows, and doing it per
// row would issue one query per edge — a performance regression the v2 migration
// must not introduce. The emit path must therefore ask once, with every page id
// in a single call. The second assertion additionally shows the v2 cursor path
// skips the per-cursor re-read entirely: a page taken after a v2 cursor still
// costs exactly one lookup.
func TestCardCursorWalk_Due_BatchesFSRSLookupOncePerPage(t *testing.T) {
	t.Parallel()

	repo := newCardWalkFixture("")
	uc := newCardWalkUsecase(repo, repo.fsrs)

	page1 := fetchCardWalkPage(t, uc, CardOrderByDue, nil)
	if repo.fsrs.calls != 1 {
		t.Fatalf("page 1 issued %d FSRS lookups, want exactly 1; batches = %v", repo.fsrs.calls, repo.fsrs.batches)
	}
	want := cardWalkIDs(page1)
	got := repo.fsrs.batches[0]
	if len(got) != len(want) {
		t.Fatalf("FSRS batch = %v, want every page id in one call: %v", got, want)
	}
	inBatch := map[string]bool{}
	for _, id := range got {
		inBatch[id] = true
	}
	for _, id := range want {
		if !inBatch[id] {
			t.Fatalf("FSRS batch %v is missing page row %q", got, id)
		}
	}

	next := encodeCardWalkCursor(page1, page1.EndCur)
	fetchCardWalkPage(t, uc, CardOrderByDue, &next)
	if repo.fsrs.calls != 2 {
		t.Fatalf("page 2 after a v2 cursor issued %d total FSRS lookups, want 2 (one per page emit); batches = %v",
			repo.fsrs.calls, repo.fsrs.batches)
	}
}

// ---------------------------------------------------------------------------
// Backward paging, legacy cursors, and the emitted ordering metadata.
// ---------------------------------------------------------------------------

// TestCardCursorWalk_BackwardV2_BoundaryRowEdited_NoDuplicate is the backward
// mirror of S1. Paging backwards from the tail, the leading boundary row of the
// first backward page is edited so its ordering key moves to the tail of the
// listing. With the captured key the previous page still ends exactly where the
// first began and repeats nothing the caller has already seen.
func TestCardCursorWalk_BackwardV2_BoundaryRowEdited_NoDuplicate(t *testing.T) {
	t.Parallel()

	repo := newCardWalkFixture("")
	uc := newCardWalkUsecase(repo, repo.fsrs)

	// Tail of the listing under (updated_at ASC, id ASC): c-d, c-e.
	page1 := fetchCardWalkPageBackward(t, uc, CardOrderByUpdatedAt, nil, 2)
	if got := cardWalkIDs(page1); len(got) != 2 || got[0] != "c-d" || got[1] != "c-e" {
		t.Fatalf("backward page 1 = %v, want [c-d c-e]", got)
	}
	if !page1.HasPrev {
		t.Fatal("backward page 1 must report hasPreviousPage: c-a/c-b/c-c are still ahead")
	}
	if page1.HasNext {
		t.Fatal("backward page 1 without a before cursor is the tail; hasNextPage must be false")
	}

	prev := encodeCardWalkCursor(page1, page1.StartCur)

	// c-d — the leading boundary row of the page just served — is edited so it
	// now sorts last. A v1 cursor would re-read it and walk from the bottom.
	setCardWalkUpdatedAt(t, repo, "c-d", cdWalkAt(99))

	page2 := fetchCardWalkPageBackward(t, uc, CardOrderByUpdatedAt, &prev, 2)
	got := cardWalkIDs(page2)
	if len(got) != 2 || got[0] != "c-b" || got[1] != "c-c" {
		t.Fatalf("backward page 2 = %v, want [c-b c-c]", got)
	}
	for _, id := range got {
		if id == "c-d" || id == "c-e" {
			t.Fatalf("backward page 2 repeated %q from page 1: %v", id, got)
		}
	}
	if !page2.HasNext {
		t.Fatal("a backward page taken before a cursor must report hasNextPage")
	}
	if !page2.HasPrev {
		t.Fatal("c-a is still ahead of backward page 2; hasPreviousPage must be true")
	}
}

// TestCardCursorWalk_BackwardV1Cursor_StillDuplicates pins the defect on the
// backward edge: an id-only cursor re-reads the edited boundary row, finds it
// now sorts last, and hands back rows the caller already saw.
func TestCardCursorWalk_BackwardV1Cursor_StillDuplicates(t *testing.T) {
	t.Parallel()

	repo := newCardWalkFixture("")
	uc := newCardWalkUsecase(repo, repo.fsrs)

	page1 := fetchCardWalkPageBackward(t, uc, CardOrderByUpdatedAt, nil, 2)
	legacy := cursor.Encode(page1.StartCur)

	setCardWalkUpdatedAt(t, repo, "c-d", cdWalkAt(99))

	page2 := fetchCardWalkPageBackward(t, uc, CardOrderByUpdatedAt, &legacy, 2)
	got := cardWalkIDs(page2)
	if len(got) == 0 || got[len(got)-1] != "c-e" {
		t.Fatalf("v1 cursor should re-serve c-e after the boundary row moves to the tail; backward page 2 = %v", got)
	}
}

// TestCardCursorWalk_LegacyBareIDStillPages verifies the oldest cursor form — a
// bare UUID with no envelope at all — still decodes and pages.
func TestCardCursorWalk_LegacyBareIDStillPages(t *testing.T) {
	t.Parallel()

	repo := newCardWalkFixture("")
	uc := newCardWalkUsecase(repo, repo.fsrs)

	page1 := fetchCardWalkPage(t, uc, CardOrderByUpdatedAt, nil)
	bare := page1.EndCur

	page2 := fetchCardWalkPage(t, uc, CardOrderByUpdatedAt, &bare)
	if got := cardWalkIDs(page2); len(got) != 2 || got[0] != "c-c" || got[1] != "c-d" {
		t.Fatalf("legacy bare-id cursor page 2 = %v, want [c-c c-d]", got)
	}
}

// TestCardCursorWalk_OrderKeysCoverEveryReturnedRow verifies the usecase output
// carries the ordering metadata the resolver needs to emit a v2 cursor for every
// edge, not just the page boundaries — on both mutable orderings, including the
// DUE key that is resolved through the COALESCE rather than read off the row.
func TestCardCursorWalk_OrderKeysCoverEveryReturnedRow(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		orderBy CardOrderBy
		column  repository.CardOrderBy
		want    func(repo *cardWalkRepo, c *domain.Card) time.Time
	}{
		{
			name:    "updated_at",
			orderBy: CardOrderByUpdatedAt,
			column:  repository.CardOrderByUpdatedAt,
			want:    func(_ *cardWalkRepo, c *domain.Card) time.Time { return c.UpdatedAt },
		},
		{
			name:    "due",
			orderBy: CardOrderByDue,
			column:  repository.CardOrderByDue,
			want: func(repo *cardWalkRepo, c *domain.Card) time.Time {
				return repo.cardWalkKey(repository.CardOrderByDue, c)
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			repo := newCardWalkFixture("c-b")
			out := fetchCardWalkPage(t, newCardWalkUsecase(repo, repo.fsrs), tc.orderBy, nil)

			if out.Ordering != (PageOrdering{OrderBy: string(tc.column), Direction: string(repository.SortAsc)}) {
				t.Fatalf("Ordering = %+v, want (%s, ASC)", out.Ordering, tc.column)
			}
			if len(out.Cards) == 0 {
				t.Fatal("fixture returned no rows; the OrderKeys assertion would be vacuous")
			}
			for _, c := range out.Cards {
				got, ok := out.OrderKeys[c.ID]
				if !ok {
					t.Fatalf("OrderKeys is missing row %q", c.ID)
				}
				if want := encodeTimeOrderKey(tc.want(repo, c)); got != want {
					t.Fatalf("OrderKeys[%q] = %q, want %q", c.ID, got, want)
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// v2 cursor rejection paths
// ---------------------------------------------------------------------------

// listCardsWithCursor runs one forward page after the supplied raw cursor
// string, returning only the error, for the rejection cases below.
func listCardsWithCursor(uc CardUsecase, orderBy CardOrderBy, cur string) error {
	_, err := uc.ListCardsByCardgroupConnection(authedCtx("u1"), CardConnectionInput{
		CardgroupID: "cg1",
		First:       intPtr(2),
		After:       &cur,
		OrderBy:     orderByPtr(orderBy),
	})
	return err
}

// TestResolveCardCursor_V2ForeignCardgroup_ReturnsCursorNotFound verifies the
// cross-cardgroup guard is not bypassed by a v2 cursor. A v2 cursor can hydrate
// its ordering column without the repository, but the card lookup and the
// BelongsToCardgroup check must still run or the endpoint becomes an existence
// oracle over card ids outside the requested cardgroup.
func TestResolveCardCursor_V2ForeignCardgroup_ReturnsCursorNotFound(t *testing.T) {
	t.Parallel()

	repo := newCardWalkFixture("")
	uc := newCardWalkUsecase(repo, repo.fsrs)

	cur := cursor.EncodeV2(cursor.Payload{
		ID:        "c-foreign",
		OrderBy:   string(repository.CardOrderByUpdatedAt),
		Direction: string(repository.SortAsc),
		OrderKey:  encodeTimeOrderKey(cdWalkAt(3)),
	})
	assertValidationError(t, listCardsWithCursor(uc, CardOrderByUpdatedAt, cur), "after", "cursor not found")
}

// TestResolveCardCursor_V2UnknownID_ReturnsCursorNotFound verifies a v2 cursor
// for a card that no longer exists is rejected exactly like a v1 one — the
// hydration lookup still runs even though the ordering key is embedded.
func TestResolveCardCursor_V2UnknownID_ReturnsCursorNotFound(t *testing.T) {
	t.Parallel()

	repo := newCardWalkFixture("")
	uc := newCardWalkUsecase(repo, repo.fsrs)

	cur := cursor.EncodeV2(cursor.Payload{
		ID:        "c-deleted",
		OrderBy:   string(repository.CardOrderByUpdatedAt),
		Direction: string(repository.SortAsc),
		OrderKey:  encodeTimeOrderKey(cdWalkAt(1)),
	})
	assertValidationError(t, listCardsWithCursor(uc, CardOrderByUpdatedAt, cur), "after", "cursor not found")
}

// TestResolveCardCursor_V2OrderingMismatch_ReturnsBadUserInput verifies a cursor
// taken under a different column or direction is rejected with the existing
// BAD_USER_INPUT shape rather than silently mis-paging against a value that
// belongs to another column.
func TestResolveCardCursor_V2OrderingMismatch_ReturnsBadUserInput(t *testing.T) {
	t.Parallel()

	cases := map[string]cursor.Payload{
		"different column": {
			ID: "c-a", OrderBy: string(repository.CardOrderByDue),
			Direction: string(repository.SortAsc), OrderKey: encodeTimeOrderKey(cdWalkAt(1)),
		},
		"different direction": {
			ID: "c-a", OrderBy: string(repository.CardOrderByUpdatedAt),
			Direction: string(repository.SortDesc), OrderKey: encodeTimeOrderKey(cdWalkAt(1)),
		},
	}
	for name, p := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			repo := newCardWalkFixture("")
			uc := newCardWalkUsecase(repo, repo.fsrs)
			assertValidationError(t, listCardsWithCursor(uc, CardOrderByUpdatedAt, cursor.EncodeV2(p)), "after", "")
		})
	}
}

// TestResolveCardCursor_V2MalformedOrderKey_ReturnsBadUserInput verifies a v2
// cursor whose ordering-key value does not parse into the active column's type
// is a client error, not an INTERNAL fault.
func TestResolveCardCursor_V2MalformedOrderKey_ReturnsBadUserInput(t *testing.T) {
	t.Parallel()

	repo := newCardWalkFixture("")
	uc := newCardWalkUsecase(repo, repo.fsrs)

	cur := cursor.EncodeV2(cursor.Payload{
		ID:        "c-a",
		OrderBy:   string(repository.CardOrderByUpdatedAt),
		Direction: string(repository.SortAsc),
		OrderKey:  "not-a-timestamp",
	})
	assertValidationError(t, listCardsWithCursor(uc, CardOrderByUpdatedAt, cur), "after", "invalid cursor")
}

// TestApplyCardOrderKey_PerColumn covers the decode half of the ordering key for
// every column in the allowlist, including the ID column (whose key is empty
// because the id is already carried) and the impossible default arm, which must
// stay INTERNAL rather than degrade to BAD_USER_INPUT.
func TestApplyCardOrderKey_PerColumn(t *testing.T) {
	t.Parallel()

	when := encodeTimeOrderKey(cdWalkBase)

	c := &repository.CardCursor{ID: "c-a"}
	if err := applyCardOrderKey(c, repository.CardOrderByID, ""); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.Due != nil || c.CreatedAt != nil || c.UpdatedAt != nil {
		t.Fatalf("orderBy=id must hydrate no extra column, got %+v", c)
	}

	c = &repository.CardCursor{ID: "c-a"}
	if err := applyCardOrderKey(c, repository.CardOrderByCreatedAt, when); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.CreatedAt == nil || !c.CreatedAt.Equal(cdWalkBase) {
		t.Fatalf("orderBy=created_at must hydrate CreatedAt, got %+v", c)
	}

	c = &repository.CardCursor{ID: "c-a"}
	if err := applyCardOrderKey(c, repository.CardOrderByUpdatedAt, when); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.UpdatedAt == nil || !c.UpdatedAt.Equal(cdWalkBase) {
		t.Fatalf("orderBy=updated_at must hydrate UpdatedAt, got %+v", c)
	}

	c = &repository.CardCursor{ID: "c-a"}
	if err := applyCardOrderKey(c, repository.CardOrderByDue, when); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.Due == nil || !c.Due.Equal(cdWalkBase) {
		t.Fatalf("orderBy=due must hydrate Due, got %+v", c)
	}

	for _, orderBy := range []repository.CardOrderBy{
		repository.CardOrderByCreatedAt,
		repository.CardOrderByUpdatedAt,
		repository.CardOrderByDue,
	} {
		if err := applyCardOrderKey(&repository.CardCursor{}, orderBy, "not-a-timestamp"); !eris.Is(err, errCursorKeyMalformed) {
			t.Fatalf("orderBy=%s with an unparseable key must return errCursorKeyMalformed, got %v", orderBy, err)
		}
	}

	err := applyCardOrderKey(&repository.CardCursor{}, repository.CardOrderBy("not_a_real_column"), "")
	assertInternalChain(t, err, "usecase: card: unhandled orderBy")
}

// TestCardOrderKey_PerColumn covers the encode half of the ordering key for
// every column in the allowlist, mirroring TestApplyCardOrderKey_PerColumn on
// the decode side. The walk test above only drives UPDATED_AT and DUE — the
// in-memory walk repo rejects any other ordering — so without this test the
// CREATED_AT arm would carry no assertion at all, and a copy-paste that
// serialized card.UpdatedAt there would compile and pass the whole suite while
// emitting a cursor anchored to the wrong column. The three timestamps are
// deliberately distinct instants so no cross-wired arm can pass.
func TestCardOrderKey_PerColumn(t *testing.T) {
	t.Parallel()

	card := &domain.Card{
		ID:          "c-a",
		CardgroupID: domain.CardgroupID("cg1"),
		CreatedAt:   cdWalkBase,
		UpdatedAt:   cdWalkBase.Add(time.Hour),
	}
	due := cdWalkBase.Add(2 * time.Hour)

	cases := []struct {
		name    string
		orderBy repository.CardOrderBy
		want    string
	}{
		{name: "id", orderBy: repository.CardOrderByID, want: ""},
		{name: "created_at", orderBy: repository.CardOrderByCreatedAt, want: encodeTimeOrderKey(card.CreatedAt)},
		{name: "updated_at", orderBy: repository.CardOrderByUpdatedAt, want: encodeTimeOrderKey(card.UpdatedAt)},
		{name: "due", orderBy: repository.CardOrderByDue, want: encodeTimeOrderKey(due)},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := cardOrderKey(tc.orderBy, card, due)
			if err != nil {
				t.Fatalf("cardOrderKey(%s) returned an unexpected error: %v", tc.orderBy, err)
			}
			if got != tc.want {
				t.Fatalf("cardOrderKey(%s) = %q, want %q", tc.orderBy, got, tc.want)
			}
		})
	}
}

// TestCardOrderKey_UnknownOrderBy covers the encode half's impossible default
// arm: an unmapped column is a caller bug, surfaced as INTERNAL rather than an
// empty key that would silently produce an unanchored cursor.
func TestCardOrderKey_UnknownOrderBy(t *testing.T) {
	t.Parallel()

	_, err := cardOrderKey(repository.CardOrderBy("not_a_real_column"),
		&domain.Card{ID: "c-a", CardgroupID: domain.CardgroupID("cg1")}, time.Time{})
	assertInternalChain(t, err, "usecase: card: unhandled orderBy")
}

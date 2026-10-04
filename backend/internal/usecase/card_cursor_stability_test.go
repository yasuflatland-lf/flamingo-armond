package usecase

import (
	"context"
	"fmt"
	"sort"
	"testing"

	"backend/internal/cursor"
	"backend/internal/domain"
	"backend/internal/repository"
)

type cardWalkRepo struct {
	mockCardRepository
	cards []*domain.Card
}

func (r *cardWalkRepo) FindByID(context.Context, string) (*domain.Card, error) {
	panic("ID cursor must not hydrate a card")
}

func (r *cardWalkRepo) FindPageByCardgroup(
	_ context.Context, cardgroupID string, after *repository.CardCursor, first int, _ *string,
) ([]*domain.Card, int64, error) {
	var rows []*domain.Card
	var total int64
	for _, c := range r.cards {
		if !c.BelongsToCardgroup(domain.CardgroupID(cardgroupID)) {
			continue
		}
		total++
		if after == nil || c.ID > after.ID {
			rows = append(rows, c)
		}
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].ID < rows[j].ID })
	if len(rows) > first {
		rows = rows[:first]
	}
	return rows, total, nil
}

func newCardWalkFixture() *cardWalkRepo {
	return &cardWalkRepo{cards: []*domain.Card{
		{ID: "c-e", CardgroupID: "cg1"},
		{ID: "c-c", CardgroupID: "cg1"},
		{ID: "c-a", CardgroupID: "cg1"},
		{ID: "c-d", CardgroupID: "cg1"},
		{ID: "c-b", CardgroupID: "cg1"},
		{ID: "c-foreign", CardgroupID: "cg2"},
	}}
}

func newCardWalkUsecase(repo *cardWalkRepo) CardUsecase {
	return NewCardUsecase(repo,
		&mockCardgroupRepoForCard{findResult: &domain.Cardgroup{ID: "cg1", OwnerID: "u1"}},
		newTestLogger(),
	)
}

func cardWalkIDs(out *CardConnectionOutput) []string {
	ids := make([]string, 0, len(out.Cards))
	for _, c := range out.Cards {
		ids = append(ids, c.ID)
	}
	return ids
}

func encodeCardWalkCursor(out *CardConnectionOutput, id string) string {
	return cursor.EncodeV2(cursor.Payload{
		ID: id, OrderBy: out.Ordering.OrderBy, Direction: out.Ordering.Direction, OrderKey: out.OrderKeys[id],
	})
}

func fetchCardWalkPage(t *testing.T, uc CardUsecase, after *string) *CardConnectionOutput {
	t.Helper()
	out, err := uc.ListCardsByCardgroupConnection(authedCtx("u1"), CardConnectionInput{
		CardgroupID: "cg1", First: intPtr(2), After: after,
	})
	if err != nil {
		t.Fatalf("unexpected error paging: %v", err)
	}
	return out
}

func TestCardCursorWalk_LegacyBareIDStillPages(t *testing.T) {
	t.Parallel()
	uc := newCardWalkUsecase(newCardWalkFixture())
	page1 := fetchCardWalkPage(t, uc, nil)
	page2 := fetchCardWalkPage(t, uc, &page1.EndCur)
	if got := cardWalkIDs(page2); len(got) != 2 || got[0] != "c-c" || got[1] != "c-d" {
		t.Fatalf("legacy bare-id cursor page 2 = %v, want [c-c c-d]", got)
	}
}

// TestCardCursorWalk_IDOrderingEmitsEmptyOrderKeys pins that card connections
// are fixed at (id, ASC): every page reports that ordering with an empty
// non-nil OrderKeys map, and the walk visits every in-scope card exactly once.
func TestCardCursorWalk_IDOrderingEmitsEmptyOrderKeys(t *testing.T) {
	t.Parallel()
	uc := newCardWalkUsecase(newCardWalkFixture())
	out := fetchCardWalkPage(t, uc, nil)
	var seen []string
	for page := 0; ; page++ {
		if page >= 3 {
			t.Fatal("walk did not terminate after three pages")
		}
		if out.Ordering != (PageOrdering{OrderBy: "id", Direction: "ASC"}) {
			t.Fatalf("Ordering = %+v, want (id, ASC)", out.Ordering)
		}
		if out.OrderKeys == nil || len(out.OrderKeys) != 0 {
			t.Fatalf("OrderKeys = %v, want an empty map", out.OrderKeys)
		}
		seen = append(seen, cardWalkIDs(out)...)
		if !out.HasNext {
			break
		}
		next := encodeCardWalkCursor(out, out.EndCur)
		out = fetchCardWalkPage(t, uc, &next)
	}
	if want := "[c-a c-b c-c c-d c-e]"; fmt.Sprint(seen) != want {
		t.Fatalf("walk = %v, want %s", seen, want)
	}
}

func listCardsWithCursor(uc CardUsecase, cur string) error {
	_, err := uc.ListCardsByCardgroupConnection(authedCtx("u1"), CardConnectionInput{
		CardgroupID: "cg1", First: intPtr(2), After: &cur,
	})
	return err
}

func TestResolveCardCursor_V2OrderingMismatch_ReturnsBadUserInput(t *testing.T) {
	t.Parallel()
	cases := map[string]cursor.Payload{
		"different column":    {ID: "c-a", OrderBy: "due", Direction: "ASC", OrderKey: "2026-07-20T12:01:00Z"},
		"different direction": {ID: "c-a", OrderBy: "id", Direction: "DESC"},
	}
	for name, p := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			uc := newCardWalkUsecase(newCardWalkFixture())
			assertValidationError(t, listCardsWithCursor(uc, cursor.EncodeV2(p)), "after", "cursor does not match the requested ordering")
		})
	}
}

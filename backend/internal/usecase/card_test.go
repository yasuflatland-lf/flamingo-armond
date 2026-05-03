package usecase

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"testing"
	"time"

	"github.com/rotisserie/eris"
	"github.com/vektah/gqlparser/v2/gqlerror"
	"gorm.io/gorm"

	"backend/internal/domain"
	"backend/internal/gqlerr"
	"backend/internal/repository"
)

type mockCardRepository struct {
	findResult          *domain.Card
	findErr             error
	findByIDsResult     map[string]*domain.Card
	findByIDsErr        error
	findByCardgroupRows []*domain.Card
	findByCardgroupErr  error
	createErr           error
	capturedCreate      *domain.Card
	updateResult        *domain.Card
	updateErr           error
	capturedPatch       repository.CardUpdate
	deleteErr           error
	deleteCalled        bool

	deleteByIDsResult int64
	deleteByIDsErr    error
	deleteByIDsCalls  int
	capturedDeleteIDs []string
	capturedDeleteOwn string

	findPageRows  []*domain.Card
	findPageTotal int64
	findPageErr   error
	findDueRows   []*domain.Card
	findDueErr    error
	updateFSRSErr error
	capturedFSRS  domain.FSRSState
	// captured arguments from the most recent FindPageByCardgroup call.
	capturedFindPage struct {
		cardgroupID string
		after       *repository.CardCursor
		before      *repository.CardCursor
		first       int
		last        int
		orderBy     repository.CardOrderBy
		dir         repository.SortOrder
	}

	findByCardgroupAndFrontResult                *domain.Card
	findByCardgroupAndFrontErr                   error
	findByCardgroupAndFrontCalledWithCardgroupID string
	findByCardgroupAndFrontCalledWithFront       string
}

func (m *mockCardRepository) FindByID(_ context.Context, _ string) (*domain.Card, error) {
	return m.findResult, m.findErr
}
func (m *mockCardRepository) FindByIDTx(_ context.Context, _ *gorm.DB, _ string) (*domain.Card, error) {
	return m.findResult, m.findErr
}
func (m *mockCardRepository) FindByIDs(_ context.Context, _ []string) (map[string]*domain.Card, error) {
	return m.findByIDsResult, m.findByIDsErr
}
func (m *mockCardRepository) FindByCardgroup(_ context.Context, _ string) ([]*domain.Card, error) {
	return m.findByCardgroupRows, m.findByCardgroupErr
}
func (m *mockCardRepository) DeleteByIDsTx(_ context.Context, _ *gorm.DB, ownerID string, ids []string) (int64, error) {
	m.deleteByIDsCalls++
	m.capturedDeleteOwn = ownerID
	m.capturedDeleteIDs = append([]string(nil), ids...)
	return m.deleteByIDsResult, m.deleteByIDsErr
}
func (m *mockCardRepository) Create(_ context.Context, card *domain.Card) error {
	m.capturedCreate = card
	return m.createErr
}
func (m *mockCardRepository) FindByCardgroupAndFront(_ context.Context, cardgroupID string, front string) (*domain.Card, error) {
	m.findByCardgroupAndFrontCalledWithCardgroupID = cardgroupID
	m.findByCardgroupAndFrontCalledWithFront = front
	return m.findByCardgroupAndFrontResult, m.findByCardgroupAndFrontErr
}
func (m *mockCardRepository) Update(_ context.Context, _ string, patch repository.CardUpdate) (*domain.Card, error) {
	m.capturedPatch = patch
	return m.updateResult, m.updateErr
}
func (m *mockCardRepository) UpdateFSRSStateTx(_ context.Context, _ *gorm.DB, _ string, state domain.FSRSState) error {
	m.capturedFSRS = state
	return m.updateFSRSErr
}
func (m *mockCardRepository) Delete(_ context.Context, _ string) error {
	m.deleteCalled = true
	return m.deleteErr
}
func (m *mockCardRepository) FindDueCardsTx(_ context.Context, _ *gorm.DB, _ string, _ time.Time, _ int) ([]*domain.Card, error) {
	return m.findDueRows, m.findDueErr
}
func (m *mockCardRepository) FindPageByCardgroup(
	_ context.Context,
	cardgroupID string,
	after, before *repository.CardCursor,
	first, last int,
	orderBy repository.CardOrderBy,
	dir repository.SortOrder,
) ([]*domain.Card, int64, error) {
	m.capturedFindPage.cardgroupID = cardgroupID
	m.capturedFindPage.after = after
	m.capturedFindPage.before = before
	m.capturedFindPage.first = first
	m.capturedFindPage.last = last
	m.capturedFindPage.orderBy = orderBy
	m.capturedFindPage.dir = dir
	return m.findPageRows, m.findPageTotal, m.findPageErr
}

type mockCardgroupRepoForCard struct {
	findResult *domain.Cardgroup
	findErr    error
}

func (m *mockCardgroupRepoForCard) FindByID(_ context.Context, _ string) (*domain.Cardgroup, error) {
	return m.findResult, m.findErr
}

func TestCardUsecase_Create(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name          string
		ctx           context.Context
		cardgroup     *domain.Cardgroup
		cardgroupErr  error
		input         CreateCardInput
		wantErrCode   string
		wantErrField  string
		wantCreatedID string
	}{
		{
			name:        "anonymous",
			ctx:         anonCtx(),
			input:       CreateCardInput{CardgroupID: "cg1", Front: "front", Back: "back"},
			wantErrCode: "UNAUTHENTICATED",
		},
		{
			name:         "missing cardgroup",
			ctx:          authedCtx("u1"),
			cardgroupErr: repository.ErrNotFound,
			input:        CreateCardInput{CardgroupID: "missing", Front: "front", Back: "back"},
			wantErrCode:  "BAD_USER_INPUT",
			wantErrField: "cardgroupId",
		},
		{
			name:        "non owner",
			ctx:         authedCtx("u2"),
			cardgroup:   &domain.Cardgroup{ID: "cg1", OwnerID: "u1"},
			input:       CreateCardInput{CardgroupID: "cg1", Front: "front", Back: "back"},
			wantErrCode: "UNAUTHENTICATED",
		},
		{
			name:         "empty front",
			ctx:          authedCtx("u1"),
			cardgroup:    &domain.Cardgroup{ID: "cg1", OwnerID: "u1"},
			input:        CreateCardInput{CardgroupID: "cg1", Front: "", Back: "back"},
			wantErrCode:  "BAD_USER_INPUT",
			wantErrField: "front",
		},
		{
			name:          "success trims and initializes fsrs",
			ctx:           authedCtx("u1"),
			cardgroup:     &domain.Cardgroup{ID: "cg1", OwnerID: "u1"},
			input:         CreateCardInput{CardgroupID: "cg1", Front: " front ", Back: " back "},
			wantCreatedID: "cg1",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			cardRepo := &mockCardRepository{}
			cgRepo := &mockCardgroupRepoForCard{findResult: tc.cardgroup, findErr: tc.cardgroupErr}
			uc := NewCardUsecase(nil, cardRepo, cgRepo)

			got, err := uc.Create(tc.ctx, tc.input)

			if tc.wantErrCode != "" {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				assertGQLErr(t, err, tc.wantErrCode, tc.wantErrField)
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got == nil || got.CardgroupID != tc.wantCreatedID {
				t.Fatalf("unexpected card: %+v", got)
			}
			if cardRepo.capturedCreate == nil {
				t.Fatal("expected repo.Create call")
			}
			if got.Front != "front" || got.Back != "back" {
				t.Fatalf("expected trimmed text, got front=%q back=%q", got.Front, got.Back)
			}
			if got.FSRS.State != domain.FSRSStateNew || got.FSRS.Stability != 2.5 || got.FSRS.Difficulty != 5.0 {
				t.Fatalf("unexpected FSRS defaults: %+v", got.FSRS)
			}
		})
	}
}

func TestCardUsecase_Update_NonOwnerAndPatch(t *testing.T) {
	t.Parallel()

	existing := &domain.Card{
		ID:          "card1",
		CardgroupID: "cg1",
		Front:       "old front",
		Back:        "old back",
		FSRS:        domain.NewFSRSStateForNewCard(time.Now()),
	}

	t.Run("non owner", func(t *testing.T) {
		t.Parallel()
		uc := NewCardUsecase(nil, &mockCardRepository{findResult: existing},
			&mockCardgroupRepoForCard{findResult: &domain.Cardgroup{ID: "cg1", OwnerID: "u1"}},
		)
		_, err := uc.Update(authedCtx("u2"), "card1", UpdateCardInput{Front: ptr("new")})
		assertGQLErr(t, err, "UNAUTHENTICATED", "")
	})

	t.Run("valid partial update", func(t *testing.T) {
		t.Parallel()
		newFront := "new front"
		cardRepo := &mockCardRepository{
			findResult:   existing,
			updateResult: &domain.Card{ID: "card1", CardgroupID: "cg1", Front: newFront, Back: "old back"},
		}
		uc := NewCardUsecase(nil, cardRepo,
			&mockCardgroupRepoForCard{findResult: &domain.Cardgroup{ID: "cg1", OwnerID: "u1"}},
		)
		got, err := uc.Update(authedCtx("u1"), "card1", UpdateCardInput{Front: ptr(" new front ")})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.Front != newFront {
			t.Fatalf("front = %q, want %q", got.Front, newFront)
		}
		if cardRepo.capturedPatch.Front == nil || *cardRepo.capturedPatch.Front != newFront {
			t.Fatalf("unexpected patch: %+v", cardRepo.capturedPatch)
		}
		if cardRepo.capturedPatch.Back != nil {
			t.Fatalf("back should be unchanged: %+v", cardRepo.capturedPatch)
		}
	})
}

func TestCardUsecase_Delete_NotFoundMasksExistence(t *testing.T) {
	t.Parallel()

	uc := NewCardUsecase(nil, &mockCardRepository{findErr: repository.ErrNotFound},
		&mockCardgroupRepoForCard{},
	)

	err := uc.Delete(authedCtx("u1"), "missing")
	assertGQLErr(t, err, "UNAUTHENTICATED", "")
}

func TestCardUsecase_CardsByCardgroup(t *testing.T) {
	t.Parallel()

	want := []*domain.Card{{ID: "c1", CardgroupID: "cg1"}, {ID: "c2", CardgroupID: "cg1"}}
	cardRepo := &mockCardRepository{findByCardgroupRows: want}
	uc := NewCardUsecase(nil, cardRepo,
		&mockCardgroupRepoForCard{findResult: &domain.Cardgroup{ID: "cg1", OwnerID: "u1"}},
	)

	got, err := uc.CardsByCardgroup(authedCtx("u1"), "cg1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d cards, want 2", len(got))
	}
}

func TestCardUsecase_Card_NotFoundReturnsNil(t *testing.T) {
	t.Parallel()

	uc := NewCardUsecase(nil, &mockCardRepository{findErr: repository.ErrNotFound},
		&mockCardgroupRepoForCard{},
	)

	got, err := uc.Card(authedCtx("u1"), "missing")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if got != nil {
		t.Fatalf("expected nil card, got %+v", got)
	}
}

func TestCardUsecase_RepoErrorsBecomeInternal(t *testing.T) {
	t.Parallel()

	uc := NewCardUsecase(nil, &mockCardRepository{findErr: errors.New("db died")},
		&mockCardgroupRepoForCard{},
	)
	_, err := uc.Card(authedCtx("u1"), "card1")
	assertGQLErr(t, err, "INTERNAL", "")
}

func TestCardUsecase_ListCardsByCardgroupConnection_Anonymous(t *testing.T) {
	t.Parallel()
	uc := NewCardUsecase(nil, &mockCardRepository{}, &mockCardgroupRepoForCard{})
	_, err := uc.ListCardsByCardgroupConnection(anonCtx(), CardConnectionInput{CardgroupID: "cg1"})
	assertGQLErr(t, err, "UNAUTHENTICATED", "")
}

func TestCardUsecase_ListCardsByCardgroupConnection_NonOwner(t *testing.T) {
	t.Parallel()
	uc := NewCardUsecase(nil, &mockCardRepository{},
		&mockCardgroupRepoForCard{findResult: &domain.Cardgroup{ID: "cg1", OwnerID: "u1"}},
	)
	_, err := uc.ListCardsByCardgroupConnection(authedCtx("u2"), CardConnectionInput{CardgroupID: "cg1"})
	assertGQLErr(t, err, "UNAUTHENTICATED", "")
}

func TestCardUsecase_ListCardsByCardgroupConnection_InvalidOrderBy(t *testing.T) {
	t.Parallel()
	uc := NewCardUsecase(nil, &mockCardRepository{},
		&mockCardgroupRepoForCard{findResult: &domain.Cardgroup{ID: "cg1", OwnerID: "u1"}},
	)
	bad := CardOrderBy("STABILITY")
	_, err := uc.ListCardsByCardgroupConnection(authedCtx("u1"), CardConnectionInput{
		CardgroupID: "cg1",
		OrderBy:     &bad,
	})
	assertGQLErr(t, err, "BAD_USER_INPUT", "orderBy")
}

func TestCardUsecase_ListCardsByCardgroupConnection_BothFirstAndLast(t *testing.T) {
	t.Parallel()
	uc := NewCardUsecase(nil, &mockCardRepository{},
		&mockCardgroupRepoForCard{findResult: &domain.Cardgroup{ID: "cg1", OwnerID: "u1"}},
	)
	first := 5
	last := 5
	_, err := uc.ListCardsByCardgroupConnection(authedCtx("u1"), CardConnectionInput{
		CardgroupID: "cg1",
		First:       &first,
		Last:        &last,
	})
	assertGQLErr(t, err, "BAD_USER_INPUT", "first")
}

func TestCardUsecase_ListCardsByCardgroupConnection_DefaultsAndPaging(t *testing.T) {
	t.Parallel()

	// Mock returns first+1 (=21) rows, signalling another page exists.
	rows := make([]*domain.Card, 21)
	for i := range rows {
		rows[i] = &domain.Card{ID: fmt.Sprintf("card-%02d", i), CardgroupID: "cg1"}
	}
	cardRepo := &mockCardRepository{findPageRows: rows, findPageTotal: 50}
	uc := NewCardUsecase(nil, cardRepo,
		&mockCardgroupRepoForCard{findResult: &domain.Cardgroup{ID: "cg1", OwnerID: "u1"}},
	)

	out, err := uc.ListCardsByCardgroupConnection(authedCtx("u1"), CardConnectionInput{
		CardgroupID: "cg1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out.Cards) != 20 {
		t.Fatalf("expected 20 cards after trim, got %d", len(out.Cards))
	}
	if !out.HasNext {
		t.Fatal("expected HasNext=true")
	}
	if out.HasPrev {
		t.Fatal("expected HasPrev=false on first page")
	}
	if out.TotalCount != 50 {
		t.Fatalf("expected TotalCount=50, got %d", out.TotalCount)
	}
	if out.StartCur != "card-00" {
		t.Fatalf("expected StartCur=card-00, got %q", out.StartCur)
	}
	if out.EndCur != "card-19" {
		t.Fatalf("expected EndCur=card-19, got %q", out.EndCur)
	}
	// The repository must have been asked for first+1 = 21 rows.
	if cardRepo.capturedFindPage.first != 21 {
		t.Fatalf("expected repo.first=21, got %d", cardRepo.capturedFindPage.first)
	}
	if cardRepo.capturedFindPage.orderBy != repository.CardOrderByID {
		t.Fatalf("expected default orderBy=id, got %q", cardRepo.capturedFindPage.orderBy)
	}
	if cardRepo.capturedFindPage.dir != repository.SortAsc {
		t.Fatalf("expected default dir=ASC, got %q", cardRepo.capturedFindPage.dir)
	}
}

func intPtr(v int) *int                     { return &v }
func orderByPtr(v CardOrderBy) *CardOrderBy { return &v }

// TestCardUsecase_ListCardsByCardgroupConnection_CursorCrossCardgroup makes
// sure a cursor pointing at a card in a different cardgroup is rejected with
// BAD_USER_INPUT instead of leaking through to the repo (which would happily
// query rows from any cardgroup once the SQL is built).
func TestCardUsecase_ListCardsByCardgroupConnection_CursorCrossCardgroup(t *testing.T) {
	t.Parallel()

	t.Run("after rejected", func(t *testing.T) {
		t.Parallel()
		cardRepo := &mockCardRepository{
			findResult: &domain.Card{ID: "c-foreign", CardgroupID: "cg-other"},
		}
		uc := NewCardUsecase(nil, cardRepo,
			&mockCardgroupRepoForCard{findResult: &domain.Cardgroup{ID: "cg1", OwnerID: "u1"}},
		)
		_, err := uc.ListCardsByCardgroupConnection(authedCtx("u1"), CardConnectionInput{
			CardgroupID: "cg1",
			First:       intPtr(10),
			After:       ptr("c-foreign"),
			OrderBy:     orderByPtr(CardOrderByDue),
		})
		assertGQLErr(t, err, "BAD_USER_INPUT", "after")
	})

	t.Run("before rejected", func(t *testing.T) {
		t.Parallel()
		cardRepo := &mockCardRepository{
			findResult: &domain.Card{ID: "c-foreign", CardgroupID: "cg-other"},
		}
		uc := NewCardUsecase(nil, cardRepo,
			&mockCardgroupRepoForCard{findResult: &domain.Cardgroup{ID: "cg1", OwnerID: "u1"}},
		)
		_, err := uc.ListCardsByCardgroupConnection(authedCtx("u1"), CardConnectionInput{
			CardgroupID: "cg1",
			Last:        intPtr(10),
			Before:      ptr("c-foreign"),
			OrderBy:     orderByPtr(CardOrderByDue),
		})
		assertGQLErr(t, err, "BAD_USER_INPUT", "before")
	})
}

// TestCardUsecase_ListCardsByCardgroupConnection_BackwardPaging exercises the
// last/before flow: the +1 trick fires on the leading edge and HasNext is true
// because `before != nil`.
func TestCardUsecase_ListCardsByCardgroupConnection_BackwardPaging(t *testing.T) {
	t.Parallel()

	rows := []*domain.Card{
		{ID: "c-A", CardgroupID: "cg1"},
		{ID: "c-B", CardgroupID: "cg1"},
		{ID: "c-C", CardgroupID: "cg1"},
		{ID: "c-D", CardgroupID: "cg1"},
	}
	cardRepo := &mockCardRepository{findPageRows: rows, findPageTotal: 10}
	uc := NewCardUsecase(nil, cardRepo,
		&mockCardgroupRepoForCard{findResult: &domain.Cardgroup{ID: "cg1", OwnerID: "u1"}},
	)

	out, err := uc.ListCardsByCardgroupConnection(authedCtx("u1"), CardConnectionInput{
		CardgroupID: "cg1",
		Last:        intPtr(3),
		Before:      ptr("c-X"),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out.Cards) != 3 {
		t.Fatalf("expected 3 cards after trim, got %d", len(out.Cards))
	}
	if out.Cards[0].ID != "c-B" || out.Cards[1].ID != "c-C" || out.Cards[2].ID != "c-D" {
		t.Fatalf("expected [c-B, c-C, c-D], got %v",
			[]string{out.Cards[0].ID, out.Cards[1].ID, out.Cards[2].ID})
	}
	if !out.HasPrev {
		t.Fatal("expected HasPrev=true (len(rows) > last signals more rows precede)")
	}
	if !out.HasNext {
		t.Fatal("expected HasNext=true because before!=nil")
	}
	if out.StartCur != "c-B" || out.EndCur != "c-D" {
		t.Fatalf("expected StartCur=c-B EndCur=c-D, got %q/%q", out.StartCur, out.EndCur)
	}
	// Repo should have been asked for last+1 trailing rows.
	if cardRepo.capturedFindPage.last != 4 {
		t.Fatalf("expected repo.last=4 (last+1), got %d", cardRepo.capturedFindPage.last)
	}
}

// TestCardUsecase_ListCardsByCardgroupConnection_ResolveCursorHydratesDueField
// pins down that resolveCursor populates the field matching the active
// orderBy on the *CardCursor passed to FindPageByCardgroup.
func TestCardUsecase_ListCardsByCardgroupConnection_ResolveCursorHydratesDueField(t *testing.T) {
	t.Parallel()

	dueT := time.Date(2030, 1, 2, 3, 4, 5, 0, time.UTC)
	createdT := time.Date(2031, 5, 6, 7, 8, 9, 0, time.UTC)
	updatedT := time.Date(2032, 7, 8, 9, 10, 11, 0, time.UTC)

	cases := []struct {
		name     string
		orderBy  CardOrderBy
		assertFn func(t *testing.T, c *repository.CardCursor)
	}{
		{
			name:    "Due",
			orderBy: CardOrderByDue,
			assertFn: func(t *testing.T, c *repository.CardCursor) {
				if c.Due == nil || !c.Due.Equal(dueT) {
					t.Fatalf("expected Due=%v, got %v", dueT, c.Due)
				}
			},
		},
		{
			name:    "CreatedAt",
			orderBy: CardOrderByCreatedAt,
			assertFn: func(t *testing.T, c *repository.CardCursor) {
				if c.CreatedAt == nil || !c.CreatedAt.Equal(createdT) {
					t.Fatalf("expected CreatedAt=%v, got %v", createdT, c.CreatedAt)
				}
			},
		},
		{
			name:    "UpdatedAt",
			orderBy: CardOrderByUpdatedAt,
			assertFn: func(t *testing.T, c *repository.CardCursor) {
				if c.UpdatedAt == nil || !c.UpdatedAt.Equal(updatedT) {
					t.Fatalf("expected UpdatedAt=%v, got %v", updatedT, c.UpdatedAt)
				}
			},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			cardRepo := &mockCardRepository{
				findResult: &domain.Card{
					ID:          "c-1",
					CardgroupID: "cg1",
					FSRS:        domain.FSRSState{Due: dueT},
					CreatedAt:   createdT,
					UpdatedAt:   updatedT,
				},
			}
			uc := NewCardUsecase(nil, cardRepo,
				&mockCardgroupRepoForCard{findResult: &domain.Cardgroup{ID: "cg1", OwnerID: "u1"}},
			)
			_, err := uc.ListCardsByCardgroupConnection(authedCtx("u1"), CardConnectionInput{
				CardgroupID: "cg1",
				First:       intPtr(5),
				After:       ptr("c-1"),
				OrderBy:     orderByPtr(tc.orderBy),
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			got := cardRepo.capturedFindPage.after
			if got == nil {
				t.Fatal("expected after cursor to be passed to repo, got nil")
			}
			if got.ID != "c-1" {
				t.Fatalf("expected cursor ID=c-1, got %q", got.ID)
			}
			tc.assertFn(t, got)
		})
	}
}

// decodeJSONRecords parses newline-delimited JSON log lines from buf.
func decodeJSONRecords(t *testing.T, data []byte) []map[string]any {
	t.Helper()
	var records []map[string]any
	for _, line := range bytes.Split(bytes.TrimSpace(data), []byte("\n")) {
		if len(line) == 0 {
			continue
		}
		var rec map[string]any
		if err := json.Unmarshal(line, &rec); err != nil {
			t.Fatalf("decode log line %q: %v", line, err)
		}
		records = append(records, rec)
	}
	return records
}

func TestCardUsecase_Create_Duplicate(t *testing.T) {
	t.Parallel()

	fixture := &domain.Card{ID: "fixture-id", CardgroupID: "cg1", Front: "hello", Back: "fixture-back"}
	cardRepo := &mockCardRepository{
		createErr:                     repository.ErrCardDuplicateFront,
		findByCardgroupAndFrontResult: fixture,
	}
	cgRepo := &mockCardgroupRepoForCard{findResult: &domain.Cardgroup{ID: "cg1", OwnerID: "u1"}}
	uc := NewCardUsecase(nil, cardRepo, cgRepo)

	// Submit with surrounding whitespace to regression-guard the TrimSpace contract.
	_, err := uc.Create(authedCtx("u1"), CreateCardInput{CardgroupID: "cg1", Front: "  hello  ", Back: "world"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	assertGQLErr(t, err, string(gqlerr.CodeBadUserInput), "front")

	gqlErr, ok := err.(*gqlerror.Error)
	if !ok {
		t.Fatalf("expected *gqlerror.Error, got %T", err)
	}
	if got, _ := gqlErr.Extensions["reason"].(string); got != "CARD_DUPLICATE_FRONT" {
		t.Fatalf("expected extensions.reason=%q, got %q", "CARD_DUPLICATE_FRONT", got)
	}
	if got, _ := gqlErr.Extensions["existingCardId"].(string); got != "fixture-id" {
		t.Fatalf("expected extensions.existingCardId=%q, got %q", "fixture-id", got)
	}
	if got, _ := gqlErr.Extensions["existingBack"].(string); got != "fixture-back" {
		t.Fatalf("expected extensions.existingBack=%q, got %q", "fixture-back", got)
	}

	// Verify FindByCardgroupAndFront was called with the trimmed front and correct cardgroupID.
	if cardRepo.findByCardgroupAndFrontCalledWithCardgroupID != "cg1" {
		t.Errorf("expected FindByCardgroupAndFront cardgroupID=%q, got %q",
			"cg1", cardRepo.findByCardgroupAndFrontCalledWithCardgroupID)
	}
	if cardRepo.findByCardgroupAndFrontCalledWithFront != "hello" {
		t.Errorf("expected FindByCardgroupAndFront front=%q (trimmed), got %q",
			"hello", cardRepo.findByCardgroupAndFrontCalledWithFront)
	}
}

func TestCardUsecase_Create_DuplicateLookupRace(t *testing.T) {
	// Not parallel: mutates the global slog default.
	const wantCardgroupID = "cg-test-id"

	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	defer slog.SetDefault(prev)

	cardRepo := &mockCardRepository{
		createErr:                  repository.ErrCardDuplicateFront,
		findByCardgroupAndFrontErr: eris.New("db: connection reset"),
	}
	cgRepo := &mockCardgroupRepoForCard{findResult: &domain.Cardgroup{ID: wantCardgroupID, OwnerID: "u1"}}
	uc := NewCardUsecase(nil, cardRepo, cgRepo)

	_, err := uc.Create(authedCtx("u1"), CreateCardInput{CardgroupID: wantCardgroupID, Front: "hello", Back: "world"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !gqlerr.IsCode(err, gqlerr.CodeInternal) {
		t.Fatalf("expected INTERNAL error, got %v", err)
	}

	records := decodeJSONRecords(t, buf.Bytes())
	if len(records) == 0 {
		t.Fatal("expected at least one ERROR log record, got none")
	}
	rec0 := records[0]

	if rec0["level"] != "ERROR" {
		t.Errorf("expected level=ERROR, got %v", rec0["level"])
	}

	chain, ok := rec0["error_chain"].(map[string]any)
	if !ok {
		t.Fatalf("error_chain is not a JSON object: %T", rec0["error_chain"])
	}
	root, hasRoot := chain["root"].(map[string]any)
	if !hasRoot {
		t.Error("error_chain must have root entry (got external-only shape; stub may be using stdlib errors)")
	}
	if root != nil {
		if stack, _ := root["stack"].([]any); len(stack) == 0 {
			t.Error("error_chain.root.stack must contain at least one frame")
		}
	}

	if rec0["cardgroup_id"] != wantCardgroupID {
		t.Errorf("expected cardgroup_id=%q in ERROR log, got %v", wantCardgroupID, rec0["cardgroup_id"])
	}
	if _, hasFront := rec0["front"]; hasFront {
		t.Error("ERROR log must not contain 'front' field (user-supplied content)")
	}
}

package usecase

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/rotisserie/eris"
	"gorm.io/gorm"

	"backend/internal/domain"
	"backend/internal/repository"
)

type mockCardRepository struct {
	findResult      *domain.Card
	findErr         error
	findByIDsResult map[string]*domain.Card
	findByIDsErr    error
	createErr       error
	capturedCreate  *domain.Card
	updateResult    *domain.Card
	updateErr       error
	capturedPatch   repository.CardUpdate
	deleteErr       error
	deleteCalled    bool

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
	// captured arguments from the most recent FindPageByCardgroup call.
	capturedFindPage struct {
		cardgroupID string
		after       *repository.CardCursor
		before      *repository.CardCursor
		first       int
		last        int
		orderBy     repository.CardOrderBy
		dir         repository.SortOrder
		search      *string
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
func (m *mockCardRepository) Delete(_ context.Context, _ string) error {
	m.deleteCalled = true
	return m.deleteErr
}
func (m *mockCardRepository) FindDueCardsForUser(_ context.Context, _ string, _ string, _ time.Time, _ int) ([]*domain.Card, error) {
	return m.findDueRows, m.findDueErr
}
func (m *mockCardRepository) FindDueCardsForUserTx(_ context.Context, _ *gorm.DB, _ string, _ string, _ time.Time, _ int) ([]*domain.Card, error) {
	return m.findDueRows, m.findDueErr
}
func (m *mockCardRepository) FindPageByCardgroup(
	_ context.Context,
	cardgroupID string,
	after, before *repository.CardCursor,
	first, last int,
	orderBy repository.CardOrderBy,
	dir repository.SortOrder,
	search *string,
) ([]*domain.Card, int64, error) {
	m.capturedFindPage.cardgroupID = cardgroupID
	m.capturedFindPage.after = after
	m.capturedFindPage.before = before
	m.capturedFindPage.first = first
	m.capturedFindPage.last = last
	m.capturedFindPage.orderBy = orderBy
	m.capturedFindPage.dir = dir
	m.capturedFindPage.search = search
	return m.findPageRows, m.findPageTotal, m.findPageErr
}
func (m *mockCardRepository) FindPageByCardgroupForUser(
	_ context.Context,
	_ string,
	cardgroupID string,
	after, before *repository.CardCursor,
	first, last int,
	orderBy repository.CardOrderBy,
	dir repository.SortOrder,
	search *string,
) ([]*domain.Card, int64, error) {
	return m.FindPageByCardgroup(context.Background(), cardgroupID, after, before, first, last, orderBy, dir, search)
}

type mockUserCardFSRSRepository struct {
	byCardID  map[string]*domain.UserCardFSRS
	findErr   error
	upserted  *domain.UserCardFSRS
	upsertErr error
}

func (m *mockUserCardFSRSRepository) FindByUserAndCardIDs(_ context.Context, _ string, ids []string) (map[string]*domain.UserCardFSRS, error) {
	if m.findErr != nil {
		return nil, m.findErr
	}
	out := make(map[string]*domain.UserCardFSRS, len(ids))
	for _, id := range ids {
		if ucs := m.byCardID[id]; ucs != nil {
			out[id] = ucs
		}
	}
	return out, nil
}

func (m *mockUserCardFSRSRepository) FindByUserAndCardIDsTx(_ context.Context, _ *gorm.DB, _ string, ids []string) (map[string]*domain.UserCardFSRS, error) {
	return m.FindByUserAndCardIDs(context.Background(), "", ids)
}

func (m *mockUserCardFSRSRepository) UpsertTx(_ context.Context, _ *gorm.DB, u *domain.UserCardFSRS) error {
	if m.upsertErr != nil {
		return m.upsertErr
	}
	cp := *u
	m.upserted = &cp
	return nil
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
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			cardRepo := &mockCardRepository{}
			cgRepo := &mockCardgroupRepoForCard{findResult: tc.cardgroup, findErr: tc.cardgroupErr}
			uc := NewCardUsecase(nil, cardRepo, cgRepo, nil, newTestLogger())

			got, err := uc.Create(tc.ctx, tc.input)

			if tc.wantErrCode != "" {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				switch tc.wantErrCode {
				case "UNAUTHENTICATED":
					assertUnauthenticated(t, err)
				case "BAD_USER_INPUT":
					assertValidationError(t, err, tc.wantErrField, "")
				default:
					t.Fatalf("unhandled wantErrCode %q in table test", tc.wantErrCode)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.Card == nil || got.Card.CardgroupID != tc.wantCreatedID {
				t.Fatalf("unexpected outcome: %+v", got)
			}
			if cardRepo.capturedCreate == nil {
				t.Fatal("expected repo.Create call")
			}
			if got.Card.Front != "front" || got.Card.Back != "back" {
				t.Fatalf("expected trimmed text, got front=%q back=%q", got.Card.Front, got.Card.Back)
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
	}

	t.Run("non owner", func(t *testing.T) {
		t.Parallel()
		uc := NewCardUsecase(nil, &mockCardRepository{findResult: existing},
			&mockCardgroupRepoForCard{findResult: &domain.Cardgroup{ID: "cg1", OwnerID: "u1"}},
			nil, newTestLogger(),
		)
		_, err := uc.Update(authedCtx("u2"), "card1", UpdateCardInput{Front: ptr("new")})
		assertUnauthenticated(t, err)
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
			nil, newTestLogger(),
		)
		outcome, err := uc.Update(authedCtx("u1"), "card1", UpdateCardInput{Front: ptr(" new front ")})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if outcome.Card == nil {
			t.Fatal("expected non-nil Card on success")
		}
		if outcome.Card.Front != newFront {
			t.Fatalf("front = %q, want %q", outcome.Card.Front, newFront)
		}
		if outcome.Validation != nil {
			t.Fatalf("expected nil Validation on success, got %+v", outcome.Validation)
		}
		if cardRepo.capturedPatch.Front == nil || *cardRepo.capturedPatch.Front != newFront {
			t.Fatalf("unexpected patch: %+v", cardRepo.capturedPatch)
		}
		if cardRepo.capturedPatch.Back != nil {
			t.Fatalf("back should be unchanged: %+v", cardRepo.capturedPatch)
		}
	})
}

func TestCardUsecase_Update_EmptyFront_ValidationVariant(t *testing.T) {
	t.Parallel()

	existing := &domain.Card{
		ID:          "card1",
		CardgroupID: "cg1",
		Front:       "old front",
		Back:        "old back",
	}
	cardRepo := &mockCardRepository{findResult: existing}
	uc := NewCardUsecase(nil, cardRepo,
		&mockCardgroupRepoForCard{findResult: &domain.Cardgroup{ID: "cg1", OwnerID: "u1"}},
		nil, newTestLogger(),
	)

	outcome, err := uc.Update(authedCtx("u1"), "card1", UpdateCardInput{Front: ptr("")})

	if err != nil {
		t.Fatalf("expected nil error (validation goes to outcome), got: %v", err)
	}
	if outcome.Card != nil {
		t.Fatal("expected nil Card on validation failure")
	}
	if outcome.Validation == nil {
		t.Fatal("expected non-nil Validation on empty front")
	}
	if outcome.Validation.Field != "front" {
		t.Fatalf("expected Validation.Field=%q, got %q", "front", outcome.Validation.Field)
	}
	if cardRepo.capturedPatch.Front != nil {
		t.Fatal("repository.Update must not be called on validation failure")
	}
}

func TestCardUsecase_Update_RepoError_InfraChannel(t *testing.T) {
	t.Parallel()

	existing := &domain.Card{
		ID:          "card1",
		CardgroupID: "cg1",
		Front:       "old front",
		Back:        "old back",
	}
	cardRepo := &mockCardRepository{
		findResult: existing,
		updateErr:  errors.New("db: storage failure"),
	}
	uc := NewCardUsecase(nil, cardRepo,
		&mockCardgroupRepoForCard{findResult: &domain.Cardgroup{ID: "cg1", OwnerID: "u1"}},
		nil, newTestLogger(),
	)

	_, err := uc.Update(authedCtx("u1"), "card1", UpdateCardInput{Front: ptr("new front")})

	if err == nil {
		t.Fatal("expected error from repo, got nil")
	}
	assertInternalChain(t, err, "usecase: update card: repo update")
}

func TestCardUsecase_Delete_NotFoundMasksExistence(t *testing.T) {
	t.Parallel()

	uc := NewCardUsecase(nil, &mockCardRepository{findErr: repository.ErrNotFound},
		&mockCardgroupRepoForCard{},
		nil, newTestLogger(),
	)

	err := uc.Delete(authedCtx("u1"), "missing")
	assertUnauthenticated(t, err)
}

func TestCardUsecase_Card_NotFoundReturnsNil(t *testing.T) {
	t.Parallel()

	uc := NewCardUsecase(nil, &mockCardRepository{findErr: repository.ErrNotFound},
		&mockCardgroupRepoForCard{},
		nil, newTestLogger(),
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
		nil, newTestLogger(),
	)
	_, err := uc.Card(authedCtx("u1"), "card1")
	assertInternalChain(t, err, "usecase: card: find by id")
}

func TestCardUsecase_ListCardsByCardgroupConnection_Anonymous(t *testing.T) {
	t.Parallel()
	uc := NewCardUsecase(nil, &mockCardRepository{}, &mockCardgroupRepoForCard{}, nil, newTestLogger())
	_, err := uc.ListCardsByCardgroupConnection(anonCtx(), CardConnectionInput{CardgroupID: "cg1"})
	assertUnauthenticated(t, err)
}

func TestCardUsecase_ListCardsByCardgroupConnection_NonOwner(t *testing.T) {
	t.Parallel()
	uc := NewCardUsecase(nil, &mockCardRepository{},
		&mockCardgroupRepoForCard{findResult: &domain.Cardgroup{ID: "cg1", OwnerID: "u1"}},
		nil, newTestLogger(),
	)
	_, err := uc.ListCardsByCardgroupConnection(authedCtx("u2"), CardConnectionInput{CardgroupID: "cg1"})
	assertUnauthenticated(t, err)
}

func TestCardUsecase_ListCardsByCardgroupConnection_InvalidOrderBy(t *testing.T) {
	t.Parallel()
	uc := NewCardUsecase(nil, &mockCardRepository{},
		&mockCardgroupRepoForCard{findResult: &domain.Cardgroup{ID: "cg1", OwnerID: "u1"}},
		nil, newTestLogger(),
	)
	bad := CardOrderBy("STABILITY")
	_, err := uc.ListCardsByCardgroupConnection(authedCtx("u1"), CardConnectionInput{
		CardgroupID: "cg1",
		OrderBy:     &bad,
	})
	assertValidationError(t, err, "orderBy", "")
}

func TestCardUsecase_ListCardsByCardgroupConnection_BothFirstAndLast(t *testing.T) {
	t.Parallel()
	uc := NewCardUsecase(nil, &mockCardRepository{},
		&mockCardgroupRepoForCard{findResult: &domain.Cardgroup{ID: "cg1", OwnerID: "u1"}},
		nil, newTestLogger(),
	)
	first := 5
	last := 5
	_, err := uc.ListCardsByCardgroupConnection(authedCtx("u1"), CardConnectionInput{
		CardgroupID: "cg1",
		First:       &first,
		Last:        &last,
	})
	assertValidationError(t, err, "first", "")
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
		nil, newTestLogger(),
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
			nil, newTestLogger(),
		)
		_, err := uc.ListCardsByCardgroupConnection(authedCtx("u1"), CardConnectionInput{
			CardgroupID: "cg1",
			First:       intPtr(10),
			After:       ptr("c-foreign"),
			OrderBy:     orderByPtr(CardOrderByDue),
		})
		assertValidationError(t, err, "after", "")
	})

	t.Run("before rejected", func(t *testing.T) {
		t.Parallel()
		cardRepo := &mockCardRepository{
			findResult: &domain.Card{ID: "c-foreign", CardgroupID: "cg-other"},
		}
		uc := NewCardUsecase(nil, cardRepo,
			&mockCardgroupRepoForCard{findResult: &domain.Cardgroup{ID: "cg1", OwnerID: "u1"}},
			nil, newTestLogger(),
		)
		_, err := uc.ListCardsByCardgroupConnection(authedCtx("u1"), CardConnectionInput{
			CardgroupID: "cg1",
			Last:        intPtr(10),
			Before:      ptr("c-foreign"),
			OrderBy:     orderByPtr(CardOrderByDue),
		})
		assertValidationError(t, err, "before", "")
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
		nil, newTestLogger(),
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
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			cardRepo := &mockCardRepository{
				findResult: &domain.Card{
					ID:          "c-1",
					CardgroupID: "cg1",
					CreatedAt:   createdT,
					UpdatedAt:   updatedT,
				},
			}
			userFSRSRepo := &mockUserCardFSRSRepository{
				byCardID: map[string]*domain.UserCardFSRS{
					"c-1": {
						UserID: "u1",
						CardID: "c-1",
						State:  domain.FSRSState{Due: dueT},
					},
				},
			}
			uc := NewCardUsecase(nil, cardRepo,
				&mockCardgroupRepoForCard{findResult: &domain.Cardgroup{ID: "cg1", OwnerID: "u1"}},
				userFSRSRepo, newTestLogger(),
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

// TestCardUsecase_ResolveCursor_MalformedV1_ReturnsBadUserInput verifies that
// a "v1:" envelope with an invalid base64 payload is rejected with
// BAD_USER_INPUT. The cursor decode failure must not surface as INTERNAL.
func TestCardUsecase_ResolveCursor_MalformedV1_ReturnsBadUserInput(t *testing.T) {
	t.Parallel()

	uc := NewCardUsecase(nil, &mockCardRepository{},
		&mockCardgroupRepoForCard{findResult: &domain.Cardgroup{ID: "cg1", OwnerID: "u1"}},
		nil, newTestLogger(),
	)
	malformed := "v1:!!!not-base64!!!"
	_, err := uc.resolveCursor(
		context.Background(),
		&malformed, "cg1", repository.CardOrderByID, "after",
	)
	assertValidationError(t, err, "after", "")
}

// TestCardUsecase_ResolveCursor_V1EncodedID verifies that a v1 encoded cursor
// decodes to the raw ID and proceeds without error for the ID-only orderBy
// (no DB lookup required for CardOrderByID).
func TestCardUsecase_ResolveCursor_V1EncodedID(t *testing.T) {
	t.Parallel()

	uc := NewCardUsecase(nil, &mockCardRepository{},
		&mockCardgroupRepoForCard{findResult: &domain.Cardgroup{ID: "cg1", OwnerID: "u1"}},
		nil, newTestLogger(),
	)
	// "v1:" + base64.RawURLEncoding.EncodeToString([]byte("card-abc")) == "v1:Y2FyZC1hYmM"
	encoded := "v1:Y2FyZC1hYmM"
	c, err := uc.resolveCursor(
		context.Background(),
		&encoded, "cg1", repository.CardOrderByID, "after",
	)
	if err != nil {
		t.Fatalf("unexpected error for v1 encoded cursor: %v", err)
	}
	if c == nil {
		t.Fatal("expected non-nil cursor, got nil")
	}
	if c.ID != "card-abc" {
		t.Fatalf("expected decoded ID=card-abc, got %q", c.ID)
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

// TestCardUsecase_Create_Duplicate verifies that the (cardgroup_id, front)
// unique-index collision is surfaced via outcome.Duplicate as a typed value,
// not as an error. The usecase returns the existing card's identity as
// data so the resolver maps it to the GraphQL `CardDuplicateFrontError` union variant.
func TestCardUsecase_Create_Duplicate(t *testing.T) {
	t.Parallel()

	fixture := &domain.Card{ID: "fixture-id", CardgroupID: "cg1", Front: "hello", Back: "fixture-back"}
	cardRepo := &mockCardRepository{
		createErr:                     repository.ErrCardDuplicateFront,
		findByCardgroupAndFrontResult: fixture,
	}
	cgRepo := &mockCardgroupRepoForCard{findResult: &domain.Cardgroup{ID: "cg1", OwnerID: "u1"}}
	uc := NewCardUsecase(nil, cardRepo, cgRepo, nil, newTestLogger())

	// Submit with surrounding whitespace to regression-guard the TrimSpace contract.
	got, err := uc.Create(authedCtx("u1"), CreateCardInput{CardgroupID: "cg1", Front: "  hello  ", Back: "world"})
	if err != nil {
		t.Fatalf("expected nil error (duplicate is data, not error), got %v", err)
	}
	if got.Card != nil {
		t.Fatalf("expected outcome.Card to be nil on duplicate, got %+v", got.Card)
	}
	if got.Duplicate == nil {
		t.Fatal("expected outcome.Duplicate to be non-nil on duplicate-front collision")
	}
	if got.Duplicate.ExistingID != "fixture-id" {
		t.Errorf("expected outcome.Duplicate.ExistingID=%q, got %q", "fixture-id", got.Duplicate.ExistingID)
	}
	if got.Duplicate.ExistingBack != "fixture-back" {
		t.Errorf("expected outcome.Duplicate.ExistingBack=%q, got %q", "fixture-back", got.Duplicate.ExistingBack)
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

// Two race-path tests exist: TestCardUsecase_Create_DuplicateLookupRace tests the
// "unrelated DB error" branch; TestCardUsecase_Create_DuplicateLookupRace_RowVanished
// tests the actual documented race — the duplicate row vanished before the lookup
// (repository.ErrNotFound, a stdlib sentinel). The latter proves that production's
// eris.Wrap at the call site makes the error_chain rich even for stdlib sentinels.
func TestCardUsecase_Create_DuplicateLookupRace(t *testing.T) {
	t.Parallel()
	const wantCardgroupID = "cg-test-id"

	cardRepo := &mockCardRepository{
		createErr:                  repository.ErrCardDuplicateFront,
		findByCardgroupAndFrontErr: eris.New("db: connection reset"),
	}
	cgRepo := &mockCardgroupRepoForCard{findResult: &domain.Cardgroup{ID: wantCardgroupID, OwnerID: "u1"}}
	uc := NewCardUsecase(nil, cardRepo, cgRepo, nil, newTestLogger())

	_, err := uc.Create(authedCtx("u1"), CreateCardInput{CardgroupID: wantCardgroupID, Front: "hello", Back: "world"})
	// Load-bearing: eris.Wrap at the call site must produce a rich chain even for external errors.
	assertInternalChain(t, err, "usecase: lookup duplicate card after 23505")
}

// TestCardUsecase_Create_DuplicateLookupRace_RowVanished exercises the documented
// race scenario: the duplicate row disappears between the failed INSERT and the
// subsequent SELECT, causing FindByCardgroupAndFront to return repository.ErrNotFound
// (a stdlib errors.New sentinel). The load-bearing assertion is that production's
// eris.Wrap at the call site constructs a rich chain even when the underlying error
// is a stdlib sentinel.
func TestCardUsecase_Create_DuplicateLookupRace_RowVanished(t *testing.T) {
	t.Parallel()
	const wantCardgroupID = "cg-vanished-id"

	cardRepo := &mockCardRepository{
		createErr:                  repository.ErrCardDuplicateFront,
		findByCardgroupAndFrontErr: repository.ErrNotFound,
	}
	cgRepo := &mockCardgroupRepoForCard{findResult: &domain.Cardgroup{ID: wantCardgroupID, OwnerID: "u1"}}
	uc := NewCardUsecase(nil, cardRepo, cgRepo, nil, newTestLogger())

	_, err := uc.Create(authedCtx("u1"), CreateCardInput{CardgroupID: wantCardgroupID, Front: "hello", Back: "world"})
	// Load-bearing: eris.Wrap in production must produce a rich chain even when
	// the wrapped error is a stdlib sentinel (no stack of its own).
	assertInternalChain(t, err, "usecase: lookup duplicate card after 23505")
}

// strPtr returns a pointer to s. Helper used by search passthrough tests.
func strPtr(s string) *string { return &s }

type stubNotionWriter struct {
	mu     sync.Mutex
	calls  []notionWriteCall
	err    error
	called chan struct{}
}

type notionWriteCall struct{ PageID, Text string }

func (s *stubNotionWriter) AppendParagraph(_ context.Context, pageID, text string) error {
	s.mu.Lock()
	s.calls = append(s.calls, notionWriteCall{pageID, text})
	s.mu.Unlock()
	if s.called != nil {
		close(s.called)
	}
	return s.err
}

func TestCardUsecase_Create_triggersNotionWriteback(t *testing.T) {
	t.Parallel()

	stub := &stubNotionWriter{called: make(chan struct{})}
	cardRepo := &mockCardRepository{}
	cgRepo := &mockCardgroupRepoForCard{findResult: &domain.Cardgroup{ID: "cg1", OwnerID: "u1"}}
	uc := NewCardUsecase(nil, cardRepo, cgRepo, nil, newTestLogger()).WithNotionWritebacker(stub, "page-xyz")

	got, err := uc.Create(authedCtx("u1"), CreateCardInput{CardgroupID: "cg1", Front: "front", Back: "back"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Card == nil {
		t.Fatal("expected outcome.Card to be non-nil")
	}

	select {
	case <-stub.called:
	case <-time.After(2 * time.Second):
		t.Fatal("notion writeback not invoked within timeout")
	}

	stub.mu.Lock()
	calls := append([]notionWriteCall(nil), stub.calls...)
	stub.mu.Unlock()

	if len(calls) != 1 {
		t.Fatalf("expected 1 AppendParagraph call, got %d", len(calls))
	}
	if calls[0].PageID != "page-xyz" {
		t.Errorf("expected PageID=%q, got %q", "page-xyz", calls[0].PageID)
	}
	if calls[0].Text != "front back" {
		t.Errorf("expected Text=%q, got %q", "front back", calls[0].Text)
	}
}

func TestCardUsecase_Create_duplicateDoesNotTriggerWriteback(t *testing.T) {
	t.Parallel()

	fixture := &domain.Card{ID: "existing-id", CardgroupID: "cg1", Front: "front", Back: "existing-back"}
	stub := &stubNotionWriter{called: make(chan struct{})}
	cardRepo := &mockCardRepository{
		createErr:                     repository.ErrCardDuplicateFront,
		findByCardgroupAndFrontResult: fixture,
	}
	cgRepo := &mockCardgroupRepoForCard{findResult: &domain.Cardgroup{ID: "cg1", OwnerID: "u1"}}
	uc := NewCardUsecase(nil, cardRepo, cgRepo, nil, newTestLogger()).WithNotionWritebacker(stub, "page-xyz")

	got, err := uc.Create(authedCtx("u1"), CreateCardInput{CardgroupID: "cg1", Front: "front", Back: "back"})
	if err != nil {
		t.Fatalf("expected nil error (duplicate is data), got %v", err)
	}
	if got.Duplicate == nil {
		t.Fatal("expected outcome.Duplicate to be non-nil")
	}
	if got.Card != nil {
		t.Fatalf("expected outcome.Card to be nil on duplicate, got %+v", got.Card)
	}

	select {
	case <-stub.called:
		t.Fatal("write-back should not be invoked for duplicates")
	case <-time.After(50 * time.Millisecond):
		// ok: no write-back observed
	}

	stub.mu.Lock()
	n := len(stub.calls)
	stub.mu.Unlock()

	if n != 0 {
		t.Fatalf("expected 0 AppendParagraph calls on duplicate, got %d", n)
	}
}

func TestCardUsecase_Create_writebackErrorDoesNotAffectOutcome(t *testing.T) {
	t.Parallel()

	stub := &stubNotionWriter{
		err:    errors.New("notion: 500 internal"),
		called: make(chan struct{}),
	}
	cardRepo := &mockCardRepository{}
	cgRepo := &mockCardgroupRepoForCard{findResult: &domain.Cardgroup{ID: "cg1", OwnerID: "u1"}}
	uc := NewCardUsecase(nil, cardRepo, cgRepo, nil, newTestLogger()).WithNotionWritebacker(stub, "page-xyz")

	got, err := uc.Create(authedCtx("u1"), CreateCardInput{CardgroupID: "cg1", Front: "front", Back: "back"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Card == nil {
		t.Fatal("expected outcome.Card to be non-nil even when writeback errors")
	}

	select {
	case <-stub.called:
	case <-time.After(2 * time.Second):
		t.Fatal("notion writeback goroutine did not fire within timeout")
	}
}

func TestCardUsecase_Create_noWritebackWhenNotConfigured(t *testing.T) {
	t.Parallel()

	cardRepo := &mockCardRepository{}
	cgRepo := &mockCardgroupRepoForCard{findResult: &domain.Cardgroup{ID: "cg1", OwnerID: "u1"}}
	uc := NewCardUsecase(nil, cardRepo, cgRepo, nil, newTestLogger())

	got, err := uc.Create(authedCtx("u1"), CreateCardInput{CardgroupID: "cg1", Front: "front", Back: "back"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Card == nil {
		t.Fatal("expected outcome.Card to be non-nil")
	}
}

// TestCardUsecase_ListCardsByCardgroupConnection_SearchPassthrough verifies that
// the usecase normalizes the Search field before forwarding to the repository:
//   - nil stays nil (no filter)
//   - a non-empty, non-whitespace string is passed trimmed
//   - empty string is normalized to nil (no filter)
//   - whitespace-only string is normalized to nil (no filter)
//   - a string with leading/trailing spaces is trimmed before forwarding
func TestCardUsecase_ListCardsByCardgroupConnection_SearchPassthrough(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		searchInput *string
		wantSearch  *string
	}{
		{
			name:        "nil search is forwarded as nil",
			searchInput: nil,
			wantSearch:  nil,
		},
		{
			name:        "non-empty search string is forwarded unchanged",
			searchInput: strPtr("apple"),
			wantSearch:  strPtr("apple"),
		},
		{
			name:        "empty string is normalized to nil",
			searchInput: strPtr(""),
			wantSearch:  nil,
		},
		{
			name:        "whitespace-only string is normalized to nil",
			searchInput: strPtr("   "),
			wantSearch:  nil,
		},
		{
			name:        "leading and trailing spaces are trimmed",
			searchInput: strPtr("  apple  "),
			wantSearch:  strPtr("apple"),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			cardRepo := &mockCardRepository{
				findPageRows:  []*domain.Card{},
				findPageTotal: 0,
			}
			cgRepo := &mockCardgroupRepoForCard{
				findResult: &domain.Cardgroup{ID: "cg1", OwnerID: "u1"},
			}
			uc := NewCardUsecase(nil, cardRepo, cgRepo, nil, newTestLogger())

			_, err := uc.ListCardsByCardgroupConnection(authedCtx("u1"), CardConnectionInput{
				CardgroupID: "cg1",
				Search:      tc.searchInput,
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			got := cardRepo.capturedFindPage.search
			if tc.wantSearch == nil {
				if got != nil {
					t.Fatalf("expected search=nil, got %q", *got)
				}
				return
			}
			if got == nil {
				t.Fatalf("expected search=%q, got nil", *tc.wantSearch)
			}
			if *got != *tc.wantSearch {
				t.Fatalf("expected search=%q, got %q", *tc.wantSearch, *got)
			}
		})
	}
}

package usecase

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"backend/internal/domain"
	"backend/internal/repository"
)

type mockCardRepository struct {
	findResult          *domain.Card
	findErr             error
	findByCardgroupRows []*domain.Card
	findByCardgroupErr  error
	createErr           error
	capturedCreate      *domain.Card
	updateResult        *domain.Card
	updateErr           error
	capturedPatch       repository.CardUpdate
	deleteErr           error
	deleteCalled        bool

	findPageRows  []*domain.Card
	findPageTotal int64
	findPageErr   error
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
}

func (m *mockCardRepository) FindByID(_ context.Context, _ string) (*domain.Card, error) {
	return m.findResult, m.findErr
}
func (m *mockCardRepository) FindByCardgroup(_ context.Context, _ string) ([]*domain.Card, error) {
	return m.findByCardgroupRows, m.findByCardgroupErr
}
func (m *mockCardRepository) Create(_ context.Context, card *domain.Card) error {
	m.capturedCreate = card
	return m.createErr
}
func (m *mockCardRepository) Update(_ context.Context, _ string, patch repository.CardUpdate) (*domain.Card, error) {
	m.capturedPatch = patch
	return m.updateResult, m.updateErr
}
func (m *mockCardRepository) Delete(_ context.Context, _ string) error {
	m.deleteCalled = true
	return m.deleteErr
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
			uc := NewCardUsecase(cardRepo, cgRepo)

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
		uc := NewCardUsecase(
			&mockCardRepository{findResult: existing},
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
		uc := NewCardUsecase(
			cardRepo,
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

	uc := NewCardUsecase(
		&mockCardRepository{findErr: repository.ErrNotFound},
		&mockCardgroupRepoForCard{},
	)

	err := uc.Delete(authedCtx("u1"), "missing")
	assertGQLErr(t, err, "UNAUTHENTICATED", "")
}

func TestCardUsecase_CardsByCardgroup(t *testing.T) {
	t.Parallel()

	want := []*domain.Card{{ID: "c1", CardgroupID: "cg1"}, {ID: "c2", CardgroupID: "cg1"}}
	cardRepo := &mockCardRepository{findByCardgroupRows: want}
	uc := NewCardUsecase(
		cardRepo,
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

	uc := NewCardUsecase(
		&mockCardRepository{findErr: repository.ErrNotFound},
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

	uc := NewCardUsecase(
		&mockCardRepository{findErr: errors.New("db died")},
		&mockCardgroupRepoForCard{},
	)
	_, err := uc.Card(authedCtx("u1"), "card1")
	assertGQLErr(t, err, "INTERNAL", "")
}

func TestCardUsecase_ListCardsByCardgroupConnection_Anonymous(t *testing.T) {
	t.Parallel()
	uc := NewCardUsecase(&mockCardRepository{}, &mockCardgroupRepoForCard{})
	_, err := uc.ListCardsByCardgroupConnection(anonCtx(), CardConnectionInput{CardgroupID: "cg1"})
	assertGQLErr(t, err, "UNAUTHENTICATED", "")
}

func TestCardUsecase_ListCardsByCardgroupConnection_NonOwner(t *testing.T) {
	t.Parallel()
	uc := NewCardUsecase(
		&mockCardRepository{},
		&mockCardgroupRepoForCard{findResult: &domain.Cardgroup{ID: "cg1", OwnerID: "u1"}},
	)
	_, err := uc.ListCardsByCardgroupConnection(authedCtx("u2"), CardConnectionInput{CardgroupID: "cg1"})
	assertGQLErr(t, err, "UNAUTHENTICATED", "")
}

func TestCardUsecase_ListCardsByCardgroupConnection_InvalidOrderBy(t *testing.T) {
	t.Parallel()
	uc := NewCardUsecase(
		&mockCardRepository{},
		&mockCardgroupRepoForCard{findResult: &domain.Cardgroup{ID: "cg1", OwnerID: "u1"}},
	)
	bad := "STABILITY"
	_, err := uc.ListCardsByCardgroupConnection(authedCtx("u1"), CardConnectionInput{
		CardgroupID: "cg1",
		OrderBy:     &bad,
	})
	assertGQLErr(t, err, "BAD_USER_INPUT", "orderBy")
}

func TestCardUsecase_ListCardsByCardgroupConnection_BothFirstAndLast(t *testing.T) {
	t.Parallel()
	uc := NewCardUsecase(
		&mockCardRepository{},
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
	uc := NewCardUsecase(
		cardRepo,
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

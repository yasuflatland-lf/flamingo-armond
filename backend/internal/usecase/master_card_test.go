package usecase

import (
	"context"
	"testing"
	"time"

	"github.com/rotisserie/eris"
	"gorm.io/gorm"

	"backend/internal/cursor"
	"backend/internal/domain"
	"backend/internal/repository"
)

// ---------------------------------------------------------------------------
// Test doubles
// ---------------------------------------------------------------------------

// panicMasterCardRepo satisfies repository.MasterCardRepository with every
// method panicking. Concrete mocks embed it and override only the methods the
// test under exercise actually calls, mirroring panicRoleRepo in
// cmd/server/main_test.go.
type panicMasterCardRepo struct{}

func (panicMasterCardRepo) ListByMasterCardgroup(_ context.Context, _ string) ([]*domain.MasterCard, error) {
	panic("not used in this test")
}

func (panicMasterCardRepo) FindPageByMasterCardgroup(
	_ context.Context, _ string, _, _ *repository.MasterCardCursor, _, _ int,
	_ repository.MasterCardOrderBy, _ repository.SortOrder, _ *string,
) ([]*domain.MasterCard, int64, error) {
	panic("not used in this test")
}

func (panicMasterCardRepo) CountByMasterCardgroup(_ context.Context, _ string) (int64, error) {
	panic("not used in this test")
}

func (panicMasterCardRepo) Create(_ context.Context, _ *domain.MasterCard) error {
	panic("not used in this test")
}

func (panicMasterCardRepo) UpsertManyTx(_ context.Context, _ *gorm.DB, _ []*domain.MasterCard) (repository.UpsertManyTxResult, error) {
	panic("not used in this test")
}

func (panicMasterCardRepo) ListFrontsByMasterCardgroupTx(_ context.Context, _ *gorm.DB, _ string) ([]string, error) {
	panic("not used in this test")
}

func (panicMasterCardRepo) DeleteByMasterCardgroupAndFrontsTx(_ context.Context, _ *gorm.DB, _ string, _ []string) (int64, error) {
	panic("not used in this test")
}

func (panicMasterCardRepo) Delete(_ context.Context, _ string) error {
	panic("not used in this test")
}

// findPageByMasterCardgroupCall captures the translated arguments passed to
// FindPageByMasterCardgroup so a test can assert the usecase translated the
// orderBy / direction and clamped the page size correctly.
type findPageByMasterCardgroupCall struct {
	MasterCardgroupID string
	After             *repository.MasterCardCursor
	Before            *repository.MasterCardCursor
	First             int
	Last              int
	OrderBy           repository.MasterCardOrderBy
	Dir               repository.SortOrder
	Search            *string
}

// mockMasterCardReadRepo is a manual test double for repository.MasterCardRepository.
type mockMasterCardReadRepo struct {
	panicMasterCardRepo

	findPageRows  []*domain.MasterCard
	findPageTotal int64
	findPageErr   error
	findPageCalls []findPageByMasterCardgroupCall

	countRes   int64
	countErr   error
	countCalls int
}

func (m *mockMasterCardReadRepo) FindPageByMasterCardgroup(
	_ context.Context,
	masterCardgroupID string,
	after, before *repository.MasterCardCursor,
	first, last int,
	orderBy repository.MasterCardOrderBy,
	dir repository.SortOrder,
	search *string,
) ([]*domain.MasterCard, int64, error) {
	m.findPageCalls = append(m.findPageCalls, findPageByMasterCardgroupCall{
		MasterCardgroupID: masterCardgroupID,
		After:             after,
		Before:            before,
		First:             first,
		Last:              last,
		OrderBy:           orderBy,
		Dir:               dir,
		Search:            search,
	})
	if m.findPageErr != nil {
		return nil, 0, m.findPageErr
	}
	return m.findPageRows, m.findPageTotal, nil
}

func (m *mockMasterCardReadRepo) CountByMasterCardgroup(_ context.Context, _ string) (int64, error) {
	m.countCalls++
	if m.countErr != nil {
		return 0, m.countErr
	}
	return m.countRes, nil
}

// panicMasterCardgroupRepo satisfies repository.MasterCardgroupRepository with
// every method panicking; concrete mocks override only what they exercise.
type panicMasterCardgroupRepo struct{}

func (panicMasterCardgroupRepo) FindByID(_ context.Context, _ string) (*domain.MasterCardgroup, error) {
	panic("not used in this test")
}

func (panicMasterCardgroupRepo) EnsureByName(_ context.Context, _ string) (*domain.MasterCardgroup, error) {
	panic("not used in this test")
}

func (panicMasterCardgroupRepo) Create(_ context.Context, _ *domain.MasterCardgroup) error {
	panic("not used in this test")
}

func (panicMasterCardgroupRepo) Update(_ context.Context, _ string, _ repository.MasterCardgroupUpdate) (*domain.MasterCardgroup, error) {
	panic("not used in this test")
}

func (panicMasterCardgroupRepo) Delete(_ context.Context, _ string) error {
	panic("not used in this test")
}

func (panicMasterCardgroupRepo) ListDefaultStarters(_ context.Context) ([]*domain.MasterCardgroup, error) {
	panic("not used in this test")
}

func (panicMasterCardgroupRepo) FindPublishedPage(
	_ context.Context, _, _ *repository.MasterCatalogCursor, _, _ int,
	_ repository.MasterCatalogOrderBy, _ repository.SortOrder, _ *string,
) ([]*repository.MasterCatalogItem, error) {
	panic("not used in this test")
}

func (panicMasterCardgroupRepo) CountPublished(_ context.Context, _ *string) (int64, error) {
	panic("not used in this test")
}

func (panicMasterCardgroupRepo) CountAdmin(_ context.Context, _ *string) (int64, error) {
	panic("not used in this test")
}

func (panicMasterCardgroupRepo) CountCards(_ context.Context, _ string) (int64, error) {
	panic("not used in this test")
}

func (panicMasterCardgroupRepo) FindPublishedByID(_ context.Context, _ string) (*domain.MasterCardgroup, error) {
	panic("not used in this test")
}

func (panicMasterCardgroupRepo) FindAdminPage(
	_ context.Context, _, _ *repository.MasterCatalogCursor, _, _ int,
	_ repository.MasterCatalogOrderBy, _ repository.SortOrder, _ *string,
) ([]*repository.MasterCatalogItem, error) {
	panic("not used in this test")
}

func (panicMasterCardgroupRepo) Publish(_ context.Context, _ string) (*domain.MasterCardgroup, error) {
	panic("not used in this test")
}

func (panicMasterCardgroupRepo) Unpublish(_ context.Context, _ string) (*domain.MasterCardgroup, error) {
	panic("not used in this test")
}

// mockMasterCardgroupReadRepo overrides FindByID (deck incl. DRAFT) and CountCards,
// the only two methods MasterCardUsecase.AdminMaster consumes.
type mockMasterCardgroupReadRepo struct {
	panicMasterCardgroupRepo

	findByIDFn    func(id string) (*domain.MasterCardgroup, error)
	countCardsRes int64
	countCardsErr error
}

func (m *mockMasterCardgroupReadRepo) FindByID(_ context.Context, id string) (*domain.MasterCardgroup, error) {
	if m.findByIDFn != nil {
		return m.findByIDFn(id)
	}
	return nil, repository.ErrNotFound
}

func (m *mockMasterCardgroupReadRepo) CountCards(_ context.Context, _ string) (int64, error) {
	if m.countCardsErr != nil {
		return 0, m.countCardsErr
	}
	return m.countCardsRes, nil
}

// masterCardFixture returns a minimal *domain.MasterCard for assertions.
func masterCardFixture(id, mcgID string, pos int) *domain.MasterCard {
	now := time.Now().UTC()
	return &domain.MasterCard{
		ID:                id,
		MasterCardgroupID: mcgID,
		Front:             domain.CardText("front-" + id),
		Back:              domain.CardText("back-" + id),
		Position:          pos,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
}

func newMasterCardUC(t *testing.T, mc repository.MasterCardRepository, mcg repository.MasterCardgroupRepository, isAdmin bool) MasterCardUsecase {
	t.Helper()
	return NewMasterCardUsecase(mc, mcg, newTestAdminGate(isAdmin), newTestLogger())
}

// ---------------------------------------------------------------------------
// Constructor
// ---------------------------------------------------------------------------

func TestNewMasterCardUsecase_NilDepsPanic(t *testing.T) {
	t.Parallel()
	gate := newTestAdminGate(true)
	logger := newTestLogger()
	cases := map[string]func(){
		"nil masterCard": func() {
			NewMasterCardUsecase(nil, &mockMasterCardgroupReadRepo{}, gate, logger)
		},
		"nil masterCardgroup": func() {
			NewMasterCardUsecase(&mockMasterCardReadRepo{}, nil, gate, logger)
		},
		"nil adminGate": func() {
			NewMasterCardUsecase(&mockMasterCardReadRepo{}, &mockMasterCardgroupReadRepo{}, nil, logger)
		},
		"nil logger": func() {
			NewMasterCardUsecase(&mockMasterCardReadRepo{}, &mockMasterCardgroupReadRepo{}, gate, nil)
		},
	}
	for name, fn := range cases {
		t.Run(name, func(t *testing.T) {
			defer func() {
				if r := recover(); r == nil {
					t.Fatalf("expected panic for %s", name)
				}
			}()
			fn()
		})
	}
}

// ---------------------------------------------------------------------------
// AdminMaster
// ---------------------------------------------------------------------------

func TestMasterCard_AdminMaster_NonAdminForbidden(t *testing.T) {
	t.Parallel()
	uc := newMasterCardUC(t, &mockMasterCardReadRepo{}, &mockMasterCardgroupReadRepo{}, false)
	_, err := uc.AdminMaster(authedCtx("u1"), "id-1")
	// FORBIDDEN classification must be the ucerr typed error, never gqlerr.
	assertForbidden(t, err, "admin only")
}

func TestMasterCard_AdminMaster_Unauthenticated(t *testing.T) {
	t.Parallel()
	uc := newMasterCardUC(t, &mockMasterCardReadRepo{}, &mockMasterCardgroupReadRepo{}, true)
	_, err := uc.AdminMaster(anonCtx(), "id-1")
	assertUnauthenticated(t, err)
}

func TestMasterCard_AdminMaster_Success_IncludesDraft(t *testing.T) {
	t.Parallel()
	draft := masterCardgroup("id-1") // masterCardgroup() returns a DRAFT deck
	mcg := &mockMasterCardgroupReadRepo{
		findByIDFn:    func(string) (*domain.MasterCardgroup, error) { return draft, nil },
		countCardsRes: 42,
	}
	uc := newMasterCardUC(t, &mockMasterCardReadRepo{}, mcg, true)
	got, err := uc.AdminMaster(authedCtx("admin1"), "id-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got == nil || got.Master == nil {
		t.Fatalf("expected non-nil MasterWithCount, got %+v", got)
	}
	if got.Master.Status != domain.MasterStatusDraft {
		t.Fatalf("expected DRAFT deck, got %s", got.Master.Status)
	}
	if got.CardCount != 42 {
		t.Fatalf("expected CardCount 42, got %d", got.CardCount)
	}
}

func TestMasterCard_AdminMaster_NotFound(t *testing.T) {
	t.Parallel()
	mcg := &mockMasterCardgroupReadRepo{
		findByIDFn: func(string) (*domain.MasterCardgroup, error) { return nil, repository.ErrNotFound },
	}
	uc := newMasterCardUC(t, &mockMasterCardReadRepo{}, mcg, true)
	_, err := uc.AdminMaster(authedCtx("admin1"), "missing")
	// Not-found is surfaced as a validation error on "id" (mirrors master-catalog admin reads).
	assertValidationError(t, err, "id", "")
}

// ---------------------------------------------------------------------------
// ListMasterCards
// ---------------------------------------------------------------------------

func TestMasterCard_ListMasterCards_NonAdminForbidden(t *testing.T) {
	t.Parallel()
	uc := newMasterCardUC(t, &mockMasterCardReadRepo{}, &mockMasterCardgroupReadRepo{}, false)
	_, err := uc.ListMasterCards(authedCtx("u1"), MasterCardConnectionInput{MasterCardgroupID: "id-1"})
	assertForbidden(t, err, "admin only")
}

func TestMasterCard_ListMasterCards_Unauthenticated(t *testing.T) {
	t.Parallel()
	uc := newMasterCardUC(t, &mockMasterCardReadRepo{}, &mockMasterCardgroupReadRepo{}, true)
	_, err := uc.ListMasterCards(anonCtx(), MasterCardConnectionInput{MasterCardgroupID: "id-1"})
	assertUnauthenticated(t, err)
}

func TestMasterCard_ListMasterCards_DefaultPageSize(t *testing.T) {
	t.Parallel()
	mc := &mockMasterCardReadRepo{findPageTotal: 3, findPageRows: nil}
	uc := newMasterCardUC(t, mc, &mockMasterCardgroupReadRepo{}, true)
	_, err := uc.ListMasterCards(authedCtx("admin1"), MasterCardConnectionInput{MasterCardgroupID: "id-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(mc.findPageCalls) != 1 {
		t.Fatalf("expected 1 FindPage call, got %d", len(mc.findPageCalls))
	}
	// Default first=20, +1 fetch trick inflates to 21.
	if got := mc.findPageCalls[0].First; got != defaultPageSize+1 {
		t.Fatalf("expected First %d (default %d + 1 fetch), got %d", defaultPageSize+1, defaultPageSize, got)
	}
	if got := mc.findPageCalls[0].Last; got != 0 {
		t.Fatalf("expected Last 0, got %d", got)
	}
}

func TestMasterCard_ListMasterCards_PageSizeClampedToMax(t *testing.T) {
	t.Parallel()
	mc := &mockMasterCardReadRepo{}
	uc := newMasterCardUC(t, mc, &mockMasterCardgroupReadRepo{}, true)
	_, err := uc.ListMasterCards(authedCtx("admin1"), MasterCardConnectionInput{
		MasterCardgroupID: "id-1",
		First:             intPtr(1000),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Clamp to maxPageSize, then +1 fetch trick.
	if got := mc.findPageCalls[0].First; got != maxPageSize+1 {
		t.Fatalf("expected First clamped to %d (+1 fetch), got %d", maxPageSize+1, got)
	}
}

func TestMasterCard_ListMasterCards_OrderByTranslation(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		orderBy MasterCardOrderBy
		dir     SortOrder
		wantOB  repository.MasterCardOrderBy
		wantDir repository.SortOrder
	}{
		{"id asc", MasterCardOrderByID, SortOrderAsc, repository.MasterCardOrderByID, repository.SortAsc},
		{"position asc", MasterCardOrderByPosition, SortOrderAsc, repository.MasterCardOrderByPosition, repository.SortAsc},
		{"created desc", MasterCardOrderByCreatedAt, SortOrderDesc, repository.MasterCardOrderByCreatedAt, repository.SortDesc},
		{"updated desc", MasterCardOrderByUpdatedAt, SortOrderDesc, repository.MasterCardOrderByUpdatedAt, repository.SortDesc},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			mc := &mockMasterCardReadRepo{}
			uc := newMasterCardUC(t, mc, &mockMasterCardgroupReadRepo{}, true)
			ob := tc.orderBy
			dir := tc.dir
			_, err := uc.ListMasterCards(authedCtx("admin1"), MasterCardConnectionInput{
				MasterCardgroupID: "id-1",
				First:             intPtr(5),
				OrderBy:           &ob,
				OrderDirection:    &dir,
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			call := mc.findPageCalls[0]
			if call.OrderBy != tc.wantOB {
				t.Fatalf("orderBy: want %q, got %q", tc.wantOB, call.OrderBy)
			}
			if call.Dir != tc.wantDir {
				t.Fatalf("dir: want %q, got %q", tc.wantDir, call.Dir)
			}
		})
	}
}

func TestMasterCard_ListMasterCards_OrderByDefault(t *testing.T) {
	t.Parallel()
	mc := &mockMasterCardReadRepo{}
	uc := newMasterCardUC(t, mc, &mockMasterCardgroupReadRepo{}, true)
	_, err := uc.ListMasterCards(authedCtx("admin1"), MasterCardConnectionInput{MasterCardgroupID: "id-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	call := mc.findPageCalls[0]
	// Schema default is POSITION / ASC (schema/master_card.graphql), so a usecase
	// that receives nil orderBy / orderDirection must default to POSITION.
	if call.OrderBy != repository.MasterCardOrderByPosition {
		t.Fatalf("default orderBy: want POSITION, got %q", call.OrderBy)
	}
	if call.Dir != repository.SortAsc {
		t.Fatalf("default dir: want ASC, got %q", call.Dir)
	}
	if call.MasterCardgroupID != "id-1" {
		t.Fatalf("masterCardgroupID: want id-1, got %q", call.MasterCardgroupID)
	}
}

func TestMasterCard_ListMasterCards_TotalCountOnly(t *testing.T) {
	t.Parallel()
	// first==0 && last==0 short-circuit: totalCount must still surface, the row
	// fetch returns no edges. totalCount comes from CountByMasterCardgroup (the
	// separate count), computed before the short-circuit.
	mc := &mockMasterCardReadRepo{countRes: 17}
	uc := newMasterCardUC(t, mc, &mockMasterCardgroupReadRepo{}, true)
	out, err := uc.ListMasterCards(authedCtx("admin1"), MasterCardConnectionInput{
		MasterCardgroupID: "id-1",
		First:             intPtr(0),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.TotalCount != 17 {
		t.Fatalf("expected TotalCount 17, got %d", out.TotalCount)
	}
	if len(out.Cards) != 0 {
		t.Fatalf("expected no cards, got %d", len(out.Cards))
	}
	if out.HasNext || out.HasPrev {
		t.Fatalf("expected no page flags, got hasNext=%v hasPrev=%v", out.HasNext, out.HasPrev)
	}
	// The +1 fetch must request (0,0) for a totalCount-only request.
	if got := mc.findPageCalls[0].First; got != 0 {
		t.Fatalf("expected First 0, got %d", got)
	}
}

func TestMasterCard_ListMasterCards_ReturnsEdgesAndCursors(t *testing.T) {
	t.Parallel()
	rows := []*domain.MasterCard{
		masterCardFixture("a", "id-1", 1),
		masterCardFixture("b", "id-1", 2),
	}
	mc := &mockMasterCardReadRepo{findPageRows: rows, countRes: 2}
	uc := newMasterCardUC(t, mc, &mockMasterCardgroupReadRepo{}, true)
	out, err := uc.ListMasterCards(authedCtx("admin1"), MasterCardConnectionInput{
		MasterCardgroupID: "id-1",
		First:             intPtr(10),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.TotalCount != 2 {
		t.Fatalf("expected TotalCount 2, got %d", out.TotalCount)
	}
	if len(out.Cards) != 2 {
		t.Fatalf("expected 2 cards, got %d", len(out.Cards))
	}
	// Cursors are the opaque v1 envelope over the node id (the cursor encoder used
	// by ListCardsByCardgroupConnection).
	if out.StartCur != cursor.Encode("a") {
		t.Fatalf("StartCur: want %q, got %q", cursor.Encode("a"), out.StartCur)
	}
	if out.EndCur != cursor.Encode("b") {
		t.Fatalf("EndCur: want %q, got %q", cursor.Encode("b"), out.EndCur)
	}
}

func TestMasterCard_ListMasterCards_SearchNormalizedToNil(t *testing.T) {
	t.Parallel()
	mc := &mockMasterCardReadRepo{}
	uc := newMasterCardUC(t, mc, &mockMasterCardgroupReadRepo{}, true)
	blank := "   "
	_, err := uc.ListMasterCards(authedCtx("admin1"), MasterCardConnectionInput{
		MasterCardgroupID: "id-1",
		First:             intPtr(5),
		Search:            &blank,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mc.findPageCalls[0].Search != nil {
		t.Fatalf("whitespace-only search must normalize to nil, got %q", *mc.findPageCalls[0].Search)
	}
}

func TestMasterCard_ListMasterCards_SearchTrimmed(t *testing.T) {
	t.Parallel()
	mc := &mockMasterCardReadRepo{}
	uc := newMasterCardUC(t, mc, &mockMasterCardgroupReadRepo{}, true)
	raw := "  hello  "
	_, err := uc.ListMasterCards(authedCtx("admin1"), MasterCardConnectionInput{
		MasterCardgroupID: "id-1",
		First:             intPtr(5),
		Search:            &raw,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := mc.findPageCalls[0].Search
	if got == nil || *got != "hello" {
		t.Fatalf("search must be trimmed to %q, got %v", "hello", got)
	}
}

func TestMasterCard_ListMasterCards_RepoErrorWrapped(t *testing.T) {
	t.Parallel()
	sentinel := eris.New("db boom")
	mc := &mockMasterCardReadRepo{findPageErr: sentinel}
	uc := newMasterCardUC(t, mc, &mockMasterCardgroupReadRepo{}, true)
	_, err := uc.ListMasterCards(authedCtx("admin1"), MasterCardConnectionInput{
		MasterCardgroupID: "id-1",
		First:             intPtr(5),
	})
	assertInternalChain(t, err, "usecase: master card: list")
}

func TestMasterCard_ListMasterCards_CountErrorWrapped(t *testing.T) {
	t.Parallel()
	sentinel := eris.New("count boom")
	mc := &mockMasterCardReadRepo{countErr: sentinel}
	uc := newMasterCardUC(t, mc, &mockMasterCardgroupReadRepo{}, true)
	_, err := uc.ListMasterCards(authedCtx("admin1"), MasterCardConnectionInput{
		MasterCardgroupID: "id-1",
		First:             intPtr(5),
	})
	assertInternalChain(t, err, "usecase: master card: list")
}

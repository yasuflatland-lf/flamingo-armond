package usecase

import (
	"context"
	"testing"
	"time"

	"github.com/rotisserie/eris"
	"github.com/stretchr/testify/require"
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

func (panicMasterCardRepo) FindByID(_ context.Context, _ string) (*domain.MasterCard, error) {
	panic("not used in this test")
}

func (panicMasterCardRepo) FindPageByMasterCardgroup(
	_ context.Context, _ string, _, _ *repository.MasterCardCursor, _, _ int,
	_ repository.MasterCardOrderBy, _ repository.SortOrder, _ *string,
) ([]*domain.MasterCard, int64, error) {
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

func (panicMasterCardRepo) FindByMasterCardgroupAndFront(_ context.Context, _, _ string) (*domain.MasterCard, error) {
	panic("not used in this test")
}

func (panicMasterCardRepo) Update(_ context.Context, _ string, _ repository.MasterCardUpdate) (*domain.MasterCard, error) {
	panic("not used in this test")
}

func (panicMasterCardRepo) DeleteMany(_ context.Context, _ []string) (int64, error) {
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

	// findByIDFn backs FindByID for cursor-hydration tests. When nil, FindByID
	// returns repository.ErrNotFound.
	findByIDFn    func(id string) (*domain.MasterCard, error)
	findByIDCalls []string
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

func (m *mockMasterCardReadRepo) FindByID(_ context.Context, id string) (*domain.MasterCard, error) {
	m.findByIDCalls = append(m.findByIDCalls, id)
	if m.findByIDFn != nil {
		return m.findByIDFn(id)
	}
	return nil, repository.ErrNotFound
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
) ([]*repository.MasterCatalogItem, int64, error) {
	panic("not used in this test")
}

func (panicMasterCardgroupRepo) CountCards(_ context.Context, _ string) (int64, error) {
	panic("not used in this test")
}

func (panicMasterCardgroupRepo) FindPublishedByID(_ context.Context, _ string) (*domain.MasterCardgroup, error) {
	panic("not used in this test")
}

func (panicMasterCardgroupRepo) FindPageAnyStatus(
	_ context.Context, _, _ *repository.MasterCatalogCursor, _, _ int,
	_ repository.MasterCatalogOrderBy, _ repository.SortOrder, _ *string,
) ([]*repository.MasterCatalogItem, int64, error) {
	panic("not used in this test")
}

func (panicMasterCardgroupRepo) Publish(_ context.Context, _ string) (*domain.MasterCardgroup, error) {
	panic("not used in this test")
}

func (panicMasterCardgroupRepo) Unpublish(_ context.Context, _ string) (*domain.MasterCardgroup, error) {
	panic("not used in this test")
}

// mockMasterCardgroupReadRepo overrides FindByID (deck incl. DRAFT), CountCards,
// and FindPublishedByID (published-only deck). FindByID + CountCards back
// MasterCardUsecase.AdminMaster; FindPublishedByID backs ListPublicMasterCards'
// published-only visibility gate.
type mockMasterCardgroupReadRepo struct {
	panicMasterCardgroupRepo

	findByIDFn          func(id string) (*domain.MasterCardgroup, error)
	findPublishedByIDFn func(id string) (*domain.MasterCardgroup, error)
	countCardsRes       int64
	countCardsErr       error
}

func (m *mockMasterCardgroupReadRepo) FindByID(_ context.Context, id string) (*domain.MasterCardgroup, error) {
	if m.findByIDFn != nil {
		return m.findByIDFn(id)
	}
	return nil, repository.ErrNotFound
}

func (m *mockMasterCardgroupReadRepo) FindPublishedByID(_ context.Context, id string) (*domain.MasterCardgroup, error) {
	if m.findPublishedByIDFn != nil {
		return m.findPublishedByIDFn(id)
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
	return NewMasterCardUsecase(nil, mc, mcg, newTestAdminGate(isAdmin), newTestLogger())
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
			NewMasterCardUsecase(nil, nil, &mockMasterCardgroupReadRepo{}, gate, logger)
		},
		"nil masterCardgroup": func() {
			NewMasterCardUsecase(nil, &mockMasterCardReadRepo{}, nil, gate, logger)
		},
		"nil adminGate": func() {
			NewMasterCardUsecase(nil, &mockMasterCardReadRepo{}, &mockMasterCardgroupReadRepo{}, nil, logger)
		},
		"nil logger": func() {
			NewMasterCardUsecase(nil, &mockMasterCardReadRepo{}, &mockMasterCardgroupReadRepo{}, gate, nil)
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

// A non-NotFound FindByID error wraps into the internal eris chain.
func TestMasterCard_AdminMaster_FindByIDInfraErrorWrapped(t *testing.T) {
	t.Parallel()
	boom := eris.New("db down")
	mcg := &mockMasterCardgroupReadRepo{
		findByIDFn: func(string) (*domain.MasterCardgroup, error) { return nil, boom },
	}
	uc := newMasterCardUC(t, &mockMasterCardReadRepo{}, mcg, true)
	_, err := uc.AdminMaster(authedCtx("admin1"), "id-1")
	assertInternalChain(t, err, "usecase: master card: admin master")
}

// A CountCards error wraps into the internal eris chain.
func TestMasterCard_AdminMaster_CountCardsInfraErrorWrapped(t *testing.T) {
	t.Parallel()
	draft := masterCardgroup("id-1")
	mcg := &mockMasterCardgroupReadRepo{
		findByIDFn:    func(string) (*domain.MasterCardgroup, error) { return draft, nil },
		countCardsErr: eris.New("count down"),
	}
	uc := newMasterCardUC(t, &mockMasterCardReadRepo{}, mcg, true)
	_, err := uc.AdminMaster(authedCtx("admin1"), "id-1")
	assertInternalChain(t, err, "usecase: master card: admin master")
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
	// fetch returns no edges. totalCount is the search-aware count returned by
	// FindPageByMasterCardgroup, which the repository computes before its own
	// no-rows short-circuit (so a (0,0) request still observes the real count).
	mc := &mockMasterCardReadRepo{findPageTotal: 17}
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
	mc := &mockMasterCardReadRepo{findPageRows: rows, findPageTotal: 2}
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
	// StartCur / EndCur MUST be the RAW node ids, NOT cursor.Encode(...). The
	// resolver's connection layer applies the cursor encoder once; storing the
	// encoded value here would double-encode (mirrors ListCardsByCardgroupConnection).
	if out.StartCur != "a" {
		t.Fatalf("StartCur: want raw id %q, got %q", "a", out.StartCur)
	}
	if out.EndCur != "b" {
		t.Fatalf("EndCur: want raw id %q, got %q", "b", out.EndCur)
	}
	// Guard against regressing to double-encode: the stored cursors must NOT be the
	// encoded envelope.
	if out.StartCur == cursor.Encode("a") || out.EndCur == cursor.Encode("b") {
		t.Fatalf("cursors must not be cursor.Encode(...)-encoded; got StartCur=%q EndCur=%q", out.StartCur, out.EndCur)
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

// I1 regression guard: with an active search, TotalCount must equal the
// search-FILTERED count returned by FindPageByMasterCardgroup, never an
// unfiltered count. The mock returns findPageTotal=2 for a 2-match/4-card
// scenario; before the fix the usecase used a separate search-unaware count.
func TestMasterCard_ListMasterCards_SearchAwareTotalCount(t *testing.T) {
	t.Parallel()
	rows := []*domain.MasterCard{
		masterCardFixture("m1", "id-1", 0),
		masterCardFixture("m2", "id-1", 1),
	}
	// findPageTotal=2 models the repository's COUNT(*) WHERE ... ILIKE search,
	// i.e. only the matching rows. The group holds 4 cards in total, but the
	// search filter narrows the count to 2.
	mc := &mockMasterCardReadRepo{findPageRows: rows, findPageTotal: 2}
	uc := newMasterCardUC(t, mc, &mockMasterCardgroupReadRepo{}, true)
	search := "apple"
	out, err := uc.ListMasterCards(authedCtx("admin1"), MasterCardConnectionInput{
		MasterCardgroupID: "id-1",
		First:             intPtr(10),
		Search:            &search,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.TotalCount != 2 {
		t.Fatalf("expected search-filtered TotalCount 2, got %d", out.TotalCount)
	}
	// The search reached the repository (trimmed, non-nil).
	got := mc.findPageCalls[0].Search
	if got == nil || *got != "apple" {
		t.Fatalf("expected search %q to reach repo, got %v", "apple", got)
	}
}

// Mixed-direction (First AND Last both non-nil) is rejected with BAD_USER_INPUT
// before any repository access (mirrors card.go's BothFirstAndLast rejection).
func TestMasterCard_ListMasterCards_BothFirstAndLast(t *testing.T) {
	t.Parallel()
	uc := newMasterCardUC(t, &mockMasterCardReadRepo{}, &mockMasterCardgroupReadRepo{}, true)
	_, err := uc.ListMasterCards(authedCtx("admin1"), MasterCardConnectionInput{
		MasterCardgroupID: "id-1",
		First:             intPtr(5),
		Last:              intPtr(5),
	})
	assertValidationError(t, err, "first", "")
}

// ---------------------------------------------------------------------------
// resolveMasterCardOrderBy
// ---------------------------------------------------------------------------

func TestMasterCard_ResolveMasterCardOrderBy_Invalid(t *testing.T) {
	t.Parallel()
	t.Run("invalid orderBy", func(t *testing.T) {
		t.Parallel()
		bad := MasterCardOrderBy("NOPE")
		_, _, err := resolveMasterCardOrderBy(&bad, nil)
		assertValidationError(t, err, "orderBy", "")
	})
	t.Run("invalid orderDirection", func(t *testing.T) {
		t.Parallel()
		badDir := SortOrder("SIDEWAYS")
		_, _, err := resolveMasterCardOrderBy(nil, &badDir)
		assertValidationError(t, err, "orderDirection", "")
	})
}

// ---------------------------------------------------------------------------
// Cursor hydration (resolveMasterCardCursor via ListMasterCards)
// ---------------------------------------------------------------------------

// After-cursor for POSITION ordering hydrates the position column from FindByID
// and forwards the populated cursor to the repository page query.
func TestMasterCard_ListMasterCards_CursorHydratesPosition(t *testing.T) {
	t.Parallel()
	cursorCard := masterCardFixture("cur-1", "id-1", 7)
	mc := &mockMasterCardReadRepo{
		findByIDFn: func(string) (*domain.MasterCard, error) { return cursorCard, nil },
	}
	uc := newMasterCardUC(t, mc, &mockMasterCardgroupReadRepo{}, true)
	ob := MasterCardOrderByPosition
	after := cursor.Encode("cur-1")
	_, err := uc.ListMasterCards(authedCtx("admin1"), MasterCardConnectionInput{
		MasterCardgroupID: "id-1",
		First:             intPtr(5),
		After:             &after,
		OrderBy:           &ob,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	gotAfter := mc.findPageCalls[0].After
	if gotAfter == nil {
		t.Fatal("expected non-nil after cursor passed to repo")
	}
	if gotAfter.ID != "cur-1" {
		t.Fatalf("after cursor ID: want cur-1, got %q", gotAfter.ID)
	}
	if gotAfter.Position == nil || *gotAfter.Position != 7 {
		t.Fatalf("after cursor must hydrate position 7, got %v", gotAfter.Position)
	}
}

// After-cursor for CREATED_AT ordering hydrates the created_at time column.
func TestMasterCard_ListMasterCards_CursorHydratesCreatedAt(t *testing.T) {
	t.Parallel()
	cursorCard := masterCardFixture("cur-2", "id-1", 0)
	mc := &mockMasterCardReadRepo{
		findByIDFn: func(string) (*domain.MasterCard, error) { return cursorCard, nil },
	}
	uc := newMasterCardUC(t, mc, &mockMasterCardgroupReadRepo{}, true)
	ob := MasterCardOrderByCreatedAt
	after := cursor.Encode("cur-2")
	_, err := uc.ListMasterCards(authedCtx("admin1"), MasterCardConnectionInput{
		MasterCardgroupID: "id-1",
		First:             intPtr(5),
		After:             &after,
		OrderBy:           &ob,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	gotAfter := mc.findPageCalls[0].After
	if gotAfter == nil || gotAfter.CreatedAt == nil {
		t.Fatalf("after cursor must hydrate created_at, got %+v", gotAfter)
	}
	if !gotAfter.CreatedAt.Equal(cursorCard.CreatedAt) {
		t.Fatalf("created_at: want %v, got %v", cursorCard.CreatedAt, *gotAfter.CreatedAt)
	}
}

// A cursor whose card belongs to a DIFFERENT master cardgroup is rejected with
// BAD_USER_INPUT — FindByID is group-agnostic, so the cross-group guard is
// explicit.
func TestMasterCard_ListMasterCards_CursorCrossGroupRejected(t *testing.T) {
	t.Parallel()
	foreign := masterCardFixture("foreign-1", "other-group", 3)
	mc := &mockMasterCardReadRepo{
		findByIDFn: func(string) (*domain.MasterCard, error) { return foreign, nil },
	}
	uc := newMasterCardUC(t, mc, &mockMasterCardgroupReadRepo{}, true)
	ob := MasterCardOrderByPosition
	after := cursor.Encode("foreign-1")
	_, err := uc.ListMasterCards(authedCtx("admin1"), MasterCardConnectionInput{
		MasterCardgroupID: "id-1",
		First:             intPtr(5),
		After:             &after,
		OrderBy:           &ob,
	})
	assertValidationError(t, err, "after", "")
}

// A corrupt / undecodable cursor (a v1 envelope with a malformed base64
// payload) is rejected with BAD_USER_INPUT ("invalid cursor") before any
// FindByID lookup. A bare-id cursor is NOT a decode error — cursor.Decode passes
// it through and the cross-group/not-found guard handles it — so the corrupt
// fixture must use the v1 envelope to exercise the decode-failure branch.
func TestMasterCard_ListMasterCards_CursorCorruptRejected(t *testing.T) {
	t.Parallel()
	mc := &mockMasterCardReadRepo{}
	uc := newMasterCardUC(t, mc, &mockMasterCardgroupReadRepo{}, true)
	ob := MasterCardOrderByPosition
	after := "v1:!!!not-valid-base64!!!" // v1 prefix + malformed base64 payload
	_, err := uc.ListMasterCards(authedCtx("admin1"), MasterCardConnectionInput{
		MasterCardgroupID: "id-1",
		First:             intPtr(5),
		After:             &after,
		OrderBy:           &ob,
	})
	assertValidationError(t, err, "after", "invalid cursor")
	if len(mc.findByIDCalls) != 0 {
		t.Fatalf("corrupt cursor must be rejected before FindByID, got %d calls", len(mc.findByIDCalls))
	}
}

// A FindByID infrastructure error during cursor hydration surfaces as an
// internal eris-chain wrap, not a validation/forbidden error.
func TestMasterCard_ListMasterCards_CursorHydrationInfraErrorWrapped(t *testing.T) {
	t.Parallel()
	boom := eris.New("db down")
	mc := &mockMasterCardReadRepo{
		findByIDFn: func(string) (*domain.MasterCard, error) { return nil, boom },
	}
	uc := newMasterCardUC(t, mc, &mockMasterCardgroupReadRepo{}, true)
	ob := MasterCardOrderByPosition
	after := cursor.Encode("cur-x")
	_, err := uc.ListMasterCards(authedCtx("admin1"), MasterCardConnectionInput{
		MasterCardgroupID: "id-1",
		First:             intPtr(5),
		After:             &after,
		OrderBy:           &ob,
	})
	assertInternalChain(t, err, "usecase: master card: resolve cursor")
}

// After-cursor for UPDATED_AT ordering hydrates the updated_at time column and
// leaves created_at and position nil — a future regression that hydrates the
// wrong column would fail the nil assertions.
func TestMasterCard_ListMasterCards_CursorHydratesUpdatedAt(t *testing.T) {
	t.Parallel()
	cursorCard := masterCardFixture("cur-3", "id-1", 0)
	mc := &mockMasterCardReadRepo{
		findByIDFn: func(string) (*domain.MasterCard, error) { return cursorCard, nil },
	}
	uc := newMasterCardUC(t, mc, &mockMasterCardgroupReadRepo{}, true)
	ob := MasterCardOrderByUpdatedAt
	after := cursor.Encode("cur-3")
	_, err := uc.ListMasterCards(authedCtx("admin1"), MasterCardConnectionInput{
		MasterCardgroupID: "id-1",
		First:             intPtr(5),
		After:             &after,
		OrderBy:           &ob,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	gotAfter := mc.findPageCalls[0].After
	if gotAfter == nil {
		t.Fatal("expected non-nil after cursor passed to repo")
	}
	if gotAfter.UpdatedAt == nil {
		t.Fatal("after cursor must hydrate updated_at, got nil")
	}
	if !gotAfter.UpdatedAt.Equal(cursorCard.UpdatedAt) {
		t.Fatalf("updated_at: want %v, got %v", cursorCard.UpdatedAt, *gotAfter.UpdatedAt)
	}
	// Only the UPDATED_AT column must be set; the others must remain nil so a
	// regression that also hydrates created_at or position is caught immediately.
	if gotAfter.CreatedAt != nil {
		t.Fatalf("created_at must be nil for UPDATED_AT cursor, got %v", *gotAfter.CreatedAt)
	}
	if gotAfter.Position != nil {
		t.Fatalf("position must be nil for UPDATED_AT cursor, got %v", *gotAfter.Position)
	}
}

// A corrupt Before cursor is rejected with a validation error whose Field is
// "before" — proving that the field label is wired correctly for the before
// path (the after path is covered by TestMasterCard_ListMasterCards_CursorCorruptRejected).
func TestMasterCard_ListMasterCards_CursorBeforeFieldLabel(t *testing.T) {
	t.Parallel()
	mc := &mockMasterCardReadRepo{}
	uc := newMasterCardUC(t, mc, &mockMasterCardgroupReadRepo{}, true)
	ob := MasterCardOrderByPosition
	before := "v1:!!!not-valid-base64!!!" // v1 prefix + malformed base64 payload
	_, err := uc.ListMasterCards(authedCtx("admin1"), MasterCardConnectionInput{
		MasterCardgroupID: "id-1",
		Last:              intPtr(5),
		Before:            &before,
		OrderBy:           &ob,
	})
	assertValidationError(t, err, "before", "invalid cursor")
	if len(mc.findByIDCalls) != 0 {
		t.Fatalf("corrupt cursor must be rejected before FindByID, got %d calls", len(mc.findByIDCalls))
	}
}

// ---------------------------------------------------------------------------
// Context-cancellation pass-through tests
//
// Each of the four isContextDone branches in master_card.go must return the
// raw context error unwrapped so callers can check identity via
// errors.Is(err, context.Canceled). The assertion convention mirrors
// ownership_test.go: assertCancelled (errors.Is chain check) AND
// require.Equal (pointer-identity check that the error is NOT eris-wrapped).
// ---------------------------------------------------------------------------

// TestMasterCard_AdminMaster_FindByID_PropagatesCancelled verifies that a
// context.Canceled returned by masterCardgroupRepo.FindByID passes through
// unwrapped from AdminMaster so the caller's identity check succeeds.
func TestMasterCard_AdminMaster_FindByID_PropagatesCancelled(t *testing.T) {
	t.Parallel()
	mcg := &mockMasterCardgroupReadRepo{
		findByIDFn: func(string) (*domain.MasterCardgroup, error) { return nil, context.Canceled },
	}
	uc := newMasterCardUC(t, &mockMasterCardReadRepo{}, mcg, true)
	_, err := uc.AdminMaster(authedCtx("admin1"), "id-1")

	assertCancelled(t, err)
	require.Equal(t, context.Canceled, err, "expected unwrapped context.Canceled, got %v", err)
}

// TestMasterCard_AdminMaster_CountCards_PropagatesCancelled verifies that a
// context.Canceled returned by masterCardgroupRepo.CountCards passes through
// unwrapped from AdminMaster so the caller's identity check succeeds.
func TestMasterCard_AdminMaster_CountCards_PropagatesCancelled(t *testing.T) {
	t.Parallel()
	draft := masterCardgroup("id-1")
	mcg := &mockMasterCardgroupReadRepo{
		findByIDFn:    func(string) (*domain.MasterCardgroup, error) { return draft, nil },
		countCardsErr: context.Canceled,
	}
	uc := newMasterCardUC(t, &mockMasterCardReadRepo{}, mcg, true)
	_, err := uc.AdminMaster(authedCtx("admin1"), "id-1")

	assertCancelled(t, err)
	require.Equal(t, context.Canceled, err, "expected unwrapped context.Canceled, got %v", err)
}

// TestMasterCard_ListMasterCards_FindPage_PropagatesCancelled verifies that a
// context.Canceled returned by masterCardRepo.FindPageByMasterCardgroup passes
// through unwrapped from ListMasterCards so the caller's identity check succeeds.
func TestMasterCard_ListMasterCards_FindPage_PropagatesCancelled(t *testing.T) {
	t.Parallel()
	mc := &mockMasterCardReadRepo{findPageErr: context.Canceled}
	uc := newMasterCardUC(t, mc, &mockMasterCardgroupReadRepo{}, true)
	_, err := uc.ListMasterCards(authedCtx("admin1"), MasterCardConnectionInput{
		MasterCardgroupID: "id-1",
		First:             intPtr(5),
	})

	assertCancelled(t, err)
	require.Equal(t, context.Canceled, err, "expected unwrapped context.Canceled, got %v", err)
}

// TestMasterCard_ListMasterCards_CursorHydration_PropagatesCancelled verifies
// that a context.Canceled returned by masterCardRepo.FindByID during cursor
// hydration (resolveMasterCardCursor, reached when orderBy is non-ID) passes
// through unwrapped from ListMasterCards so the caller's identity check succeeds.
func TestMasterCard_ListMasterCards_CursorHydration_PropagatesCancelled(t *testing.T) {
	t.Parallel()
	mc := &mockMasterCardReadRepo{
		findByIDFn: func(string) (*domain.MasterCard, error) { return nil, context.Canceled },
	}
	uc := newMasterCardUC(t, mc, &mockMasterCardgroupReadRepo{}, true)
	ob := MasterCardOrderByPosition // non-ID ordering triggers FindByID in resolveMasterCardCursor
	after := cursor.Encode("cur-1")
	_, err := uc.ListMasterCards(authedCtx("admin1"), MasterCardConnectionInput{
		MasterCardgroupID: "id-1",
		First:             intPtr(5),
		After:             &after,
		OrderBy:           &ob,
	})

	assertCancelled(t, err)
	require.Equal(t, context.Canceled, err, "expected unwrapped context.Canceled, got %v", err)
}

package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/rotisserie/eris"

	"backend/internal/auth"
	"backend/internal/cursor"
	"backend/internal/domain"
	"backend/internal/repository"
	"backend/internal/usecase/ucerr"
)

// mockMasterCatalogRepository is a manual test double for MasterCatalogRepository.
type mockMasterCatalogRepository struct {
	// FindPublishedPage (the search-aware total is carried by the page method)
	findPageResult []*repository.MasterCatalogItem
	findPageTotal  int64
	findPageErr    error
	findPageCalls  []findPublishedPageCall

	// FindByID (admin cursor resolution) and FindPublishedByID (published cursor
	// resolution + ImportMaster's published check) are backed by separate funcs so
	// a test can assert which scope a cursor was hydrated through. FindPublishedByID
	// falls back to findByIDFn when findPublishedByIDFn is nil, so existing tests
	// that only set findByIDFn keep working.
	findByIDFn          func(id string) (*domain.MasterCardgroup, error)
	findPublishedByIDFn func(id string) (*domain.MasterCardgroup, error)

	// admin methods (FindPageAnyStatus carries the status-unfiltered, search-aware total)
	findPageAnyStatus      []*repository.MasterCatalogItem
	findPageAnyStatusTotal int64
	findAdminErr           error
	findAdminCalls         []findPublishedPageCall
	countCardsRes          int64
	countCardsErr          error
	createCalls            []*domain.MasterCardgroup
	createErr              error
	updateFn               func(id string, patch repository.MasterCardgroupUpdate) (*domain.MasterCardgroup, error)
	deleteErr              error
	publishFn              func(id string) (*domain.MasterCardgroup, error)
	unpublishFn            func(id string) (*domain.MasterCardgroup, error)
}

type findPublishedPageCall struct {
	After   *repository.MasterCatalogCursor
	Before  *repository.MasterCatalogCursor
	First   int
	Last    int
	OrderBy repository.MasterCatalogOrderBy
	Dir     repository.SortOrder
	Search  *string
}

func (m *mockMasterCatalogRepository) FindPublishedPage(
	_ context.Context,
	after, before *repository.MasterCatalogCursor,
	first, last int,
	orderBy repository.MasterCatalogOrderBy,
	dir repository.SortOrder,
	search *string,
) ([]*repository.MasterCatalogItem, int64, error) {
	m.findPageCalls = append(m.findPageCalls, findPublishedPageCall{
		After: after, Before: before, First: first, Last: last,
		OrderBy: orderBy, Dir: dir, Search: search,
	})
	if m.findPageErr != nil {
		return nil, 0, m.findPageErr
	}
	return m.findPageResult, m.findPageTotal, nil
}

func (m *mockMasterCatalogRepository) FindPublishedByID(_ context.Context, id string) (*domain.MasterCardgroup, error) {
	if m.findPublishedByIDFn != nil {
		return m.findPublishedByIDFn(id)
	}
	if m.findByIDFn != nil {
		return m.findByIDFn(id)
	}
	return nil, repository.ErrNotFound
}

func (m *mockMasterCatalogRepository) FindByID(_ context.Context, id string) (*domain.MasterCardgroup, error) {
	if m.findByIDFn != nil {
		return m.findByIDFn(id)
	}
	return nil, repository.ErrNotFound
}

func (m *mockMasterCatalogRepository) FindPageAnyStatus(
	_ context.Context,
	after, before *repository.MasterCatalogCursor,
	first, last int,
	orderBy repository.MasterCatalogOrderBy,
	dir repository.SortOrder,
	search *string,
) ([]*repository.MasterCatalogItem, int64, error) {
	m.findAdminCalls = append(m.findAdminCalls, findPublishedPageCall{
		After: after, Before: before, First: first, Last: last,
		OrderBy: orderBy, Dir: dir, Search: search,
	})
	if m.findAdminErr != nil {
		return nil, 0, m.findAdminErr
	}
	return m.findPageAnyStatus, m.findPageAnyStatusTotal, nil
}

func (m *mockMasterCatalogRepository) CountCards(_ context.Context, _ string) (int64, error) {
	return m.countCardsRes, m.countCardsErr
}

func (m *mockMasterCatalogRepository) Create(_ context.Context, mc *domain.MasterCardgroup) error {
	if m.createErr != nil {
		return m.createErr
	}
	m.createCalls = append(m.createCalls, mc)
	return nil
}

func (m *mockMasterCatalogRepository) Update(_ context.Context, id string, patch repository.MasterCardgroupUpdate) (*domain.MasterCardgroup, error) {
	if m.updateFn != nil {
		return m.updateFn(id, patch)
	}
	return nil, repository.ErrNotFound
}

func (m *mockMasterCatalogRepository) Delete(_ context.Context, _ string) error { return m.deleteErr }

func (m *mockMasterCatalogRepository) Publish(_ context.Context, id string) (*domain.MasterCardgroup, error) {
	if m.publishFn != nil {
		return m.publishFn(id)
	}
	return nil, repository.ErrNotFound
}

func (m *mockMasterCatalogRepository) Unpublish(_ context.Context, id string) (*domain.MasterCardgroup, error) {
	if m.unpublishFn != nil {
		return m.unpublishFn(id)
	}
	return nil, repository.ErrNotFound
}

// catalogItem builds a MasterCatalogItem with the given id and published status.
func catalogItem(id string, cardCount int64) *repository.MasterCatalogItem {
	now := time.Now().UTC()
	return &repository.MasterCatalogItem{
		Cardgroup: &domain.MasterCardgroup{
			ID:        id,
			Name:      domain.CardgroupName("Deck " + id),
			Status:    domain.MasterStatusPublished,
			Version:   1,
			CreatedAt: now,
			UpdatedAt: now,
		},
		CardCount: cardCount,
	}
}

// ---------------------------------------------------------------------------
// Constructor
// ---------------------------------------------------------------------------

func TestNewMasterCatalogUsecase_NilRepoPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic on nil repo")
		}
	}()
	NewMasterCatalogUsecase(nil, &mockCopyMasterToUserUC{}, &stubCardgroupCounter{}, newTestAdminGate(true), newTestLogger())
}

func TestNewMasterCatalogUsecase_NilDeckUCPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic on nil deckUC")
		}
	}()
	NewMasterCatalogUsecase(&mockMasterCatalogRepository{}, nil, &stubCardgroupCounter{}, newTestAdminGate(true), newTestLogger())
}

func TestNewMasterCatalogUsecase_NilCounterPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic on nil cgCounter")
		}
	}()
	NewMasterCatalogUsecase(&mockMasterCatalogRepository{}, &mockCopyMasterToUserUC{}, nil, newTestAdminGate(true), newTestLogger())
}

func TestNewMasterCatalogUsecase_NilLoggerPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic on nil logger")
		}
	}()
	NewMasterCatalogUsecase(&mockMasterCatalogRepository{}, &mockCopyMasterToUserUC{}, &stubCardgroupCounter{}, newTestAdminGate(true), nil)
}

// ---------------------------------------------------------------------------
// Authentication gate
// ---------------------------------------------------------------------------

func TestListPublishedConnection_Unauthenticated(t *testing.T) {
	uc := NewMasterCatalogUsecase(&mockMasterCatalogRepository{}, &mockCopyMasterToUserUC{}, &stubCardgroupCounter{}, newTestAdminGate(true), newTestLogger())

	_, err := uc.ListPublishedConnection(context.Background(), MasterCatalogConnectionInput{})
	if !errors.Is(err, ucerr.ErrUnauthenticated) {
		t.Fatalf("want ErrUnauthenticated, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// Forward pagination + published-only semantics
// ---------------------------------------------------------------------------

func TestListPublishedConnection_Forward_TrimsExtraRow_SetsHasNext(t *testing.T) {
	repo := &mockMasterCatalogRepository{
		findPageTotal: 5,
		// first=2 → usecase asks for 3 (+1). Repo returns 3 → trim to 2, hasNext=true.
		findPageResult: []*repository.MasterCatalogItem{
			catalogItem("a", 10), catalogItem("b", 20), catalogItem("c", 30),
		},
	}
	uc := NewMasterCatalogUsecase(repo, &mockCopyMasterToUserUC{}, &stubCardgroupCounter{}, newTestAdminGate(true), newTestLogger())

	out, err := uc.ListPublishedConnection(authedCtx("u1"), MasterCatalogConnectionInput{
		First: intPtr(2),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out.Items) != 2 {
		t.Fatalf("want 2 items after trim, got %d", len(out.Items))
	}
	if !out.HasNext {
		t.Fatal("want HasNext=true")
	}
	if out.HasPrev {
		t.Fatal("want HasPrev=false (no after cursor)")
	}
	if out.TotalCount != 5 {
		t.Fatalf("want TotalCount=5, got %d", out.TotalCount)
	}
	if out.StartCur != "a" || out.EndCur != "b" {
		t.Fatalf("want start=a end=b, got start=%s end=%s", out.StartCur, out.EndCur)
	}
	if out.Items[0].CardCount != 10 {
		t.Fatalf("want cardCount 10 preserved, got %d", out.Items[0].CardCount)
	}

	// Verify the +1 fetch reached the repo with the default SORT_ORDER/ASC.
	if len(repo.findPageCalls) != 1 {
		t.Fatalf("want 1 FindPublishedPage call, got %d", len(repo.findPageCalls))
	}
	call := repo.findPageCalls[0]
	if call.First != 3 {
		t.Fatalf("want repo first=3 (+1 trick), got %d", call.First)
	}
	if call.OrderBy != repository.MasterCatalogOrderBySortOrder {
		t.Fatalf("want default orderBy sort_order, got %q", call.OrderBy)
	}
	if call.Dir != repository.SortAsc {
		t.Fatalf("want default dir ASC, got %q", call.Dir)
	}
}

func TestListPublishedConnection_Forward_NoExtraRow_NoNextPage(t *testing.T) {
	repo := &mockMasterCatalogRepository{
		findPageTotal:  2,
		findPageResult: []*repository.MasterCatalogItem{catalogItem("a", 1), catalogItem("b", 2)},
	}
	uc := NewMasterCatalogUsecase(repo, &mockCopyMasterToUserUC{}, &stubCardgroupCounter{}, newTestAdminGate(true), newTestLogger())

	out, err := uc.ListPublishedConnection(authedCtx("u1"), MasterCatalogConnectionInput{First: intPtr(5)})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out.Items) != 2 {
		t.Fatalf("want 2 items, got %d", len(out.Items))
	}
	if out.HasNext {
		t.Fatal("want HasNext=false when no extra row returned")
	}
}

// ---------------------------------------------------------------------------
// Backward pagination
// ---------------------------------------------------------------------------

func TestListPublishedConnection_Backward_TrimsLeadingRow_SetsHasPrev(t *testing.T) {
	// Cursor hydration requires a published row for `before`.
	cur := cursor.Encode("z")
	repo := &mockMasterCatalogRepository{
		findPageTotal: 10,
		findByIDFn: func(id string) (*domain.MasterCardgroup, error) {
			return catalogItem(id, 0).Cardgroup, nil
		},
		// last=2 → repo asked for 3; repo (already reversed) returns 3 → trim leading.
		findPageResult: []*repository.MasterCatalogItem{
			catalogItem("x", 1), catalogItem("y", 2), catalogItem("w", 3),
		},
	}
	uc := NewMasterCatalogUsecase(repo, &mockCopyMasterToUserUC{}, &stubCardgroupCounter{}, newTestAdminGate(true), newTestLogger())

	out, err := uc.ListPublishedConnection(authedCtx("u1"), MasterCatalogConnectionInput{
		Last:   intPtr(2),
		Before: &cur,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out.Items) != 2 {
		t.Fatalf("want 2 items after backward trim, got %d", len(out.Items))
	}
	if !out.HasPrev {
		t.Fatal("want HasPrev=true (extra row trimmed)")
	}
	if !out.HasNext {
		t.Fatal("want HasNext=true (before cursor present)")
	}
	// Leading row trimmed: keep the last 2 (y, w).
	if out.Items[0].Cardgroup.ID != "y" || out.Items[1].Cardgroup.ID != "w" {
		t.Fatalf("want [y w] after leading trim, got [%s %s]",
			out.Items[0].Cardgroup.ID, out.Items[1].Cardgroup.ID)
	}

	call := repo.findPageCalls[0]
	if call.Last != 3 {
		t.Fatalf("want repo last=3 (+1 trick), got %d", call.Last)
	}
	if call.Before == nil || call.Before.ID != "z" {
		t.Fatalf("want before cursor id=z hydrated, got %+v", call.Before)
	}
}

// ---------------------------------------------------------------------------
// Order resolution
// ---------------------------------------------------------------------------

func TestListPublishedConnection_OrderByName_Desc(t *testing.T) {
	repo := &mockMasterCatalogRepository{findPageTotal: 0, findPageResult: nil}
	uc := NewMasterCatalogUsecase(repo, &mockCopyMasterToUserUC{}, &stubCardgroupCounter{}, newTestAdminGate(true), newTestLogger())

	ob := MasterCatalogOrderByName
	dir := SortOrderDesc
	_, err := uc.ListPublishedConnection(authedCtx("u1"), MasterCatalogConnectionInput{
		First:          intPtr(3),
		OrderBy:        &ob,
		OrderDirection: &dir,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	call := repo.findPageCalls[0]
	if call.OrderBy != repository.MasterCatalogOrderByName {
		t.Fatalf("want repo orderBy name, got %q", call.OrderBy)
	}
	if call.Dir != repository.SortDesc {
		t.Fatalf("want repo dir DESC, got %q", call.Dir)
	}
}

// ---------------------------------------------------------------------------
// Page-size validation
// ---------------------------------------------------------------------------

func TestListPublishedConnection_FirstAndLast_Rejected(t *testing.T) {
	uc := NewMasterCatalogUsecase(&mockMasterCatalogRepository{}, &mockCopyMasterToUserUC{}, &stubCardgroupCounter{}, newTestAdminGate(true), newTestLogger())

	_, err := uc.ListPublishedConnection(authedCtx("u1"), MasterCatalogConnectionInput{
		First: intPtr(2),
		Last:  intPtr(2),
	})
	var ve *ucerr.ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("want ValidationError, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// totalCount computed before short-circuit
// ---------------------------------------------------------------------------

func TestListPublishedConnection_TotalCountOnlyRequest(t *testing.T) {
	repo := &mockMasterCatalogRepository{
		findPageTotal:  42,
		findPageResult: []*repository.MasterCatalogItem{},
	}
	uc := NewMasterCatalogUsecase(repo, &mockCopyMasterToUserUC{}, &stubCardgroupCounter{}, newTestAdminGate(true), newTestLogger())

	// first=0 → no rows fetched, but the page method still runs once: assemblePage
	// always calls the fetch closure, and findCatalogPage counts before its zero-page
	// short-circuit, so the search-aware total survives a totalCount-only request.
	out, err := uc.ListPublishedConnection(authedCtx("u1"), MasterCatalogConnectionInput{First: intPtr(0)})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.TotalCount != 42 {
		t.Fatalf("want TotalCount=42 even for empty page, got %d", out.TotalCount)
	}
	if len(repo.findPageCalls) != 1 {
		t.Fatalf("want FindPublishedPage invoked once, got %d", len(repo.findPageCalls))
	}
}

// ---------------------------------------------------------------------------
// Cursor errors
// ---------------------------------------------------------------------------

func TestListPublishedConnection_InvalidCursor(t *testing.T) {
	repo := &mockMasterCatalogRepository{findPageTotal: 0}
	uc := NewMasterCatalogUsecase(repo, &mockCopyMasterToUserUC{}, &stubCardgroupCounter{}, newTestAdminGate(true), newTestLogger())

	bad := "%%%not-a-cursor"
	_, err := uc.ListPublishedConnection(authedCtx("u1"), MasterCatalogConnectionInput{
		First: intPtr(2),
		After: &bad,
	})
	var ve *ucerr.ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("want ValidationError for bad cursor, got %v", err)
	}
	if ve.Field != "after" {
		t.Fatalf("want field=after, got %q", ve.Field)
	}
}

func TestListPublishedConnection_CursorNotPublished(t *testing.T) {
	// A draft (or absent) row is reported as ErrNotFound by FindPublishedByID,
	// so the cursor is rejected as cursor-not-found (draft never leaks).
	cur := cursor.Encode("draft-id")
	repo := &mockMasterCatalogRepository{
		findPageTotal: 0,
		findByIDFn: func(_ string) (*domain.MasterCardgroup, error) {
			return nil, repository.ErrNotFound
		},
	}
	uc := NewMasterCatalogUsecase(repo, &mockCopyMasterToUserUC{}, &stubCardgroupCounter{}, newTestAdminGate(true), newTestLogger())

	_, err := uc.ListPublishedConnection(authedCtx("u1"), MasterCatalogConnectionInput{
		First: intPtr(2),
		After: &cur,
	})
	var ve *ucerr.ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("want ValidationError for draft cursor, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// Repo error propagation
// ---------------------------------------------------------------------------

func TestListPublishedConnection_FindPageError_Wrapped(t *testing.T) {
	repo := &mockMasterCatalogRepository{findPageTotal: 1, findPageErr: eris.New("db down")}
	uc := NewMasterCatalogUsecase(repo, &mockCopyMasterToUserUC{}, &stubCardgroupCounter{}, newTestAdminGate(true), newTestLogger())

	_, err := uc.ListPublishedConnection(authedCtx("u1"), MasterCatalogConnectionInput{First: intPtr(2)})
	if err == nil {
		t.Fatal("want error from find-page failure")
	}
	// The page method now delivers both rows and total, so its failure is the only
	// repo error path. It must not be misclassified as unauthenticated.
	if errors.Is(err, ucerr.ErrUnauthenticated) {
		t.Fatal("find-page error must not surface as unauthenticated")
	}
}

// ---------------------------------------------------------------------------
// Search normalization at the usecase boundary
//
// nil and whitespace-only search both mean "no filter"; a non-empty search is
// trimmed. The normalized value must reach BOTH the count and the page query so
// totalCount and the page agree under an active filter, mirroring ListMasterCards.
// ---------------------------------------------------------------------------

func TestListPublishedConnection_SearchNormalizedToNil(t *testing.T) {
	t.Parallel()
	repo := &mockMasterCatalogRepository{}
	uc := NewMasterCatalogUsecase(repo, &mockCopyMasterToUserUC{}, &stubCardgroupCounter{}, newTestAdminGate(true), newTestLogger())

	blank := "   "
	_, err := uc.ListPublishedConnection(authedCtx("u1"), MasterCatalogConnectionInput{
		First:  intPtr(5),
		Search: &blank,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := repo.findPageCalls[0].Search; got != nil {
		t.Fatalf("whitespace-only search must normalize to nil for page, got %q", *got)
	}
}

func TestListPublishedConnection_SearchTrimmed(t *testing.T) {
	t.Parallel()
	repo := &mockMasterCatalogRepository{}
	uc := NewMasterCatalogUsecase(repo, &mockCopyMasterToUserUC{}, &stubCardgroupCounter{}, newTestAdminGate(true), newTestLogger())

	raw := "  hello  "
	_, err := uc.ListPublishedConnection(authedCtx("u1"), MasterCatalogConnectionInput{
		First:  intPtr(5),
		Search: &raw,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := repo.findPageCalls[0].Search; got == nil || *got != "hello" {
		t.Fatalf("page search must be trimmed to %q, got %v", "hello", got)
	}
}

func TestListAdminConnection_SearchNormalizedToNil(t *testing.T) {
	t.Parallel()
	repo := &mockMasterCatalogRepository{}
	uc := NewMasterCatalogUsecase(repo, &mockCopyMasterToUserUC{}, &stubCardgroupCounter{}, newTestAdminGate(true), newTestLogger())

	blank := "   "
	_, err := uc.ListAdminConnection(authedCtx("admin1"), MasterCatalogConnectionInput{
		First:  intPtr(5),
		Search: &blank,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := repo.findAdminCalls[0].Search; got != nil {
		t.Fatalf("whitespace-only search must normalize to nil for admin page, got %q", *got)
	}
}

func TestListAdminConnection_SearchTrimmed(t *testing.T) {
	t.Parallel()
	repo := &mockMasterCatalogRepository{}
	uc := NewMasterCatalogUsecase(repo, &mockCopyMasterToUserUC{}, &stubCardgroupCounter{}, newTestAdminGate(true), newTestLogger())

	raw := "  hello  "
	_, err := uc.ListAdminConnection(authedCtx("admin1"), MasterCatalogConnectionInput{
		First:  intPtr(5),
		Search: &raw,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := repo.findAdminCalls[0].Search; got == nil || *got != "hello" {
		t.Fatalf("admin page search must be trimmed to %q, got %v", "hello", got)
	}
}

// ---------------------------------------------------------------------------
// Cursor-resolution scope (guards the published-vs-admin split through the
// unified helper). Published cursors hydrate via FindPublishedByID (drafts are
// rejected as cursor-not-found); admin cursors hydrate via FindByID (drafts are
// valid). An inverted scope would swap these behaviors.
// ---------------------------------------------------------------------------

func TestListPublishedConnection_CursorRejectsDraftViaPublishedScope(t *testing.T) {
	t.Parallel()
	cur := cursor.Encode("draft-id")
	repo := &mockMasterCatalogRepository{
		// FindByID would resolve the draft (admin scope) — present to prove the
		// published path does NOT use it.
		findByIDFn: func(id string) (*domain.MasterCardgroup, error) {
			return &domain.MasterCardgroup{ID: id, Name: domain.CardgroupName("Draft"), Status: domain.MasterStatusDraft}, nil
		},
		// FindPublishedByID rejects the draft, which is the path the published list
		// must take.
		findPublishedByIDFn: func(string) (*domain.MasterCardgroup, error) {
			return nil, repository.ErrNotFound
		},
	}
	uc := NewMasterCatalogUsecase(repo, &mockCopyMasterToUserUC{}, &stubCardgroupCounter{}, newTestAdminGate(true), newTestLogger())

	_, err := uc.ListPublishedConnection(authedCtx("u1"), MasterCatalogConnectionInput{
		First: intPtr(2),
		After: &cur,
	})
	var ve *ucerr.ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("published cursor must hydrate via FindPublishedByID and reject a draft, got %v", err)
	}
	if ve.Field != "after" {
		t.Fatalf("want field=after, got %q", ve.Field)
	}
}

func TestListAdminConnection_CursorAcceptsDraftViaFindByID(t *testing.T) {
	t.Parallel()
	cur := cursor.Encode("draft-id")
	repo := &mockMasterCatalogRepository{
		findPageAnyStatusTotal: 1,
		findPageAnyStatus:      []*repository.MasterCatalogItem{catalogItem("x", 1)},
		// FindByID resolves the draft — the admin list includes DRAFT decks.
		findByIDFn: func(id string) (*domain.MasterCardgroup, error) {
			return &domain.MasterCardgroup{ID: id, Name: domain.CardgroupName("Draft"), Status: domain.MasterStatusDraft}, nil
		},
		// FindPublishedByID would reject the draft — present to prove the admin path
		// does NOT use it (an inverted scope would surface as cursor-not-found).
		findPublishedByIDFn: func(string) (*domain.MasterCardgroup, error) {
			return nil, repository.ErrNotFound
		},
	}
	uc := NewMasterCatalogUsecase(repo, &mockCopyMasterToUserUC{}, &stubCardgroupCounter{}, newTestAdminGate(true), newTestLogger())

	_, err := uc.ListAdminConnection(authedCtx("admin1"), MasterCatalogConnectionInput{
		Last:   intPtr(2),
		Before: &cur,
	})
	if err != nil {
		t.Fatalf("admin cursor must hydrate a draft via FindByID, got %v", err)
	}
	if repo.findAdminCalls[0].Before == nil || repo.findAdminCalls[0].Before.ID != "draft-id" {
		t.Fatalf("want before cursor id=draft-id hydrated, got %+v", repo.findAdminCalls[0].Before)
	}
}

// ---------------------------------------------------------------------------
// Order resolver direct unit coverage
// ---------------------------------------------------------------------------

func TestResolveMasterCatalogOrderBy_Defaults(t *testing.T) {
	field, dir, err := resolveMasterCatalogOrderBy(nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if field != repository.MasterCatalogOrderBySortOrder {
		t.Fatalf("want default sort_order, got %q", field)
	}
	if dir != repository.SortAsc {
		t.Fatalf("want default ASC, got %q", dir)
	}
}

func TestResolveMasterCatalogOrderBy_Invalid(t *testing.T) {
	bad := MasterCatalogOrderBy("BOGUS")
	_, _, err := resolveMasterCatalogOrderBy(&bad, nil)
	var ve *ucerr.ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("want ValidationError for bogus orderBy, got %v", err)
	}
}

func TestResolveMasterCatalogPageSize_ClampsAtMax(t *testing.T) {
	first, _, err := resolveStandardPageSize(intPtr(1000), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if first != maxPageSize {
		t.Fatalf("want clamp to %d, got %d", maxPageSize, first)
	}
}

func TestResolveMasterCatalogPageSize_DefaultWhenAbsent(t *testing.T) {
	first, last, err := resolveStandardPageSize(nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if first != defaultPageSize || last != 0 {
		t.Fatalf("want default first=%d last=0, got first=%d last=%d", defaultPageSize, first, last)
	}
}

// ---------------------------------------------------------------------------
// ImportMaster
// ---------------------------------------------------------------------------

// mockCopyMasterToUserUC stubs the masterDeckUsecaseFacade for ImportMaster,
// SeedDefaultStarters, and MergeMaster tests.
type mockCopyMasterToUserUC struct {
	fn          func(ctx context.Context, masterID, ownerID string) (*domain.Cardgroup, error)
	seedResult  []*domain.Cardgroup
	seedErr     error
	mergeResult *MergeMasterResult
	mergeErr    error
	mergeFn     func(ctx context.Context, masterID string, destCGID domain.CardgroupID, ownerID domain.UserID) (*MergeMasterResult, error)
	previewOut  PreviewMergeResult
	previewErr  error
}

func (m *mockCopyMasterToUserUC) CopyMasterToUser(ctx context.Context, masterID, ownerID string) (*domain.Cardgroup, error) {
	// Mirror mockMasterCatalogRepository.FindPublishedByID: a nil fn (the placeholder
	// passed on paths that never reach the copy) yields an attributed error rather
	// than an unattributed nil-function panic if a future test wires it incorrectly.
	if m.fn == nil {
		return nil, eris.New("mockCopyMasterToUserUC: fn not set")
	}
	return m.fn(ctx, masterID, ownerID)
}

func (m *mockCopyMasterToUserUC) SeedForNewUser(_ context.Context, _ string) ([]*domain.Cardgroup, error) {
	return m.seedResult, m.seedErr
}

func (m *mockCopyMasterToUserUC) MergeMasterIntoCardgroup(ctx context.Context, masterID string, destCGID domain.CardgroupID, ownerID domain.UserID) (*MergeMasterResult, error) {
	if m.mergeFn != nil {
		return m.mergeFn(ctx, masterID, destCGID, ownerID)
	}
	if m.mergeErr != nil {
		return nil, m.mergeErr
	}
	return m.mergeResult, nil
}

func (m *mockCopyMasterToUserUC) PreviewMergeMasterIntoCardgroup(_ context.Context, _ string, _ domain.CardgroupID, _ domain.UserID) (PreviewMergeResult, error) {
	if m.previewErr != nil {
		return PreviewMergeResult{}, m.previewErr
	}
	return m.previewOut, nil
}

func TestImportMaster_Unauthenticated(t *testing.T) {
	uc := NewMasterCatalogUsecase(&mockMasterCatalogRepository{}, &mockCopyMasterToUserUC{}, &stubCardgroupCounter{}, newTestAdminGate(true), newTestLogger())

	_, err := uc.ImportMaster(context.Background(), "m1")
	if !errors.Is(err, ucerr.ErrUnauthenticated) {
		t.Fatalf("expected ErrUnauthenticated, got %v", err)
	}
}

func TestImportMaster_UnknownOrDraft_ReturnsNotFoundOutcome(t *testing.T) {
	// Default mock FindPublishedByID returns repository.ErrNotFound (covers both
	// unknown id and draft — FindPublishedByID filters status='published').
	repo := &mockMasterCatalogRepository{}
	copyUC := &mockCopyMasterToUserUC{fn: func(context.Context, string, string) (*domain.Cardgroup, error) {
		t.Fatal("copy must not run when the master is not published")
		return nil, nil
	}}
	uc := NewMasterCatalogUsecase(repo, copyUC, &stubCardgroupCounter{}, newTestAdminGate(true), newTestLogger())

	out, err := uc.ImportMaster(authedCtx("u1"), "missing")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !out.NotFound {
		t.Fatal("expected NotFound outcome")
	}
	if out.Cardgroup != nil {
		t.Fatal("expected nil cardgroup on NotFound")
	}
}

func TestImportMaster_MasterUnpublishedMidFlight_ReturnsNotFoundOutcome(t *testing.T) {
	// TOCTOU: the master is published when the FindPublishedByID gate runs, but the
	// delegated copy's own in-tx published-scoped re-read surfaces repository.ErrNotFound
	// (an unpublish landed in between). ImportMaster must collapse that into the same
	// NotFound outcome as a pre-gate unknown/draft — not a silent import, not an error.
	repo := &mockMasterCatalogRepository{
		findPublishedByIDFn: func(id string) (*domain.MasterCardgroup, error) {
			return &domain.MasterCardgroup{ID: id, Name: domain.CardgroupName("Deck"), Status: domain.MasterStatusPublished}, nil
		},
	}
	copyUC := &mockCopyMasterToUserUC{fn: func(context.Context, string, string) (*domain.Cardgroup, error) {
		return nil, repository.ErrNotFound
	}}
	uc := NewMasterCatalogUsecase(repo, copyUC, &stubCardgroupCounter{}, newTestAdminGate(true), newTestLogger())

	out, err := uc.ImportMaster(authedCtx("u1"), "m1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !out.NotFound {
		t.Fatal("expected NotFound outcome for a master unpublished mid-flight")
	}
	if out.Cardgroup != nil {
		t.Fatal("expected nil cardgroup on NotFound")
	}
}

func TestImportMaster_Published_CopiesAndReturnsCardgroup(t *testing.T) {
	repo := &mockMasterCatalogRepository{
		findByIDFn: func(id string) (*domain.MasterCardgroup, error) {
			return &domain.MasterCardgroup{ID: id, Name: domain.CardgroupName("Deck"), Status: domain.MasterStatusPublished}, nil
		},
	}
	want := &domain.Cardgroup{ID: domain.CardgroupID("new-cg"), OwnerID: "u1", Name: domain.CardgroupName("Deck")}
	var gotMaster, gotOwner string
	copyUC := &mockCopyMasterToUserUC{fn: func(_ context.Context, masterID, ownerID string) (*domain.Cardgroup, error) {
		gotMaster, gotOwner = masterID, ownerID
		return want, nil
	}}
	uc := NewMasterCatalogUsecase(repo, copyUC, &stubCardgroupCounter{}, newTestAdminGate(true), newTestLogger())

	out, err := uc.ImportMaster(authedCtx("u1"), "m1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.NotFound {
		t.Fatal("did not expect NotFound")
	}
	if out.Cardgroup != want {
		t.Fatalf("expected the copied cardgroup, got %v", out.Cardgroup)
	}
	if gotMaster != "m1" || gotOwner != "u1" {
		t.Fatalf("copy called with (%q,%q), want (m1,u1)", gotMaster, gotOwner)
	}
}

// TestImportMaster_NonAdminAtLimit_ReturnsLimitOutcome pins the quota on the
// import path: a non-admin owner already holding domain.GeneralUserCardgroupLimit
// cardgroups gets the LimitReached outcome and the copy — the only thing that
// would create a cardgroup row — is never reached.
func TestImportMaster_NonAdminAtLimit_ReturnsLimitOutcome(t *testing.T) {
	repo := &mockMasterCatalogRepository{
		findPublishedByIDFn: func(id string) (*domain.MasterCardgroup, error) {
			return &domain.MasterCardgroup{ID: id, Name: domain.CardgroupName("Deck"), Status: domain.MasterStatusPublished}, nil
		},
	}
	copyUC := &mockCopyMasterToUserUC{fn: func(context.Context, string, string) (*domain.Cardgroup, error) {
		t.Fatal("copy must not run when the caller is at the cardgroup limit")
		return nil, nil
	}}
	counter := &stubCardgroupCounter{count: domain.GeneralUserCardgroupLimit}
	uc := NewMasterCatalogUsecase(repo, copyUC, counter, newTestAdminGate(false), newTestLogger())

	out, err := uc.ImportMaster(authedCtx("u1"), "m1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.LimitReached == nil {
		t.Fatal("expected LimitReached outcome")
	}
	if out.LimitReached.Limit != domain.GeneralUserCardgroupLimit || out.LimitReached.Current != domain.GeneralUserCardgroupLimit {
		t.Fatalf("limit info = %+v, want {Limit:%d Current:%d}", *out.LimitReached, domain.GeneralUserCardgroupLimit, domain.GeneralUserCardgroupLimit)
	}
	if out.Cardgroup != nil || out.NotFound {
		t.Fatalf("expected only LimitReached set, got %+v", out)
	}
}

// TestImportMaster_NonAdminAtLimit_UnknownMaster_ReturnsNotFound pins the gate
// ORDER, not just the two outcomes: the published gate runs before the quota, so
// a capped non-admin probing an unknown id receives the non-disclosure NotFound
// outcome rather than LimitReached. The counter.calls == 0 assertion is what
// makes a reordering fail loudly — swapping the two blocks would consult the
// quota first and answer LimitReached, turning the import path into an
// existence oracle for a caller who cannot import anything anyway.
func TestImportMaster_NonAdminAtLimit_UnknownMaster_ReturnsNotFound(t *testing.T) {
	// Default mock FindPublishedByID returns repository.ErrNotFound (unknown id).
	repo := &mockMasterCatalogRepository{}
	copyUC := &mockCopyMasterToUserUC{fn: func(context.Context, string, string) (*domain.Cardgroup, error) {
		t.Fatal("copy must not run for an unknown master")
		return nil, nil
	}}
	counter := &stubCardgroupCounter{count: domain.GeneralUserCardgroupLimit}
	uc := NewMasterCatalogUsecase(repo, copyUC, counter, newTestAdminGate(false), newTestLogger())

	out, err := uc.ImportMaster(authedCtx("u1"), "missing")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !out.NotFound {
		t.Fatalf("expected NotFound outcome, got %+v", out)
	}
	if out.LimitReached != nil {
		t.Fatalf("published gate must precede the quota, got LimitReached %+v", *out.LimitReached)
	}
	if out.Cardgroup != nil {
		t.Fatal("expected nil cardgroup on NotFound")
	}
	if counter.calls != 0 {
		t.Fatalf("quota must not be consulted for an unknown master, got %d calls", counter.calls)
	}
}

// TestImportMaster_AdminAtLimit_Succeeds pins the admin exemption: the same
// at-cap count that rejects a general user does not block an admin, and the
// counter is never consulted (checkCardgroupLimit short-circuits on IsAdmin).
func TestImportMaster_AdminAtLimit_Succeeds(t *testing.T) {
	repo := &mockMasterCatalogRepository{
		findPublishedByIDFn: func(id string) (*domain.MasterCardgroup, error) {
			return &domain.MasterCardgroup{ID: id, Name: domain.CardgroupName("Deck"), Status: domain.MasterStatusPublished}, nil
		},
	}
	want := &domain.Cardgroup{ID: domain.CardgroupID("new-cg"), OwnerID: "admin-1", Name: domain.CardgroupName("Deck")}
	copyUC := &mockCopyMasterToUserUC{fn: func(context.Context, string, string) (*domain.Cardgroup, error) {
		return want, nil
	}}
	counter := &stubCardgroupCounter{count: domain.GeneralUserCardgroupLimit}
	uc := NewMasterCatalogUsecase(repo, copyUC, counter, newTestAdminGate(true), newTestLogger())

	out, err := uc.ImportMaster(authedCtx("admin-1"), "m1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.LimitReached != nil {
		t.Fatalf("admin must be exempt from the cardgroup limit, got %+v", *out.LimitReached)
	}
	if out.Cardgroup != want {
		t.Fatalf("expected the copied cardgroup, got %v", out.Cardgroup)
	}
	if counter.calls != 0 {
		t.Fatalf("expected CountByOwner NOT to be called for an admin owner, got %d calls", counter.calls)
	}
}

// TestImportMaster_CountByOwnerError_Wrapped verifies that a non-context error
// from the quota's count propagates wrapped with the shared helper's prefix and
// that no copy is attempted.
func TestImportMaster_CountByOwnerError_Wrapped(t *testing.T) {
	repo := &mockMasterCatalogRepository{
		findPublishedByIDFn: func(id string) (*domain.MasterCardgroup, error) {
			return &domain.MasterCardgroup{ID: id, Name: domain.CardgroupName("Deck"), Status: domain.MasterStatusPublished}, nil
		},
	}
	copyUC := &mockCopyMasterToUserUC{fn: func(context.Context, string, string) (*domain.Cardgroup, error) {
		t.Fatal("copy must not run when the limit check fails")
		return nil, nil
	}}
	counter := &stubCardgroupCounter{err: eris.New("boom")}
	uc := NewMasterCatalogUsecase(repo, copyUC, counter, newTestAdminGate(false), newTestLogger())

	_, err := uc.ImportMaster(authedCtx("u1"), "m1")
	assertInternalChain(t, err, "usecase: cardgroup: count by owner")
}

func TestImportMaster_CopyError_Wrapped(t *testing.T) {
	repo := &mockMasterCatalogRepository{
		findByIDFn: func(id string) (*domain.MasterCardgroup, error) {
			return &domain.MasterCardgroup{ID: id, Name: domain.CardgroupName("Deck"), Status: domain.MasterStatusPublished}, nil
		},
	}
	copyUC := &mockCopyMasterToUserUC{fn: func(context.Context, string, string) (*domain.Cardgroup, error) {
		return nil, eris.New("boom")
	}}
	uc := NewMasterCatalogUsecase(repo, copyUC, &stubCardgroupCounter{}, newTestAdminGate(true), newTestLogger())

	_, err := uc.ImportMaster(authedCtx("u1"), "m1")
	assertInternalChain(t, err, "usecase: master catalog: import: copy master to user")
}

// TestImportMaster_DeletedOwner_ReturnsUnauthenticated pins the deleted-account
// import path. The copy writes the new cardgroup through cardgroupRepo.CreateTx,
// which classifies the owner-FK violation as ErrCardgroupOwnerNotFound; the eris
// wraps applied inside the master-deck usecase keep the sentinel reachable via
// errors.Is. ImportMaster must translate it to ucerr.ErrUnauthenticated so the
// caller is signed out instead of seeing an operator-paging INTERNAL error —
// matching what createCardgroup already does for the same deleted account.
func TestImportMaster_DeletedOwner_ReturnsUnauthenticated(t *testing.T) {
	repo := &mockMasterCatalogRepository{
		findByIDFn: func(id string) (*domain.MasterCardgroup, error) {
			return &domain.MasterCardgroup{ID: id, Name: domain.CardgroupName("Deck"), Status: domain.MasterStatusPublished}, nil
		},
	}
	copyUC := &mockCopyMasterToUserUC{fn: func(context.Context, string, string) (*domain.Cardgroup, error) {
		return nil, eris.Wrap(repository.ErrCardgroupOwnerNotFound, "usecase: master deck: copy master to user")
	}}
	uc := NewMasterCatalogUsecase(repo, copyUC, &stubCardgroupCounter{}, newTestAdminGate(true), newTestLogger())

	out, err := uc.ImportMaster(authedCtx("deleted-user"), "m1")

	assertUnauthenticated(t, err)
	// The deleted owner must not be laundered into the non-disclosure not-found
	// outcome reserved for an unpublished master.
	if out.NotFound {
		t.Fatal("deleted owner must not collapse into the NotFound outcome")
	}
}

// TestSeedDefaultStarters_DeletedOwner_ReturnsUnauthenticated covers the sibling
// onboarding path, which reaches the same cardgroupRepo.CreateTx write site.
func TestSeedDefaultStarters_DeletedOwner_ReturnsUnauthenticated(t *testing.T) {
	deck := &mockCopyMasterToUserUC{
		seedErr: eris.Wrap(repository.ErrCardgroupOwnerNotFound, "usecase: master deck: seed for new user"),
	}
	uc := NewMasterCatalogUsecase(&mockMasterCatalogRepository{}, deck, &stubCardgroupCounter{}, newTestAdminGate(true), newTestLogger())

	_, err := uc.SeedDefaultStarters(authedCtx("deleted-user"))

	assertUnauthenticated(t, err)
}

func TestImportMaster_VerifyPublishedError_Wrapped(t *testing.T) {
	// A non-ErrNotFound failure from FindPublishedByID (e.g. a DB outage) is an
	// internal error, not the errors-as-data NotFound outcome. The copy primitive
	// must not run when the published-check itself fails.
	repo := &mockMasterCatalogRepository{
		findByIDFn: func(string) (*domain.MasterCardgroup, error) {
			return nil, eris.New("db down")
		},
	}
	copyUC := &mockCopyMasterToUserUC{fn: func(context.Context, string, string) (*domain.Cardgroup, error) {
		t.Fatal("copy must not run when the published-check fails")
		return nil, nil
	}}
	uc := NewMasterCatalogUsecase(repo, copyUC, &stubCardgroupCounter{}, newTestAdminGate(true), newTestLogger())

	_, err := uc.ImportMaster(authedCtx("u1"), "m1")
	assertInternalChain(t, err, "usecase: master catalog: import: verify published")
}

func TestImportMaster_FindPublishedByID_ContextCancelled_PassesThrough(t *testing.T) {
	// context.Canceled from the published-check must propagate unwrapped so its
	// identity survives errors.Is at the resolver boundary (FromUsecaseError →
	// CANCELLED). An eris.Wrap here would break the == identity contract.
	repo := &mockMasterCatalogRepository{
		findByIDFn: func(string) (*domain.MasterCardgroup, error) {
			return nil, context.Canceled
		},
	}
	uc := NewMasterCatalogUsecase(repo, &mockCopyMasterToUserUC{}, &stubCardgroupCounter{}, newTestAdminGate(true), newTestLogger())

	_, err := uc.ImportMaster(authedCtx("u1"), "m1")
	assertCancelled(t, err)
	if err != context.Canceled {
		t.Fatalf("expected unwrapped context.Canceled, got %v", err)
	}
}

func TestImportMaster_CopyMasterToUser_ContextCancelled_PassesThrough(t *testing.T) {
	// context.Canceled surfaced by the copy primitive must propagate unwrapped.
	repo := &mockMasterCatalogRepository{
		findByIDFn: func(id string) (*domain.MasterCardgroup, error) {
			return &domain.MasterCardgroup{ID: id, Name: domain.CardgroupName("Deck"), Status: domain.MasterStatusPublished}, nil
		},
	}
	copyUC := &mockCopyMasterToUserUC{fn: func(context.Context, string, string) (*domain.Cardgroup, error) {
		return nil, context.Canceled
	}}
	uc := NewMasterCatalogUsecase(repo, copyUC, &stubCardgroupCounter{}, newTestAdminGate(true), newTestLogger())

	_, err := uc.ImportMaster(authedCtx("u1"), "m1")
	assertCancelled(t, err)
	if err != context.Canceled {
		t.Fatalf("expected unwrapped context.Canceled, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// FindPublishedMaster
// ---------------------------------------------------------------------------

func TestFindPublishedMaster_Unauthenticated(t *testing.T) {
	uc := NewMasterCatalogUsecase(&mockMasterCatalogRepository{}, &mockCopyMasterToUserUC{}, &stubCardgroupCounter{}, newTestAdminGate(true), newTestLogger())

	_, err := uc.FindPublishedMaster(context.Background(), "m1")
	if !errors.Is(err, ucerr.ErrUnauthenticated) {
		t.Fatalf("expected ErrUnauthenticated, got %v", err)
	}
}

// A DRAFT or unknown deck surfaces from FindPublishedByID as ErrNotFound, which
// FindPublishedMaster collapses to (nil, nil) so the resolver returns GraphQL
// null — draft existence is never disclosed (non-disclosure gate).
func TestFindPublishedMaster_DraftOrUnknown_ReturnsNil(t *testing.T) {
	repo := &mockMasterCatalogRepository{
		findPublishedByIDFn: func(string) (*domain.MasterCardgroup, error) { return nil, repository.ErrNotFound },
	}
	uc := NewMasterCatalogUsecase(repo, &mockCopyMasterToUserUC{}, &stubCardgroupCounter{}, newTestAdminGate(true), newTestLogger())

	got, err := uc.FindPublishedMaster(authedCtx("u1"), "missing")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != nil {
		t.Fatalf("expected nil deck for unknown/draft id, got %+v", got)
	}
}

func TestFindPublishedMaster_Success(t *testing.T) {
	want := &domain.MasterCardgroup{ID: "m1", Name: domain.CardgroupName("Deck"), Status: domain.MasterStatusPublished, Version: 1}
	repo := &mockMasterCatalogRepository{
		findPublishedByIDFn: func(id string) (*domain.MasterCardgroup, error) {
			if id != "m1" {
				t.Fatalf("FindPublishedByID called with %q, want m1", id)
			}
			return want, nil
		},
	}
	uc := NewMasterCatalogUsecase(repo, &mockCopyMasterToUserUC{}, &stubCardgroupCounter{}, newTestAdminGate(true), newTestLogger())

	got, err := uc.FindPublishedMaster(authedCtx("u1"), "m1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != want {
		t.Fatalf("expected the published deck, got %+v", got)
	}
}

// A non-ErrNotFound failure from FindPublishedByID (e.g. a DB outage) is an
// internal error wrapped into the eris chain, not the nil-data non-disclosure path.
func TestFindPublishedMaster_InfraError_Wrapped(t *testing.T) {
	repo := &mockMasterCatalogRepository{
		findPublishedByIDFn: func(string) (*domain.MasterCardgroup, error) { return nil, eris.New("db down") },
	}
	uc := NewMasterCatalogUsecase(repo, &mockCopyMasterToUserUC{}, &stubCardgroupCounter{}, newTestAdminGate(true), newTestLogger())

	_, err := uc.FindPublishedMaster(authedCtx("u1"), "m1")
	assertInternalChain(t, err, "usecase: master catalog: find published master")
}

func TestFindPublishedMaster_ContextCancelled_PassesThrough(t *testing.T) {
	// context.Canceled from the published-check must propagate unwrapped so its
	// identity survives errors.Is at the resolver boundary (FromUsecaseError →
	// CANCELLED). An eris.Wrap here would break the == identity contract.
	repo := &mockMasterCatalogRepository{
		findPublishedByIDFn: func(string) (*domain.MasterCardgroup, error) { return nil, context.Canceled },
	}
	uc := NewMasterCatalogUsecase(repo, &mockCopyMasterToUserUC{}, &stubCardgroupCounter{}, newTestAdminGate(true), newTestLogger())

	_, err := uc.FindPublishedMaster(authedCtx("u1"), "m1")
	assertCancelled(t, err)
	if err != context.Canceled {
		t.Fatalf("expected unwrapped context.Canceled, got %v", err)
	}
}

func TestListPublishedConnection_AfterWithLast_Rejected(t *testing.T) {
	uc := NewMasterCatalogUsecase(&mockMasterCatalogRepository{}, &mockCopyMasterToUserUC{}, &stubCardgroupCounter{}, newTestAdminGate(true), newTestLogger())

	after := "v1:abc"
	_, err := uc.ListPublishedConnection(authedCtx("u1"), MasterCatalogConnectionInput{
		Last:  intPtr(2),
		After: &after,
	})
	var ve *ucerr.ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("want ValidationError for after+last, got %v", err)
	}
}

func TestListAdminConnection_AfterWithLast_Rejected(t *testing.T) {
	uc := NewMasterCatalogUsecase(&mockMasterCatalogRepository{}, &mockCopyMasterToUserUC{}, &stubCardgroupCounter{}, newTestAdminGate(true), newTestLogger())

	after := "v1:abc"
	_, err := uc.ListAdminConnection(authedCtx("u1"), MasterCatalogConnectionInput{
		Last:  intPtr(2),
		After: &after,
	})
	var ve *ucerr.ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("want ValidationError for after+last, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// SeedDefaultStarters
// ---------------------------------------------------------------------------

func TestSeedDefaultStarters_Unauthenticated_ReturnsErrUnauthenticated(t *testing.T) {
	uc := NewMasterCatalogUsecase(&mockMasterCatalogRepository{}, &mockCopyMasterToUserUC{}, &stubCardgroupCounter{}, newTestAdminGate(true), newTestLogger())
	_, err := uc.SeedDefaultStarters(context.Background())
	if !errors.Is(err, ucerr.ErrUnauthenticated) {
		t.Fatalf("expected ErrUnauthenticated, got %v", err)
	}
}

func TestSeedDefaultStarters_HappyPath_ReturnsSeededCardgroups(t *testing.T) {
	want := []*domain.Cardgroup{{ID: domain.CardgroupID("cg-1"), OwnerID: "user-1", Name: domain.CardgroupName("Starter")}}
	deck := &mockCopyMasterToUserUC{seedResult: want}
	uc := NewMasterCatalogUsecase(&mockMasterCatalogRepository{}, deck, &stubCardgroupCounter{}, newTestAdminGate(true), newTestLogger())
	got, err := uc.SeedDefaultStarters(authedCtx("user-1"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0].ID != "cg-1" {
		t.Fatalf("expected seeded cardgroup cg-1, got %+v", got)
	}
}

func TestSeedDefaultStarters_NoDefaults_ReturnsEmpty(t *testing.T) {
	deck := &mockCopyMasterToUserUC{seedResult: nil}
	uc := NewMasterCatalogUsecase(&mockMasterCatalogRepository{}, deck, &stubCardgroupCounter{}, newTestAdminGate(true), newTestLogger())
	got, err := uc.SeedDefaultStarters(authedCtx("user-1"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected empty slice, got %+v", got)
	}
}

func TestSeedDefaultStarters_InfraError_WrapsChain(t *testing.T) {
	deck := &mockCopyMasterToUserUC{seedErr: eris.New("db down")}
	uc := NewMasterCatalogUsecase(&mockMasterCatalogRepository{}, deck, &stubCardgroupCounter{}, newTestAdminGate(true), newTestLogger())
	_, err := uc.SeedDefaultStarters(authedCtx("user-1"))
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	assertInternalChain(t, err, "usecase: master catalog: seed default starters")
}

func TestSeedDefaultStarters_ContextCancelled_PassesThrough(t *testing.T) {
	deck := &mockCopyMasterToUserUC{seedErr: context.Canceled}
	uc := NewMasterCatalogUsecase(&mockMasterCatalogRepository{}, deck, &stubCardgroupCounter{}, newTestAdminGate(true), newTestLogger())
	_, err := uc.SeedDefaultStarters(authedCtx("user-1"))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// MergeMaster
// ---------------------------------------------------------------------------

func TestMasterCatalogUsecase_MergeMaster_DraftMaster_NotFound(t *testing.T) {
	t.Parallel()
	// Default mockMasterCatalogRepository.FindPublishedByID returns ErrNotFound
	// when both findPublishedByIDFn and findByIDFn are nil — covers unknown id
	// and draft alike (non-disclosure gate).
	uc := NewMasterCatalogUsecase(
		&mockMasterCatalogRepository{},
		&mockCopyMasterToUserUC{},
		&stubCardgroupCounter{},
		newTestAdminGate(true),
		newTestLogger(),
	)
	out, err := uc.MergeMaster(authedCtx("u1"), "master-id", "cg-id")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !out.NotFound {
		t.Fatal("expected NotFound=true")
	}
	if out.Cardgroup != nil {
		t.Fatal("expected nil Cardgroup on NotFound outcome")
	}
}

func TestMasterCatalogUsecase_MergeMaster_Success_PassesCounts(t *testing.T) {
	t.Parallel()
	cg := &domain.Cardgroup{
		ID:      domain.CardgroupID("cg-id"),
		OwnerID: domain.UserID("owner"),
		Name:    domain.CardgroupName("My Deck"),
	}
	repo := &mockMasterCatalogRepository{
		findPublishedByIDFn: func(id string) (*domain.MasterCardgroup, error) {
			return &domain.MasterCardgroup{ID: id, Name: domain.CardgroupName("Master"), Status: domain.MasterStatusPublished}, nil
		},
	}
	deck := &mockCopyMasterToUserUC{
		mergeResult: &MergeMasterResult{Cardgroup: cg, Added: 5, Updated: 2},
	}
	uc := NewMasterCatalogUsecase(repo, deck, &stubCardgroupCounter{}, newTestAdminGate(true), newTestLogger())

	out, err := uc.MergeMaster(authedCtx("u1"), "master-id", "cg-id")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.NotFound {
		t.Fatal("expected NotFound=false on happy path")
	}
	if out.Added != 5 {
		t.Fatalf("want Added=5, got %d", out.Added)
	}
	if out.Updated != 2 {
		t.Fatalf("want Updated=2, got %d", out.Updated)
	}
	if out.Cardgroup == nil {
		t.Fatal("expected non-nil Cardgroup")
	}
	if string(out.Cardgroup.ID) != "cg-id" {
		t.Fatalf("want Cardgroup.ID=cg-id, got %q", out.Cardgroup.ID)
	}
}

func TestMasterCatalogUsecase_MergeMaster_Unauthenticated(t *testing.T) {
	t.Parallel()
	uc := NewMasterCatalogUsecase(
		&mockMasterCatalogRepository{},
		&mockCopyMasterToUserUC{},
		&stubCardgroupCounter{},
		newTestAdminGate(true),
		newTestLogger(),
	)
	// context.Background() carries no auth user.
	_, err := uc.MergeMaster(context.Background(), "master-id", "cg-id")
	if !errors.Is(err, ucerr.ErrUnauthenticated) {
		t.Fatalf("expected ErrUnauthenticated, got %v", err)
	}
}

// TestMasterCatalogUsecase_MergeMaster_WrappedValidationError_StillClassifies pins
// that a ucerr.ValidationError returned by the delegate survives eris.Wrap and is
// still classifiable via errors.As at the caller boundary. MergeMaster wraps the
// delegate error unconditionally, so this property must hold for gqlerr.FromUsecaseError
// to map it to BAD_USER_INPUT correctly.
func TestMasterCatalogUsecase_MergeMaster_WrappedValidationError_StillClassifies(t *testing.T) {
	t.Parallel()
	repo := &mockMasterCatalogRepository{
		findPublishedByIDFn: func(id string) (*domain.MasterCardgroup, error) {
			return &domain.MasterCardgroup{ID: id, Name: domain.CardgroupName("Master"), Status: domain.MasterStatusPublished}, nil
		},
	}
	deck := &mockCopyMasterToUserUC{
		mergeErr: ucerr.NewValidationError("cardgroupId", "cardgroup not found"),
	}
	uc := NewMasterCatalogUsecase(repo, deck, &stubCardgroupCounter{}, newTestAdminGate(true), newTestLogger())

	_, err := uc.MergeMaster(authedCtx("u1"), "master-id", "cg-id")
	if err == nil {
		t.Fatal("expected error from merge delegate")
	}
	var ve *ucerr.ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("ValidationError must survive eris.Wrap chain, got %T: %v", err, err)
	}
	if ve.Field != "cardgroupId" {
		t.Fatalf("want Field=cardgroupId, got %q", ve.Field)
	}
}

// TestMasterCatalogUsecase_MergeMaster_WrappedUnauthenticated_StillClassifies pins
// that ucerr.ErrUnauthenticated from the delegate survives eris.Wrap and is still
// classifiable via errors.Is at the caller boundary. MergeMaster wraps the delegate
// error unconditionally, so this property must hold for gqlerr.FromUsecaseError to
// map it to UNAUTHENTICATED correctly.
func TestMasterCatalogUsecase_MergeMaster_WrappedUnauthenticated_StillClassifies(t *testing.T) {
	t.Parallel()
	repo := &mockMasterCatalogRepository{
		findPublishedByIDFn: func(id string) (*domain.MasterCardgroup, error) {
			return &domain.MasterCardgroup{ID: id, Name: domain.CardgroupName("Master"), Status: domain.MasterStatusPublished}, nil
		},
	}
	deck := &mockCopyMasterToUserUC{
		mergeErr: ucerr.ErrUnauthenticated,
	}
	uc := NewMasterCatalogUsecase(repo, deck, &stubCardgroupCounter{}, newTestAdminGate(true), newTestLogger())

	_, err := uc.MergeMaster(authedCtx("u1"), "master-id", "cg-id")
	if !errors.Is(err, ucerr.ErrUnauthenticated) {
		t.Fatalf("ErrUnauthenticated must survive eris.Wrap chain, got %T: %v", err, err)
	}
}

func TestMasterCatalogUsecase_MergeMaster_VerifyPublishedError_Wrapped(t *testing.T) {
	// A non-ErrNotFound failure from FindPublishedByID (e.g. a DB outage) is an
	// internal error, not the errors-as-data NotFound outcome. The merge delegate
	// must not run when the published-check itself fails.
	t.Parallel()
	repo := &mockMasterCatalogRepository{
		findPublishedByIDFn: func(string) (*domain.MasterCardgroup, error) {
			return nil, eris.New("db down")
		},
	}
	deck := &mockCopyMasterToUserUC{mergeFn: func(context.Context, string, domain.CardgroupID, domain.UserID) (*MergeMasterResult, error) {
		t.Fatal("merge delegate must not run when the published-check fails")
		return nil, nil
	}}
	uc := NewMasterCatalogUsecase(repo, deck, &stubCardgroupCounter{}, newTestAdminGate(true), newTestLogger())

	_, err := uc.MergeMaster(authedCtx("u1"), "master-id", "cg-id")
	assertInternalChain(t, err, "usecase: master catalog: merge: verify published")
}

func TestMasterCatalogUsecase_MergeMaster_DelegateError_Wrapped(t *testing.T) {
	// A non-ucerr infra error from the merge delegate is wrapped into the eris chain
	// with the merge-step prefix, mirroring how ImportMaster wraps CopyMasterToUser.
	t.Parallel()
	repo := &mockMasterCatalogRepository{
		findPublishedByIDFn: func(id string) (*domain.MasterCardgroup, error) {
			return &domain.MasterCardgroup{ID: id, Name: domain.CardgroupName("Master"), Status: domain.MasterStatusPublished}, nil
		},
	}
	deck := &mockCopyMasterToUserUC{mergeErr: errors.New("db down")}
	uc := NewMasterCatalogUsecase(repo, deck, &stubCardgroupCounter{}, newTestAdminGate(true), newTestLogger())

	_, err := uc.MergeMaster(authedCtx("u1"), "master-id", "cg-id")
	assertInternalChain(t, err, "usecase: master catalog: merge: merge master into cardgroup")
}

func TestMasterCatalogUsecase_MergeMaster_MasterUnpublishedMidFlight_ReturnsNotFoundOutcome(t *testing.T) {
	// TOCTOU: the master is published at the FindPublishedByID gate, but the merge
	// delegate's in-tx published-scoped re-read surfaces repository.ErrNotFound. MergeMaster
	// must collapse that into MergeMasterOutcome{NotFound:true}, matching a pre-gate
	// unknown/draft — not a silent merge, not an error. A ucerr.ValidationError from the
	// destination ownership gate is unaffected (it is not repository.ErrNotFound).
	t.Parallel()
	repo := &mockMasterCatalogRepository{
		findPublishedByIDFn: func(id string) (*domain.MasterCardgroup, error) {
			return &domain.MasterCardgroup{ID: id, Name: domain.CardgroupName("Master"), Status: domain.MasterStatusPublished}, nil
		},
	}
	deck := &mockCopyMasterToUserUC{mergeErr: repository.ErrNotFound}
	uc := NewMasterCatalogUsecase(repo, deck, &stubCardgroupCounter{}, newTestAdminGate(true), newTestLogger())

	out, err := uc.MergeMaster(authedCtx("u1"), "master-id", "cg-id")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !out.NotFound {
		t.Fatal("expected NotFound outcome for a master unpublished mid-flight")
	}
	if out.Cardgroup != nil {
		t.Fatal("expected nil cardgroup on NotFound")
	}
}

// TestMasterCatalogUsecase_MergeMaster_DestVanishedAfterCommit_NotNotFoundOutcome
// wires the real master-deck usecase so the post-commit destination read-back is
// exercised end to end. The merge commits, then the destination cardgroup is
// deleted concurrently and FindByID yields repository.ErrNotFound. MergeMaster
// must surface an internal-class error rather than MergeMasterOutcome{NotFound:true},
// which would tell the learner the catalog deck does not exist and hide both the
// committed merge and the deleted destination.
func TestMasterCatalogUsecase_MergeMaster_DestVanishedAfterCommit_NotNotFoundOutcome(t *testing.T) {
	t.Parallel()
	const ownerID = "u1"
	const destID = "22222222-2222-7222-8222-222222222222"
	const masterID = "master-id"

	repo := &mockMasterCatalogRepository{
		findPublishedByIDFn: func(id string) (*domain.MasterCardgroup, error) {
			return &domain.MasterCardgroup{ID: id, Name: domain.CardgroupName("Master"), Status: domain.MasterStatusPublished}, nil
		},
	}
	deck := NewMasterDeckUsecaseWithTx(
		&fakeMasterCGRepo{byID: map[string]*domain.MasterCardgroup{masterID: masterCG(masterID, "Master")}},
		&fakeMasterCardRepo{byMaster: map[string][]*domain.MasterCard{
			masterID: {masterCard("mc-1", masterID, "alpha", "first", 0)},
		}},
		&fakeUserCardRepo{},
		&fakeUserCG{
			byID:              map[string]*domain.Cardgroup{destID: mustCardgroup(t, destID, ownerID, "My Deck")},
			findByIDErr:       repository.ErrNotFound,
			findByIDErrOnCall: 2, // call 1 = ownership gate (succeeds), call 2 = post-commit read (destination deleted)
		},
		stubTxRunner,
		newTestLogger(),
	)
	uc := NewMasterCatalogUsecase(repo, deck, &stubCardgroupCounter{}, newTestAdminGate(true), newTestLogger())

	out, err := uc.MergeMaster(authedCtx(ownerID), masterID, destID)
	if out.NotFound {
		t.Fatal("a vanished destination must not be reported as a missing master")
	}
	assertInternalChain(t, err, "usecase: master catalog: merge: merge master into cardgroup")
}

func TestMasterCatalogUsecase_MergeMaster_VerifyPublished_ContextCancelled_PassesThrough(t *testing.T) {
	// context.Canceled from the published-check must propagate unwrapped so its
	// identity survives errors.Is at the resolver boundary (FromUsecaseError →
	// CANCELLED). An eris.Wrap here would break the == identity contract.
	t.Parallel()
	repo := &mockMasterCatalogRepository{
		findPublishedByIDFn: func(string) (*domain.MasterCardgroup, error) {
			return nil, context.Canceled
		},
	}
	uc := NewMasterCatalogUsecase(repo, &mockCopyMasterToUserUC{}, &stubCardgroupCounter{}, newTestAdminGate(true), newTestLogger())

	_, err := uc.MergeMaster(authedCtx("u1"), "master-id", "cg-id")
	assertCancelled(t, err)
	if err != context.Canceled {
		t.Fatalf("expected unwrapped context.Canceled, got %v", err)
	}
}

func TestMasterCatalogUsecase_MergeMaster_Delegate_ContextCancelled_PassesThrough(t *testing.T) {
	// context.Canceled surfaced by the merge delegate must propagate unwrapped.
	t.Parallel()
	repo := &mockMasterCatalogRepository{
		findPublishedByIDFn: func(id string) (*domain.MasterCardgroup, error) {
			return &domain.MasterCardgroup{ID: id, Name: domain.CardgroupName("Master"), Status: domain.MasterStatusPublished}, nil
		},
	}
	deck := &mockCopyMasterToUserUC{mergeErr: context.Canceled}
	uc := NewMasterCatalogUsecase(repo, deck, &stubCardgroupCounter{}, newTestAdminGate(true), newTestLogger())

	_, err := uc.MergeMaster(authedCtx("u1"), "master-id", "cg-id")
	assertCancelled(t, err)
	if err != context.Canceled {
		t.Fatalf("expected unwrapped context.Canceled, got %v", err)
	}
}

// --- PreviewMergeMaster tests ------------------------------------------------

func TestPreviewMergeMaster_UnauthenticatedCaller(t *testing.T) {
	t.Parallel()
	uc := NewMasterCatalogUsecase(&mockMasterCatalogRepository{}, &mockCopyMasterToUserUC{}, &stubCardgroupCounter{}, newTestAdminGate(true), newTestLogger())

	_, err := uc.PreviewMergeMaster(context.Background(), "m1", "cg-1")
	if !errors.Is(err, ucerr.ErrUnauthenticated) {
		t.Fatalf("expected ErrUnauthenticated, got %v", err)
	}
}

// TestMasterCatalog_EmptySubCaller_RejectedAsUnauthenticated pins the requireCallerSub
// gate on the import/merge paths. A NON-NIL *auth.AuthUser whose Sub is empty must be
// rejected as unauthenticated rather than treated as an authenticated owner with id "".
// The prior `caller == nil` guard let this case through (the pointer is non-nil), which
// would have carried an empty ownership identity into the copy/merge delegates.
func TestMasterCatalog_EmptySubCaller_RejectedAsUnauthenticated(t *testing.T) {
	t.Parallel()
	// A non-nil authenticated user carrying an empty Sub — the exact case
	// requireCallerSub guards and the old nil-only check missed.
	ctx := auth.ContextWithUser(context.Background(), &auth.AuthUser{Sub: ""})
	uc := NewMasterCatalogUsecase(&mockMasterCatalogRepository{}, &mockCopyMasterToUserUC{}, &stubCardgroupCounter{}, newTestAdminGate(true), newTestLogger())

	t.Run("ImportMaster", func(t *testing.T) {
		t.Parallel()
		if _, err := uc.ImportMaster(ctx, "m1"); !errors.Is(err, ucerr.ErrUnauthenticated) {
			t.Fatalf("expected ErrUnauthenticated, got %v", err)
		}
	})
	t.Run("MergeMaster", func(t *testing.T) {
		t.Parallel()
		if _, err := uc.MergeMaster(ctx, "m1", "cg-1"); !errors.Is(err, ucerr.ErrUnauthenticated) {
			t.Fatalf("expected ErrUnauthenticated, got %v", err)
		}
	})
	t.Run("PreviewMergeMaster", func(t *testing.T) {
		t.Parallel()
		if _, err := uc.PreviewMergeMaster(ctx, "m1", "cg-1"); !errors.Is(err, ucerr.ErrUnauthenticated) {
			t.Fatalf("expected ErrUnauthenticated, got %v", err)
		}
	})
}

func TestPreviewMergeMaster_NotFoundWhenMasterUnpublished(t *testing.T) {
	// FindPublishedByID returning ErrNotFound (unknown or draft) must yield NotFound:true,
	// not an error — the non-disclosure gate collapses unknown + draft into one signal.
	t.Parallel()
	repo := &mockMasterCatalogRepository{} // default: returns repository.ErrNotFound
	uc := NewMasterCatalogUsecase(repo, &mockCopyMasterToUserUC{}, &stubCardgroupCounter{}, newTestAdminGate(true), newTestLogger())

	out, err := uc.PreviewMergeMaster(authedCtx("u1"), "m1", "cg-1")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !out.NotFound {
		t.Fatalf("expected NotFound=true for unknown/draft master")
	}
}

func TestPreviewMergeMaster_HappyPathDelegatesToDeckUC(t *testing.T) {
	// A published master: PreviewMergeMaster must delegate to the deck usecase and
	// pass its Added/Updated tallies straight through.
	t.Parallel()
	repo := &mockMasterCatalogRepository{
		findPublishedByIDFn: func(id string) (*domain.MasterCardgroup, error) {
			return &domain.MasterCardgroup{ID: id, Name: domain.CardgroupName("Starter"), Status: domain.MasterStatusPublished}, nil
		},
	}
	deck := &mockCopyMasterToUserUC{
		previewOut: PreviewMergeResult{Added: 3, Updated: 1},
	}
	uc := NewMasterCatalogUsecase(repo, deck, &stubCardgroupCounter{}, newTestAdminGate(true), newTestLogger())

	out, err := uc.PreviewMergeMaster(authedCtx("u1"), "master-id", "cg-id")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.NotFound {
		t.Fatal("expected NotFound=false for a published master")
	}
	if out.Added != 3 || out.Updated != 1 {
		t.Fatalf("expected Added=3 Updated=1, got Added=%d Updated=%d", out.Added, out.Updated)
	}
}

// TestListPublishedConnection_CursorFindByIDCancelled pins the cursor-hydration
// fetchByID wrap site (resolveMasterCatalogCursor): a context.Canceled surfaced
// while hydrating the cursor must reach the caller unwrapped so the resolver
// routes it to Cancelled via errors.Is. assertCancelled alone is too weak —
// errors.Is walks the eris chain, so it passes even for
// eris.Wrap(context.Canceled, ...); the bare err == context.Canceled check pins
// that no wrap snuck in. See
// docs/backend/error-wrapping/pin-unwrapped-context-error-with-identity-check.md.
func TestListPublishedConnection_CursorFindByIDCancelled(t *testing.T) {
	t.Parallel()
	cur := cursor.Encode("cur-1")
	repo := &mockMasterCatalogRepository{
		findByIDFn: func(string) (*domain.MasterCardgroup, error) { return nil, context.Canceled },
	}
	uc := NewMasterCatalogUsecase(repo, &mockCopyMasterToUserUC{}, &stubCardgroupCounter{}, newTestAdminGate(true), newTestLogger())

	_, err := uc.ListPublishedConnection(authedCtx("u1"), MasterCatalogConnectionInput{
		First: intPtr(5),
		After: &cur,
	})

	assertCancelled(t, err)
	if err != context.Canceled {
		t.Fatalf("expected unwrapped context.Canceled, got %v", err)
	}
}

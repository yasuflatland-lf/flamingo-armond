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
	After  *repository.MasterCatalogCursor
	First  int
	Search *string
}

func (m *mockMasterCatalogRepository) FindPublishedPage(
	_ context.Context,
	after *repository.MasterCatalogCursor,
	first int,
	search *string,
) ([]*repository.MasterCatalogItem, int64, error) {
	m.findPageCalls = append(m.findPageCalls, findPublishedPageCall{
		After: after, First: first, Search: search,
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
	after *repository.MasterCatalogCursor,
	first int,
	search *string,
) ([]*repository.MasterCatalogItem, int64, error) {
	m.findAdminCalls = append(m.findAdminCalls, findPublishedPageCall{
		After: after, First: first, Search: search,
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
	NewMasterCatalogUsecase(nil, &mockCopyMasterToUserUC{}, newTestAdminGate(true), newTestLogger())
}

func TestNewMasterCatalogUsecase_NilDeckUCPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic on nil deckUC")
		}
	}()
	NewMasterCatalogUsecase(&mockMasterCatalogRepository{}, nil, newTestAdminGate(true), newTestLogger())
}

func TestNewMasterCatalogUsecase_NilLoggerPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic on nil logger")
		}
	}()
	NewMasterCatalogUsecase(&mockMasterCatalogRepository{}, &mockCopyMasterToUserUC{}, newTestAdminGate(true), nil)
}

// ---------------------------------------------------------------------------
// Authentication gate
// ---------------------------------------------------------------------------

func TestListPublishedConnection_Unauthenticated(t *testing.T) {
	uc := NewMasterCatalogUsecase(&mockMasterCatalogRepository{}, &mockCopyMasterToUserUC{}, newTestAdminGate(true), newTestLogger())

	_, err := uc.ListPublishedConnection(context.Background(), MasterCatalogConnectionInput{})
	if !errors.Is(err, ucerr.ErrUnauthenticated) {
		t.Fatalf("want ErrUnauthenticated, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// Forward pagination + catalog-visibility semantics
// ---------------------------------------------------------------------------

func TestListPublishedConnection_Forward_TrimsExtraRow_SetsHasNext(t *testing.T) {
	repo := &mockMasterCatalogRepository{
		findPageTotal: 5,
		// first=2 → usecase asks for 3 (+1). Repo returns 3 → trim to 2, hasNext=true.
		findPageResult: []*repository.MasterCatalogItem{
			catalogItem("a", 10), catalogItem("b", 20), catalogItem("c", 30),
		},
	}
	uc := NewMasterCatalogUsecase(repo, &mockCopyMasterToUserUC{}, newTestAdminGate(true), newTestLogger())

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

	// Verify the +1 fetch reached the repo under the fixed (sort_order, ASC).
	if len(repo.findPageCalls) != 1 {
		t.Fatalf("want 1 FindPublishedPage call, got %d", len(repo.findPageCalls))
	}
	call := repo.findPageCalls[0]
	if call.First != 3 {
		t.Fatalf("want repo first=3 (+1 trick), got %d", call.First)
	}
	if want := (PageOrdering{OrderBy: "sort_order", Direction: "ASC"}); out.Ordering != want {
		t.Fatalf("want ordering %+v, got %+v", want, out.Ordering)
	}
}

func TestListPublishedConnection_Forward_NoExtraRow_NoNextPage(t *testing.T) {
	repo := &mockMasterCatalogRepository{
		findPageTotal:  2,
		findPageResult: []*repository.MasterCatalogItem{catalogItem("a", 1), catalogItem("b", 2)},
	}
	uc := NewMasterCatalogUsecase(repo, &mockCopyMasterToUserUC{}, newTestAdminGate(true), newTestLogger())

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
// Relay argument guard
// ---------------------------------------------------------------------------

// TestListPublishedConnection_AfterWithoutFirst pins the resolveRelayPage wiring
// in listMasterCatalogCore: a resolvable after cursor with no first is rejected
// with BAD_USER_INPUT on "after" and the page query never runs.
func TestListPublishedConnection_AfterWithoutFirst(t *testing.T) {
	t.Parallel()
	repo := &mockMasterCatalogRepository{
		findByIDFn: func(id string) (*domain.MasterCardgroup, error) {
			return &domain.MasterCardgroup{ID: id, Name: domain.CardgroupName("Pub"), Status: domain.MasterStatusPublished}, nil
		},
	}
	uc := NewMasterCatalogUsecase(repo, &mockCopyMasterToUserUC{}, newTestAdminGate(true), newTestLogger())

	after := cursor.Encode("m1")
	_, err := uc.ListPublishedConnection(authedCtx("u1"), MasterCatalogConnectionInput{After: &after})
	assertValidationError(t, err, "after", "after requires first")
	if len(repo.findPageCalls) != 0 {
		t.Fatalf("page query must not run, got %d calls", len(repo.findPageCalls))
	}
}

// TestListAdminConnection_AfterWithoutFirst is the admin-surface twin of
// TestListPublishedConnection_AfterWithoutFirst (a DRAFT cursor is valid here).
func TestListAdminConnection_AfterWithoutFirst(t *testing.T) {
	t.Parallel()
	repo := &mockMasterCatalogRepository{
		findByIDFn: func(id string) (*domain.MasterCardgroup, error) {
			return &domain.MasterCardgroup{ID: id, Name: domain.CardgroupName("Draft"), Status: domain.MasterStatusDraft}, nil
		},
	}
	uc := NewMasterCatalogUsecase(repo, &mockCopyMasterToUserUC{}, newTestAdminGate(true), newTestLogger())

	after := cursor.Encode("m1")
	_, err := uc.ListAdminConnection(authedCtx("admin1"), MasterCatalogConnectionInput{After: &after})
	assertValidationError(t, err, "after", "after requires first")
	if len(repo.findAdminCalls) != 0 {
		t.Fatalf("page query must not run, got %d calls", len(repo.findAdminCalls))
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
	uc := NewMasterCatalogUsecase(repo, &mockCopyMasterToUserUC{}, newTestAdminGate(true), newTestLogger())

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
	uc := NewMasterCatalogUsecase(repo, &mockCopyMasterToUserUC{}, newTestAdminGate(true), newTestLogger())

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
	uc := NewMasterCatalogUsecase(repo, &mockCopyMasterToUserUC{}, newTestAdminGate(true), newTestLogger())

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
	uc := NewMasterCatalogUsecase(repo, &mockCopyMasterToUserUC{}, newTestAdminGate(true), newTestLogger())

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
	uc := NewMasterCatalogUsecase(repo, &mockCopyMasterToUserUC{}, newTestAdminGate(true), newTestLogger())

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
	uc := NewMasterCatalogUsecase(repo, &mockCopyMasterToUserUC{}, newTestAdminGate(true), newTestLogger())

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
	uc := NewMasterCatalogUsecase(repo, &mockCopyMasterToUserUC{}, newTestAdminGate(true), newTestLogger())

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
	uc := NewMasterCatalogUsecase(repo, &mockCopyMasterToUserUC{}, newTestAdminGate(true), newTestLogger())

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
	uc := NewMasterCatalogUsecase(repo, &mockCopyMasterToUserUC{}, newTestAdminGate(true), newTestLogger())

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
	uc := NewMasterCatalogUsecase(repo, &mockCopyMasterToUserUC{}, newTestAdminGate(true), newTestLogger())

	_, err := uc.ListAdminConnection(authedCtx("admin1"), MasterCatalogConnectionInput{
		First: intPtr(2),
		After: &cur,
	})
	if err != nil {
		t.Fatalf("admin cursor must hydrate a draft via FindByID, got %v", err)
	}
	if repo.findAdminCalls[0].After == nil || repo.findAdminCalls[0].After.ID != "draft-id" {
		t.Fatalf("want after cursor id=draft-id hydrated, got %+v", repo.findAdminCalls[0].After)
	}
}

// ---------------------------------------------------------------------------
// Fixed ordering
// ---------------------------------------------------------------------------

// TestMasterCatalog_FixedOrdering_SortOrderAsc verifies both catalog
// connections serve every page under the fixed (sort_order, ASC) ordering.
func TestMasterCatalog_FixedOrdering_SortOrderAsc(t *testing.T) {
	want := PageOrdering{OrderBy: "sort_order", Direction: "ASC"}
	uc := NewMasterCatalogUsecase(&mockMasterCatalogRepository{}, &mockCopyMasterToUserUC{}, newTestAdminGate(true), newTestLogger())

	pub, err := uc.ListPublishedConnection(authedCtx("u1"), MasterCatalogConnectionInput{First: intPtr(3)})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pub.Ordering != want {
		t.Fatalf("published ordering: want %+v, got %+v", want, pub.Ordering)
	}

	admin, err := uc.ListAdminConnection(authedCtx("admin1"), MasterCatalogConnectionInput{First: intPtr(3)})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if admin.Ordering != want {
		t.Fatalf("admin ordering: want %+v, got %+v", want, admin.Ordering)
	}
}

// ---------------------------------------------------------------------------
// ImportMaster
// ---------------------------------------------------------------------------

// mockCopyMasterToUserUC stubs the masterDeckUsecaseFacade for ImportMaster,
// SeedDefaultStarters, and MergeMaster tests.
type mockCopyMasterToUserUC struct {
	fn          func(ctx context.Context, masterID, ownerID string, enforceQuota bool) (CopyMasterToUserResult, error)
	seedResult  []*domain.Cardgroup
	seedErr     error
	mergeResult *MergeMasterResult
	mergeErr    error
	mergeFn     func(ctx context.Context, masterID string, destCGID domain.CardgroupID, ownerID domain.UserID) (*MergeMasterResult, error)
	previewOut  PreviewMergeResult
	previewErr  error
}

func (m *mockCopyMasterToUserUC) CopyMasterToUser(ctx context.Context, masterID, ownerID string, enforceQuota bool) (CopyMasterToUserResult, error) {
	// Mirror mockMasterCatalogRepository.FindPublishedByID: a nil fn (the placeholder
	// passed on paths that never reach the copy) yields an attributed error rather
	// than an unattributed nil-function panic if a future test wires it incorrectly.
	if m.fn == nil {
		return CopyMasterToUserResult{}, eris.New("mockCopyMasterToUserUC: fn not set")
	}
	return m.fn(ctx, masterID, ownerID, enforceQuota)
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
	uc := NewMasterCatalogUsecase(&mockMasterCatalogRepository{}, &mockCopyMasterToUserUC{}, newTestAdminGate(true), newTestLogger())

	_, err := uc.ImportMaster(context.Background(), "m1")
	if !errors.Is(err, ucerr.ErrUnauthenticated) {
		t.Fatalf("expected ErrUnauthenticated, got %v", err)
	}
}

func TestImportMaster_UnknownOrDraft_ReturnsNotFoundOutcome(t *testing.T) {
	// Default mock FindPublishedByID returns repository.ErrNotFound (covers both
	// unknown id and draft — FindPublishedByID filters status='published').
	repo := &mockMasterCatalogRepository{}
	copyUC := &mockCopyMasterToUserUC{fn: func(context.Context, string, string, bool) (CopyMasterToUserResult, error) {
		t.Fatal("copy must not run when the master is not published")
		return CopyMasterToUserResult{}, nil
	}}
	uc := NewMasterCatalogUsecase(repo, copyUC, newTestAdminGate(true), newTestLogger())

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
	// delegated copy's own published-scoped re-read surfaces repository.ErrNotFound
	// (an unpublish landed in between). ImportMaster must collapse that into the same
	// NotFound outcome as a pre-gate unknown/draft — not a silent import, not an error.
	repo := &mockMasterCatalogRepository{
		findPublishedByIDFn: func(id string) (*domain.MasterCardgroup, error) {
			return &domain.MasterCardgroup{ID: id, Name: domain.CardgroupName("Deck"), Status: domain.MasterStatusPublished}, nil
		},
	}
	copyUC := &mockCopyMasterToUserUC{fn: func(context.Context, string, string, bool) (CopyMasterToUserResult, error) {
		return CopyMasterToUserResult{}, repository.ErrNotFound
	}}
	uc := NewMasterCatalogUsecase(repo, copyUC, newTestAdminGate(true), newTestLogger())

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
	copyUC := &mockCopyMasterToUserUC{fn: func(_ context.Context, masterID, ownerID string, _ bool) (CopyMasterToUserResult, error) {
		gotMaster, gotOwner = masterID, ownerID
		return CopyMasterToUserResult{Cardgroup: want}, nil
	}}
	uc := NewMasterCatalogUsecase(repo, copyUC, newTestAdminGate(true), newTestLogger())

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
// import path: a non-admin caller asks the copy to enforce the quota, and a
// LimitReached result from the copy transaction surfaces as the LimitReached
// outcome with no cardgroup.
func TestImportMaster_NonAdminAtLimit_ReturnsLimitOutcome(t *testing.T) {
	repo := &mockMasterCatalogRepository{
		findPublishedByIDFn: func(id string) (*domain.MasterCardgroup, error) {
			return &domain.MasterCardgroup{ID: id, Name: domain.CardgroupName("Deck"), Status: domain.MasterStatusPublished}, nil
		},
	}
	want := &CardgroupLimitInfo{Limit: domain.GeneralUserCardgroupLimit, Current: domain.GeneralUserCardgroupLimit}
	var gotEnforce bool
	copyUC := &mockCopyMasterToUserUC{fn: func(_ context.Context, _, _ string, enforceQuota bool) (CopyMasterToUserResult, error) {
		gotEnforce = enforceQuota
		return CopyMasterToUserResult{LimitReached: want}, nil
	}}
	uc := NewMasterCatalogUsecase(repo, copyUC, newTestAdminGate(false), newTestLogger())

	out, err := uc.ImportMaster(authedCtx("u1"), "m1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !gotEnforce {
		t.Fatal("a non-admin import must ask the copy to enforce the quota")
	}
	if out.LimitReached == nil || *out.LimitReached != *want {
		t.Fatalf("limit info = %+v, want %+v", out.LimitReached, *want)
	}
	if out.Cardgroup != nil || out.NotFound {
		t.Fatalf("expected only LimitReached set, got %+v", out)
	}
}

// TestImportMaster_NonAdminAtLimit_UnknownMaster_ReturnsNotFound pins the gate
// ORDER, not just the two outcomes: the published gate runs before the quota, so
// a capped non-admin probing an unknown id receives the non-disclosure NotFound
// outcome rather than LimitReached. The copy (which now owns the quota) must
// never run — reaching it first would answer LimitReached and turn the import
// path into an existence oracle for a caller who cannot import anything anyway.
func TestImportMaster_NonAdminAtLimit_UnknownMaster_ReturnsNotFound(t *testing.T) {
	// Default mock FindPublishedByID returns repository.ErrNotFound (unknown id).
	repo := &mockMasterCatalogRepository{}
	copyUC := &mockCopyMasterToUserUC{fn: func(context.Context, string, string, bool) (CopyMasterToUserResult, error) {
		t.Fatal("copy (and therefore the quota) must not run for an unknown master")
		return CopyMasterToUserResult{}, nil
	}}
	uc := NewMasterCatalogUsecase(repo, copyUC, newTestAdminGate(false), newTestLogger())

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
}

// TestImportMaster_AdminAtLimit_Succeeds pins the admin exemption: an admin's
// import asks the copy NOT to enforce the quota, so the copy runs and its
// cardgroup is returned.
func TestImportMaster_AdminAtLimit_Succeeds(t *testing.T) {
	repo := &mockMasterCatalogRepository{
		findPublishedByIDFn: func(id string) (*domain.MasterCardgroup, error) {
			return &domain.MasterCardgroup{ID: id, Name: domain.CardgroupName("Deck"), Status: domain.MasterStatusPublished}, nil
		},
	}
	want := &domain.Cardgroup{ID: domain.CardgroupID("new-cg"), OwnerID: "admin-1", Name: domain.CardgroupName("Deck")}
	gotEnforce := true
	copyUC := &mockCopyMasterToUserUC{fn: func(_ context.Context, _, _ string, enforceQuota bool) (CopyMasterToUserResult, error) {
		gotEnforce = enforceQuota
		return CopyMasterToUserResult{Cardgroup: want}, nil
	}}
	uc := NewMasterCatalogUsecase(repo, copyUC, newTestAdminGate(true), newTestLogger())

	out, err := uc.ImportMaster(authedCtx("admin-1"), "m1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotEnforce {
		t.Fatal("an admin import must not ask the copy to enforce the quota")
	}
	if out.LimitReached != nil {
		t.Fatalf("admin must be exempt from the cardgroup limit, got %+v", *out.LimitReached)
	}
	if out.Cardgroup != want {
		t.Fatalf("expected the copied cardgroup, got %v", out.Cardgroup)
	}
}

// TestImportMaster_IsAdminError_Wrapped verifies that a non-context error from
// the admin check propagates wrapped with the import prefix and that no copy is
// attempted.
func TestImportMaster_IsAdminError_Wrapped(t *testing.T) {
	repo := &mockMasterCatalogRepository{
		findPublishedByIDFn: func(id string) (*domain.MasterCardgroup, error) {
			return &domain.MasterCardgroup{ID: id, Name: domain.CardgroupName("Deck"), Status: domain.MasterStatusPublished}, nil
		},
	}
	copyUC := &mockCopyMasterToUserUC{fn: func(context.Context, string, string, bool) (CopyMasterToUserResult, error) {
		t.Fatal("copy must not run when the admin check fails")
		return CopyMasterToUserResult{}, nil
	}}
	uc := NewMasterCatalogUsecase(repo, copyUC, NewAdminGate(&mockAdminChecker{err: errors.New("auth down")}), newTestLogger())

	_, err := uc.ImportMaster(authedCtx("u1"), "m1")
	assertInternalChain(t, err, "usecase: master catalog: import: check admin")
}

// TestImportMaster_IsAdminCancelled_IdentityPreserved pins that a cancelled
// admin check propagates as the bare context.Canceled (identity, not errors.Is)
// and that no copy is attempted.
func TestImportMaster_IsAdminCancelled_IdentityPreserved(t *testing.T) {
	repo := &mockMasterCatalogRepository{
		findPublishedByIDFn: func(id string) (*domain.MasterCardgroup, error) {
			return &domain.MasterCardgroup{ID: id, Name: domain.CardgroupName("Deck"), Status: domain.MasterStatusPublished}, nil
		},
	}
	copyUC := &mockCopyMasterToUserUC{fn: func(context.Context, string, string, bool) (CopyMasterToUserResult, error) {
		t.Fatal("copy must not run when the admin check is cancelled")
		return CopyMasterToUserResult{}, nil
	}}
	uc := NewMasterCatalogUsecase(repo, copyUC, NewAdminGate(&mockAdminChecker{err: context.Canceled}), newTestLogger())

	_, err := uc.ImportMaster(authedCtx("u1"), "m1")
	if err != context.Canceled {
		t.Fatalf("expected unwrapped context.Canceled, got %v", err)
	}
}

func TestImportMaster_CopyError_Wrapped(t *testing.T) {
	repo := &mockMasterCatalogRepository{
		findByIDFn: func(id string) (*domain.MasterCardgroup, error) {
			return &domain.MasterCardgroup{ID: id, Name: domain.CardgroupName("Deck"), Status: domain.MasterStatusPublished}, nil
		},
	}
	copyUC := &mockCopyMasterToUserUC{fn: func(context.Context, string, string, bool) (CopyMasterToUserResult, error) {
		return CopyMasterToUserResult{}, eris.New("boom")
	}}
	uc := NewMasterCatalogUsecase(repo, copyUC, newTestAdminGate(true), newTestLogger())

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
	copyUC := &mockCopyMasterToUserUC{fn: func(context.Context, string, string, bool) (CopyMasterToUserResult, error) {
		return CopyMasterToUserResult{}, eris.Wrap(repository.ErrCardgroupOwnerNotFound, "usecase: master deck: copy master to user")
	}}
	uc := NewMasterCatalogUsecase(repo, copyUC, newTestAdminGate(true), newTestLogger())

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
	uc := NewMasterCatalogUsecase(&mockMasterCatalogRepository{}, deck, newTestAdminGate(true), newTestLogger())

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
	copyUC := &mockCopyMasterToUserUC{fn: func(context.Context, string, string, bool) (CopyMasterToUserResult, error) {
		t.Fatal("copy must not run when the published-check fails")
		return CopyMasterToUserResult{}, nil
	}}
	uc := NewMasterCatalogUsecase(repo, copyUC, newTestAdminGate(true), newTestLogger())

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
	uc := NewMasterCatalogUsecase(repo, &mockCopyMasterToUserUC{}, newTestAdminGate(true), newTestLogger())

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
	copyUC := &mockCopyMasterToUserUC{fn: func(context.Context, string, string, bool) (CopyMasterToUserResult, error) {
		return CopyMasterToUserResult{}, context.Canceled
	}}
	uc := NewMasterCatalogUsecase(repo, copyUC, newTestAdminGate(true), newTestLogger())

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
	uc := NewMasterCatalogUsecase(&mockMasterCatalogRepository{}, &mockCopyMasterToUserUC{}, newTestAdminGate(true), newTestLogger())

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
	uc := NewMasterCatalogUsecase(repo, &mockCopyMasterToUserUC{}, newTestAdminGate(true), newTestLogger())

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
		// Non-zero so a dropped CountCards hydration fails the assertion below.
		countCardsRes: 7,
	}
	uc := NewMasterCatalogUsecase(repo, &mockCopyMasterToUserUC{}, newTestAdminGate(true), newTestLogger())

	got, err := uc.FindPublishedMaster(authedCtx("u1"), "m1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got == nil || got.Master != want {
		t.Fatalf("expected the published deck, got %+v", got)
	}
	if got.CardCount != 7 {
		t.Fatalf("expected CardCount 7, got %d", got.CardCount)
	}
}

// A non-ErrNotFound failure from FindPublishedByID (e.g. a DB outage) is an
// internal error wrapped into the eris chain, not the nil-data non-disclosure path.
func TestFindPublishedMaster_InfraError_Wrapped(t *testing.T) {
	repo := &mockMasterCatalogRepository{
		findPublishedByIDFn: func(string) (*domain.MasterCardgroup, error) { return nil, eris.New("db down") },
	}
	uc := NewMasterCatalogUsecase(repo, &mockCopyMasterToUserUC{}, newTestAdminGate(true), newTestLogger())

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
	uc := NewMasterCatalogUsecase(repo, &mockCopyMasterToUserUC{}, newTestAdminGate(true), newTestLogger())

	_, err := uc.FindPublishedMaster(authedCtx("u1"), "m1")
	assertCancelled(t, err)
	if err != context.Canceled {
		t.Fatalf("expected unwrapped context.Canceled, got %v", err)
	}
}

// A CountCards failure after the published gate passes is an infrastructure
// error wrapped with the count-cards prefix, not the nil-data non-disclosure path.
func TestFindPublishedMaster_CountCardsInfraError_Wrapped(t *testing.T) {
	deck := &domain.MasterCardgroup{ID: "m1", Name: domain.CardgroupName("Deck"), Status: domain.MasterStatusPublished, Version: 1}
	repo := &mockMasterCatalogRepository{
		findPublishedByIDFn: func(string) (*domain.MasterCardgroup, error) { return deck, nil },
		countCardsErr:       eris.New("count down"),
	}
	uc := NewMasterCatalogUsecase(repo, &mockCopyMasterToUserUC{}, newTestAdminGate(true), newTestLogger())

	_, err := uc.FindPublishedMaster(authedCtx("u1"), "m1")
	assertInternalChain(t, err, "usecase: master catalog: find published master: count cards")
}

func TestFindPublishedMaster_CountCards_PropagatesCancelled(t *testing.T) {
	// context.Canceled from CountCards must propagate unwrapped for the same
	// identity contract as the published-check pass-through above.
	deck := &domain.MasterCardgroup{ID: "m1", Name: domain.CardgroupName("Deck"), Status: domain.MasterStatusPublished, Version: 1}
	repo := &mockMasterCatalogRepository{
		findPublishedByIDFn: func(string) (*domain.MasterCardgroup, error) { return deck, nil },
		countCardsErr:       context.Canceled,
	}
	uc := NewMasterCatalogUsecase(repo, &mockCopyMasterToUserUC{}, newTestAdminGate(true), newTestLogger())

	_, err := uc.FindPublishedMaster(authedCtx("u1"), "m1")
	assertCancelled(t, err)
	if err != context.Canceled {
		t.Fatalf("expected unwrapped context.Canceled, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// SeedDefaultStarters
// ---------------------------------------------------------------------------

func TestSeedDefaultStarters_Unauthenticated_ReturnsErrUnauthenticated(t *testing.T) {
	uc := NewMasterCatalogUsecase(&mockMasterCatalogRepository{}, &mockCopyMasterToUserUC{}, newTestAdminGate(true), newTestLogger())
	_, err := uc.SeedDefaultStarters(context.Background())
	if !errors.Is(err, ucerr.ErrUnauthenticated) {
		t.Fatalf("expected ErrUnauthenticated, got %v", err)
	}
}

func TestSeedDefaultStarters_HappyPath_ReturnsSeededCardgroups(t *testing.T) {
	want := []*domain.Cardgroup{{ID: domain.CardgroupID("cg-1"), OwnerID: "user-1", Name: domain.CardgroupName("Starter")}}
	deck := &mockCopyMasterToUserUC{seedResult: want}
	uc := NewMasterCatalogUsecase(&mockMasterCatalogRepository{}, deck, newTestAdminGate(true), newTestLogger())
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
	uc := NewMasterCatalogUsecase(&mockMasterCatalogRepository{}, deck, newTestAdminGate(true), newTestLogger())
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
	uc := NewMasterCatalogUsecase(&mockMasterCatalogRepository{}, deck, newTestAdminGate(true), newTestLogger())
	_, err := uc.SeedDefaultStarters(authedCtx("user-1"))
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	assertInternalChain(t, err, "usecase: master catalog: seed default starters")
}

func TestSeedDefaultStarters_ContextCancelled_PassesThrough(t *testing.T) {
	deck := &mockCopyMasterToUserUC{seedErr: context.Canceled}
	uc := NewMasterCatalogUsecase(&mockMasterCatalogRepository{}, deck, newTestAdminGate(true), newTestLogger())
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
	uc := NewMasterCatalogUsecase(repo, deck, newTestAdminGate(true), newTestLogger())

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
	uc := NewMasterCatalogUsecase(repo, deck, newTestAdminGate(true), newTestLogger())

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
	uc := NewMasterCatalogUsecase(repo, deck, newTestAdminGate(true), newTestLogger())

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
	uc := NewMasterCatalogUsecase(repo, deck, newTestAdminGate(true), newTestLogger())

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
	uc := NewMasterCatalogUsecase(repo, deck, newTestAdminGate(true), newTestLogger())

	_, err := uc.MergeMaster(authedCtx("u1"), "master-id", "cg-id")
	assertInternalChain(t, err, "usecase: master catalog: merge: merge master into cardgroup")
}

func TestMasterCatalogUsecase_MergeMaster_MasterUnpublishedMidFlight_ReturnsNotFoundOutcome(t *testing.T) {
	// TOCTOU: the master is published at the FindPublishedByID gate, but the merge
	// delegate's published-scoped re-read surfaces repository.ErrNotFound. MergeMaster
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
	uc := NewMasterCatalogUsecase(repo, deck, newTestAdminGate(true), newTestLogger())

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

// TestMasterCatalogUsecase_MergeMaster_UnpublishCommitsAfterGate_NoCardsImported
// wires the real master-deck usecase and sequences the unpublish exactly where
// the race used to live: the catalog gate reads the master and finds it
// published, the admin's unpublish commits the instant that read returns, and
// only then does the merge transaction open. The merge's in-transaction probe
// must see the unpublished deck, so the caller gets the non-disclosure not-found
// outcome and not one card is written. Pre-existing coverage starts from an
// already-unpublished deck; this one starts from a published gate, which is the
// interleaving the pooled probe used to admit.
func TestMasterCatalogUsecase_MergeMaster_UnpublishCommitsAfterGate_NoCardsImported(t *testing.T) {
	t.Parallel()
	const ownerID = "u1"
	const destID = "22222222-2222-7222-8222-222222222222"
	const masterID = "master-id"

	deckCG := &fakeMasterCGRepo{byID: map[string]*domain.MasterCardgroup{masterID: masterCG(masterID, "Master")}}
	userCard := &fakeUserCardRepo{}
	repo := &mockMasterCatalogRepository{
		findPublishedByIDFn: func(id string) (*domain.MasterCardgroup, error) {
			// The gate observes a published deck, and the unpublish commits the
			// moment it returns — before the merge transaction body runs.
			delete(deckCG.byID, id)
			return &domain.MasterCardgroup{ID: id, Name: domain.CardgroupName("Master"), Status: domain.MasterStatusPublished}, nil
		},
	}
	deck := newMasterDeckUsecaseWithTx(
		deckCG,
		&fakeMasterCardRepo{byMaster: map[string][]*domain.MasterCard{
			// The master cards outlive the unpublish, so only the published probe
			// can stop the merge here.
			masterID: {masterCard("mc-1", masterID, "alpha", "first", 0)},
		}},
		userCard,
		&fakeUserCG{byID: map[string]*domain.Cardgroup{destID: mustCardgroup(t, destID, ownerID, "My Deck")}},
		stubTxRunner,
		newTestLogger(),
	)
	uc := NewMasterCatalogUsecase(repo, deck, newTestAdminGate(true), newTestLogger())

	out, err := uc.MergeMaster(authedCtx(ownerID), masterID, destID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !out.NotFound {
		t.Fatal("an unpublish committed after the gate must yield the not-found outcome")
	}
	if out.Cardgroup != nil {
		t.Fatal("expected nil cardgroup on NotFound")
	}
	if userCard.upsertCall != 0 || len(userCard.captured) != 0 {
		t.Fatalf("no cards may be imported, got %d upsert calls", userCard.upsertCall)
	}
	if len(deckCG.txHandles) != 1 {
		t.Fatalf("the merge must probe the master on its transaction exactly once, got %d", len(deckCG.txHandles))
	}
	if deckCG.pooledCalls != 0 {
		t.Fatalf("the merge must not probe the master on a pooled connection, got %d calls", deckCG.pooledCalls)
	}
}

// TestMasterCatalogUsecase_PreviewMergeMaster_UnpublishCommitsAfterGate_ReturnsNotFoundOutcome
// is the dry-run half of the parity decision: the same interleaving that makes
// the merge report not-found must make the preview report not-found too, so a
// learner is never shown a tally for a deck the confirm will refuse.
func TestMasterCatalogUsecase_PreviewMergeMaster_UnpublishCommitsAfterGate_ReturnsNotFoundOutcome(t *testing.T) {
	t.Parallel()
	const ownerID = "u1"
	const destID = "22222222-2222-7222-8222-222222222222"
	const masterID = "master-id"

	deckCG := &fakeMasterCGRepo{byID: map[string]*domain.MasterCardgroup{masterID: masterCG(masterID, "Master")}}
	userCard := &fakeUserCardRepo{}
	repo := &mockMasterCatalogRepository{
		findPublishedByIDFn: func(id string) (*domain.MasterCardgroup, error) {
			delete(deckCG.byID, id)
			return &domain.MasterCardgroup{ID: id, Name: domain.CardgroupName("Master"), Status: domain.MasterStatusPublished}, nil
		},
	}
	deck := newMasterDeckUsecaseWithTx(
		deckCG,
		&fakeMasterCardRepo{byMaster: map[string][]*domain.MasterCard{
			masterID: {masterCard("mc-1", masterID, "alpha", "first", 0)},
		}},
		userCard,
		&fakeUserCG{byID: map[string]*domain.Cardgroup{destID: mustCardgroup(t, destID, ownerID, "My Deck")}},
		stubTxRunner,
		newTestLogger(),
	)
	uc := NewMasterCatalogUsecase(repo, deck, newTestAdminGate(true), newTestLogger())

	out, err := uc.PreviewMergeMaster(authedCtx(ownerID), masterID, destID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !out.NotFound {
		t.Fatal("the dry run must report not-found for the same state the merge refuses")
	}
	if out.Added != 0 || out.Updated != 0 {
		t.Fatalf("no tally may be reported on the not-found path, got %+v", out)
	}
	if userCard.countFrontsCalls != 0 {
		t.Fatalf("no overlap tally may be computed, got %d calls", userCard.countFrontsCalls)
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
	deck := newMasterDeckUsecaseWithTx(
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
	uc := NewMasterCatalogUsecase(repo, deck, newTestAdminGate(true), newTestLogger())

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
	uc := NewMasterCatalogUsecase(repo, &mockCopyMasterToUserUC{}, newTestAdminGate(true), newTestLogger())

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
	uc := NewMasterCatalogUsecase(repo, deck, newTestAdminGate(true), newTestLogger())

	_, err := uc.MergeMaster(authedCtx("u1"), "master-id", "cg-id")
	assertCancelled(t, err)
	if err != context.Canceled {
		t.Fatalf("expected unwrapped context.Canceled, got %v", err)
	}
}

// --- PreviewMergeMaster tests ------------------------------------------------

func TestPreviewMergeMaster_UnauthenticatedCaller(t *testing.T) {
	t.Parallel()
	uc := NewMasterCatalogUsecase(&mockMasterCatalogRepository{}, &mockCopyMasterToUserUC{}, newTestAdminGate(true), newTestLogger())

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
	uc := NewMasterCatalogUsecase(&mockMasterCatalogRepository{}, &mockCopyMasterToUserUC{}, newTestAdminGate(true), newTestLogger())

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
	uc := NewMasterCatalogUsecase(repo, &mockCopyMasterToUserUC{}, newTestAdminGate(true), newTestLogger())

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
	uc := NewMasterCatalogUsecase(repo, deck, newTestAdminGate(true), newTestLogger())

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
	uc := NewMasterCatalogUsecase(repo, &mockCopyMasterToUserUC{}, newTestAdminGate(true), newTestLogger())

	_, err := uc.ListPublishedConnection(authedCtx("u1"), MasterCatalogConnectionInput{
		First: intPtr(5),
		After: &cur,
	})

	assertCancelled(t, err)
	if err != context.Canceled {
		t.Fatalf("expected unwrapped context.Canceled, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// v2 cursors — the catalog orders by the admin-mutable sort_order column, so
// its cursors must carry the ordering-key value captured at serve time rather
// than re-reading it off the current row.
// ---------------------------------------------------------------------------

// TestListPublishedConnection_V2Cursor_UsesEmbeddedOrderKey verifies the
// repository receives the sort_order value the cursor captured, NOT the value
// the row currently holds. The stub row deliberately reports a different
// sort_order so a re-hydration regression would fail this assertion.
func TestListPublishedConnection_V2Cursor_UsesEmbeddedOrderKey(t *testing.T) {
	t.Parallel()

	cur := cursor.EncodeV2(cursor.Payload{
		ID:        "cur-1",
		OrderBy:   "sort_order",
		Direction: string(repository.SortAsc),
		OrderKey:  "7",
	})
	repo := &mockMasterCatalogRepository{
		findByIDFn: func(id string) (*domain.MasterCardgroup, error) {
			mcg := catalogItem(id, 0).Cardgroup
			mcg.SortOrder = 999 // an admin re-ordered the deck since the page was served
			return mcg, nil
		},
	}
	uc := NewMasterCatalogUsecase(repo, &mockCopyMasterToUserUC{}, newTestAdminGate(true), newTestLogger())

	_, err := uc.ListPublishedConnection(authedCtx("u1"), MasterCatalogConnectionInput{
		First: intPtr(2),
		After: &cur,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(repo.findPageCalls) != 1 {
		t.Fatalf("want 1 FindPublishedPage call, got %d", len(repo.findPageCalls))
	}
	after := repo.findPageCalls[0].After
	if after == nil || after.SortOrder == nil {
		t.Fatalf("want a hydrated after cursor, got %+v", after)
	}
	if *after.SortOrder != 7 {
		t.Fatalf("after.SortOrder = %d, want the captured 7 (not the current 999)", *after.SortOrder)
	}
	if after.ID != "cur-1" {
		t.Fatalf("after.ID = %q, want cur-1", after.ID)
	}
}

// TestListPublishedConnection_V2Cursor_OrderingMismatch_BadUserInput verifies a
// cursor taken under a different column or direction is rejected with the
// existing BAD_USER_INPUT shape rather than silently mis-paging.
func TestListPublishedConnection_V2Cursor_OrderingMismatch_BadUserInput(t *testing.T) {
	t.Parallel()

	cases := map[string]cursor.Payload{
		"different column": {
			ID: "cur-1", OrderBy: "name",
			Direction: string(repository.SortAsc), OrderKey: "Deck cur-1",
		},
		"different direction": {
			ID: "cur-1", OrderBy: "sort_order",
			Direction: string(repository.SortDesc), OrderKey: "7",
		},
	}
	for name, p := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			cur := cursor.EncodeV2(p)
			repo := &mockMasterCatalogRepository{
				findByIDFn: func(id string) (*domain.MasterCardgroup, error) { return catalogItem(id, 0).Cardgroup, nil },
			}
			uc := NewMasterCatalogUsecase(repo, &mockCopyMasterToUserUC{}, newTestAdminGate(true), newTestLogger())

			_, err := uc.ListPublishedConnection(authedCtx("u1"), MasterCatalogConnectionInput{
				First: intPtr(2),
				After: &cur,
			})
			assertValidationError(t, err, "after", "")
			if len(repo.findPageCalls) != 0 {
				t.Fatal("a mismatched cursor must be rejected before the repository page query runs")
			}
		})
	}
}

// TestListPublishedConnection_V2Cursor_MalformedOrderKey_BadUserInput verifies a
// sort_order value that is not an integer is a client error, not INTERNAL.
func TestListPublishedConnection_V2Cursor_MalformedOrderKey_BadUserInput(t *testing.T) {
	t.Parallel()

	cur := cursor.EncodeV2(cursor.Payload{
		ID:        "cur-1",
		OrderBy:   "sort_order",
		Direction: string(repository.SortAsc),
		OrderKey:  "not-an-int",
	})
	repo := &mockMasterCatalogRepository{
		findByIDFn: func(id string) (*domain.MasterCardgroup, error) { return catalogItem(id, 0).Cardgroup, nil },
	}
	uc := NewMasterCatalogUsecase(repo, &mockCopyMasterToUserUC{}, newTestAdminGate(true), newTestLogger())

	_, err := uc.ListPublishedConnection(authedCtx("u1"), MasterCatalogConnectionInput{
		First: intPtr(2),
		After: &cur,
	})
	assertValidationError(t, err, "after", "")
}

// TestListPublishedConnection_V2Cursor_DraftScopeStillEnforced verifies the
// published-scope gate is not bypassed by a v2 cursor. A v2 cursor can hydrate
// its ordering column without the repository, but FindPublishedByID must still
// run or a draft deck's existence leaks through the catalog.
func TestListPublishedConnection_V2Cursor_DraftScopeStillEnforced(t *testing.T) {
	t.Parallel()

	cur := cursor.EncodeV2(cursor.Payload{
		ID:        "draft-id",
		OrderBy:   "sort_order",
		Direction: string(repository.SortAsc),
		OrderKey:  "7",
	})
	repo := &mockMasterCatalogRepository{
		findByIDFn: func(string) (*domain.MasterCardgroup, error) { return nil, repository.ErrNotFound },
	}
	uc := NewMasterCatalogUsecase(repo, &mockCopyMasterToUserUC{}, newTestAdminGate(true), newTestLogger())

	_, err := uc.ListPublishedConnection(authedCtx("u1"), MasterCatalogConnectionInput{
		First: intPtr(2),
		After: &cur,
	})
	assertValidationError(t, err, "after", "cursor not found")
}

// TestListPublishedConnection_CarriesOrderingAndOrderKeys verifies the output
// carries the ordering metadata the resolver needs to emit a v2 cursor for
// every edge, defaulting to the schema's (SORT_ORDER, ASC).
func TestListPublishedConnection_CarriesOrderingAndOrderKeys(t *testing.T) {
	t.Parallel()

	first := catalogItem("a", 1)
	first.Cardgroup.SortOrder = 3
	second := catalogItem("b", 2)
	second.Cardgroup.SortOrder = 5
	repo := &mockMasterCatalogRepository{
		findPageTotal:  2,
		findPageResult: []*repository.MasterCatalogItem{first, second},
	}
	uc := NewMasterCatalogUsecase(repo, &mockCopyMasterToUserUC{}, newTestAdminGate(true), newTestLogger())

	out, err := uc.ListPublishedConnection(authedCtx("u1"), MasterCatalogConnectionInput{First: intPtr(5)})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := PageOrdering{
		OrderBy:   "sort_order",
		Direction: string(repository.SortAsc),
	}
	if out.Ordering != want {
		t.Fatalf("Ordering = %+v, want %+v", out.Ordering, want)
	}
	if out.OrderKeys["a"] != "3" || out.OrderKeys["b"] != "5" {
		t.Fatalf("OrderKeys = %v, want a=3 b=5", out.OrderKeys)
	}
}

// TestMasterCatalogOrderKeyCodec covers both halves of the ordering-key codec:
// sort_order serializes to a key that decodes back into the cursor, and a
// malformed key is errCursorKeyMalformed rather than an INTERNAL fault.
func TestMasterCatalogOrderKeyCodec(t *testing.T) {
	t.Parallel()

	mcg := &domain.MasterCardgroup{ID: "m1", Name: domain.CardgroupName("Deck m1"), SortOrder: -4}

	gotKey := masterCatalogOrderKey(mcg)
	if gotKey != "-4" {
		t.Fatalf("key = %q, want %q", gotKey, "-4")
	}
	if keys := masterCatalogOrderKeys([]*MasterCatalogItem{{Cardgroup: mcg}}); keys["m1"] != "-4" {
		t.Fatalf("OrderKeys = %v, want m1=-4", keys)
	}
	c := &repository.MasterCatalogCursor{ID: "m1"}
	if err := applyMasterCatalogOrderKey(c, gotKey); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.SortOrder == nil || *c.SortOrder != -4 {
		t.Fatalf("want SortOrder=-4, got %+v", c)
	}

	err := applyMasterCatalogOrderKey(&repository.MasterCatalogCursor{}, "not-an-int")
	if !errors.Is(err, errCursorKeyMalformed) {
		t.Fatalf("want errCursorKeyMalformed, got %v", err)
	}
}

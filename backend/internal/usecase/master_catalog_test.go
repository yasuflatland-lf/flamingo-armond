package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/rotisserie/eris"

	"backend/internal/cursor"
	"backend/internal/domain"
	"backend/internal/repository"
	"backend/internal/usecase/ucerr"
)

// mockMasterCatalogRepository is a manual test double for MasterCatalogRepository.
type mockMasterCatalogRepository struct {
	// FindPublishedPage
	findPageResult []*repository.MasterCatalogItem
	findPageErr    error
	findPageCalls  []findPublishedPageCall

	// CountPublished
	countResult int64
	countErr    error
	countCalls  []countPublishedCall

	// FindPublishedByID
	findByIDFn func(id string) (*domain.MasterCardgroup, error)
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

type countPublishedCall struct {
	Search *string
}

func (m *mockMasterCatalogRepository) FindPublishedPage(
	_ context.Context,
	after, before *repository.MasterCatalogCursor,
	first, last int,
	orderBy repository.MasterCatalogOrderBy,
	dir repository.SortOrder,
	search *string,
) ([]*repository.MasterCatalogItem, error) {
	m.findPageCalls = append(m.findPageCalls, findPublishedPageCall{
		After: after, Before: before, First: first, Last: last,
		OrderBy: orderBy, Dir: dir, Search: search,
	})
	if m.findPageErr != nil {
		return nil, m.findPageErr
	}
	return m.findPageResult, nil
}

func (m *mockMasterCatalogRepository) CountPublished(_ context.Context, search *string) (int64, error) {
	m.countCalls = append(m.countCalls, countPublishedCall{Search: search})
	if m.countErr != nil {
		return 0, m.countErr
	}
	return m.countResult, nil
}

func (m *mockMasterCatalogRepository) FindPublishedByID(_ context.Context, id string) (*domain.MasterCardgroup, error) {
	if m.findByIDFn != nil {
		return m.findByIDFn(id)
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

func TestNewMasterCatalogUsecase_NilLoggerPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic on nil logger")
		}
	}()
	NewMasterCatalogUsecase(&mockMasterCatalogRepository{}, nil)
}

// ---------------------------------------------------------------------------
// Authentication gate
// ---------------------------------------------------------------------------

func TestListPublishedConnection_Unauthenticated(t *testing.T) {
	uc := NewMasterCatalogUsecase(&mockMasterCatalogRepository{}, newTestLogger())

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
		countResult: 5,
		// first=2 → usecase asks for 3 (+1). Repo returns 3 → trim to 2, hasNext=true.
		findPageResult: []*repository.MasterCatalogItem{
			catalogItem("a", 10), catalogItem("b", 20), catalogItem("c", 30),
		},
	}
	uc := NewMasterCatalogUsecase(repo, newTestLogger())

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
		countResult:    2,
		findPageResult: []*repository.MasterCatalogItem{catalogItem("a", 1), catalogItem("b", 2)},
	}
	uc := NewMasterCatalogUsecase(repo, newTestLogger())

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
		countResult: 10,
		findByIDFn: func(id string) (*domain.MasterCardgroup, error) {
			return catalogItem(id, 0).Cardgroup, nil
		},
		// last=2 → repo asked for 3; repo (already reversed) returns 3 → trim leading.
		findPageResult: []*repository.MasterCatalogItem{
			catalogItem("x", 1), catalogItem("y", 2), catalogItem("w", 3),
		},
	}
	uc := NewMasterCatalogUsecase(repo, newTestLogger())

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
	repo := &mockMasterCatalogRepository{countResult: 0, findPageResult: nil}
	uc := NewMasterCatalogUsecase(repo, newTestLogger())

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
	uc := NewMasterCatalogUsecase(&mockMasterCatalogRepository{}, newTestLogger())

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
		countResult:    42,
		findPageResult: []*repository.MasterCatalogItem{},
	}
	uc := NewMasterCatalogUsecase(repo, newTestLogger())

	// first=0 → no rows fetched, but COUNT still runs.
	out, err := uc.ListPublishedConnection(authedCtx("u1"), MasterCatalogConnectionInput{First: intPtr(0)})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.TotalCount != 42 {
		t.Fatalf("want TotalCount=42 even for empty page, got %d", out.TotalCount)
	}
	if len(repo.countCalls) != 1 {
		t.Fatalf("want CountPublished invoked once, got %d", len(repo.countCalls))
	}
}

// ---------------------------------------------------------------------------
// Cursor errors
// ---------------------------------------------------------------------------

func TestListPublishedConnection_InvalidCursor(t *testing.T) {
	repo := &mockMasterCatalogRepository{countResult: 0}
	uc := NewMasterCatalogUsecase(repo, newTestLogger())

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
		countResult: 0,
		findByIDFn: func(_ string) (*domain.MasterCardgroup, error) {
			return nil, repository.ErrNotFound
		},
	}
	uc := NewMasterCatalogUsecase(repo, newTestLogger())

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

func TestListPublishedConnection_CountError_Wrapped(t *testing.T) {
	repo := &mockMasterCatalogRepository{countErr: eris.New("db down")}
	uc := NewMasterCatalogUsecase(repo, newTestLogger())

	_, err := uc.ListPublishedConnection(authedCtx("u1"), MasterCatalogConnectionInput{First: intPtr(2)})
	if err == nil {
		t.Fatal("want error from count failure")
	}
	if errors.Is(err, ucerr.ErrUnauthenticated) {
		t.Fatal("count error must not surface as unauthenticated")
	}
}

func TestListPublishedConnection_FindPageError_Wrapped(t *testing.T) {
	repo := &mockMasterCatalogRepository{countResult: 1, findPageErr: eris.New("db down")}
	uc := NewMasterCatalogUsecase(repo, newTestLogger())

	_, err := uc.ListPublishedConnection(authedCtx("u1"), MasterCatalogConnectionInput{First: intPtr(2)})
	if err == nil {
		t.Fatal("want error from find-page failure")
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
	first, _, err := resolveMasterCatalogPageSize(intPtr(1000), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if first != masterCatalogMaxPageSize {
		t.Fatalf("want clamp to %d, got %d", masterCatalogMaxPageSize, first)
	}
}

func TestResolveMasterCatalogPageSize_DefaultWhenAbsent(t *testing.T) {
	first, last, err := resolveMasterCatalogPageSize(nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if first != defaultPageSize || last != 0 {
		t.Fatalf("want default first=%d last=0, got first=%d last=%d", defaultPageSize, first, last)
	}
}

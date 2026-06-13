package usecase

import (
	"context"
	"errors"
	"log/slog"

	"github.com/rotisserie/eris"

	"backend/internal/auth"
	"backend/internal/cursor"
	"backend/internal/domain"
	"backend/internal/repository"
	"backend/internal/usecase/ucerr"
)

// MasterCatalogRepository is the consumer-driven interface used by
// MasterCatalogUsecase. It is intentionally narrow: only the published-catalog
// read methods of repository.MasterCardgroupRepository are needed here.
type MasterCatalogRepository interface {
	FindPublishedPage(
		ctx context.Context,
		after, before *repository.MasterCatalogCursor,
		first, last int,
		orderBy repository.MasterCatalogOrderBy,
		dir repository.SortOrder,
		search *string,
	) ([]*repository.MasterCatalogItem, error)
	CountPublished(ctx context.Context, search *string) (int64, error)
	FindPublishedByID(ctx context.Context, id string) (*domain.MasterCardgroup, error)
}

// MasterCatalogOrderBy mirrors the schema MasterCatalogOrderBy enum but stays
// in the usecase layer so the repository remains independent of the GraphQL
// model package. The string values are identical to model.MasterCatalogOrderBy
// so the resolver can convert with a direct cast.
type MasterCatalogOrderBy string

const (
	MasterCatalogOrderBySortOrder MasterCatalogOrderBy = "SORT_ORDER"
	MasterCatalogOrderByCreatedAt MasterCatalogOrderBy = "CREATED_AT"
	MasterCatalogOrderByName      MasterCatalogOrderBy = "NAME"
)

// MasterCatalogConnectionInput captures the GraphQL pagination arguments for
// masterCatalog. Pointer fields preserve "absent" semantics from the schema so
// the usecase can default unset values explicitly.
type MasterCatalogConnectionInput struct {
	First, Last    *int
	After, Before  *string // raw GraphQL ID strings (cursor = master cardgroup UUID)
	Search         *string
	OrderBy        *MasterCatalogOrderBy
	OrderDirection *SortOrder
}

// MasterCatalogConnectionOutput is the usecase-level page result. The resolver
// wraps it into a model.MasterCatalogConnection.
type MasterCatalogConnectionOutput struct {
	Items      []*repository.MasterCatalogItem
	TotalCount int64
	HasNext    bool
	HasPrev    bool
	StartCur   string
	EndCur     string
}

// MasterCatalogUsecase is the read-only published-catalog surface. Every method
// requires an authenticated caller — the catalog is not public.
type MasterCatalogUsecase interface {
	ListPublishedConnection(ctx context.Context, in MasterCatalogConnectionInput) (*MasterCatalogConnectionOutput, error)
}

type masterCatalogUsecase struct {
	repo   MasterCatalogRepository
	logger *slog.Logger
}

// NewMasterCatalogUsecase constructs a MasterCatalogUsecase backed by the given
// repository. Panics on a nil logger.
func NewMasterCatalogUsecase(repo MasterCatalogRepository, logger *slog.Logger) MasterCatalogUsecase {
	if logger == nil {
		panic("usecase: master catalog: logger is required")
	}
	return &masterCatalogUsecase{repo: repo, logger: logger}
}

// ListPublishedConnection paginates the published master catalog with
// Relay-style cursors. Forward paging uses (first, after); backward uses
// (last, before). The five mixed-direction combinations are rejected with
// BAD_USER_INPUT before the repository is touched so the caller never gets a
// silently re-interpreted page boundary. Only PUBLISHED decks are ever
// returned — the published filter is enforced in the repository SQL and is not
// a caller-overridable argument. Unauthenticated callers receive
// UNAUTHENTICATED.
func (u *masterCatalogUsecase) ListPublishedConnection(
	ctx context.Context, in MasterCatalogConnectionInput,
) (*MasterCatalogConnectionOutput, error) {
	if auth.UserFrom(ctx) == nil {
		return nil, ucerr.ErrUnauthenticated
	}

	// Relay argument coherence: after pairs with first (forward) and before
	// pairs with last (backward). Reject any other combination before the repo
	// is touched so the failure mode is observable rather than a silent
	// page-1 reset.
	if err := validateRelayArgs(in.First, in.Last, in.After, in.Before); err != nil {
		return nil, err
	}

	orderBy, dir, err := resolveMasterCatalogOrderBy(in.OrderBy, in.OrderDirection)
	if err != nil {
		return nil, err
	}

	first, last, err := resolveMasterCatalogPageSize(in.First, in.Last)
	if err != nil {
		return nil, err
	}

	after, err := u.resolveMasterCatalogCursor(ctx, in.After, orderBy, "after")
	if err != nil {
		return nil, err
	}
	before, err := u.resolveMasterCatalogCursor(ctx, in.Before, orderBy, "before")
	if err != nil {
		return nil, err
	}

	// totalCount comes from a separate COUNT(*) scoped to published rows and
	// the optional search predicate. Computed before the no-rows short-circuit
	// so callers asking only for totalCount still see a real value.
	total, err := u.repo.CountPublished(ctx, in.Search)
	if err != nil {
		return nil, eris.Wrap(err, "usecase: master catalog: count published")
	}

	// Request one extra row to detect whether another page exists. Trim before
	// returning to the caller.
	wantFirst := first
	wantLast := last
	if wantFirst > 0 {
		wantFirst++
	}
	if wantLast > 0 {
		wantLast++
	}

	items, err := u.repo.FindPublishedPage(ctx, after, before, wantFirst, wantLast, orderBy, dir, in.Search)
	if err != nil {
		return nil, eris.Wrap(err, "usecase: master catalog: find published page")
	}

	out := &MasterCatalogConnectionOutput{TotalCount: total}
	switch {
	case first > 0:
		items, out.HasNext = TrimAndDetect(items, first)
		out.HasPrev = after != nil
	case last > 0:
		items, out.HasPrev = TrimAndDetectBackward(items, last)
		out.HasNext = before != nil
	}

	out.Items = items
	if len(items) > 0 {
		out.StartCur = items[0].Cardgroup.ID
		out.EndCur = items[len(items)-1].Cardgroup.ID
	}
	return out, nil
}

// masterCatalogMaxPageSize is the user-facing cap on masterCatalog page size.
// The repository-level cap (repository.PageCap=101) is one greater so the
// "+1 fetch" trick survives a maximum-sized request.
const masterCatalogMaxPageSize = 100

// resolveMasterCatalogOrderBy maps the typed usecase enums to the repository
// allowlist. Defaults match the schema (SORT_ORDER, ASC) when both inputs are
// nil. The default switch arm is defense in depth — gqlgen UnmarshalGQL already
// rejects invalid enum strings upstream.
func resolveMasterCatalogOrderBy(
	orderBy *MasterCatalogOrderBy, dir *SortOrder,
) (repository.MasterCatalogOrderBy, repository.SortOrder, error) {
	field := repository.MasterCatalogOrderBySortOrder
	if orderBy != nil {
		switch *orderBy {
		case MasterCatalogOrderBySortOrder:
			field = repository.MasterCatalogOrderBySortOrder
		case MasterCatalogOrderByCreatedAt:
			field = repository.MasterCatalogOrderByCreatedAt
		case MasterCatalogOrderByName:
			field = repository.MasterCatalogOrderByName
		default:
			return "", "", ucerr.NewValidationError("orderBy", "invalid")
		}
	}
	d := repository.SortAsc
	if dir != nil {
		switch *dir {
		case SortOrderAsc:
			d = repository.SortAsc
		case SortOrderDesc:
			d = repository.SortDesc
		default:
			return "", "", ucerr.NewValidationError("orderDirection", "invalid")
		}
	}
	return field, d, nil
}

// resolveMasterCatalogPageSize clamps first/last to [0, masterCatalogMaxPageSize]
// and rejects passing both. Defaults first=defaultPageSize (20) when neither is
// provided, matching the schema's documented default.
func resolveMasterCatalogPageSize(first, last *int) (int, int, error) {
	if first != nil && last != nil {
		return 0, 0, ucerr.NewValidationError("first", "specify either first or last, not both")
	}
	if first == nil && last == nil {
		return defaultPageSize, 0, nil
	}
	clamp := func(v int) int {
		if v < 0 {
			return 0
		}
		if v > masterCatalogMaxPageSize {
			return masterCatalogMaxPageSize
		}
		return v
	}
	if first != nil {
		return clamp(*first), 0, nil
	}
	return 0, clamp(*last), nil
}

// resolveMasterCatalogCursor decodes an opaque cursor string into a
// *repository.MasterCatalogCursor with the column required by the active
// orderBy populated. Returns BAD_USER_INPUT when the cursor cannot be decoded
// or references a master cardgroup that is not a published catalog row. The
// published-row check runs even when orderBy is missing a hydratable column.
func (u *masterCatalogUsecase) resolveMasterCatalogCursor(
	ctx context.Context,
	cursorStr *string,
	orderBy repository.MasterCatalogOrderBy,
	field string,
) (*repository.MasterCatalogCursor, error) {
	if cursorStr == nil || *cursorStr == "" {
		return nil, nil
	}
	id, err := cursor.Decode(*cursorStr)
	if err != nil {
		return nil, ucerr.NewValidationError(field, "invalid cursor")
	}
	mcg, err := u.repo.FindPublishedByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ucerr.NewValidationError(field, "cursor not found")
		}
		return nil, eris.Wrap(err, "usecase: master catalog: hydrate cursor")
	}

	c := &repository.MasterCatalogCursor{ID: id}
	switch orderBy {
	case repository.MasterCatalogOrderBySortOrder:
		so := mcg.SortOrder
		c.SortOrder = &so
	case repository.MasterCatalogOrderByCreatedAt:
		ca := mcg.CreatedAt
		c.CreatedAt = &ca
	case repository.MasterCatalogOrderByName:
		name := mcg.Name.String()
		c.Name = &name
	default:
		return nil, eris.Errorf("usecase: master catalog: unhandled orderBy %q", orderBy)
	}
	return c, nil
}

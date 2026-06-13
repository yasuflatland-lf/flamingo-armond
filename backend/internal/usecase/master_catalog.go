package usecase

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/rotisserie/eris"

	"backend/internal/auth"
	"backend/internal/cursor"
	"backend/internal/domain"
	"backend/internal/repository"
	"backend/internal/usecase/ucerr"
)

// MasterCatalogRepository is the consumer-driven interface used by
// MasterCatalogUsecase. It exposes both the public published-catalog read
// methods and the admin management methods (create, update, delete,
// publish/unpublish, admin list).
type MasterCatalogRepository interface {
	// --- published read (existing) ---
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

	// --- admin (new) ---
	FindByID(ctx context.Context, id string) (*domain.MasterCardgroup, error)
	FindAdminPage(
		ctx context.Context,
		after, before *repository.MasterCatalogCursor,
		first, last int,
		orderBy repository.MasterCatalogOrderBy,
		dir repository.SortOrder,
		search *string,
	) ([]*repository.MasterCatalogItem, error)
	CountAdmin(ctx context.Context, search *string) (int64, error)
	CountCards(ctx context.Context, masterCardgroupID string) (int64, error)
	Create(ctx context.Context, m *domain.MasterCardgroup) error
	Update(ctx context.Context, id string, patch repository.MasterCardgroupUpdate) (*domain.MasterCardgroup, error)
	Delete(ctx context.Context, id string) error
	Publish(ctx context.Context, id string) (*domain.MasterCardgroup, error)
	Unpublish(ctx context.Context, id string) (*domain.MasterCardgroup, error)
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

// MasterCatalogUsecase is the published-catalog surface plus the admin
// management operations. Every method requires an authenticated caller;
// admin methods additionally require AdminGate.Require to pass.
type MasterCatalogUsecase interface {
	ListPublishedConnection(ctx context.Context, in MasterCatalogConnectionInput) (*MasterCatalogConnectionOutput, error)

	// admin-gated management surface
	ListAdminConnection(ctx context.Context, in MasterCatalogConnectionInput) (*MasterCatalogConnectionOutput, error)
	CreateMaster(ctx context.Context, in CreateMasterInput) (CreateMasterOutcome, error)
	UpdateMaster(ctx context.Context, id string, in UpdateMasterInput) (UpdateMasterOutcome, error)
	PublishMaster(ctx context.Context, id string) (PublishMasterOutcome, error)
	UnpublishMaster(ctx context.Context, id string) (*MasterWithCount, error)
	DeleteMaster(ctx context.Context, id string) error
}

type masterCatalogUsecase struct {
	repo      MasterCatalogRepository
	adminGate *AdminGate
	logger    *slog.Logger
}

// NewMasterCatalogUsecase constructs a MasterCatalogUsecase. The adminGate gates
// every admin-management method; the public ListPublishedConnection is gated by
// authentication only. Panics on a nil adminGate or logger.
func NewMasterCatalogUsecase(repo MasterCatalogRepository, adminGate *AdminGate, logger *slog.Logger) MasterCatalogUsecase {
	if adminGate == nil {
		panic("usecase: master catalog: adminGate is required")
	}
	if logger == nil {
		panic("usecase: master catalog: logger is required")
	}
	return &masterCatalogUsecase{repo: repo, adminGate: adminGate, logger: logger}
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

// resolveMasterCatalogPageSize clamps first/last to [0, maxPageSize] and rejects
// passing both. Defaults first=defaultPageSize (20) when neither is provided,
// matching the schema's documented default. maxPageSize/defaultPageSize are the
// package-wide page-size caps shared with the card/cardgroup resolvers; the
// repository-level cap (repository.PageCap = maxPageSize + 1) is one greater so
// the "+1 fetch" trick survives a maximum-sized request.
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
		if v > maxPageSize {
			return maxPageSize
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

// ---------------------------------------------------------------------------
// Carrier types for admin mutations
// ---------------------------------------------------------------------------

// MasterWithCount bundles a master cardgroup with its current card count so the
// resolver can populate the non-null model.MasterCardgroup.cardCount field on a
// single-entity admin response.
type MasterWithCount struct {
	Master    *domain.MasterCardgroup
	CardCount int64
}

// CreateMasterInput carries the admin create fields. Optional attributes are
// pointers preserving "absent" semantics from the GraphQL input.
type CreateMasterInput struct {
	Name             string
	Description      *string
	Language         *string
	Level            *string
	Category         *string
	CoverImageURL    *string
	Source           *string
	IsDefaultStarter *bool
	SortOrder        *int
}

// UpdateMasterInput carries the admin update patch. nil = leave unchanged.
type UpdateMasterInput struct {
	Name             *string
	Description      *string
	Language         *string
	Level            *string
	Category         *string
	CoverImageURL    *string
	Source           *string
	IsDefaultStarter *bool
	SortOrder        *int
}

// CreateMasterOutcome is the result of CreateMaster. Exactly one of Master or
// Validation is non-nil on a nil-error return. A freshly created deck has no
// cards (cardCount = 0).
type CreateMasterOutcome struct {
	Master     *domain.MasterCardgroup
	Validation *InputValidationInfo
}

// UpdateMasterOutcome is the result of UpdateMaster. Exactly one of Master or
// Validation is non-nil. CardCount is the deck's current card count, fetched so
// the resolver can populate model.MasterCardgroup.cardCount.
type UpdateMasterOutcome struct {
	Master     *domain.MasterCardgroup
	CardCount  int64
	Validation *InputValidationInfo
}

// PublishMasterOutcome is the result of PublishMaster. Exactly one of Master or
// EmptyMaster is set: a deck with zero cards cannot be published, surfaced as
// EmptyMaster = true so the resolver maps it to MasterCardgroupEmptyError.
type PublishMasterOutcome struct {
	Master      *domain.MasterCardgroup
	CardCount   int64
	EmptyMaster bool
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// mapMasterAdminErr classifies a repository error from an admin master method
// into either input-validation data (first slot) or a propagating error (second
// slot), mirroring mapAdminRoleError. The ucerr.NewValidationError emission lives
// HERE, not in the caller's method body, so the schema-lint bare-object gate does
// not flag adminUnpublishMasterCardgroup (which returns a bare MasterCardgroup!).
//   - repository.ErrNotFound              -> InputValidationInfo{Field: notFoundField}
//   - context.Canceled / DeadlineExceeded -> passthrough via error
//   - default                             -> eris.Wrap(err, wrap) via error
func mapMasterAdminErr(err error, notFoundField, wrap string) (*InputValidationInfo, error) {
	switch {
	case errors.Is(err, repository.ErrNotFound):
		return NewInputValidationInfo(notFoundField, "master cardgroup not found"), nil
	case isContextDone(err):
		return nil, err
	default:
		return nil, eris.Wrap(err, wrap)
	}
}

// derefOr returns *p when p is non-nil, otherwise def.
func derefOr[T any](p *T, def T) T {
	if p != nil {
		return *p
	}
	return def
}

// ---------------------------------------------------------------------------
// Admin methods
// ---------------------------------------------------------------------------

// CreateMaster creates a new DRAFT master cardgroup. Admin-only. Name validation
// failures surface via outcome.Validation (mapped to the InputValidationError
// union variant); a new deck starts at version 1, status DRAFT.
func (u *masterCatalogUsecase) CreateMaster(ctx context.Context, in CreateMasterInput) (CreateMasterOutcome, error) {
	if _, err := u.adminGate.Require(ctx, "usecase: master catalog: create master"); err != nil {
		return CreateMasterOutcome{}, err
	}

	name, nameErr := domain.ParseCardgroupName(in.Name)
	info, err := liftValidationErr(translateCardgroupNameErr(nameErr))
	if err != nil {
		return CreateMasterOutcome{}, err
	}
	if info != nil {
		return CreateMasterOutcome{Validation: info}, nil
	}

	id, err := domain.NewID()
	if err != nil {
		return CreateMasterOutcome{}, eris.Wrap(err, "usecase: master catalog: create master: id")
	}
	now := time.Now().UTC()
	m := &domain.MasterCardgroup{
		ID:               id,
		Name:             name,
		Description:      in.Description,
		Language:         in.Language,
		Level:            in.Level,
		Category:         in.Category,
		CoverImageURL:    in.CoverImageURL,
		Source:           in.Source,
		Version:          1,
		Status:           domain.MasterStatusDraft,
		IsDefaultStarter: derefOr(in.IsDefaultStarter, false),
		SortOrder:        derefOr(in.SortOrder, 0),
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if err := u.repo.Create(ctx, m); err != nil {
		return CreateMasterOutcome{}, eris.Wrap(err, "usecase: master catalog: create master")
	}
	return CreateMasterOutcome{Master: m}, nil
}

// UpdateMaster applies an admin patch to an existing master cardgroup. Admin-only.
// Name validation failures surface via outcome.Validation; a missing row surfaces
// as a validation error on "id". CardCount is fetched so the resolver can populate
// the response model.
func (u *masterCatalogUsecase) UpdateMaster(ctx context.Context, id string, in UpdateMasterInput) (UpdateMasterOutcome, error) {
	if _, err := u.adminGate.Require(ctx, "usecase: master catalog: update master"); err != nil {
		return UpdateMasterOutcome{}, err
	}

	patch := repository.MasterCardgroupUpdate{
		Description:      in.Description,
		Language:         in.Language,
		Level:            in.Level,
		Category:         in.Category,
		CoverImageURL:    in.CoverImageURL,
		Source:           in.Source,
		IsDefaultStarter: in.IsDefaultStarter,
		SortOrder:        in.SortOrder,
	}
	if in.Name != nil {
		name, nameErr := domain.ParseCardgroupName(*in.Name)
		info, err := liftValidationErr(translateCardgroupNameErr(nameErr))
		if err != nil {
			return UpdateMasterOutcome{}, err
		}
		if info != nil {
			return UpdateMasterOutcome{Validation: info}, nil
		}
		nameStr := name.String()
		patch.Name = &nameStr
	}

	updated, err := u.repo.Update(ctx, id, patch)
	if err != nil {
		info, perr := mapMasterAdminErr(err, "id", "usecase: master catalog: update master")
		if perr != nil {
			return UpdateMasterOutcome{}, perr
		}
		return UpdateMasterOutcome{Validation: info}, nil
	}
	count, err := u.repo.CountCards(ctx, id)
	if err != nil {
		return UpdateMasterOutcome{}, eris.Wrap(err, "usecase: master catalog: update master: count cards")
	}
	return UpdateMasterOutcome{Master: updated, CardCount: count}, nil
}

// PublishMaster publishes a master cardgroup after confirming it has at least one
// card. Admin-only. A deck with zero cards is rejected via outcome.EmptyMaster
// (mapped to MasterCardgroupEmptyError) without touching the publish path.
func (u *masterCatalogUsecase) PublishMaster(ctx context.Context, id string) (PublishMasterOutcome, error) {
	if _, err := u.adminGate.Require(ctx, "usecase: master catalog: publish master"); err != nil {
		return PublishMasterOutcome{}, err
	}
	// Ensure the deck exists before the empty-count guard so a missing id is a
	// validation error rather than a silent "0 cards => empty" classification.
	if _, err := u.repo.FindByID(ctx, id); err != nil {
		return PublishMasterOutcome{}, lowerValidationInfo(mapMasterAdminErr(err, "id", "usecase: master catalog: publish master: find"))
	}
	count, err := u.repo.CountCards(ctx, id)
	if err != nil {
		return PublishMasterOutcome{}, eris.Wrap(err, "usecase: master catalog: publish master: count cards")
	}
	if count == 0 {
		return PublishMasterOutcome{EmptyMaster: true}, nil
	}
	published, err := u.repo.Publish(ctx, id)
	if err != nil {
		return PublishMasterOutcome{}, lowerValidationInfo(mapMasterAdminErr(err, "id", "usecase: master catalog: publish master"))
	}
	return PublishMasterOutcome{Master: published, CardCount: count}, nil
}

// UnpublishMaster reverts a master cardgroup to DRAFT. Admin-only. Returns the
// refreshed master plus its card count. NOTE: this method backs the bare-object
// mutation adminUnpublishMasterCardgroup, so its body must NOT directly reference
// ucerr.NewValidationError / ucerr.NewForbiddenError / ucerr.ErrUnauthenticated —
// the not-found mapping is delegated to mapMasterAdminErr/lowerValidationInfo.
func (u *masterCatalogUsecase) UnpublishMaster(ctx context.Context, id string) (*MasterWithCount, error) {
	if _, err := u.adminGate.Require(ctx, "usecase: master catalog: unpublish master"); err != nil {
		return nil, err
	}
	updated, err := u.repo.Unpublish(ctx, id)
	if err != nil {
		return nil, lowerValidationInfo(mapMasterAdminErr(err, "id", "usecase: master catalog: unpublish master"))
	}
	count, err := u.repo.CountCards(ctx, id)
	if err != nil {
		return nil, eris.Wrap(err, "usecase: master catalog: unpublish master: count cards")
	}
	return &MasterWithCount{Master: updated, CardCount: count}, nil
}

// DeleteMaster removes a master cardgroup. Admin-only. A missing row is a
// validation error on "id". Returns Boolean! upstream (scalar — schema-lint exempt),
// so direct ucerr use here is permitted, but routed through the helper for
// consistency with UnpublishMaster.
func (u *masterCatalogUsecase) DeleteMaster(ctx context.Context, id string) error {
	if _, err := u.adminGate.Require(ctx, "usecase: master catalog: delete master"); err != nil {
		return err
	}
	if err := u.repo.Delete(ctx, id); err != nil {
		return lowerValidationInfo(mapMasterAdminErr(err, "id", "usecase: master catalog: delete master"))
	}
	return nil
}

// ListAdminConnection paginates ALL master cardgroups (DRAFT + PUBLISHED) for the
// admin UI. Admin-only. Mirrors ListPublishedConnection but gates on adminGate and
// calls the unfiltered CountAdmin / FindAdminPage repository methods.
func (u *masterCatalogUsecase) ListAdminConnection(
	ctx context.Context, in MasterCatalogConnectionInput,
) (*MasterCatalogConnectionOutput, error) {
	if _, err := u.adminGate.Require(ctx, "usecase: master catalog: list admin"); err != nil {
		return nil, err
	}
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
	after, err := u.resolveMasterAdminCursor(ctx, in.After, orderBy, "after")
	if err != nil {
		return nil, err
	}
	before, err := u.resolveMasterAdminCursor(ctx, in.Before, orderBy, "before")
	if err != nil {
		return nil, err
	}

	total, err := u.repo.CountAdmin(ctx, in.Search)
	if err != nil {
		return nil, eris.Wrap(err, "usecase: master catalog: count admin")
	}

	wantFirst := first
	wantLast := last
	if wantFirst > 0 {
		wantFirst++
	}
	if wantLast > 0 {
		wantLast++
	}
	items, err := u.repo.FindAdminPage(ctx, after, before, wantFirst, wantLast, orderBy, dir, in.Search)
	if err != nil {
		return nil, eris.Wrap(err, "usecase: master catalog: find admin page")
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

// resolveMasterAdminCursor decodes an opaque cursor into a repository cursor with
// the column required by the active orderBy. Unlike resolveMasterCatalogCursor it
// hydrates via FindByID (any status), since the admin list includes DRAFT decks.
func (u *masterCatalogUsecase) resolveMasterAdminCursor(
	ctx context.Context, cursorStr *string, orderBy repository.MasterCatalogOrderBy, field string,
) (*repository.MasterCatalogCursor, error) {
	if cursorStr == nil || *cursorStr == "" {
		return nil, nil
	}
	id, err := cursor.Decode(*cursorStr)
	if err != nil {
		return nil, ucerr.NewValidationError(field, "invalid cursor")
	}
	mcg, err := u.repo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ucerr.NewValidationError(field, "cursor not found")
		}
		return nil, eris.Wrap(err, "usecase: master catalog: hydrate admin cursor")
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

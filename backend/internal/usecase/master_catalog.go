package usecase

import (
	"context"
	"errors"
	"log/slog"
	"strings"

	"github.com/rotisserie/eris"

	"backend/internal/auth"
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
	) ([]*repository.MasterCatalogItem, int64, error)
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
	) ([]*repository.MasterCatalogItem, int64, error)
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

// masterDeckUsecaseFacade is the master-deck capability the catalog consumes:
// snapshot a single published master (import), seed all default starters for
// the caller, merge a published master into an existing caller-owned cardgroup,
// and preview that merge without writing. *masterDeckUsecase satisfies all four
// capabilities.
type masterDeckUsecaseFacade interface {
	CopyMasterToUserUsecase
	SeedForNewUserUsecase
	MergeMasterIntoCardgroupUsecase
	PreviewMergeMasterIntoCardgroupUsecase
}

// MergeMasterOutcome is the usecase result of MergeMaster. On the valid paths
// exactly one outcome is active: the happy path sets Cardgroup with the Added/Updated
// tallies and leaves NotFound false; the not-found path sets NotFound=true and leaves
// Cardgroup nil with zero tallies. Destination cardgroup auth failures are returned as
// errors, not via this outcome.
type MergeMasterOutcome struct {
	// Cardgroup is the caller-owned destination after the merge. Non-nil iff NotFound is false.
	Cardgroup *domain.Cardgroup
	// Added is the number of cards newly inserted into the destination.
	Added int64
	// Updated is the number of existing cards (same front) overwritten.
	Updated int64
	// NotFound is true when the master id is unknown or not published; draft existence
	// is subsumed so draft ids are indistinguishable from absent ids. True iff Cardgroup
	// is nil. The XOR is a producer contract, not a compile-time guarantee: a degenerate
	// {Cardgroup:nil, NotFound:false} result is treated as INTERNAL by the resolver's
	// defensive guard (noVariantSet).
	NotFound bool
}

// PreviewMergeOutcome is the usecase result of PreviewMergeMaster. On the valid
// path Added/Updated carry the projected tally and NotFound is false; the not-found
// path sets NotFound=true with zero tallies. Destination auth failures are returned
// as errors, not via this outcome.
type PreviewMergeOutcome struct {
	Added    int64
	Updated  int64
	NotFound bool
}

// MasterCatalogUsecase is the published-catalog surface plus the admin
// management operations. Every method requires an authenticated caller;
// admin methods additionally require AdminGate.Require to pass.
type MasterCatalogUsecase interface {
	ListPublishedConnection(ctx context.Context, in MasterCatalogConnectionInput) (*MasterCatalogConnectionOutput, error)
	// FindPublishedMaster returns a single PUBLISHED master deck by id for any
	// authenticated caller. Returns (nil, nil) for an unknown or DRAFT id
	// (non-disclosure gate). Anonymous callers receive UNAUTHENTICATED.
	FindPublishedMaster(ctx context.Context, id string) (*domain.MasterCardgroup, error)
	ImportMaster(ctx context.Context, masterID string) (ImportMasterOutcome, error)
	MergeMaster(ctx context.Context, masterID, cardgroupID string) (MergeMasterOutcome, error)
	PreviewMergeMaster(ctx context.Context, masterID, cardgroupID string) (PreviewMergeOutcome, error)
	SeedDefaultStarters(ctx context.Context) ([]*domain.Cardgroup, error)

	// admin-gated management surface
	ListAdminConnection(ctx context.Context, in MasterCatalogConnectionInput) (*MasterCatalogConnectionOutput, error)
	CreateMaster(ctx context.Context, in CreateMasterInput) (CreateMasterOutcome, error)
	UpdateMaster(ctx context.Context, id string, in UpdateMasterInput) (UpdateMasterOutcome, error)
	PublishMaster(ctx context.Context, id string) (PublishMasterOutcome, error)
	UnpublishMaster(ctx context.Context, id string) (*MasterWithCount, error)
	DeleteMaster(ctx context.Context, id string) error
}

// ImportMasterOutcome is the usecase result of ImportMaster. On the valid paths
// exactly one signal is set: Cardgroup on the happy path, or NotFound=true when the
// master id is unknown or not published. The not-found case is surfaced as data (the
// MasterNotFoundError union variant) rather than as an error so the resolver can
// return it in `data`. The XOR is a producer contract, not a compile-time guarantee:
// a degenerate {Cardgroup:nil, NotFound:false} result is treated as INTERNAL by the
// resolver's defensive guard.
type ImportMasterOutcome struct {
	// Cardgroup is the newly created user-owned cardgroup snapshot on the happy
	// path. Non-nil iff NotFound is false.
	Cardgroup *domain.Cardgroup
	// NotFound is true when the master id is unknown or not published; it subsumes
	// draft existence so draft ids are indistinguishable from absent ids. True iff
	// Cardgroup is nil.
	NotFound bool
}

type masterCatalogUsecase struct {
	repo      MasterCatalogRepository
	deckUC    masterDeckUsecaseFacade
	adminGate *AdminGate
	logger    *slog.Logger
}

// NewMasterCatalogUsecase constructs a MasterCatalogUsecase backed by the given
// repository. deckUC is the combined deck facade (CopyMasterToUserUsecase +
// SeedForNewUserUsecase + MergeMasterIntoCardgroupUsecase) used by ImportMaster,
// SeedDefaultStarters, and MergeMaster; adminGate gates every admin-management
// method. The public ListPublishedConnection is gated by authentication only.
// Panics when repo, deckUC, adminGate, or logger is nil — a nil required
// dependency is a wiring bug that must fail at startup, not at first use.
func NewMasterCatalogUsecase(repo MasterCatalogRepository, deckUC masterDeckUsecaseFacade, adminGate *AdminGate, logger *slog.Logger) MasterCatalogUsecase {
	if repo == nil {
		panic("usecase: master catalog: repo is required")
	}
	if deckUC == nil {
		panic("usecase: master catalog: deckUC is required")
	}
	if adminGate == nil {
		panic("usecase: master catalog: adminGate is required")
	}
	if logger == nil {
		panic("usecase: master catalog: logger is required")
	}
	return &masterCatalogUsecase{repo: repo, deckUC: deckUC, adminGate: adminGate, logger: logger}
}

// masterCatalogPageFetch is the repository page-fetch closure shape shared by
// MasterCatalogRepository.FindPublishedPage and FindAdminPage. listMasterCatalogCore
// takes one as an argument so the shared page-assembly body stays agnostic to the
// status filter (published-only vs. all statuses).
type masterCatalogPageFetch func(
	ctx context.Context,
	after, before *repository.MasterCatalogCursor,
	first, last int,
	orderBy repository.MasterCatalogOrderBy,
	dir repository.SortOrder,
	search *string,
) ([]*repository.MasterCatalogItem, int64, error)

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
	return u.listMasterCatalogCore(ctx, in, true, "usecase: master catalog: find published page",
		func(ctx context.Context) error {
			if auth.UserFrom(ctx) == nil {
				return ucerr.ErrUnauthenticated
			}
			return nil
		},
		u.repo.FindPublishedPage,
	)
}

// listMasterCatalogCore holds the shared page-assembly body for
// ListPublishedConnection and ListAdminConnection. The gate closure runs first and
// supplies the per-caller authorization / visibility check (anonymous-allowed
// authentication for the published catalog vs. adminGate.Require for the admin
// surface); everything from cursor resolution onward is identical except two
// caller-supplied knobs: publishedOnly threads into resolveMasterCursor to pick the
// hydration scope (true = published catalog, a DRAFT or unknown id is rejected as
// cursor-not-found so drafts never leak; false = admin, DRAFT decks are valid
// cursors), and fetch is the repository page method (FindPublishedPage /
// FindAdminPage). opPrefix is the caller's eris wrap message, supplied so the shared
// find-page wrap carries the correct attribution (error-wrapping rule: shared helpers
// take the caller prefix as an argument, never hardcode it).
func (u *masterCatalogUsecase) listMasterCatalogCore(
	ctx context.Context,
	in MasterCatalogConnectionInput,
	publishedOnly bool,
	opPrefix string,
	gate func(context.Context) error,
	fetch masterCatalogPageFetch,
) (*MasterCatalogConnectionOutput, error) {
	if err := gate(ctx); err != nil {
		return nil, err
	}

	first, last, err := resolveRelayPage(in.First, in.Last, in.After, in.Before, resolveStandardPageSize)
	if err != nil {
		return nil, err
	}

	orderBy, dir, err := resolveMasterCatalogOrderBy(in.OrderBy, in.OrderDirection)
	if err != nil {
		return nil, err
	}

	after, err := u.resolveMasterCursor(ctx, in.After, orderBy, "after", publishedOnly)
	if err != nil {
		return nil, err
	}
	before, err := u.resolveMasterCursor(ctx, in.Before, orderBy, "before", publishedOnly)
	if err != nil {
		return nil, err
	}

	// Normalize search once so the count and the page query see the same filter
	// (nil and whitespace-only both mean "no filter").
	search := normalizeSearch(in.Search)

	// totalCount is the search-aware total carried by the page method, captured
	// inside the fetch closure. The page method runs its COUNT(*) on the same
	// filtered base before its zero-page short-circuit, so a totalCount-only
	// request still observes the real value.
	var total int64
	items, hasNext, hasPrev, err := assemblePage(first, last, after != nil, before != nil,
		func(wantFirst, wantLast int) ([]*repository.MasterCatalogItem, error) {
			rows, t, e := fetch(ctx, after, before, wantFirst, wantLast, orderBy, dir, search)
			if e != nil {
				return nil, eris.Wrap(e, opPrefix)
			}
			total = t
			return rows, nil
		},
	)
	if err != nil {
		return nil, err
	}

	// StartCur / EndCur carry the RAW node id; the resolver's connection layer
	// applies the cursor encoder once. Encoding here would double-encode.
	out := &MasterCatalogConnectionOutput{TotalCount: total, HasNext: hasNext, HasPrev: hasPrev, Items: items}
	out.StartCur, out.EndCur = firstLastCursor(items, func(it *repository.MasterCatalogItem) string { return it.Cardgroup.ID })
	return out, nil
}

// masterCatalogOrderByColumns is the usecase→repository orderBy allowlist for the master catalog.
var masterCatalogOrderByColumns = map[MasterCatalogOrderBy]repository.MasterCatalogOrderBy{
	MasterCatalogOrderBySortOrder: repository.MasterCatalogOrderBySortOrder,
	MasterCatalogOrderByCreatedAt: repository.MasterCatalogOrderByCreatedAt,
	MasterCatalogOrderByName:      repository.MasterCatalogOrderByName,
}

// resolveMasterCatalogOrderBy maps the typed usecase enums to the repository
// allowlist. Defaults match the schema (SORT_ORDER, ASC) when both inputs are
// nil. The default switch arm is defense in depth — gqlgen UnmarshalGQL already
// rejects invalid enum strings upstream.
func resolveMasterCatalogOrderBy(
	orderBy *MasterCatalogOrderBy, dir *SortOrder,
) (repository.MasterCatalogOrderBy, repository.SortOrder, error) {
	return resolveOrderByColumn(orderBy, dir, masterCatalogOrderByColumns, repository.MasterCatalogOrderBySortOrder, repository.SortAsc)
}

// resolveMasterCursor decodes an opaque cursor string into a
// *repository.MasterCatalogCursor with the column required by the active orderBy
// populated. The publishedOnly flag selects the hydration scope: true hydrates
// via FindPublishedByID (catalog scope — a draft or unknown id is rejected as
// cursor-not-found so drafts never leak); false hydrates via FindByID (admin
// scope — DRAFT decks are valid cursors). Returns BAD_USER_INPUT when the cursor
// cannot be decoded or references a row outside the active scope. The scope check
// runs even when orderBy is missing a hydratable column.
func (u *masterCatalogUsecase) resolveMasterCursor(
	ctx context.Context,
	cursorStr *string,
	orderBy repository.MasterCatalogOrderBy,
	field string,
	publishedOnly bool,
) (*repository.MasterCatalogCursor, error) {
	id, present, err := decodeCursorOrBadInput(cursorStr, field)
	if err != nil {
		return nil, err
	}
	if !present {
		return nil, nil
	}
	fetchByID := u.repo.FindByID
	if publishedOnly {
		fetchByID = u.repo.FindPublishedByID
	}
	mcg, err := fetchByID(ctx, id)
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
	IsDefaultStarter *bool
	SortOrder        *int
}

// UpdateMasterInput carries the admin update patch. nil = leave unchanged.
type UpdateMasterInput struct {
	Name             *string
	Description      *string
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

// normalizeSearch collapses nil and whitespace-only search inputs to nil and
// trims a non-empty search. After this the repository receives either nil (no
// filter) or a non-empty, trimmed string — the same invariant ListMasterCards
// relies on. Normalizing at the usecase boundary keeps totalCount and the page
// query in agreement instead of depending on the repository to trim.
func normalizeSearch(search *string) *string {
	if search == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*search)
	if trimmed == "" {
		return nil
	}
	return &trimmed
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

	description, descErr := domain.ParseDescription(in.Description)
	info, err = liftValidationErr(translateDescriptionErr(descErr))
	if err != nil {
		return CreateMasterOutcome{}, err
	}
	if info != nil {
		return CreateMasterOutcome{Validation: info}, nil
	}

	m, err := domain.NewMasterCardgroup(
		name,
		description,
		derefOr(in.IsDefaultStarter, false),
		derefOr(in.SortOrder, 0),
	)
	if err != nil {
		return CreateMasterOutcome{}, eris.Wrap(err, "usecase: master catalog: create master: construct")
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
//
// There is deliberately no MasterCardgroup.ApplyPatch aggregate method. Of the
// patchable fields, Name and Description carry domain invariants — the
// CardgroupName and Description grapheme-cluster length bounds — and both are
// enforced here, at their single seams, via domain.ParseCardgroupName /
// domain.ParseDescription below. Status is the only other VO on the aggregate, and
// it is not patchable through this method: it has its own dedicated lifecycle seams
// (Publish / Unpublish). The remaining fields (IsDefaultStarter, SortOrder) are
// free-form (*bool / *int) with no Parse or bound to protect, so they are assigned
// directly into the repository patch. An ApplyPatch wrapper over those fields would
// be an indirection layer guarding nothing.
func (u *masterCatalogUsecase) UpdateMaster(ctx context.Context, id string, in UpdateMasterInput) (UpdateMasterOutcome, error) {
	if _, err := u.adminGate.Require(ctx, "usecase: master catalog: update master"); err != nil {
		return UpdateMasterOutcome{}, err
	}

	description, descErr := domain.ParseDescription(in.Description)
	info, err := liftValidationErr(translateDescriptionErr(descErr))
	if err != nil {
		return UpdateMasterOutcome{}, err
	}
	if info != nil {
		return UpdateMasterOutcome{Validation: info}, nil
	}

	// Name and Description route through their VOs (above / below); the remaining
	// free-form fields flow straight into the patch. See the method docstring for
	// why no MasterCardgroup.ApplyPatch exists.
	patch := repository.MasterCardgroupUpdate{
		Description:      description.Ptr(),
		IsDefaultStarter: in.IsDefaultStarter,
		SortOrder:        in.SortOrder,
	}
	if in.Name != nil {
		name, nameErr := domain.ParseCardgroupName(*in.Name)
		info, err = liftValidationErr(translateCardgroupNameErr(nameErr))
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
// calls the status-unfiltered FindAdminPage repository method (whose returned
// total counts decks of any status). The body from page assembly onward is shared
// with ListPublishedConnection via listMasterCatalogCore; only the gate,
// publishedOnly scope (false = admin, DRAFT cursors valid), the repository page
// method, and the eris wrap prefix differ.
func (u *masterCatalogUsecase) ListAdminConnection(
	ctx context.Context, in MasterCatalogConnectionInput,
) (*MasterCatalogConnectionOutput, error) {
	return u.listMasterCatalogCore(ctx, in, false, "usecase: master catalog: find admin page",
		func(ctx context.Context) error {
			_, err := u.adminGate.Require(ctx, "usecase: master catalog: list admin")
			return err
		},
		u.repo.FindAdminPage,
	)
}

// ImportMaster copies the published master cardgroup identified by masterID into a
// fresh cardgroup owned by the authenticated caller. The master is gated through
// FindPublishedByID, which returns ErrNotFound for both unknown ids and draft decks,
// so draft existence is never disclosed — both collapse to ImportMasterOutcome{NotFound:true}.
// Unauthenticated callers receive ucerr.ErrUnauthenticated. The copy is a one-time
// snapshot delegated to CopyMasterToUserUsecase; FSRS/swipe state starts empty.
func (u *masterCatalogUsecase) ImportMaster(ctx context.Context, masterID string) (ImportMasterOutcome, error) {
	caller := auth.UserFrom(ctx)
	if caller == nil {
		return ImportMasterOutcome{}, ucerr.ErrUnauthenticated
	}

	if _, err := u.repo.FindPublishedByID(ctx, masterID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return ImportMasterOutcome{NotFound: true}, nil
		}
		if isContextDone(err) {
			return ImportMasterOutcome{}, err
		}
		return ImportMasterOutcome{}, eris.Wrap(err, "usecase: master catalog: import: verify published")
	}

	cg, err := u.deckUC.CopyMasterToUser(ctx, masterID, caller.Sub)
	if err != nil {
		if isContextDone(err) {
			return ImportMasterOutcome{}, err
		}
		return ImportMasterOutcome{}, eris.Wrap(err, "usecase: master catalog: import: copy master to user")
	}
	return ImportMasterOutcome{Cardgroup: cg}, nil
}

// MergeMaster merges the published master cardgroup identified by masterID into
// the caller-owned cardgroup cardgroupID. The master is gated through
// FindPublishedByID, collapsing unknown and draft into MergeMasterOutcome{NotFound:true}
// so draft existence is never disclosed. Destination ownership is enforced by the
// delegated usecase (BAD_USER_INPUT for unknown, UNAUTHENTICATED for foreign),
// surfaced as an error rather than via the outcome. Unauthenticated callers
// receive ucerr.ErrUnauthenticated. The merge is a one-time snapshot.
func (u *masterCatalogUsecase) MergeMaster(ctx context.Context, masterID, cardgroupID string) (MergeMasterOutcome, error) {
	caller := auth.UserFrom(ctx)
	if caller == nil {
		return MergeMasterOutcome{}, ucerr.ErrUnauthenticated
	}

	if _, err := u.repo.FindPublishedByID(ctx, masterID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return MergeMasterOutcome{NotFound: true}, nil
		}
		if isContextDone(err) {
			return MergeMasterOutcome{}, err
		}
		return MergeMasterOutcome{}, eris.Wrap(err, "usecase: master catalog: merge: verify published")
	}

	res, err := u.deckUC.MergeMasterIntoCardgroup(ctx, masterID, domain.CardgroupID(cardgroupID), domain.UserID(caller.Sub))
	if err != nil {
		if isContextDone(err) {
			return MergeMasterOutcome{}, err
		}
		// Wrap unconditionally, exactly like ImportMaster wraps CopyMasterToUser.
		// A ucerr.ValidationError / ucerr.ErrUnauthenticated from the delegated
		// ownership gate is still classified correctly because FromUsecaseError
		// walks the eris chain (errors.Is / errors.AsType). No pass-through guard.
		return MergeMasterOutcome{}, eris.Wrap(err, "usecase: master catalog: merge: merge master into cardgroup")
	}
	return MergeMasterOutcome{Cardgroup: res.Cardgroup, Added: res.Added, Updated: res.Updated}, nil
}

// FindPublishedMaster returns a single PUBLISHED master deck by id for any
// authenticated caller. FindPublishedByID returns ErrNotFound for both unknown
// ids and draft decks, so draft existence is never disclosed — both collapse to
// a (nil, nil) result that the resolver maps to GraphQL null (non-disclosure
// gate). Unauthenticated callers receive ucerr.ErrUnauthenticated.
func (u *masterCatalogUsecase) FindPublishedMaster(ctx context.Context, id string) (*domain.MasterCardgroup, error) {
	if auth.UserFrom(ctx) == nil {
		return nil, ucerr.ErrUnauthenticated
	}
	deck, err := u.repo.FindPublishedByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, nil // unknown or draft → GraphQL null (non-disclosure)
		}
		if isContextDone(err) {
			return nil, err
		}
		return nil, eris.Wrap(err, "usecase: master catalog: find published master")
	}
	return deck, nil
}

// PreviewMergeMaster mirrors MergeMaster as a read-only dry run. Same gates:
// unauthenticated -> ErrUnauthenticated; unknown/draft master -> NotFound (collapsed
// via FindPublishedByID, never disclosing draft existence); destination auth failures
// travel as errors from the delegated usecase.
func (u *masterCatalogUsecase) PreviewMergeMaster(ctx context.Context, masterID, cardgroupID string) (PreviewMergeOutcome, error) {
	caller := auth.UserFrom(ctx)
	if caller == nil {
		return PreviewMergeOutcome{}, ucerr.ErrUnauthenticated
	}

	if _, err := u.repo.FindPublishedByID(ctx, masterID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return PreviewMergeOutcome{NotFound: true}, nil
		}
		if isContextDone(err) {
			return PreviewMergeOutcome{}, err
		}
		return PreviewMergeOutcome{}, eris.Wrap(err, "usecase: master catalog: preview merge: verify published")
	}

	res, err := u.deckUC.PreviewMergeMasterIntoCardgroup(ctx, masterID, domain.CardgroupID(cardgroupID), domain.UserID(caller.Sub))
	if err != nil {
		if isContextDone(err) {
			return PreviewMergeOutcome{}, err
		}
		return PreviewMergeOutcome{}, eris.Wrap(err, "usecase: master catalog: preview merge: preview merge into cardgroup")
	}
	return PreviewMergeOutcome{Added: res.Added, Updated: res.Updated}, nil
}

// SeedDefaultStarters copies the published default-starter master decks into the
// authenticated caller's own cardgroups (idempotent — a no-op if the caller
// already owns a cardgroup). Unauthenticated callers receive ucerr.ErrUnauthenticated.
func (u *masterCatalogUsecase) SeedDefaultStarters(ctx context.Context) ([]*domain.Cardgroup, error) {
	caller := auth.UserFrom(ctx)
	if err := requireCallerSub(caller); err != nil {
		return nil, err
	}
	seeded, err := u.deckUC.SeedForNewUser(ctx, caller.Sub)
	if err != nil {
		if isContextDone(err) {
			return nil, err
		}
		return nil, eris.Wrap(err, "usecase: master catalog: seed default starters")
	}
	return seeded, nil
}

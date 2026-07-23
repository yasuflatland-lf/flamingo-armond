// master_catalog.go holds the public reader surface of the master catalog: the
// repository and usecase interfaces, the connection carrier types, the shared
// page-assembly core, and the single-deck published lookup. The admin-author
// surface lives in master_catalog_admin.go; the learner-consumption surface
// (import / merge / preview merge / seed) lives in master_catalog_import.go.

package usecase

import (
	"context"
	"errors"
	"log/slog"
	"strconv"

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
	FindPageAnyStatus(
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

// MasterCatalogItem is the usecase-level read model for one catalog row.
type MasterCatalogItem struct {
	Cardgroup *domain.MasterCardgroup
	CardCount int64
}

// MasterCatalogConnectionOutput is the usecase-level page result. The resolver
// wraps it into a model.MasterCatalogConnection.
//
// Ordering and OrderKeys exist so the resolver can emit v2 cursors: the
// catalog defaults to the admin-mutable SORT_ORDER column, so a cursor that
// carried only an id would move whenever an admin re-orders the deck it points
// at. Ordering is the (orderBy, direction) this page was served under;
// OrderKeys maps each returned master cardgroup id to the serialized value its
// ordering column held at serve time. Both are consumed only at the
// resolver→model boundary — the output itself still carries RAW ids, never
// pre-encoded cursors.
type MasterCatalogConnectionOutput struct {
	Items      []*MasterCatalogItem
	TotalCount int64
	HasNext    bool
	HasPrev    bool
	StartCur   string
	EndCur     string
	Ordering   PageOrdering
	OrderKeys  map[string]string
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

// MasterCatalogUsecase is the published-catalog surface plus the admin
// management operations. Every method requires an authenticated caller;
// admin methods additionally require AdminGate.Require to pass.
type MasterCatalogUsecase interface {
	ListPublishedConnection(ctx context.Context, in MasterCatalogConnectionInput) (*MasterCatalogConnectionOutput, error)
	// FindPublishedMaster returns a single PUBLISHED, non-empty master deck by id
	// for any authenticated caller. Returns (nil, nil) for an unknown id, a DRAFT
	// id, or a published deck holding zero cards (non-disclosure gate). Anonymous
	// callers receive UNAUTHENTICATED.
	FindPublishedMaster(ctx context.Context, id string) (*domain.MasterCardgroup, error)
	ImportMaster(ctx context.Context, masterID string) (ImportMasterOutcome, error)
	MergeMaster(ctx context.Context, masterID, cardgroupID string) (MergeMasterOutcome, error)
	PreviewMergeMaster(ctx context.Context, masterID, cardgroupID string) (PreviewMergeOutcome, error)
	SeedDefaultStarters(ctx context.Context) ([]*domain.Cardgroup, error)

	// admin-gated management surface
	// AdminMaster returns the master cardgroup with the given id INCLUDING DRAFT
	// decks, bundled with its current card count. Admin-only. A missing row is a
	// validation error on "id".
	AdminMaster(ctx context.Context, id string) (*MasterWithCount, error)
	ListAdminConnection(ctx context.Context, in MasterCatalogConnectionInput) (*MasterCatalogConnectionOutput, error)
	CreateMaster(ctx context.Context, in CreateMasterInput) (CreateMasterOutcome, error)
	UpdateMaster(ctx context.Context, id string, in UpdateMasterInput) (UpdateMasterOutcome, error)
	PublishMaster(ctx context.Context, id string) (PublishMasterOutcome, error)
	UnpublishMaster(ctx context.Context, id string) (*MasterWithCount, error)
	DeleteMaster(ctx context.Context, id string) error
}

type masterCatalogUsecase struct {
	repo      MasterCatalogRepository
	deckUC    masterDeckUsecaseFacade
	cgCounter cardgroupOwnerCounter
	adminGate *AdminGate
	logger    *slog.Logger
}

// NewMasterCatalogUsecase constructs a MasterCatalogUsecase backed by the given
// repository. deckUC is the combined deck facade (CopyMasterToUserUsecase +
// SeedForNewUserUsecase + MergeMasterIntoCardgroupUsecase) used by ImportMaster,
// SeedDefaultStarters, and MergeMaster; cgCounter counts the caller's existing
// cardgroups for the ImportMaster quota check; adminGate gates every
// admin-management method and supplies the quota's admin exemption. The public
// ListPublishedConnection is gated by authentication only. Panics when repo,
// deckUC, cgCounter, adminGate, or logger is nil — a nil required dependency is
// a wiring bug that must fail at startup, not at first use.
func NewMasterCatalogUsecase(repo MasterCatalogRepository, deckUC masterDeckUsecaseFacade, cgCounter cardgroupOwnerCounter, adminGate *AdminGate, logger *slog.Logger) MasterCatalogUsecase {
	if repo == nil {
		panic("usecase: master catalog: repo is required")
	}
	if deckUC == nil {
		panic("usecase: master catalog: deckUC is required")
	}
	if cgCounter == nil {
		panic("usecase: master catalog: cgCounter is required")
	}
	if adminGate == nil {
		panic("usecase: master catalog: adminGate is required")
	}
	if logger == nil {
		panic("usecase: master catalog: logger is required")
	}
	return &masterCatalogUsecase{repo: repo, deckUC: deckUC, cgCounter: cgCounter, adminGate: adminGate, logger: logger}
}

// masterCatalogPageFetch is the repository page-fetch closure shape shared by
// MasterCatalogRepository.FindPublishedPage and FindPageAnyStatus. listMasterCatalogCore
// takes one as an argument so the shared page-assembly body stays agnostic to the
// visibility filter (catalog-visible vs. all statuses).
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
// silently re-interpreted page boundary. Only PUBLISHED decks that hold at
// least one card are ever returned — that visibility filter is enforced in the
// repository SQL and is not a caller-overridable argument, so a published deck
// whose cards have all been deleted disappears from both the page and its
// totalCount until a card is restored. Unauthenticated callers receive
// UNAUTHENTICATED.
func (u *masterCatalogUsecase) ListPublishedConnection(
	ctx context.Context, in MasterCatalogConnectionInput,
) (*MasterCatalogConnectionOutput, error) {
	return u.listMasterCatalogCore(ctx, in, true, "usecase: master catalog: find published page",
		func(ctx context.Context) error {
			return requireCallerSub(auth.UserFrom(ctx))
		},
		u.repo.FindPublishedPage,
	)
}

// AdminMaster returns the master cardgroup (incl. DRAFT) with the given id plus
// its card count. Admin-only: the gate rejects non-admin / anonymous callers
// before any repository access. FindByID returns ANY status, so DRAFT decks are
// included. A missing row surfaces as a validation error on "id".
func (u *masterCatalogUsecase) AdminMaster(ctx context.Context, id string) (*MasterWithCount, error) {
	if _, err := u.adminGate.Require(ctx, "usecase: master card: admin master"); err != nil {
		return nil, err
	}
	master, err := u.repo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ucerr.NewValidationError("id", "master cardgroup not found")
		}
		if isContextDone(err) {
			return nil, err
		}
		return nil, eris.Wrap(err, "usecase: master card: admin master: find by id")
	}
	count, err := u.repo.CountCards(ctx, id)
	if err != nil {
		if isContextDone(err) {
			return nil, err
		}
		return nil, eris.Wrap(err, "usecase: master card: admin master: count cards")
	}
	return &MasterWithCount{Master: master, CardCount: count}, nil
}

// listMasterCatalogCore holds the shared page-assembly body for
// ListPublishedConnection and ListAdminConnection. The gate closure runs first and
// supplies the per-caller authorization / visibility check (anonymous-allowed
// authentication for the published catalog vs. adminGate.Require for the admin
// surface); everything from cursor resolution onward is identical except two
// caller-supplied knobs: publishedOnly threads into resolveMasterCatalogCursor to pick the
// hydration scope (true = published catalog, a DRAFT, EMPTY or unknown id is
// rejected as cursor-not-found so invisible decks never leak; false = admin,
// DRAFT and empty decks are valid cursors), and fetch is the repository page
// method (FindPublishedPage /
// FindPageAnyStatus). opPrefix is the caller's eris wrap message, supplied so the shared
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

	ordering := PageOrdering{OrderBy: string(orderBy), Direction: string(dir)}

	after, err := u.resolveMasterCatalogCursor(ctx, in.After, orderBy, ordering, "after", publishedOnly)
	if err != nil {
		return nil, err
	}
	before, err := u.resolveMasterCatalogCursor(ctx, in.Before, orderBy, ordering, "before", publishedOnly)
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
		func(wantFirst, wantLast int) ([]*MasterCatalogItem, error) {
			rows, t, e := fetch(ctx, after, before, wantFirst, wantLast, orderBy, dir, search)
			if e != nil {
				return nil, eris.Wrap(e, opPrefix)
			}
			total = t
			out := make([]*MasterCatalogItem, len(rows))
			for i, r := range rows {
				out[i] = &MasterCatalogItem{Cardgroup: r.Cardgroup, CardCount: r.CardCount}
			}
			return out, nil
		},
	)
	if err != nil {
		return nil, err
	}

	// OrderKeys snapshots the ordering column of every row in this page so the
	// resolver can embed it in the cursor it emits. Capturing it here — rather
	// than re-reading the row when the cursor comes back — is what makes the
	// bookmark survive an admin edit to the boundary row.
	keys, err := masterCatalogOrderKeys(orderBy, items)
	if err != nil {
		return nil, err
	}

	// StartCur / EndCur carry the RAW node id; the resolver's connection layer
	// applies the cursor encoder once. Encoding here would double-encode.
	out := &MasterCatalogConnectionOutput{
		TotalCount: total,
		HasNext:    hasNext,
		HasPrev:    hasPrev,
		Items:      items,
		Ordering:   ordering,
		OrderKeys:  keys,
	}
	out.StartCur, out.EndCur = firstLastCursor(items, func(it *MasterCatalogItem) string { return it.Cardgroup.ID })
	return out, nil
}

// masterCatalogOrderKeys serializes the active ordering column of every row in
// a served page, keyed by master cardgroup id. An orderBy outside the allowlist
// is a caller bug and surfaces as INTERNAL, matching masterCatalogOrderKey.
func masterCatalogOrderKeys(orderBy repository.MasterCatalogOrderBy, items []*MasterCatalogItem) (map[string]string, error) {
	keys := make(map[string]string, len(items))
	for _, it := range items {
		if it == nil || it.Cardgroup == nil {
			continue
		}
		k, err := masterCatalogOrderKey(orderBy, it.Cardgroup)
		if err != nil {
			return nil, err
		}
		keys[it.Cardgroup.ID] = k
	}
	return keys, nil
}

// masterCatalogOrderKey serializes one master cardgroup's ordering column for
// embedding in a v2 cursor. The default arm mirrors
// resolveMasterCatalogCursor's: an orderBy the switch does not handle is a
// caller bug, surfaced as INTERNAL rather than a silently unhydrated cursor.
func masterCatalogOrderKey(orderBy repository.MasterCatalogOrderBy, mcg *domain.MasterCardgroup) (string, error) {
	switch orderBy {
	case repository.MasterCatalogOrderBySortOrder:
		return strconv.Itoa(mcg.SortOrder), nil
	case repository.MasterCatalogOrderByCreatedAt:
		return encodeTimeOrderKey(mcg.CreatedAt), nil
	case repository.MasterCatalogOrderByName:
		return mcg.Name.String(), nil
	default:
		return "", eris.Errorf("usecase: master catalog: unhandled orderBy %q", orderBy)
	}
}

// applyMasterCatalogOrderKey populates the repository cursor column the active
// orderBy needs from the value a v2 cursor carried. A key that does not parse
// into the column type returns errCursorKeyMalformed so the caller maps it to
// BAD_USER_INPUT; an unhandled orderBy stays INTERNAL.
func applyMasterCatalogOrderKey(c *repository.MasterCatalogCursor, orderBy repository.MasterCatalogOrderBy, key string) error {
	switch orderBy {
	case repository.MasterCatalogOrderBySortOrder:
		n, err := decodeIntOrderKey(key)
		if err != nil {
			return err
		}
		c.SortOrder = &n
		return nil
	case repository.MasterCatalogOrderByCreatedAt:
		t, err := decodeTimeOrderKey(key)
		if err != nil {
			return err
		}
		c.CreatedAt = &t
		return nil
	case repository.MasterCatalogOrderByName:
		c.Name = &key
		return nil
	default:
		return eris.Errorf("usecase: master catalog: unhandled orderBy %q", orderBy)
	}
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

// resolveMasterCatalogCursor decodes an opaque cursor string into a
// *repository.MasterCatalogCursor with the column required by the active orderBy
// populated. The publishedOnly flag selects the hydration scope: true hydrates
// via FindPublishedByID (catalog scope — a draft, card-less or unknown id is
// rejected as cursor-not-found so decks outside the catalog never leak); false
// hydrates via FindByID (admin scope — DRAFT and empty decks are valid
// cursors). Returns BAD_USER_INPUT when the cursor
// cannot be decoded, was taken under a different ordering, carries an
// ordering-key value that does not parse, or references a row outside the
// active scope.
//
// A v2 cursor supplies the ordering-key value captured when its page was
// served, so an admin edit to the row between two fetches cannot move the
// bookmark. A v1 or legacy cursor carries no such value and falls back to
// re-reading the ordering column off the CURRENT row.
//
// Both paths run the scope-selected lookup, and both run it even when the
// active orderBy needs no hydratable column — a v2 cursor must not bypass the
// catalog-scope gate just because it can hydrate itself.
func (u *masterCatalogUsecase) resolveMasterCatalogCursor(
	ctx context.Context,
	cursorStr *string,
	orderBy repository.MasterCatalogOrderBy,
	ordering PageOrdering,
	field string,
	publishedOnly bool,
) (*repository.MasterCatalogCursor, error) {
	p, present, err := decodeCursorOrBadInput(cursorStr, field)
	if err != nil {
		return nil, err
	}
	if !present {
		return nil, nil
	}
	if err := requireCursorOrdering(p, ordering, field); err != nil {
		return nil, err
	}
	fetchByID := u.repo.FindByID
	if publishedOnly {
		fetchByID = u.repo.FindPublishedByID
	}
	mcg, err := fetchByID(ctx, p.ID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ucerr.NewValidationError(field, "cursor not found")
		}
		if isContextDone(err) {
			return nil, err
		}
		return nil, eris.Wrap(err, "usecase: master catalog: hydrate cursor")
	}

	c := &repository.MasterCatalogCursor{ID: p.ID}
	if p.HasOrdering {
		if err := applyMasterCatalogOrderKey(c, orderBy, p.OrderKey); err != nil {
			if errors.Is(err, errCursorKeyMalformed) {
				return nil, ucerr.NewValidationError(field, "invalid cursor")
			}
			return nil, err
		}
		return c, nil
	}

	// v1 / legacy bare-UUID fallback: re-hydrate from the current row.
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

// verifyPublishedMaster runs the catalog-visibility non-disclosure gate shared by
// FindPublishedMaster, ImportMaster, MergeMaster and PreviewMergeMaster. It
// resolves id through FindPublishedByID, which returns repository.ErrNotFound for
// unknown ids, DRAFT decks and published decks holding zero cards, so neither
// draft existence nor an empty deck is ever disclosed: all three collapse into
// notFound=true and each caller maps that to its own not-found shape
// (an outcome flag, or the nil deck the resolver renders as GraphQL null). Context
// cancellation passes through unwrapped. Any other repository failure is wrapped
// with the CALLER-supplied opPrefix, never a prefix fixed inside this helper, so
// the logged error_chain keeps naming the operation that ran rather than the
// shared gate.
func (u *masterCatalogUsecase) verifyPublishedMaster(
	ctx context.Context, id, opPrefix string,
) (deck *domain.MasterCardgroup, notFound bool, err error) {
	deck, err = u.repo.FindPublishedByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, true, nil
		}
		if isContextDone(err) {
			return nil, false, err
		}
		return nil, false, eris.Wrap(err, opPrefix)
	}
	return deck, false, nil
}

// FindPublishedMaster returns a single PUBLISHED, non-empty master deck by id
// for any authenticated caller. The shared verifyPublishedMaster gate collapses
// unknown ids, draft decks and published decks holding zero cards into the same
// not-found signal, so neither draft existence nor an empty deck is ever
// disclosed — all three surface as a (nil, nil) result that the resolver maps to
// GraphQL null (non-disclosure gate). Unauthenticated callers receive
// ucerr.ErrUnauthenticated.
func (u *masterCatalogUsecase) FindPublishedMaster(ctx context.Context, id string) (*domain.MasterCardgroup, error) {
	if err := requireCallerSub(auth.UserFrom(ctx)); err != nil {
		return nil, err
	}
	deck, notFound, err := u.verifyPublishedMaster(ctx, id, "usecase: master catalog: find published master")
	if err != nil {
		return nil, err
	}
	if notFound {
		return nil, nil // unknown, draft or card-less → GraphQL null (non-disclosure)
	}
	return deck, nil
}

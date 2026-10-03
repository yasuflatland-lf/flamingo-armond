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
		after *repository.MasterCatalogCursor,
		first int,
		search *string,
	) ([]*repository.MasterCatalogItem, int64, error)
	FindPublishedByID(ctx context.Context, id string) (*domain.MasterCardgroup, error)

	// --- admin (new) ---
	FindByID(ctx context.Context, id string) (*domain.MasterCardgroup, error)
	FindPageAnyStatus(
		ctx context.Context,
		after *repository.MasterCatalogCursor,
		first int,
		search *string,
	) ([]*repository.MasterCatalogItem, int64, error)
	CountCards(ctx context.Context, masterCardgroupID string) (int64, error)
	Create(ctx context.Context, m *domain.MasterCardgroup) error
	Update(ctx context.Context, id string, patch repository.MasterCardgroupUpdate) (*domain.MasterCardgroup, error)
	Delete(ctx context.Context, id string) error
	Publish(ctx context.Context, id string) (*domain.MasterCardgroup, error)
	Unpublish(ctx context.Context, id string) (*domain.MasterCardgroup, error)
}

// masterCatalogOrdering is the one fixed ordering of both catalog connections
// (masterCatalog and adminMasters), (sort_order ASC, id ASC). The tokens match
// what earlier v2 cursors embedded for the then-default ordering, so those
// cursors keep paging.
var masterCatalogOrdering = PageOrdering{OrderBy: "sort_order", Direction: "ASC"}

// MasterCatalogConnectionInput captures the GraphQL pagination arguments for
// masterCatalog. Pointer fields preserve "absent" semantics from the schema so
// the usecase can default unset values explicitly.
type MasterCatalogConnectionInput struct {
	First  *int
	After  *string // raw GraphQL ID string (cursor = master cardgroup UUID)
	Search *string
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
// catalog orders by the admin-mutable sort_order column, so a cursor that
// carried only an id would move whenever an admin re-orders the deck it points
// at. Ordering is the fixed (orderBy, direction) this page was served under;
// OrderKeys maps each returned master cardgroup id to the serialized sort_order
// it held at serve time. Both are consumed only at the resolver→model
// boundary — the output itself still carries RAW ids, never pre-encoded
// cursors.
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
	// for any authenticated caller, bundled with its live card count. Returns
	// (nil, nil) for an unknown id, a DRAFT id, or a published deck holding zero
	// cards (non-disclosure gate). Anonymous callers receive UNAUTHENTICATED.
	FindPublishedMaster(ctx context.Context, id string) (*MasterWithCount, error)
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
	adminGate *AdminGate
	logger    *slog.Logger
}

// NewMasterCatalogUsecase constructs a MasterCatalogUsecase backed by the given
// repository. deckUC is the combined deck facade (CopyMasterToUserUsecase +
// SeedForNewUserUsecase + MergeMasterIntoCardgroupUsecase) used by ImportMaster,
// SeedDefaultStarters, and MergeMaster; adminGate gates every admin-management
// method and supplies the ImportMaster quota's admin exemption; the quota itself
// is enforced inside the copy transaction. The public ListPublishedConnection is
// gated by authentication only. Panics when repo, deckUC, adminGate, or logger
// is nil — a nil required dependency is a wiring bug that must fail at startup.
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
// MasterCatalogRepository.FindPublishedPage and FindPageAnyStatus. listMasterCatalogCore
// takes one as an argument so the shared page-assembly body stays agnostic to the
// visibility filter (catalog-visible vs. all statuses).
type masterCatalogPageFetch func(
	ctx context.Context,
	after *repository.MasterCatalogCursor,
	first int,
	search *string,
) ([]*repository.MasterCatalogItem, int64, error)

// ListPublishedConnection paginates the published master catalog with
// forward-only Relay-style cursors (first, after). An `after` without a
// positive `first` is rejected with BAD_USER_INPUT before the repository is
// touched (see validateRelayArgs). Only PUBLISHED decks that hold at least one
// card are ever returned — that visibility filter is enforced in the repository
// SQL and is not a caller-overridable argument, so a published deck whose cards
// have all been deleted disappears from both the page and its totalCount until
// a card is restored. Unauthenticated callers receive UNAUTHENTICATED.
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

	first, err := resolveRelayPage(in.First, in.After, resolveStandardPageSize)
	if err != nil {
		return nil, err
	}

	after, err := u.resolveMasterCatalogCursor(ctx, in.After, "after", publishedOnly)
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
	items, hasNext, hasPrev, err := assemblePage(first, after != nil,
		func(want int) ([]*MasterCatalogItem, error) {
			rows, t, e := fetch(ctx, after, want, search)
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
	// StartCur / EndCur carry the RAW node id; the resolver's connection layer
	// applies the cursor encoder once. Encoding here would double-encode.
	out := &MasterCatalogConnectionOutput{
		TotalCount: total,
		HasNext:    hasNext,
		HasPrev:    hasPrev,
		Items:      items,
		Ordering:   masterCatalogOrdering,
		OrderKeys:  masterCatalogOrderKeys(items),
	}
	out.StartCur, out.EndCur = firstLastCursor(items, func(it *MasterCatalogItem) string { return it.Cardgroup.ID })
	return out, nil
}

// masterCatalogOrderKeys serializes the sort_order ordering column of every row
// in a served page, keyed by master cardgroup id.
func masterCatalogOrderKeys(items []*MasterCatalogItem) map[string]string {
	keys := make(map[string]string, len(items))
	for _, it := range items {
		if it == nil || it.Cardgroup == nil {
			continue
		}
		keys[it.Cardgroup.ID] = masterCatalogOrderKey(it.Cardgroup)
	}
	return keys
}

// masterCatalogOrderKey serializes one master cardgroup's sort_order for
// embedding in a v2 cursor.
func masterCatalogOrderKey(mcg *domain.MasterCardgroup) string {
	return strconv.Itoa(mcg.SortOrder)
}

// applyMasterCatalogOrderKey populates the repository cursor's SortOrder from
// the value a v2 cursor carried. A key that does not parse returns
// errCursorKeyMalformed so the caller maps it to BAD_USER_INPUT.
func applyMasterCatalogOrderKey(c *repository.MasterCatalogCursor, key string) error {
	n, err := decodeIntOrderKey(key)
	if err != nil {
		return err
	}
	c.SortOrder = &n
	return nil
}

// resolveMasterCatalogCursor decodes an opaque cursor string into a
// *repository.MasterCatalogCursor with the sort_order ordering column
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
// Both paths run the scope-selected lookup — a v2 cursor must not bypass the
// catalog-scope gate just because it can hydrate itself.
func (u *masterCatalogUsecase) resolveMasterCatalogCursor(
	ctx context.Context,
	cursorStr *string,
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
	if err := requireCursorOrdering(p, masterCatalogOrdering, field); err != nil {
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
		return nil, wrapInfraErr(err, "usecase: master catalog: hydrate cursor")
	}

	c := &repository.MasterCatalogCursor{ID: p.ID}
	if p.HasOrdering {
		if err := applyMasterCatalogOrderKey(c, p.OrderKey); err != nil {
			return nil, ucerr.NewValidationError(field, "invalid cursor")
		}
		return c, nil
	}

	// v1 / legacy bare-UUID fallback: re-hydrate from the current row.
	so := mcg.SortOrder
	c.SortOrder = &so
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
		return nil, false, wrapInfraErr(err, opPrefix)
	}
	return deck, false, nil
}

// FindPublishedMaster returns a single PUBLISHED, non-empty master deck by id
// for any authenticated caller, bundled with its live card count. The shared
// verifyPublishedMaster gate collapses unknown ids, draft decks and published
// decks holding zero cards into the same not-found signal, so neither draft
// existence nor an empty deck is ever disclosed — all three surface as a
// (nil, nil) result that the resolver maps to GraphQL null (non-disclosure
// gate). Unauthenticated callers receive ucerr.ErrUnauthenticated.
func (u *masterCatalogUsecase) FindPublishedMaster(ctx context.Context, id string) (*MasterWithCount, error) {
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
	count, err := u.repo.CountCards(ctx, id)
	if err != nil {
		return nil, wrapInfraErr(err, "usecase: master catalog: find published master: count cards")
	}
	return &MasterWithCount{Master: deck, CardCount: count}, nil
}

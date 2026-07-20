// Package usecase wires authorization, repository calls, and domain rules
// behind GraphQL resolvers. Resolvers should not import repository directly.
package usecase

import (
	"context"
	"errors"
	"log/slog"

	"github.com/rotisserie/eris"

	"backend/internal/auth"
	"backend/internal/domain"
	"backend/internal/repository"
	"backend/internal/usecase/ucerr"
)

// CardgroupRepository is the consumer-driven interface used by CardgroupUsecase.
// FindByIDs is intentionally omitted; it is used only by the loader layer.
type CardgroupRepository interface {
	FindByID(ctx context.Context, id string) (*domain.Cardgroup, error)
	FindPageByOwner(
		ctx context.Context,
		ownerID string,
		after, before *repository.CardgroupCursor,
		first, last int,
		orderBy repository.CardgroupOrderBy,
		dir repository.SortOrder,
		search *string,
	) ([]*domain.Cardgroup, int64, error)
	CountByOwner(ctx context.Context, ownerID string, search *string) (int64, error)
	Create(ctx context.Context, cg *domain.Cardgroup) error
	Update(ctx context.Context, id string, patch repository.CardgroupUpdate) (*domain.Cardgroup, error)
	Delete(ctx context.Context, id string) error
}

// CardgroupOrderBy mirrors the schema CardgroupOrderBy enum but stays in
// the usecase layer so the repository remains independent of the GraphQL
// model package. The string values are identical to model.CardgroupOrderBy
// so the resolver can convert with a direct cast.
type CardgroupOrderBy string

const (
	CardgroupOrderByID        CardgroupOrderBy = "ID"
	CardgroupOrderByCreatedAt CardgroupOrderBy = "CREATED_AT"
	CardgroupOrderByUpdatedAt CardgroupOrderBy = "UPDATED_AT"
	CardgroupOrderByName      CardgroupOrderBy = "NAME"
)

// CardgroupConnectionInput captures the GraphQL pagination arguments for
// myCardgroupsConnection. Pointer fields preserve "absent" semantics from
// the schema so the usecase can default unset values explicitly.
type CardgroupConnectionInput struct {
	First, Last    *int
	After, Before  *string // raw GraphQL ID strings (cursor = cardgroup UUID)
	Search         *string
	OrderBy        *CardgroupOrderBy
	OrderDirection *SortOrder
}

// CardgroupConnectionOutput is the usecase-level page result. The resolver
// wraps it into a model.CardgroupConnection.
//
// Ordering and OrderKeys exist so the resolver can emit v2 cursors: the
// cardgroup listing defaults to the mutable UPDATED_AT column, so a cursor
// that carried only an id would move whenever the row it points at is edited.
// Ordering is the (orderBy, direction) this page was served under; OrderKeys
// maps each returned cardgroup id to the serialized value its ordering column
// held at serve time (empty string when the ordering key IS the id). Both are
// consumed only at the resolver→model boundary — the output itself still
// carries RAW ids, never pre-encoded cursors.
type CardgroupConnectionOutput struct {
	Cardgroups []*domain.Cardgroup
	TotalCount int64
	HasNext    bool
	HasPrev    bool
	StartCur   string
	EndCur     string
	Ordering   PageOrdering
	OrderKeys  map[string]string
}

// CardgroupUsecase is the cardgroup CRUD and paginated-list surface.
type CardgroupUsecase interface {
	Cardgroup(ctx context.Context, id string) (*domain.Cardgroup, error)
	Create(ctx context.Context, in CreateCardgroupInput) (CreateCardgroupOutcome, error)
	Update(ctx context.Context, id string, in UpdateCardgroupInput) (UpdateCardgroupOutcome, error)
	Delete(ctx context.Context, id string) error
	ListCardgroupsByOwnerConnection(ctx context.Context, in CardgroupConnectionInput) (*CardgroupConnectionOutput, error)
}

type cardgroupUsecase struct {
	repo   CardgroupRepository
	admin  AdminChecker
	logger *slog.Logger
}

// NewCardgroupUsecase constructs a CardgroupUsecase backed by the given repository.
func NewCardgroupUsecase(repo CardgroupRepository, admin AdminChecker, logger *slog.Logger) CardgroupUsecase {
	if admin == nil {
		panic("usecase: cardgroup: admin checker is required")
	}
	if logger == nil {
		panic("usecase: cardgroup: logger is required")
	}
	return &cardgroupUsecase{repo: repo, admin: admin, logger: logger}
}

// Cardgroup returns a single cardgroup by id. A missing row and a row owned by
// another user both return (nil, nil) so the nullable GraphQL field resolves to
// null with no error in either case. Collapsing the two into a byte-identical
// response stops an authenticated caller from using the query as an existence
// oracle over other users' cardgroup ids — the same non-disclosure collapse the
// write paths make via authorizeCardgroupOrUnauthenticated and that
// setLastViewedCardgroup makes for "not found or not owned".
func (u *cardgroupUsecase) Cardgroup(ctx context.Context, id string) (*domain.Cardgroup, error) {
	user := auth.UserFrom(ctx)
	if err := requireCallerSub(user); err != nil {
		return nil, err
	}
	cg, err := u.repo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, nil
		}
		return nil, eris.Wrap(err, "usecase: cardgroup: find by id")
	}
	if !cg.IsOwnedBy(domain.UserID(user.Sub)) {
		// Foreign-owned reads collapse to the same (nil, nil) not-found shape as
		// a missing row so existence is not leaked.
		return nil, nil
	}
	return cg, nil
}

// CreateCardgroupInput carries the fields required to create a new cardgroup.
type CreateCardgroupInput struct{ Name string }

// CreateCardgroupOutcome is the result of CardgroupUsecase.Create. Exactly one
// of Cardgroup, Validation, or LimitReached is non-nil on a nil-error return: a
// successful insert carries the new Cardgroup; a name that fails validation
// surfaces via Validation so the resolver maps it to the CreateCardgroupResult
// union's InputValidationError variant; a non-admin owner who already holds the
// per-user cardgroup limit surfaces via LimitReached.
type CreateCardgroupOutcome struct {
	Cardgroup    *domain.Cardgroup
	Validation   *InputValidationInfo
	LimitReached *CardgroupLimitInfo
}

// CardgroupLimitInfo reports that the authenticated owner has reached the
// per-user cardgroup limit. Limit is the cap; Current is the owner's count at
// the time the create was rejected. Current is always >= Limit when this
// struct is constructed.
type CardgroupLimitInfo struct {
	Limit   int
	Current int
}

// cardgroupOwnerCounter is the narrow surface checkCardgroupLimit needs.
// Keeping it separate from CardgroupRepository lets tests stub only the
// counting method.
type cardgroupOwnerCounter interface {
	CountByOwner(ctx context.Context, ownerID string, search *string) (int64, error)
}

// checkCardgroupLimit returns a non-nil *CardgroupLimitInfo when a non-admin
// owner already holds domain.GeneralUserCardgroupLimit cardgroups. Admins are
// exempt (returns nil, nil without counting). Context cancellation and deadline
// errors pass through unwrapped; other infrastructure failures propagate wrapped.
func checkCardgroupLimit(ctx context.Context, counter cardgroupOwnerCounter, admin AdminChecker, ownerID string) (*CardgroupLimitInfo, error) {
	isAdmin, err := admin.IsAdmin(ctx, ownerID)
	if err != nil {
		if isContextDone(err) {
			return nil, err
		}
		return nil, eris.Wrap(err, "usecase: cardgroup: check admin")
	}
	if isAdmin {
		return nil, nil
	}
	count, err := counter.CountByOwner(ctx, ownerID, nil)
	if err != nil {
		if isContextDone(err) {
			return nil, err
		}
		return nil, eris.Wrap(err, "usecase: cardgroup: count by owner")
	}
	if domain.GeneralUserCardgroupQuotaReached(count) {
		return &CardgroupLimitInfo{Limit: domain.GeneralUserCardgroupLimit, Current: int(count)}, nil
	}
	return nil, nil
}

// Create creates a new cardgroup owned by the authenticated caller.
func (u *cardgroupUsecase) Create(ctx context.Context, in CreateCardgroupInput) (CreateCardgroupOutcome, error) {
	user := auth.UserFrom(ctx)
	if err := requireCallerSub(user); err != nil {
		return CreateCardgroupOutcome{}, err
	}

	name, nameErr := domain.ParseCardgroupName(in.Name)
	info, err := liftValidationErr(translateCardgroupNameErr(nameErr))
	if err != nil {
		return CreateCardgroupOutcome{}, err
	}
	if info != nil {
		return CreateCardgroupOutcome{Validation: info}, nil
	}

	// Name validation (no DB) runs first to avoid the IsAdmin + Count queries
	// on the common invalid-name path. Non-admins are capped at
	// domain.GeneralUserCardgroupLimit cardgroups; admins are exempt.
	limit, err := checkCardgroupLimit(ctx, u.repo, u.admin, user.Sub)
	if err != nil {
		return CreateCardgroupOutcome{}, err
	}
	if limit != nil {
		return CreateCardgroupOutcome{LimitReached: limit}, nil
	}

	cg, err := domain.NewCardgroup(domain.UserID(user.Sub), name)
	if err != nil {
		return CreateCardgroupOutcome{}, eris.Wrap(err, "usecase: cardgroup: new cardgroup")
	}

	if err := u.repo.Create(ctx, cg); err != nil {
		// The owner FK no longer resolves: the caller's account was deleted while
		// their JWT was still valid. Surface UNAUTHENTICATED so the client signs
		// them out instead of paging an operator with an INTERNAL error.
		if errors.Is(err, repository.ErrCardgroupOwnerNotFound) {
			return CreateCardgroupOutcome{}, ucerr.ErrUnauthenticated
		}
		if translated := translateTextLengthViolation(err); translated != nil {
			info, lerr := liftValidationErr(translated)
			if lerr != nil {
				return CreateCardgroupOutcome{}, lerr
			}
			return CreateCardgroupOutcome{Validation: info}, nil
		}
		return CreateCardgroupOutcome{}, eris.Wrap(err, "usecase: cardgroup: create")
	}
	return CreateCardgroupOutcome{Cardgroup: cg}, nil
}

// UpdateCardgroupInput carries the fields that may be patched on an existing cardgroup.
type UpdateCardgroupInput struct{ Name *string }

// UpdateCardgroupOutcome is the result of CardgroupUsecase.Update. Exactly one
// of Cardgroup or Validation is non-nil on a nil-error return: a successful
// patch carries the updated Cardgroup; a name that fails validation surfaces
// via Validation so the resolver maps it to the UpdateCardgroupResult union's
// InputValidationError variant.
type UpdateCardgroupOutcome struct {
	Cardgroup  *domain.Cardgroup
	Validation *InputValidationInfo
}

// Update applies a partial patch to the cardgroup identified by id.
// Non-owners and missing rows both return UNAUTHENTICATED to prevent ID enumeration.
// A nil Name field is treated as an empty patch: the existing row is returned
// without any database write.
func (u *cardgroupUsecase) Update(ctx context.Context, id string, in UpdateCardgroupInput) (UpdateCardgroupOutcome, error) {
	user := auth.UserFrom(ctx)
	if err := requireCallerSub(user); err != nil {
		return UpdateCardgroupOutcome{}, err
	}

	// Ownership classification (not-found / not-owned / context-done) is routed
	// through the shared ownership.go seam. findOwnedCardgroup returns the loaded
	// row so the patch below can operate on it without a second lookup; a missing
	// row surfaces as UNAUTHENTICATED to prevent ID enumeration.
	existing, err := findOwnedCardgroup(ctx, u.repo, domain.CardgroupID(id), domain.UserID(user.Sub), ucerr.ErrUnauthenticated)
	if err != nil {
		return UpdateCardgroupOutcome{}, err
	}

	// Empty patch: no DB write, return current row.
	if in.Name == nil {
		return UpdateCardgroupOutcome{Cardgroup: existing}, nil
	}

	name, nameErr := domain.ParseCardgroupName(*in.Name)
	info, err := liftValidationErr(translateCardgroupNameErr(nameErr))
	if err != nil {
		return UpdateCardgroupOutcome{}, err
	}
	if info != nil {
		return UpdateCardgroupOutcome{Validation: info}, nil
	}

	if err := existing.Rename(name); err != nil {
		return UpdateCardgroupOutcome{}, eris.Wrap(err, "usecase: cardgroup: rename")
	}

	nameStr := existing.Name.String()
	updated, err := u.repo.Update(ctx, id, repository.CardgroupUpdate{Name: &nameStr})
	if err != nil {
		if translated := translateTextLengthViolation(err); translated != nil {
			info, lerr := liftValidationErr(translated)
			if lerr != nil {
				return UpdateCardgroupOutcome{}, lerr
			}
			return UpdateCardgroupOutcome{Validation: info}, nil
		}
		return UpdateCardgroupOutcome{}, eris.Wrap(err, "usecase: cardgroup: update")
	}
	return UpdateCardgroupOutcome{Cardgroup: updated}, nil
}

// Delete removes the cardgroup identified by id.
// Non-owners and missing rows both return UNAUTHENTICATED to prevent ID enumeration.
func (u *cardgroupUsecase) Delete(ctx context.Context, id string) error {
	user := auth.UserFrom(ctx)
	if err := requireCallerSub(user); err != nil {
		return err
	}

	// Ownership classification (not-found / not-owned / context-done) is routed
	// through the shared ownership.go seam; a missing or foreign row surfaces as
	// UNAUTHENTICATED to prevent ID enumeration.
	if err := authorizeCardgroupOrUnauthenticated(ctx, u.repo, domain.CardgroupID(id), domain.UserID(user.Sub)); err != nil {
		return err
	}

	if err := u.repo.Delete(ctx, id); err != nil {
		return eris.Wrap(err, "usecase: cardgroup: delete")
	}
	return nil
}

// ListCardgroupsByOwnerConnection paginates the authenticated caller's
// cardgroups with Relay-style cursors. Forward paging uses (first, after);
// backward uses (last, before). The five mixed-direction combinations are
// rejected with BAD_USER_INPUT before the repository is touched so the
// caller never gets a silently re-interpreted page boundary. Cursors that
// reference a cardgroup belonging to another owner are also rejected as
// BAD_USER_INPUT (returning UNAUTHENTICATED would leak existence of other
// users' cardgroups).
func (u *cardgroupUsecase) ListCardgroupsByOwnerConnection(
	ctx context.Context, in CardgroupConnectionInput,
) (*CardgroupConnectionOutput, error) {
	user := auth.UserFrom(ctx)
	if err := requireCallerSub(user); err != nil {
		return nil, err
	}

	first, last, err := resolveRelayPage(in.First, in.Last, in.After, in.Before, resolveStandardPageSize)
	if err != nil {
		return nil, err
	}

	orderBy, dir, err := resolveCardgroupOrderBy(in.OrderBy, in.OrderDirection)
	if err != nil {
		return nil, err
	}

	ordering := PageOrdering{OrderBy: string(orderBy), Direction: string(dir)}

	after, err := u.resolveCardgroupCursor(ctx, in.After, user.Sub, orderBy, ordering, "after")
	if err != nil {
		return nil, err
	}
	before, err := u.resolveCardgroupCursor(ctx, in.Before, user.Sub, orderBy, ordering, "before")
	if err != nil {
		return nil, err
	}

	// totalCount comes from FindPageByOwner's COUNT(*) over the same filtered
	// base query, captured inside the fetch closure so it honours the active
	// search rather than an unfiltered owner total. assemblePage always invokes
	// fetch (even for a totalCount-only request), so total is set on every path.
	var total int64
	cgs, hasNext, hasPrev, err := assemblePage(first, last, after != nil, before != nil,
		func(wantFirst, wantLast int) ([]*domain.Cardgroup, error) {
			rows, t, e := u.repo.FindPageByOwner(
				ctx, user.Sub, after, before, wantFirst, wantLast, orderBy, dir, in.Search,
			)
			if e != nil {
				return nil, eris.Wrap(e, "usecase: cardgroup: find page by owner")
			}
			total = t
			return rows, nil
		},
	)
	if err != nil {
		return nil, err
	}

	// OrderKeys snapshots the ordering column of every row in this page so the
	// resolver can embed it in the cursor it emits. Capturing it here — rather
	// than re-reading the row when the cursor comes back — is what makes the
	// bookmark survive an edit to the boundary row.
	keys, err := cardgroupOrderKeys(orderBy, cgs)
	if err != nil {
		return nil, err
	}

	out := &CardgroupConnectionOutput{
		TotalCount: total,
		HasNext:    hasNext,
		HasPrev:    hasPrev,
		Cardgroups: cgs,
		Ordering:   ordering,
		OrderKeys:  keys,
	}
	out.StartCur, out.EndCur = firstLastCursor(cgs, func(cg *domain.Cardgroup) string { return string(cg.ID) })
	return out, nil
}

// cardgroupOrderKeys serializes the active ordering column of every row in a
// served page, keyed by cardgroup id. An orderBy outside the allowlist is a
// caller bug and surfaces as INTERNAL, matching cardgroupOrderKey.
func cardgroupOrderKeys(orderBy repository.CardgroupOrderBy, cgs []*domain.Cardgroup) (map[string]string, error) {
	keys := make(map[string]string, len(cgs))
	for _, cg := range cgs {
		k, err := cardgroupOrderKey(orderBy, cg)
		if err != nil {
			return nil, err
		}
		keys[string(cg.ID)] = k
	}
	return keys, nil
}

// cardgroupOrderKey serializes one cardgroup's ordering column for embedding
// in a v2 cursor. Ordering by ID needs no key — the id is already carried by
// the cursor — so it returns the empty string. The default arm mirrors
// resolveCardgroupCursor's: an orderBy the switch does not handle is a caller
// bug, surfaced as INTERNAL rather than a silently unhydrated cursor.
func cardgroupOrderKey(orderBy repository.CardgroupOrderBy, cg *domain.Cardgroup) (string, error) {
	switch orderBy {
	case repository.CardgroupOrderByID:
		return "", nil
	case repository.CardgroupOrderByName:
		return cg.Name.String(), nil
	case repository.CardgroupOrderByCreatedAt:
		return encodeTimeOrderKey(cg.CreatedAt), nil
	case repository.CardgroupOrderByUpdatedAt:
		return encodeTimeOrderKey(cg.UpdatedAt), nil
	default:
		return "", eris.Errorf("usecase: cardgroup: unhandled orderBy %q", orderBy)
	}
}

// applyCardgroupOrderKey populates the repository cursor column the active
// orderBy needs from the value a v2 cursor carried. A key that does not parse
// into the column type wraps errCursorKeyMalformed so the caller maps it to
// BAD_USER_INPUT; an unhandled orderBy stays INTERNAL.
func applyCardgroupOrderKey(c *repository.CardgroupCursor, orderBy repository.CardgroupOrderBy, key string) error {
	switch orderBy {
	case repository.CardgroupOrderByID:
		// No extra column needed; the id in the cursor is the ordering key.
		return nil
	case repository.CardgroupOrderByName:
		c.Name = &key
		return nil
	case repository.CardgroupOrderByCreatedAt:
		t, err := decodeTimeOrderKey(key)
		if err != nil {
			return err
		}
		c.CreatedAt = &t
		return nil
	case repository.CardgroupOrderByUpdatedAt:
		t, err := decodeTimeOrderKey(key)
		if err != nil {
			return err
		}
		c.UpdatedAt = &t
		return nil
	default:
		return eris.Errorf("usecase: cardgroup: unhandled orderBy %q", orderBy)
	}
}

// cardgroupOrderByColumns is the usecase→repository orderBy allowlist for cardgroups.
var cardgroupOrderByColumns = map[CardgroupOrderBy]repository.CardgroupOrderBy{
	CardgroupOrderByID:        repository.CardgroupOrderByID,
	CardgroupOrderByCreatedAt: repository.CardgroupOrderByCreatedAt,
	CardgroupOrderByUpdatedAt: repository.CardgroupOrderByUpdatedAt,
	CardgroupOrderByName:      repository.CardgroupOrderByName,
}

// resolveCardgroupOrderBy maps the typed usecase enums to the repository
// allowlist. Defaults match the schema (UPDATED_AT, DESC) when both inputs
// are nil. The default switch arm is defense in depth — gqlgen
// UnmarshalGQL already rejects invalid enum strings upstream.
func resolveCardgroupOrderBy(
	orderBy *CardgroupOrderBy, dir *SortOrder,
) (repository.CardgroupOrderBy, repository.SortOrder, error) {
	return resolveOrderByColumn(orderBy, dir, cardgroupOrderByColumns, repository.CardgroupOrderByUpdatedAt, repository.SortDesc)
}

// resolveCardgroupCursor decodes an opaque cursor string into a
// *repository.CardgroupCursor with the column required by the active orderBy
// populated. The cursor may be a v2 envelope ("v2:" + base64 JSON), a v1
// envelope ("v1:" + base64), or a legacy bare UUID; all three are accepted.
// Returns BAD_USER_INPUT when the cursor cannot be decoded, was taken under a
// different ordering, carries an ordering-key value that does not parse, the
// cardgroup cannot be found, or the cardgroup belongs to another owner — the
// last would otherwise leak existence of cardgroups outside the caller's tenant.
//
// A v2 cursor supplies the ordering-key value captured when its page was
// served, so an edit to the row between two fetches cannot move the bookmark.
// A v1 or legacy cursor carries no such value and falls back to re-reading the
// ordering column off the CURRENT row; that fallback is what duplicates or
// skips rows when the ordering column is mutable, and it exists only so
// cursors persisted by older clients keep paging.
//
// Both paths run the same repository lookup and cross-tenant check, and both
// run it even when orderBy is ID (no extra column to hydrate). Without it, an
// attacker could probe for the existence of foreign cardgroups by paging past
// a guessed cursor and observing whether any rows come back — a v2 cursor must
// not bypass that gate just because it can hydrate itself.
func (u *cardgroupUsecase) resolveCardgroupCursor(
	ctx context.Context,
	cursorStr *string,
	ownerID string,
	orderBy repository.CardgroupOrderBy,
	ordering PageOrdering,
	field string,
) (*repository.CardgroupCursor, error) {
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
	cg, err := u.repo.FindByID(ctx, p.ID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ucerr.NewValidationError(field, "cursor not found")
		}
		if isContextDone(err) {
			return nil, err
		}
		return nil, eris.Wrap(err, "usecase: cardgroup: hydrate cursor")
	}
	if !cg.IsOwnedBy(domain.UserID(ownerID)) {
		return nil, ucerr.NewValidationError(field, "cursor not found")
	}

	c := &repository.CardgroupCursor{ID: p.ID}
	if p.HasOrdering {
		if err := applyCardgroupOrderKey(c, orderBy, p.OrderKey); err != nil {
			if errors.Is(err, errCursorKeyMalformed) {
				return nil, ucerr.NewValidationError(field, "invalid cursor")
			}
			return nil, err
		}
		return c, nil
	}

	// v1 / legacy bare-UUID fallback: re-hydrate from the current row.
	switch orderBy {
	case repository.CardgroupOrderByID:
		// No extra column needed; ownership-check above is the gate.
	case repository.CardgroupOrderByName:
		name := cg.Name.String()
		c.Name = &name
	case repository.CardgroupOrderByCreatedAt:
		ca := cg.CreatedAt
		c.CreatedAt = &ca
	case repository.CardgroupOrderByUpdatedAt:
		ua := cg.UpdatedAt
		c.UpdatedAt = &ua
	default:
		return nil, eris.Errorf("usecase: cardgroup: unhandled orderBy %q", orderBy)
	}
	return c, nil
}

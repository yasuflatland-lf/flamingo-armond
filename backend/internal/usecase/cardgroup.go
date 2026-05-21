// Package usecase wires authorization, repository calls, and domain rules
// behind GraphQL resolvers. Resolvers should not import repository directly.
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
	) ([]*domain.Cardgroup, error)
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
type CardgroupConnectionOutput struct {
	Cardgroups []*domain.Cardgroup
	TotalCount int64
	HasNext    bool
	HasPrev    bool
	StartCur   string
	EndCur     string
}

// CardgroupUsecase is the application interface for cardgroup-related operations.
type CardgroupUsecase interface {
	Cardgroup(ctx context.Context, id string) (*domain.Cardgroup, error)
	Create(ctx context.Context, in CreateCardgroupInput) (CreateCardgroupOutcome, error)
	Update(ctx context.Context, id string, in UpdateCardgroupInput) (UpdateCardgroupOutcome, error)
	Delete(ctx context.Context, id string) error
	ListCardgroupsByOwnerConnection(ctx context.Context, in CardgroupConnectionInput) (*CardgroupConnectionOutput, error)
}

type cardgroupUsecase struct {
	repo   CardgroupRepository
	logger *slog.Logger
}

// NewCardgroupUsecase constructs a CardgroupUsecase backed by the given repository.
func NewCardgroupUsecase(repo CardgroupRepository, logger *slog.Logger) CardgroupUsecase {
	if logger == nil {
		panic("usecase: cardgroup: logger is required")
	}
	return &cardgroupUsecase{repo: repo, logger: logger}
}

// Cardgroup returns a single cardgroup by id. Non-owners receive UNAUTHENTICATED
// rather than NOT_FOUND so the caller cannot probe existence via ID enumeration.
// A missing row returns (nil, nil) so the nullable GraphQL field resolves to null.
func (u *cardgroupUsecase) Cardgroup(ctx context.Context, id string) (*domain.Cardgroup, error) {
	user := auth.UserFrom(ctx)
	if user == nil {
		return nil, ucerr.ErrUnauthenticated
	}
	cg, err := u.repo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, nil
		}
		return nil, eris.Wrap(err, "usecase: cardgroup: find by id")
	}
	if !cg.IsOwnedBy(user.Sub) {
		return nil, ucerr.ErrUnauthenticated
	}
	return cg, nil
}

// CreateCardgroupInput carries the fields required to create a new cardgroup.
type CreateCardgroupInput struct{ Name string }

// CreateCardgroupOutcome is the result of CardgroupUsecase.Create. Exactly one
// of Cardgroup or Validation is non-nil on a nil-error return: a successful
// insert carries the new Cardgroup; a name that fails validation surfaces via
// Validation so the resolver maps it to the CreateCardgroupResult union's
// InputValidationError variant.
type CreateCardgroupOutcome struct {
	Cardgroup  *domain.Cardgroup
	Validation *InputValidationInfo
}

// Create creates a new cardgroup owned by the authenticated caller.
func (u *cardgroupUsecase) Create(ctx context.Context, in CreateCardgroupInput) (CreateCardgroupOutcome, error) {
	user := auth.UserFrom(ctx)
	if user == nil {
		return CreateCardgroupOutcome{}, ucerr.ErrUnauthenticated
	}

	name, nameErr := domain.ParseCardgroupName(in.Name)
	info, err := liftValidationErr(translateCardgroupNameErr(nameErr))
	if err != nil {
		return CreateCardgroupOutcome{}, err
	}
	if info != nil {
		return CreateCardgroupOutcome{Validation: info}, nil
	}

	id, err := domain.NewID()
	if err != nil {
		return CreateCardgroupOutcome{}, eris.Wrap(err, "usecase: cardgroup: generate id")
	}

	now := time.Now().UTC()
	cg := &domain.Cardgroup{
		ID:        id,
		OwnerID:   user.Sub,
		Name:      name,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := u.repo.Create(ctx, cg); err != nil {
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
	if user == nil {
		return UpdateCardgroupOutcome{}, ucerr.ErrUnauthenticated
	}

	existing, err := u.repo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return UpdateCardgroupOutcome{}, ucerr.ErrUnauthenticated
		}
		return UpdateCardgroupOutcome{}, eris.Wrap(err, "usecase: cardgroup: find for update")
	}
	if !existing.IsOwnedBy(user.Sub) {
		return UpdateCardgroupOutcome{}, ucerr.ErrUnauthenticated
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
		return UpdateCardgroupOutcome{}, eris.Wrap(err, "usecase: cardgroup: update")
	}
	return UpdateCardgroupOutcome{Cardgroup: updated}, nil
}

// Delete removes the cardgroup identified by id.
// Non-owners and missing rows both return UNAUTHENTICATED to prevent ID enumeration.
func (u *cardgroupUsecase) Delete(ctx context.Context, id string) error {
	user := auth.UserFrom(ctx)
	if user == nil {
		return ucerr.ErrUnauthenticated
	}

	existing, err := u.repo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return ucerr.ErrUnauthenticated
		}
		return eris.Wrap(err, "usecase: cardgroup: find for delete")
	}
	if !existing.IsOwnedBy(user.Sub) {
		return ucerr.ErrUnauthenticated
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
	if user == nil {
		return nil, ucerr.ErrUnauthenticated
	}

	// Five mixed-direction guards. The Relay spec pairs after with first
	// (forward) and before with last (backward); any other combination is
	// either contradictory or ambiguous. Reject before the repo is touched
	// so the failure mode is observable rather than a silent page-1 reset.
	if in.After != nil && in.Before != nil {
		return nil, ucerr.NewValidationError("after", "after and before are mutually exclusive")
	}
	if in.First != nil && *in.First > 0 && in.Before != nil {
		return nil, ucerr.NewValidationError("before", "last must be > 0 when before is set (received first, not last)")
	}
	if in.Last != nil && *in.Last > 0 && in.After != nil {
		return nil, ucerr.NewValidationError("after", "first must be > 0 when after is set (received last, not first)")
	}
	if in.Before != nil && (in.First == nil || *in.First <= 0) && (in.Last == nil || *in.Last <= 0) {
		return nil, ucerr.NewValidationError("before", "last must be > 0 when before is set")
	}
	if in.After != nil && (in.First == nil || *in.First <= 0) && (in.Last == nil || *in.Last <= 0) {
		return nil, ucerr.NewValidationError("after", "first must be > 0 when after is set")
	}

	orderBy, dir, err := resolveCardgroupOrderBy(in.OrderBy, in.OrderDirection)
	if err != nil {
		return nil, err
	}

	first, last, err := resolveCardgroupPageSize(in.First, in.Last)
	if err != nil {
		return nil, err
	}

	after, err := u.resolveCardgroupCursor(ctx, in.After, user.Sub, orderBy, "after")
	if err != nil {
		return nil, err
	}
	before, err := u.resolveCardgroupCursor(ctx, in.Before, user.Sub, orderBy, "before")
	if err != nil {
		return nil, err
	}

	// totalCount comes from a separate COUNT(*) scoped to the caller and
	// the optional search predicate. Computed before the no-rows
	// short-circuit so callers asking only for totalCount still see a
	// real value. Acceptable for cardgroup-per-owner counts that stay
	// well below 10k; revisit with a denormalised counter if the cap grows.
	total, err := u.repo.CountByOwner(ctx, user.Sub, in.Search)
	if err != nil {
		return nil, eris.Wrap(err, "usecase: cardgroup: count by owner")
	}

	// Request one extra row to detect whether another page exists. Trim
	// before returning to the caller.
	wantFirst := first
	wantLast := last
	if wantFirst > 0 {
		wantFirst++
	}
	if wantLast > 0 {
		wantLast++
	}

	cgs, err := u.repo.FindPageByOwner(
		ctx, user.Sub, after, before, wantFirst, wantLast, orderBy, dir, in.Search,
	)
	if err != nil {
		return nil, eris.Wrap(err, "usecase: cardgroup: find page by owner")
	}

	out := &CardgroupConnectionOutput{TotalCount: total}
	switch {
	case first > 0:
		cgs, out.HasNext = TrimAndDetect(cgs, first)
		out.HasPrev = after != nil
	case last > 0:
		cgs, out.HasPrev = TrimAndDetectBackward(cgs, last)
		out.HasNext = before != nil
	}

	out.Cardgroups = cgs
	if len(cgs) > 0 {
		out.StartCur = cgs[0].ID
		out.EndCur = cgs[len(cgs)-1].ID
	}
	return out, nil
}

// cardgroupMaxPageSize is the user-facing cap on
// myCardgroupsConnection page size. The repository-level cap (pageCap=101)
// is one greater so the "+1 fetch" trick survives a maximum-sized request.
const cardgroupMaxPageSize = 100

// resolveCardgroupOrderBy maps the typed usecase enums to the repository
// allowlist. Defaults match the schema (UPDATED_AT, DESC) when both inputs
// are nil. The default switch arm is defense in depth — gqlgen
// UnmarshalGQL already rejects invalid enum strings upstream.
func resolveCardgroupOrderBy(
	orderBy *CardgroupOrderBy, dir *SortOrder,
) (repository.CardgroupOrderBy, repository.SortOrder, error) {
	field := repository.CardgroupOrderByUpdatedAt
	if orderBy != nil {
		switch *orderBy {
		case CardgroupOrderByID:
			field = repository.CardgroupOrderByID
		case CardgroupOrderByCreatedAt:
			field = repository.CardgroupOrderByCreatedAt
		case CardgroupOrderByUpdatedAt:
			field = repository.CardgroupOrderByUpdatedAt
		case CardgroupOrderByName:
			field = repository.CardgroupOrderByName
		default:
			return "", "", ucerr.NewValidationError("orderBy", "invalid")
		}
	}
	d := repository.SortDesc
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

// resolveCardgroupPageSize clamps first/last to [0, cardgroupMaxPageSize]
// and rejects passing both. Defaults first=defaultPageSize (20) when
// neither is provided, matching the schema's documented default.
func resolveCardgroupPageSize(first, last *int) (int, int, error) {
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
		if v > cardgroupMaxPageSize {
			return cardgroupMaxPageSize
		}
		return v
	}
	if first != nil {
		return clamp(*first), 0, nil
	}
	return 0, clamp(*last), nil
}

// resolveCardgroupCursor decodes an opaque cursor string into a
// *repository.CardgroupCursor with the column required by the active orderBy
// populated. The cursor may be a v1 envelope ("v1:" + base64) or a legacy
// bare UUID; both are accepted during the backward-compatibility window.
// Returns BAD_USER_INPUT when the cursor cannot be decoded, the cardgroup
// cannot be found, or the cardgroup belongs to another owner — the latter
// would otherwise leak existence of cardgroups outside the caller's tenant.
//
// The cross-tenant check runs even when orderBy is ID (no extra column to
// hydrate). Without it, an attacker could probe for the existence of foreign
// cardgroups by paging past a guessed cursor and observing whether any rows
// come back.
func (u *cardgroupUsecase) resolveCardgroupCursor(
	ctx context.Context,
	cursorStr *string,
	ownerID string,
	orderBy repository.CardgroupOrderBy,
	field string,
) (*repository.CardgroupCursor, error) {
	if cursorStr == nil || *cursorStr == "" {
		return nil, nil
	}
	id, err := cursor.Decode(*cursorStr)
	if err != nil {
		return nil, ucerr.NewValidationError(field, "invalid cursor")
	}
	cg, err := u.repo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ucerr.NewValidationError(field, "cursor not found")
		}
		return nil, eris.Wrap(err, "usecase: cardgroup: hydrate cursor")
	}
	if !cg.IsOwnedBy(ownerID) {
		return nil, ucerr.NewValidationError(field, "cursor not found")
	}

	c := &repository.CardgroupCursor{ID: id}
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

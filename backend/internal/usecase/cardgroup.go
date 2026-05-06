// Package usecase wires authorization, repository calls, and domain rules
// behind GraphQL resolvers. Resolvers should not import repository directly.
package usecase

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/rotisserie/eris"

	"backend/internal/auth"
	"backend/internal/domain"
	"backend/internal/gqlerr"
	"backend/internal/repository"
)

// CardgroupRepository is the consumer-driven interface used by CardgroupUsecase.
// FindByIDs is intentionally omitted; it is used only by the loader layer.
type CardgroupRepository interface {
	FindByID(ctx context.Context, id string) (*domain.Cardgroup, error)
	FindByOwner(ctx context.Context, ownerID string) ([]*domain.Cardgroup, error)
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

// CardgroupUsecase implements the cardgroup application logic.
type CardgroupUsecase struct{ repo CardgroupRepository }

// NewCardgroupUsecase constructs a CardgroupUsecase backed by the given repository.
func NewCardgroupUsecase(repo CardgroupRepository) *CardgroupUsecase {
	return &CardgroupUsecase{repo: repo}
}

// MyCardgroups returns all cardgroups owned by the authenticated caller.
func (u *CardgroupUsecase) MyCardgroups(ctx context.Context) ([]*domain.Cardgroup, error) {
	user := auth.UserFrom(ctx)
	if user == nil {
		return nil, gqlerr.Unauthenticated()
	}
	cgs, err := u.repo.FindByOwner(ctx, user.Sub)
	if err != nil {
		return nil, gqlerr.Internal(ctx, err)
	}
	return cgs, nil
}

// Cardgroup returns a single cardgroup by id. Non-owners receive UNAUTHENTICATED
// rather than NOT_FOUND so the caller cannot probe existence via ID enumeration.
// A missing row returns (nil, nil) so the nullable GraphQL field resolves to null.
func (u *CardgroupUsecase) Cardgroup(ctx context.Context, id string) (*domain.Cardgroup, error) {
	user := auth.UserFrom(ctx)
	if user == nil {
		return nil, gqlerr.Unauthenticated()
	}
	cg, err := u.repo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, nil
		}
		return nil, gqlerr.Internal(ctx, err)
	}
	if cg.OwnerID != user.Sub {
		return nil, gqlerr.Unauthenticated()
	}
	return cg, nil
}

// CreateCardgroupInput carries the fields required to create a new cardgroup.
type CreateCardgroupInput struct{ Name string }

// Create creates a new cardgroup owned by the authenticated caller.
func (u *CardgroupUsecase) Create(ctx context.Context, in CreateCardgroupInput) (*domain.Cardgroup, error) {
	user := auth.UserFrom(ctx)
	if user == nil {
		return nil, gqlerr.Unauthenticated()
	}

	trimmed := strings.TrimSpace(in.Name)
	tmp := &domain.Cardgroup{Name: trimmed}
	if err := tmp.Validate(); err != nil {
		return nil, translateCardgroupNameErr(ctx, err)
	}

	id, err := uuidV7()
	if err != nil {
		return nil, gqlerr.Internal(ctx, err)
	}

	now := time.Now().UTC()
	cg := &domain.Cardgroup{
		ID:        id,
		OwnerID:   user.Sub,
		Name:      trimmed,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := u.repo.Create(ctx, cg); err != nil {
		return nil, gqlerr.Internal(ctx, err)
	}
	return cg, nil
}

// UpdateCardgroupInput carries the fields that may be patched on an existing cardgroup.
type UpdateCardgroupInput struct{ Name *string }

// Update applies a partial patch to the cardgroup identified by id.
// Non-owners and missing rows both return UNAUTHENTICATED to prevent ID enumeration.
// A nil Name field is treated as an empty patch: the existing row is returned
// without any database write.
func (u *CardgroupUsecase) Update(ctx context.Context, id string, in UpdateCardgroupInput) (*domain.Cardgroup, error) {
	user := auth.UserFrom(ctx)
	if user == nil {
		return nil, gqlerr.Unauthenticated()
	}

	existing, err := u.repo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, gqlerr.Unauthenticated()
		}
		return nil, gqlerr.Internal(ctx, err)
	}
	if existing.OwnerID != user.Sub {
		return nil, gqlerr.Unauthenticated()
	}

	// Empty patch: no DB write, return current row.
	if in.Name == nil {
		return existing, nil
	}

	trimmed := strings.TrimSpace(*in.Name)
	tmp := &domain.Cardgroup{Name: trimmed}
	if err := tmp.Validate(); err != nil {
		return nil, translateCardgroupNameErr(ctx, err)
	}

	updated, err := u.repo.Update(ctx, id, repository.CardgroupUpdate{Name: &trimmed})
	if err != nil {
		return nil, gqlerr.Internal(ctx, err)
	}
	return updated, nil
}

// Delete removes the cardgroup identified by id.
// Non-owners and missing rows both return UNAUTHENTICATED to prevent ID enumeration.
func (u *CardgroupUsecase) Delete(ctx context.Context, id string) error {
	user := auth.UserFrom(ctx)
	if user == nil {
		return gqlerr.Unauthenticated()
	}

	existing, err := u.repo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return gqlerr.Unauthenticated()
		}
		return gqlerr.Internal(ctx, err)
	}
	if existing.OwnerID != user.Sub {
		return gqlerr.Unauthenticated()
	}

	if err := u.repo.Delete(ctx, id); err != nil {
		return gqlerr.Internal(ctx, err)
	}
	return nil
}

// translateCardgroupNameErr maps domain sentinel errors from Cardgroup.Validate
// to GraphQL-layer errors. Unexpected domain errors become INTERNAL. Callers
// must guard against err == nil before invoking.
func translateCardgroupNameErr(ctx context.Context, err error) error {
	switch {
	case errors.Is(err, domain.ErrCardgroupNameRequired):
		return gqlerr.BadUserInput("name", "name is required")
	case errors.Is(err, domain.ErrCardgroupNameTooLong):
		return gqlerr.BadUserInput("name", fmt.Sprintf("name must be at most %d characters", domain.CardgroupNameMax))
	default:
		return gqlerr.Internal(ctx, err)
	}
}

// ListCardgroupsByOwnerConnection paginates the authenticated caller's
// cardgroups with Relay-style cursors. Forward paging uses (first, after);
// backward uses (last, before). The five mixed-direction combinations are
// rejected with BAD_USER_INPUT before the repository is touched so the
// caller never gets a silently re-interpreted page boundary. Cursors that
// reference a cardgroup belonging to another owner are also rejected as
// BAD_USER_INPUT (returning UNAUTHENTICATED would leak existence of other
// users' cardgroups).
func (u *CardgroupUsecase) ListCardgroupsByOwnerConnection(
	ctx context.Context, in CardgroupConnectionInput,
) (*CardgroupConnectionOutput, error) {
	user := auth.UserFrom(ctx)
	if user == nil {
		return nil, gqlerr.Unauthenticated()
	}

	// Five mixed-direction guards. The Relay spec pairs after with first
	// (forward) and before with last (backward); any other combination is
	// either contradictory or ambiguous. Reject before the repo is touched
	// so the failure mode is observable rather than a silent page-1 reset.
	if in.After != nil && in.Before != nil {
		return nil, gqlerr.BadUserInput("after", "after and before are mutually exclusive")
	}
	if in.First != nil && *in.First > 0 && in.Before != nil {
		return nil, gqlerr.BadUserInput("before", "before requires last, not first")
	}
	if in.Last != nil && *in.Last > 0 && in.After != nil {
		return nil, gqlerr.BadUserInput("after", "after requires first, not last")
	}
	if in.Before != nil && (in.First == nil || *in.First <= 0) && (in.Last == nil || *in.Last <= 0) {
		return nil, gqlerr.BadUserInput("before", "before requires last")
	}
	if in.After != nil && (in.First == nil || *in.First <= 0) && (in.Last == nil || *in.Last <= 0) {
		return nil, gqlerr.BadUserInput("after", "after requires first")
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
		return nil, gqlerr.Internal(ctx, eris.Wrap(err, "usecase: count cardgroups by owner"))
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
		return nil, gqlerr.Internal(ctx, eris.Wrap(err, "usecase: find cardgroup page by owner"))
	}

	out := &CardgroupConnectionOutput{TotalCount: total}
	switch {
	case first > 0:
		if len(cgs) > first {
			out.HasNext = true
			cgs = cgs[:first]
		}
		out.HasPrev = after != nil
	case last > 0:
		if len(cgs) > last {
			out.HasPrev = true
			// Backward paging fetched (last+1) rows; the repository already
			// reversed them so the extra row is at the leading edge of the
			// slice. Drop it so the page boundary stays at the tail.
			cgs = cgs[len(cgs)-last:]
		}
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
			return "", "", gqlerr.BadUserInput("orderBy", "invalid")
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
			return "", "", gqlerr.BadUserInput("orderDirection", "invalid")
		}
	}
	return field, d, nil
}

// resolveCardgroupPageSize clamps first/last to [0, cardgroupMaxPageSize]
// and rejects passing both. Defaults first=defaultPageSize (20) when
// neither is provided, matching the schema's documented default.
func resolveCardgroupPageSize(first, last *int) (int, int, error) {
	if first != nil && last != nil {
		return 0, 0, gqlerr.BadUserInput("first", "specify either first or last")
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

// resolveCardgroupCursor decodes a cursor ID into a
// *repository.CardgroupCursor with the column required by the active
// orderBy populated. Returns BAD_USER_INPUT when the cursor cardgroup
// cannot be found OR when it belongs to another owner — the latter would
// otherwise leak existence of cardgroups outside the caller's tenant.
func (u *CardgroupUsecase) resolveCardgroupCursor(
	ctx context.Context,
	cursorID *string,
	ownerID string,
	orderBy repository.CardgroupOrderBy,
	field string,
) (*repository.CardgroupCursor, error) {
	if cursorID == nil || *cursorID == "" {
		return nil, nil
	}
	c := &repository.CardgroupCursor{ID: *cursorID}
	if orderBy == repository.CardgroupOrderByID {
		// Even when the cursor field is the id itself, we still need to
		// confirm cross-tenant access — otherwise an attacker can probe
		// for the existence of foreign cardgroups by paging past the
		// cursor and observing whether any rows come back.
		cg, err := u.repo.FindByID(ctx, *cursorID)
		if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				return nil, gqlerr.BadUserInput(field, "cursor not found")
			}
			return nil, gqlerr.Internal(ctx, eris.Wrap(err, "usecase: hydrate cardgroup cursor"))
		}
		if cg.OwnerID != ownerID {
			return nil, gqlerr.BadUserInput(field, "cursor not found")
		}
		return c, nil
	}
	cg, err := u.repo.FindByID(ctx, *cursorID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, gqlerr.BadUserInput(field, "cursor not found")
		}
		return nil, gqlerr.Internal(ctx, eris.Wrap(err, "usecase: hydrate cardgroup cursor"))
	}
	if cg.OwnerID != ownerID {
		return nil, gqlerr.BadUserInput(field, "cursor not found")
	}
	switch orderBy {
	case repository.CardgroupOrderByName:
		name := cg.Name
		c.Name = &name
	case repository.CardgroupOrderByCreatedAt:
		ca := cg.CreatedAt
		c.CreatedAt = &ca
	case repository.CardgroupOrderByUpdatedAt:
		ua := cg.UpdatedAt
		c.UpdatedAt = &ua
	}
	return c, nil
}

// uuidV7 returns a new UUID v7 string, or an error if the OS entropy source
// fails. The caller maps the error to gqlerr.Internal; the silent v4 fallback
// is removed because both v7 and v4 draw from the same entropy source.
func uuidV7() (string, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return "", eris.Wrap(err, "uuid: NewV7 failed")
	}
	return id.String(), nil
}

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
	Create(ctx context.Context, cg *domain.Cardgroup) error
	Update(ctx context.Context, id string, patch repository.CardgroupUpdate) (*domain.Cardgroup, error)
	Delete(ctx context.Context, id string) error
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

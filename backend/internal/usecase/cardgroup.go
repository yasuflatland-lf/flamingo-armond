// Package usecase wires authorization, repository calls, and domain rules
// behind GraphQL resolvers. Resolvers should not import repository directly.
package usecase

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/rivo/uniseg"

	"backend/internal/auth"
	"backend/internal/domain"
	"backend/internal/gqlerr"
	"backend/internal/repository"
)

const (
	cardgroupNameMin = 1
	cardgroupNameMax = 100
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

	name := strings.TrimSpace(in.Name)
	if err := validateCardgroupName(name); err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	cg := &domain.Cardgroup{
		ID:        uuidV7(),
		OwnerID:   user.Sub,
		Name:      name,
		CreatedAt: now,
		UpdatedAt: now,
	}

	// Defensive invariant check; catches any drift between usecase and domain.
	if err := cg.Validate(); err != nil {
		return nil, gqlerr.Internal(ctx, err)
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
	if err := validateCardgroupName(trimmed); err != nil {
		return nil, err
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

// validateCardgroupName checks that trimmed satisfies the 1-100 grapheme-cluster rule.
func validateCardgroupName(trimmed string) error {
	n := uniseg.GraphemeClusterCount(trimmed)
	if n < cardgroupNameMin {
		return gqlerr.BadUserInput("name", "name is required")
	}
	if n > cardgroupNameMax {
		return gqlerr.BadUserInput("name", fmt.Sprintf("name must be at most %d characters", cardgroupNameMax))
	}
	return nil
}

// uuidV7 returns a new UUID v7 string. Falls back to UUID v4 on error and logs
// a warning so the caller is never blocked.
func uuidV7() string {
	id, err := uuid.NewV7()
	if err != nil {
		slog.Warn("uuid.NewV7 failed; falling back to uuid v4", "error", err)
		return uuid.NewString()
	}
	return id.String()
}

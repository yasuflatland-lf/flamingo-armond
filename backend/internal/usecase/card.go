package usecase

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"backend/internal/auth"
	"backend/internal/domain"
	"backend/internal/gqlerr"
	"backend/internal/repository"
)

type CardRepository interface {
	FindByID(ctx context.Context, id string) (*domain.Card, error)
	FindByCardgroup(ctx context.Context, cardgroupID string) ([]*domain.Card, error)
	FindPageByCardgroup(
		ctx context.Context,
		cardgroupID string,
		after, before *repository.CardCursor,
		first, last int,
		orderBy repository.CardOrderBy,
		dir repository.SortOrder,
	) ([]*domain.Card, int64, error)
	Create(ctx context.Context, card *domain.Card) error
	Update(ctx context.Context, id string, patch repository.CardUpdate) (*domain.Card, error)
	Delete(ctx context.Context, id string) error
}

type CardgroupRepositoryForCard interface {
	FindByID(ctx context.Context, id string) (*domain.Cardgroup, error)
}

type CardUsecase struct {
	cardRepo      CardRepository
	cardgroupRepo CardgroupRepositoryForCard
}

func NewCardUsecase(cardRepo CardRepository, cardgroupRepo CardgroupRepositoryForCard) *CardUsecase {
	return &CardUsecase{cardRepo: cardRepo, cardgroupRepo: cardgroupRepo}
}

type CreateCardInput struct {
	CardgroupID string
	Front       string
	Back        string
}

type UpdateCardInput struct {
	Front *string
	Back  *string
}

// CardConnectionInput captures the GraphQL pagination arguments. Pointer
// fields preserve "absent" semantics from the schema.
type CardConnectionInput struct {
	CardgroupID    string
	First, Last    *int
	After, Before  *string // raw GraphQL ID strings (cursor = card UUID)
	OrderBy        *string // GraphQL CardOrderBy enum string
	OrderDirection *string // GraphQL SortOrder enum string
}

// CardConnectionOutput is the usecase-level page result. The resolver wraps
// it into a model.CardConnection.
type CardConnectionOutput struct {
	Cards      []*domain.Card
	TotalCount int64
	HasNext    bool
	HasPrev    bool
	StartCur   string
	EndCur     string
}

const (
	defaultPageSize = 20
	maxPageSize     = 100
)

func (u *CardUsecase) Card(ctx context.Context, id string) (*domain.Card, error) {
	user := auth.UserFrom(ctx)
	if user == nil {
		return nil, gqlerr.Unauthenticated()
	}
	card, err := u.cardRepo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, nil
		}
		return nil, gqlerr.Internal(ctx, err)
	}
	if err := u.authorizeCardgroup(ctx, card.CardgroupID, user.Sub, false); err != nil {
		return nil, err
	}
	return card, nil
}

func (u *CardUsecase) CardsByCardgroup(ctx context.Context, cardgroupID string) ([]*domain.Card, error) {
	user := auth.UserFrom(ctx)
	if user == nil {
		return nil, gqlerr.Unauthenticated()
	}
	if err := u.authorizeCardgroup(ctx, cardgroupID, user.Sub, true); err != nil {
		return nil, err
	}
	cards, err := u.cardRepo.FindByCardgroup(ctx, cardgroupID)
	if err != nil {
		return nil, gqlerr.Internal(ctx, err)
	}
	return cards, nil
}

func (u *CardUsecase) Create(ctx context.Context, in CreateCardInput) (*domain.Card, error) {
	user := auth.UserFrom(ctx)
	if user == nil {
		return nil, gqlerr.Unauthenticated()
	}
	if err := u.authorizeCardgroup(ctx, in.CardgroupID, user.Sub, true); err != nil {
		return nil, err
	}

	front := strings.TrimSpace(in.Front)
	back := strings.TrimSpace(in.Back)
	now := time.Now().UTC()
	id, err := uuidV7()
	if err != nil {
		return nil, gqlerr.Internal(ctx, err)
	}
	card := &domain.Card{
		ID:          id,
		CardgroupID: in.CardgroupID,
		Front:       front,
		Back:        back,
		FSRS:        domain.NewFSRSStateForNewCard(now),
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := card.Validate(); err != nil {
		return nil, translateCardErr(ctx, err)
	}
	if err := u.cardRepo.Create(ctx, card); err != nil {
		return nil, gqlerr.Internal(ctx, err)
	}
	return card, nil
}

func (u *CardUsecase) Update(ctx context.Context, id string, in UpdateCardInput) (*domain.Card, error) {
	user := auth.UserFrom(ctx)
	if user == nil {
		return nil, gqlerr.Unauthenticated()
	}
	existing, err := u.cardRepo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, gqlerr.Unauthenticated()
		}
		return nil, gqlerr.Internal(ctx, err)
	}
	if err := u.authorizeCardgroup(ctx, existing.CardgroupID, user.Sub, false); err != nil {
		return nil, err
	}

	patch := repository.CardUpdate{}
	candidate := *existing
	if in.Front != nil {
		front := strings.TrimSpace(*in.Front)
		patch.Front = &front
		candidate.Front = front
	}
	if in.Back != nil {
		back := strings.TrimSpace(*in.Back)
		patch.Back = &back
		candidate.Back = back
	}
	if err := candidate.Validate(); err != nil {
		return nil, translateCardErr(ctx, err)
	}

	updated, err := u.cardRepo.Update(ctx, id, patch)
	if err != nil {
		return nil, gqlerr.Internal(ctx, err)
	}
	return updated, nil
}

func (u *CardUsecase) Delete(ctx context.Context, id string) error {
	user := auth.UserFrom(ctx)
	if user == nil {
		return gqlerr.Unauthenticated()
	}
	card, err := u.cardRepo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return gqlerr.Unauthenticated()
		}
		return gqlerr.Internal(ctx, err)
	}
	if err := u.authorizeCardgroup(ctx, card.CardgroupID, user.Sub, false); err != nil {
		return err
	}
	if err := u.cardRepo.Delete(ctx, id); err != nil {
		return gqlerr.Internal(ctx, err)
	}
	return nil
}

// ListCardsByCardgroupConnection paginates a cardgroup's cards using
// Relay-style forward (first/after) or backward (last/before) cursors.
func (u *CardUsecase) ListCardsByCardgroupConnection(
	ctx context.Context, in CardConnectionInput,
) (*CardConnectionOutput, error) {
	user := auth.UserFrom(ctx)
	if user == nil {
		return nil, gqlerr.Unauthenticated()
	}
	if err := u.authorizeCardgroup(ctx, in.CardgroupID, user.Sub, true); err != nil {
		return nil, err
	}

	orderBy, dir, err := resolveOrderBy(in.OrderBy, in.OrderDirection)
	if err != nil {
		return nil, err
	}

	first, last, err := resolvePageSize(in.First, in.Last)
	if err != nil {
		return nil, err
	}

	after, err := u.resolveCursor(ctx, in.After, in.CardgroupID, orderBy, "after")
	if err != nil {
		return nil, err
	}
	before, err := u.resolveCursor(ctx, in.Before, in.CardgroupID, orderBy, "before")
	if err != nil {
		return nil, err
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

	cards, total, err := u.cardRepo.FindPageByCardgroup(
		ctx, in.CardgroupID, after, before, wantFirst, wantLast, orderBy, dir,
	)
	if err != nil {
		return nil, gqlerr.Internal(ctx, err)
	}

	out := &CardConnectionOutput{TotalCount: total}
	if first > 0 {
		if len(cards) > first {
			out.HasNext = true
			cards = cards[:first]
		}
		out.HasPrev = after != nil
	} else if last > 0 {
		if len(cards) > last {
			out.HasPrev = true
			// Backward paging fetched (last+1) trailing rows; drop the
			// leading extra so the page boundary stays at the tail.
			cards = cards[len(cards)-last:]
		}
		out.HasNext = before != nil
	}

	out.Cards = cards
	if len(cards) > 0 {
		out.StartCur = cards[0].ID
		out.EndCur = cards[len(cards)-1].ID
	}
	return out, nil
}

// resolveOrderBy maps GraphQL enum strings to the repository's allowlist.
// Defaults to (ID, ASC) when both are nil.
func resolveOrderBy(orderBy, dir *string) (repository.CardOrderBy, repository.SortOrder, error) {
	field := repository.CardOrderByID
	if orderBy != nil {
		switch *orderBy {
		case "ID":
			field = repository.CardOrderByID
		case "CREATED_AT":
			field = repository.CardOrderByCreatedAt
		case "UPDATED_AT":
			field = repository.CardOrderByUpdatedAt
		case "DUE":
			field = repository.CardOrderByDue
		default:
			return "", "", gqlerr.BadUserInput("orderBy", "invalid")
		}
	}
	d := repository.SortAsc
	if dir != nil {
		switch *dir {
		case "ASC":
			d = repository.SortAsc
		case "DESC":
			d = repository.SortDesc
		default:
			return "", "", gqlerr.BadUserInput("orderDirection", "invalid")
		}
	}
	return field, d, nil
}

// resolvePageSize clamps first/last to [0, maxPageSize] and rejects passing
// both. Defaults first=defaultPageSize when neither is provided.
func resolvePageSize(first, last *int) (int, int, error) {
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

// resolveCursor decodes a cursor ID into a *repository.CardCursor with the
// field needed for the active orderBy populated. Returns BAD_USER_INPUT when
// the cursor card cannot be found or belongs to a different cardgroup.
func (u *CardUsecase) resolveCursor(
	ctx context.Context,
	cursorID *string,
	cardgroupID string,
	orderBy repository.CardOrderBy,
	field string,
) (*repository.CardCursor, error) {
	if cursorID == nil || *cursorID == "" {
		return nil, nil
	}
	c := &repository.CardCursor{ID: *cursorID}
	if orderBy == repository.CardOrderByID {
		return c, nil
	}
	card, err := u.cardRepo.FindByID(ctx, *cursorID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, gqlerr.BadUserInput(field, "cursor not found")
		}
		return nil, gqlerr.Internal(ctx, err)
	}
	if card.CardgroupID != cardgroupID {
		return nil, gqlerr.BadUserInput(field, "cursor not found")
	}
	switch orderBy {
	case repository.CardOrderByDue:
		due := card.FSRS.Due
		c.Due = &due
	case repository.CardOrderByCreatedAt:
		ca := card.CreatedAt
		c.CreatedAt = &ca
	case repository.CardOrderByUpdatedAt:
		ua := card.UpdatedAt
		c.UpdatedAt = &ua
	}
	return c, nil
}

func (u *CardUsecase) authorizeCardgroup(ctx context.Context, id, userID string, missingAsBadInput bool) error {
	cg, err := u.cardgroupRepo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) && missingAsBadInput {
			return gqlerr.BadUserInput("cardgroupId", "cardgroup not found")
		}
		if errors.Is(err, repository.ErrNotFound) {
			return gqlerr.Unauthenticated()
		}
		return gqlerr.Internal(ctx, err)
	}
	if cg.OwnerID != userID {
		return gqlerr.Unauthenticated()
	}
	return nil
}

func translateCardErr(ctx context.Context, err error) error {
	switch {
	case errors.Is(err, domain.ErrCardCardgroupIDRequired):
		return gqlerr.BadUserInput("cardgroupId", "cardgroupId is required")
	case errors.Is(err, domain.ErrCardFrontRequired):
		return gqlerr.BadUserInput("front", "front is required")
	case errors.Is(err, domain.ErrCardFrontTooLong):
		return gqlerr.BadUserInput("front", fmt.Sprintf("front must be at most %d characters", domain.CardTextMax))
	case errors.Is(err, domain.ErrCardBackRequired):
		return gqlerr.BadUserInput("back", "back is required")
	case errors.Is(err, domain.ErrCardBackTooLong):
		return gqlerr.BadUserInput("back", fmt.Sprintf("back must be at most %d characters", domain.CardTextMax))
	default:
		return gqlerr.Internal(ctx, err)
	}
}

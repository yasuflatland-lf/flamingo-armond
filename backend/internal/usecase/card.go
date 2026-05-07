package usecase

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/rotisserie/eris"
	"gorm.io/gorm"

	"backend/internal/auth"
	"backend/internal/domain"
	"backend/internal/gqlerr"
	"backend/internal/repository"
)

type CardRepository interface {
	FindByID(ctx context.Context, id string) (*domain.Card, error)
	FindByIDs(ctx context.Context, ids []string) (map[string]*domain.Card, error)
	FindByCardgroup(ctx context.Context, cardgroupID string) ([]*domain.Card, error)
	FindPageByCardgroup(
		ctx context.Context,
		cardgroupID string,
		after, before *repository.CardCursor,
		first, last int,
		orderBy repository.CardOrderBy,
		dir repository.SortOrder,
		search *string,
	) ([]*domain.Card, int64, error)
	Create(ctx context.Context, card *domain.Card) error
	FindByCardgroupAndFront(ctx context.Context, cardgroupID, front string) (*domain.Card, error)
	Update(ctx context.Context, id string, patch repository.CardUpdate) (*domain.Card, error)
	Delete(ctx context.Context, id string) error
	DeleteByIDsTx(ctx context.Context, tx *gorm.DB, ownerID string, ids []string) (int64, error)
}

type CardgroupRepositoryForCard interface {
	FindByID(ctx context.Context, id string) (*domain.Cardgroup, error)
}

// txRunner is the function the usecase calls to run fn inside a database
// transaction. NewCardUsecase binds it to db.WithContext(ctx).Transaction(fn);
// NewCardUsecaseWithTx lets unit tests inject a stub that invokes fn with
// a fake *gorm.DB.
type txRunner func(ctx context.Context, fn func(tx *gorm.DB) error) error

type CardUsecase struct {
	cardRepo      CardRepository
	cardgroupRepo CardgroupRepositoryForCard
	tx            txRunner
}

func NewCardUsecase(db *gorm.DB, cardRepo CardRepository, cardgroupRepo CardgroupRepositoryForCard) *CardUsecase {
	uc := &CardUsecase{cardRepo: cardRepo, cardgroupRepo: cardgroupRepo}
	if db != nil {
		uc.tx = func(ctx context.Context, fn func(tx *gorm.DB) error) error {
			return db.WithContext(ctx).Transaction(fn)
		}
	}
	return uc
}

// NewCardUsecaseWithTx constructs a CardUsecase with an explicit transaction
// runner. Intended for unit tests that need to exercise BulkDelete without a
// real database. Production code must use NewCardUsecase instead.
func NewCardUsecaseWithTx(
	cardRepo CardRepository,
	cardgroupRepo CardgroupRepositoryForCard,
	tx func(ctx context.Context, fn func(tx *gorm.DB) error) error,
) *CardUsecase {
	return &CardUsecase{cardRepo: cardRepo, cardgroupRepo: cardgroupRepo, tx: tx}
}

type CreateCardInput struct {
	CardgroupID string
	Front       string
	Back        string
	// FSRS, when non-nil, overrides the new-card FSRS state. All nine fields
	// must be specified together (see domain.NewFSRSStateFromInput).
	FSRS *domain.FSRSStateOverride
}

type UpdateCardInput struct {
	Front *string
	Back  *string
}

// CardOrderBy mirrors the schema CardOrderBy enum but stays in the usecase
// layer so the repository remains independent of the GraphQL model package.
type CardOrderBy string

const (
	CardOrderByID        CardOrderBy = "ID"
	CardOrderByCreatedAt CardOrderBy = "CREATED_AT"
	CardOrderByUpdatedAt CardOrderBy = "UPDATED_AT"
	CardOrderByDue       CardOrderBy = "DUE"
)

// SortOrder mirrors the schema SortOrder enum.
type SortOrder string

const (
	SortOrderAsc  SortOrder = "ASC"
	SortOrderDesc SortOrder = "DESC"
)

// CardConnectionInput captures the GraphQL pagination arguments. Pointer
// fields preserve "absent" semantics from the schema.
type CardConnectionInput struct {
	CardgroupID   string
	First, Last   *int
	After, Before *string // raw GraphQL ID strings (cursor = card UUID)
	// Search is optional; nil disables the filter. The usecase normalizes
	// whitespace-only strings to nil before reaching the repository.
	Search         *string
	OrderBy        *CardOrderBy
	OrderDirection *SortOrder
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
	maxBulkDelete   = 100
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

	override := domain.FSRSStateOverride{}
	if in.FSRS != nil {
		override = *in.FSRS
	}
	fsrsState, err := domain.NewFSRSStateFromInput(override, now)
	if err != nil {
		return nil, translateFSRSErr(ctx, err)
	}

	card := &domain.Card{
		ID:          id,
		CardgroupID: in.CardgroupID,
		Front:       front,
		Back:        back,
		FSRS:        fsrsState,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := card.Validate(); err != nil {
		return nil, translateCardErr(ctx, err)
	}
	if err := u.cardRepo.Create(ctx, card); err != nil {
		if errors.Is(err, repository.ErrCardDuplicateFront) {
			existing, lookupErr := u.cardRepo.FindByCardgroupAndFront(ctx, in.CardgroupID, front)
			if lookupErr != nil {
				// The lookup may race with a concurrent delete (the duplicate row vanished
				// between the failed INSERT and this SELECT) or fail for an unrelated DB
				// reason. Either way, surface as Internal so the client can retry; the
				// duplicate is recoverable input, but a failed re-lookup is not.
				return nil, gqlerr.Internal(ctx,
					eris.Wrap(lookupErr, "usecase: lookup duplicate card after 23505"),
					slog.String("cardgroup_id", in.CardgroupID),
				)
			}
			return nil, gqlerr.BadUserInputCardDuplicateFront(existing.ID, existing.Back)
		}
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

	// Normalize: nil and whitespace-only both mean "no filter". After this
	// block, a non-nil search pointer is guaranteed to hold a non-empty,
	// trimmed string — the repository can rely on this invariant.
	search := in.Search
	if search != nil {
		trimmed := strings.TrimSpace(*search)
		if trimmed == "" {
			search = nil
		} else {
			search = &trimmed
		}
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
		ctx, in.CardgroupID, after, before, wantFirst, wantLast, orderBy, dir, search,
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

// resolveOrderBy maps the typed usecase enums to the repository allowlist.
// Defaults to (ID, ASC) when both are nil. The default arm is defense in
// depth — gqlgen UnmarshalGQL already rejects invalid enum strings upstream.
func resolveOrderBy(orderBy *CardOrderBy, dir *SortOrder) (repository.CardOrderBy, repository.SortOrder, error) {
	field := repository.CardOrderByID
	if orderBy != nil {
		switch *orderBy {
		case CardOrderByID:
			field = repository.CardOrderByID
		case CardOrderByCreatedAt:
			field = repository.CardOrderByCreatedAt
		case CardOrderByUpdatedAt:
			field = repository.CardOrderByUpdatedAt
		case CardOrderByDue:
			field = repository.CardOrderByDue
		default:
			return "", "", gqlerr.BadUserInput("orderBy", "invalid")
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

// translateFSRSErr maps domain-level FSRS override sentinels to GraphQL
// BAD_USER_INPUT errors with the appropriate field hint. Unknown errors are
// surfaced as INTERNAL after being scrubbed by gqlerr.Internal.
func translateFSRSErr(ctx context.Context, err error) error {
	switch {
	case errors.Is(err, domain.ErrFSRSOverridePartial):
		return gqlerr.BadUserInput("input.fsrs", "all FSRS override fields must be provided together (or none)")
	case errors.Is(err, domain.ErrFSRSOverrideStateInvalid):
		return gqlerr.BadUserInput("input.state", "state must be 0..3")
	default:
		return gqlerr.Internal(ctx, err)
	}
}

// BulkDelete removes the cards in `ids` whose cardgroup is owned by the
// authenticated caller. Ownership is enforced exclusively by the SQL subselect
// in DeleteByIDsTx (one DELETE scoped to cardgroups owned by the caller);
// foreign-owned ids are silently skipped at the SQL layer. Returns the number
// of rows actually deleted. At most maxBulkDelete ids may be supplied per call;
// exceeding the cap returns BAD_USER_INPUT.
func (u *CardUsecase) BulkDelete(ctx context.Context, ids []string) (int64, error) {
	user := auth.UserFrom(ctx)
	if user == nil {
		return 0, gqlerr.Unauthenticated()
	}
	if len(ids) > maxBulkDelete {
		return 0, gqlerr.BadUserInput("ids", fmt.Sprintf("at most %d ids per call", maxBulkDelete))
	}
	if len(ids) == 0 {
		return 0, nil
	}

	if u.tx == nil {
		return 0, gqlerr.Internal(ctx, errors.New("usecase: tx runner not configured"))
	}
	var deleted int64
	err := u.tx(ctx, func(tx *gorm.DB) error {
		n, err := u.cardRepo.DeleteByIDsTx(ctx, tx, user.Sub, ids)
		if err != nil {
			return err
		}
		deleted = n
		return nil
	})
	if err != nil {
		return 0, gqlerr.Internal(ctx, err)
	}
	if deleted < int64(len(ids)) {
		slog.Default().LogAttrs(ctx, slog.LevelInfo, "bulk delete: partial match",
			slog.String("user_id", user.Sub),
			slog.Int("requested", len(ids)),
			slog.Int64("deleted", deleted),
		)
	}
	return deleted, nil
}

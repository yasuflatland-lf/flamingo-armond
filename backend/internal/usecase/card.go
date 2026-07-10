package usecase

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/rotisserie/eris"
	"gorm.io/gorm"

	"backend/internal/auth"
	"backend/internal/domain"
	"backend/internal/repository"
	"backend/internal/usecase/ucerr"
)

type CardRepository interface {
	FindByID(ctx context.Context, id string) (*domain.Card, error)
	FindPageByCardgroupForUser(
		ctx context.Context,
		userID, cardgroupID string,
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

type UserCardFSRSRepositoryForCard interface {
	FindByUserAndCardIDs(ctx context.Context, userID string, cardIDs []string) (map[string]*domain.UserCardFSRS, error)
}

type CardObserver interface {
	OnCardCreated(ctx context.Context, card *domain.Card)
	OnCardUpdated(ctx context.Context, card *domain.Card)
}

type noopCardObserver struct{}

func (noopCardObserver) OnCardCreated(context.Context, *domain.Card) {}
func (noopCardObserver) OnCardUpdated(context.Context, *domain.Card) {}

func normalizeCardObserver(observer CardObserver) CardObserver {
	if observer == nil {
		return noopCardObserver{}
	}
	return observer
}

// CardUsecase is the card CRUD and paginated-list surface.
type CardUsecase interface {
	Card(ctx context.Context, id string) (*domain.Card, error)
	Create(ctx context.Context, in CreateCardInput) (CreateCardOutcome, error)
	Update(ctx context.Context, id string, in UpdateCardInput) (UpdateCardOutcome, error)
	Delete(ctx context.Context, id string) error
	ListCardsByCardgroupConnection(ctx context.Context, in CardConnectionInput) (*CardConnectionOutput, error)
	BulkDelete(ctx context.Context, ids []string) (int64, error)
}

type cardUsecase struct {
	cardRepo      CardRepository
	cardgroupRepo CardgroupRepositoryForCard
	userFSRSRepo  UserCardFSRSRepositoryForCard
	tx            txRunner
	observer      CardObserver
	logger        *slog.Logger
}

func NewCardUsecase(
	db *gorm.DB,
	cardRepo CardRepository,
	cardgroupRepo CardgroupRepositoryForCard,
	userCardFSRSRepo UserCardFSRSRepositoryForCard,
	observer CardObserver,
	logger *slog.Logger,
) CardUsecase {
	if logger == nil {
		panic("usecase: card: logger is required")
	}
	uc := &cardUsecase{
		cardRepo:      cardRepo,
		cardgroupRepo: cardgroupRepo,
		userFSRSRepo:  userCardFSRSRepo,
		observer:      normalizeCardObserver(observer),
		logger:        logger,
	}
	uc.tx = newTxRunner(db)
	return uc
}

// NewCardUsecaseWithTx constructs a CardUsecase with an explicit transaction
// runner. Intended for unit tests that need to exercise BulkDelete without a
// real database. Production code must use NewCardUsecase instead.
func NewCardUsecaseWithTx(
	cardRepo CardRepository,
	cardgroupRepo CardgroupRepositoryForCard,
	tx func(ctx context.Context, fn func(tx *gorm.DB) error) error,
	userCardFSRSRepo UserCardFSRSRepositoryForCard,
	observer CardObserver,
	logger *slog.Logger,
) CardUsecase {
	if logger == nil {
		panic("usecase: card: logger is required")
	}
	return &cardUsecase{
		cardRepo:      cardRepo,
		cardgroupRepo: cardgroupRepo,
		tx:            tx,
		userFSRSRepo:  userCardFSRSRepo,
		observer:      normalizeCardObserver(observer),
		logger:        logger,
	}
}

type CreateCardInput struct {
	CardgroupID string
	Front       string
	Back        string
}

// CreateCardOutcome is the usecase-level result returned by Create. Exactly one
// of Card or Duplicate is non-nil. The duplicate-front case is surfaced as a
// typed value (not an `error`) so the resolver maps it to the
// model.CardDuplicateFrontError union variant rather than placing it in the
// errors array; model.CreateCardSuccess carries the happy-path result.
type CreateCardOutcome struct {
	// Card is the newly persisted card on the happy path. Non-nil iff Duplicate is nil.
	Card *domain.Card
	// Duplicate carries the existing card's identity when the (cardgroup_id, front)
	// unique index is violated. Non-nil iff Card is nil.
	Duplicate *DuplicateCardInfo
}

// DuplicateCardInfo identifies the existing card that collided with a create
// attempt on the (cardgroup_id, front) unique index.
type DuplicateCardInfo struct {
	ExistingID   string
	ExistingBack string
}

type UpdateCardInput struct {
	Front *string
	Back  *string
}

// UpdateCardOutcome is the result of CardUsecase.Update. Exactly one of Card
// or Validation is non-nil on a nil-error return: a successful patch carries
// the updated Card; a front/back that fails validation surfaces via Validation
// so the resolver maps it to the UpdateCardResult union's InputValidationError
// variant.
type UpdateCardOutcome struct {
	Card       *domain.Card
	Validation *InputValidationInfo
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

func (u *cardUsecase) Card(ctx context.Context, id string) (*domain.Card, error) {
	user := auth.UserFrom(ctx)
	if err := requireCallerSub(user); err != nil {
		return nil, err
	}
	card, err := u.cardRepo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, nil
		}
		return nil, eris.Wrap(err, "usecase: card: find by id")
	}
	if err := authorizeCardgroupOrUnauthenticated(ctx, u.cardgroupRepo, card.CardgroupID, domain.UserID(user.Sub)); err != nil {
		return nil, err
	}
	return card, nil
}

// Create persists a new card and returns a CreateCardOutcome that signals the
// duplicate-front case as data (via outcome.Duplicate) rather than as an error.
// Real failures — unauthenticated caller, validation, infrastructure — are still
// returned as the second return value so the resolver can wrap them via
// gqlerr.FromUsecaseError into the wire-format GraphQL error.
func (u *cardUsecase) Create(ctx context.Context, in CreateCardInput) (CreateCardOutcome, error) {
	user := auth.UserFrom(ctx)
	if err := requireCallerSub(user); err != nil {
		return CreateCardOutcome{}, err
	}
	if err := authorizeCardgroupOrBadInput(ctx, u.cardgroupRepo, domain.CardgroupID(in.CardgroupID), domain.UserID(user.Sub)); err != nil {
		return CreateCardOutcome{}, err
	}

	card, err := domain.NewCard(domain.CardgroupID(in.CardgroupID), in.Front, in.Back, 0)
	if err != nil {
		return CreateCardOutcome{}, translateCardErr(err)
	}
	if err := u.cardRepo.Create(ctx, card); err != nil {
		if errors.Is(err, repository.ErrCardDuplicateFront) {
			existing, lookupErr := u.cardRepo.FindByCardgroupAndFront(ctx, in.CardgroupID, string(card.Front))
			if lookupErr != nil {
				// The lookup may race with a concurrent delete (the duplicate row vanished
				// between the failed INSERT and this SELECT) or fail for an unrelated DB
				// reason. Either way, surface as Internal so the client can retry; the
				// duplicate is recoverable input, but a failed re-lookup is not.
				return CreateCardOutcome{}, eris.Wrap(lookupErr, "usecase: lookup duplicate card after 23505")
			}
			return CreateCardOutcome{Duplicate: &DuplicateCardInfo{
				ExistingID:   existing.ID,
				ExistingBack: string(existing.Back),
			}}, nil
		}
		return CreateCardOutcome{}, eris.Wrap(err, "usecase: create card: repo create")
	}
	u.observer.OnCardCreated(ctx, card)
	return CreateCardOutcome{Card: card}, nil
}

func (u *cardUsecase) Update(ctx context.Context, id string, in UpdateCardInput) (UpdateCardOutcome, error) {
	user := auth.UserFrom(ctx)
	if err := requireCallerSub(user); err != nil {
		return UpdateCardOutcome{}, err
	}
	existing, err := u.cardRepo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return UpdateCardOutcome{}, ucerr.ErrUnauthenticated
		}
		return UpdateCardOutcome{}, eris.Wrap(err, "usecase: update card: find by id")
	}
	if err := authorizeCardgroupOrUnauthenticated(ctx, u.cardgroupRepo, existing.CardgroupID, domain.UserID(user.Sub)); err != nil {
		return UpdateCardOutcome{}, err
	}

	patch := repository.CardUpdate{}
	// UpdateFront/UpdateBack errors below are routed through eris.Wrap, not
	// translateCardErr: ParseCardText (called immediately inside each guard)
	// already returns the sentinel for empty/zero input on the validation
	// channel. If UpdateFront/UpdateBack still rejects the parsed VO, the
	// invariant has been violated by a programmer error, not bad user input.
	// INTERNAL is the honest classification — surfacing as BAD_USER_INPUT
	// would mislead the client.
	if in.Front != nil {
		front, err := domain.ParseCardText(*in.Front, domain.ErrCardFrontRequired, domain.ErrCardFrontTooLong)
		if err != nil {
			info, perr := liftValidationErr(translateCardErr(err))
			if perr != nil {
				return UpdateCardOutcome{}, perr
			}
			return UpdateCardOutcome{Validation: info}, nil
		}
		if err := existing.UpdateFront(front); err != nil {
			return UpdateCardOutcome{}, eris.Wrap(err, "usecase: card: update front")
		}
		s := existing.Front.String()
		patch.Front = &s
	}
	if in.Back != nil {
		back, err := domain.ParseCardText(*in.Back, domain.ErrCardBackRequired, domain.ErrCardBackTooLong)
		if err != nil {
			info, perr := liftValidationErr(translateCardErr(err))
			if perr != nil {
				return UpdateCardOutcome{}, perr
			}
			return UpdateCardOutcome{Validation: info}, nil
		}
		if err := existing.UpdateBack(back); err != nil {
			return UpdateCardOutcome{}, eris.Wrap(err, "usecase: card: update back")
		}
		s := existing.Back.String()
		patch.Back = &s
	}

	updated, err := u.cardRepo.Update(ctx, id, patch)
	if err != nil {
		return UpdateCardOutcome{}, eris.Wrap(err, "usecase: update card: repo update")
	}
	u.observer.OnCardUpdated(ctx, updated)
	return UpdateCardOutcome{Card: updated}, nil
}

func (u *cardUsecase) Delete(ctx context.Context, id string) error {
	user := auth.UserFrom(ctx)
	if err := requireCallerSub(user); err != nil {
		return err
	}
	card, err := u.cardRepo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return ucerr.ErrUnauthenticated
		}
		return eris.Wrap(err, "usecase: delete card: find by id")
	}
	if err := authorizeCardgroupOrUnauthenticated(ctx, u.cardgroupRepo, card.CardgroupID, domain.UserID(user.Sub)); err != nil {
		return err
	}
	if err := u.cardRepo.Delete(ctx, id); err != nil {
		return eris.Wrap(err, "usecase: delete card: repo delete")
	}
	return nil
}

// ListCardsByCardgroupConnection paginates a cardgroup's cards using
// Relay-style forward (first/after) or backward (last/before) cursors.
func (u *cardUsecase) ListCardsByCardgroupConnection(
	ctx context.Context, in CardConnectionInput,
) (*CardConnectionOutput, error) {
	user := auth.UserFrom(ctx)
	if err := requireCallerSub(user); err != nil {
		return nil, err
	}
	if err := authorizeCardgroupOrBadInput(ctx, u.cardgroupRepo, domain.CardgroupID(in.CardgroupID), domain.UserID(user.Sub)); err != nil {
		return nil, err
	}

	first, last, err := resolveRelayPage(in.First, in.Last, in.After, in.Before, resolveStandardPageSize)
	if err != nil {
		return nil, err
	}

	orderBy, dir, err := resolveCardOrderBy(in.OrderBy, in.OrderDirection)
	if err != nil {
		return nil, err
	}

	after, err := u.resolveCardCursor(ctx, in.After, in.CardgroupID, orderBy, "after")
	if err != nil {
		return nil, err
	}
	before, err := u.resolveCardCursor(ctx, in.Before, in.CardgroupID, orderBy, "before")
	if err != nil {
		return nil, err
	}

	// Normalize: nil and whitespace-only both mean "no filter". After this,
	// a non-nil search pointer is guaranteed to hold a non-empty, trimmed
	// string — the repository can rely on this invariant.
	search := normalizeSearch(in.Search)

	var total int64
	cards, hasNext, hasPrev, err := assemblePage(first, last, after != nil, before != nil,
		func(wantFirst, wantLast int) ([]*domain.Card, error) {
			rows, t, e := u.cardRepo.FindPageByCardgroupForUser(
				ctx, user.Sub, in.CardgroupID, after, before, wantFirst, wantLast, orderBy, dir, search,
			)
			if e != nil {
				return nil, eris.Wrap(e, "usecase: list cards by cardgroup: find page")
			}
			total = t
			return rows, nil
		},
	)
	if err != nil {
		return nil, err
	}

	out := &CardConnectionOutput{TotalCount: total, HasNext: hasNext, HasPrev: hasPrev, Cards: cards}
	out.StartCur, out.EndCur = firstLastCursor(cards, func(c *domain.Card) string { return c.ID })
	return out, nil
}

// cardOrderByColumns is the usecase→repository orderBy allowlist for cards.
var cardOrderByColumns = map[CardOrderBy]repository.CardOrderBy{
	CardOrderByID:        repository.CardOrderByID,
	CardOrderByCreatedAt: repository.CardOrderByCreatedAt,
	CardOrderByUpdatedAt: repository.CardOrderByUpdatedAt,
	CardOrderByDue:       repository.CardOrderByDue,
}

// resolveCardOrderBy maps the typed usecase enums to the repository allowlist.
// Defaults to (ID, ASC) when both are nil. The default arm is defense in
// depth — gqlgen UnmarshalGQL already rejects invalid enum strings upstream.
func resolveCardOrderBy(orderBy *CardOrderBy, dir *SortOrder) (repository.CardOrderBy, repository.SortOrder, error) {
	return resolveOrderByColumn(orderBy, dir, cardOrderByColumns, repository.CardOrderByID, repository.SortAsc)
}

// resolveCardCursor decodes an opaque cursor string into a *repository.CardCursor
// with the field needed for the active orderBy populated. The cursor may be a
// v1 envelope ("v1:" + base64) or a legacy bare UUID; both are accepted during
// the backward-compatibility window. Returns BAD_USER_INPUT when the cursor
// cannot be decoded, the card cannot be found, or the card belongs to a
// different cardgroup.
func (u *cardUsecase) resolveCardCursor(
	ctx context.Context,
	cursorStr *string,
	cardgroupID string,
	orderBy repository.CardOrderBy,
	field string,
) (*repository.CardCursor, error) {
	id, present, err := decodeCursorOrBadInput(cursorStr, field)
	if err != nil {
		return nil, err
	}
	if !present {
		return nil, nil
	}
	c := &repository.CardCursor{ID: id}
	if orderBy == repository.CardOrderByID {
		return c, nil
	}
	card, err := u.cardRepo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ucerr.NewValidationError(field, "cursor not found")
		}
		return nil, eris.Wrap(err, "usecase: resolve cursor: find by id")
	}
	if !card.BelongsToCardgroup(domain.CardgroupID(cardgroupID)) {
		return nil, ucerr.NewValidationError(field, "cursor not found")
	}
	switch orderBy {
	case repository.CardOrderByDue:
		due := card.CreatedAt
		if u.userFSRSRepo == nil {
			u.logger.WarnContext(ctx, "card: resolveCardCursor: falling back to createdAt for OrderByDue because userFSRSRepo is nil or not configured")
		} else if user := auth.UserFrom(ctx); user != nil {
			byCardID, err := u.userFSRSRepo.FindByUserAndCardIDs(ctx, user.Sub, []string{id})
			if err != nil {
				return nil, eris.Wrap(err, "usecase: resolve cursor: find user fsrs")
			}
			if ucs := byCardID[id]; ucs != nil {
				due = ucs.State.Due
			}
		}
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

// BulkDelete removes the cards in `ids` whose cardgroup is owned by the
// authenticated caller. Ownership is enforced exclusively by the SQL subselect
// in DeleteByIDsTx (one DELETE scoped to cardgroups owned by the caller);
// foreign-owned ids are silently skipped at the SQL layer. Returns the number
// of rows actually deleted. At most maxBulkDelete ids may be supplied per call;
// exceeding the cap returns BAD_USER_INPUT.
func (u *cardUsecase) BulkDelete(ctx context.Context, ids []string) (int64, error) {
	user := auth.UserFrom(ctx)
	if err := requireCallerSub(user); err != nil {
		return 0, err
	}
	if len(ids) > maxBulkDelete {
		return 0, ucerr.NewValidationError("ids", fmt.Sprintf("at most %d ids per call", maxBulkDelete))
	}
	if len(ids) == 0 {
		return 0, nil
	}

	if u.tx == nil {
		return 0, eris.New("usecase: tx runner not configured")
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
		return 0, eris.Wrap(err, "usecase: bulk delete: transaction")
	}
	if deleted < int64(len(ids)) {
		u.logger.LogAttrs(ctx, slog.LevelInfo, "bulk delete: partial match",
			slog.String("user_id", user.Sub),
			slog.Int("requested", len(ids)),
			slog.Int64("deleted", deleted),
		)
	}
	return deleted, nil
}

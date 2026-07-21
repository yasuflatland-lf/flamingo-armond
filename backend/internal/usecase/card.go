package usecase

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/rotisserie/eris"
	"gorm.io/gorm"

	"backend/internal/auth"
	"backend/internal/domain"
	"backend/internal/repository"
	"backend/internal/usecase/ucerr"
)

type CardRepository interface {
	FindByID(ctx context.Context, id string) (*domain.Card, error)
	// The third return is the ordering key the query sorted each returned row by,
	// keyed by card id and taken from the same result set, so the connection can
	// embed the page's real boundary in the v2 cursors it emits. It is nil for
	// the ID ordering, whose key is the id the cursor already carries.
	FindPageByCardgroupForUser(
		ctx context.Context,
		userID, cardgroupID string,
		after, before *repository.CardCursor,
		first, last int,
		orderBy repository.CardOrderBy,
		dir repository.SortOrder,
		search *string,
	) ([]*domain.Card, int64, map[string]time.Time, error)
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
//
// Ordering and OrderKeys exist so the resolver can emit v2 cursors. Unlike the
// cardgroup listing, the card connection's DEFAULT ordering (ID) is immutable
// and needs no captured key — but DUE and UPDATED_AT are both offered as opt-in
// orderings and both move under ordinary use, so a cursor taken under either
// must carry the value its row held at serve time or an edit between two page
// fetches will duplicate or skip rows. DUE is the most volatile key in the
// repository: it is not a column on cards at all but the
// COALESCE(user_card_fsrs.due, cards.created_at) the page query orders by for
// the requesting user, and every FSRS review moves it.
//
// Ordering is the (orderBy, direction) this page was served under; OrderKeys
// maps each returned card id to the serialized value its ordering key held at
// serve time (empty string when the ordering key IS the id). Both are consumed
// only at the resolver→model boundary — the output itself still carries RAW
// ids, never pre-encoded cursors.
type CardConnectionOutput struct {
	Cards      []*domain.Card
	TotalCount int64
	HasNext    bool
	HasPrev    bool
	StartCur   string
	EndCur     string
	Ordering   PageOrdering
	OrderKeys  map[string]string
}

const (
	defaultPageSize = 20
	maxPageSize     = 100
	maxBulkDelete   = 100
)

// Card reads a single card the caller owns. An unknown id and a card owned by
// someone else both return ucerr.ErrUnauthenticated, so the query cannot be used
// as an existence oracle over another user's card ids. Update / Delete collapse
// the same two cases identically.
func (u *cardUsecase) Card(ctx context.Context, id string) (*domain.Card, error) {
	user := auth.UserFrom(ctx)
	if err := requireCallerSub(user); err != nil {
		return nil, err
	}
	card, err := u.cardRepo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ucerr.ErrUnauthenticated
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
			dup, recoverErr := recoverDuplicateFront(func() (string, string, error) {
				existing, lookupErr := u.cardRepo.FindByCardgroupAndFront(ctx, in.CardgroupID, string(card.Front))
				if lookupErr != nil {
					return "", "", lookupErr
				}
				return existing.ID, string(existing.Back), nil
			}, "usecase: card: lookup duplicate after 23505")
			if recoverErr != nil {
				return CreateCardOutcome{}, recoverErr
			}
			return CreateCardOutcome{Duplicate: dup}, nil
		}
		if translated := translateTextLengthViolation(err); translated != nil {
			return CreateCardOutcome{}, translated
		}
		return CreateCardOutcome{}, eris.Wrap(err, "usecase: card: create: repo create")
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
		return UpdateCardOutcome{}, eris.Wrap(err, "usecase: card: update: find by id")
	}
	if err := authorizeCardgroupOrUnauthenticated(ctx, u.cardgroupRepo, existing.CardgroupID, domain.UserID(user.Sub)); err != nil {
		return UpdateCardOutcome{}, err
	}

	// Validate and stage each requested field through the shared stageCardText helper.
	// UpdateFront/UpdateBack errors are routed through eris.Wrap inside each apply
	// closure, not translateCardErr: ParseCardText (run inside stageCardText) already
	// returns the sentinel for empty/zero input on the validation channel. If
	// UpdateFront/UpdateBack still rejects the parsed VO, the invariant has been
	// violated by a programmer error, not bad user input. INTERNAL is the honest
	// classification — surfacing as BAD_USER_INPUT would mislead the client.
	patch := repository.CardUpdate{}
	frontStaged, frontInfo, err := stageCardText(in.Front,
		domain.ErrCardFrontRequired, domain.ErrCardFrontTooLong,
		func(text domain.CardText) (string, error) {
			if err := existing.UpdateFront(text); err != nil {
				return "", eris.Wrap(err, "usecase: card: update front")
			}
			return existing.Front.String(), nil
		})
	if err != nil {
		return UpdateCardOutcome{}, err
	}
	if frontInfo != nil {
		return UpdateCardOutcome{Validation: frontInfo}, nil
	}
	if frontStaged != nil {
		patch.Front = frontStaged
	}
	backStaged, backInfo, err := stageCardText(in.Back,
		domain.ErrCardBackRequired, domain.ErrCardBackTooLong,
		func(text domain.CardText) (string, error) {
			if err := existing.UpdateBack(text); err != nil {
				return "", eris.Wrap(err, "usecase: card: update back")
			}
			return existing.Back.String(), nil
		})
	if err != nil {
		return UpdateCardOutcome{}, err
	}
	if backInfo != nil {
		return UpdateCardOutcome{Validation: backInfo}, nil
	}
	if backStaged != nil {
		patch.Back = backStaged
	}

	updated, err := u.cardRepo.Update(ctx, id, patch)
	if err != nil {
		// Renaming a front onto one that already exists in the same cardgroup is
		// an ordinary, recoverable user mistake, not an infrastructure failure.
		// UpdateCardResult has no duplicate variant, so it travels the error
		// channel as a field-level validation error (BAD_USER_INPUT on "front")
		// rather than defaulting to INTERNAL.
		if errors.Is(err, repository.ErrCardDuplicateFront) {
			return UpdateCardOutcome{}, ucerr.NewValidationError("front",
				"A card with this front already exists in this cardgroup")
		}
		if translated := translateTextLengthViolation(err); translated != nil {
			return UpdateCardOutcome{}, translated
		}
		return UpdateCardOutcome{}, eris.Wrap(err, "usecase: card: update: repo update")
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
		return eris.Wrap(err, "usecase: card: delete: find by id")
	}
	if err := authorizeCardgroupOrUnauthenticated(ctx, u.cardgroupRepo, card.CardgroupID, domain.UserID(user.Sub)); err != nil {
		return err
	}
	if err := u.cardRepo.Delete(ctx, id); err != nil {
		return eris.Wrap(err, "usecase: card: delete: repo delete")
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

	ordering := PageOrdering{OrderBy: string(orderBy), Direction: string(dir)}

	after, err := u.resolveCardCursor(ctx, in.After, in.CardgroupID, orderBy, ordering, "after")
	if err != nil {
		return nil, err
	}
	before, err := u.resolveCardCursor(ctx, in.Before, in.CardgroupID, orderBy, ordering, "before")
	if err != nil {
		return nil, err
	}

	// Normalize: nil and whitespace-only both mean "no filter". After this,
	// a non-nil search pointer is guaranteed to hold a non-empty, trimmed
	// string — the repository can rely on this invariant.
	search := normalizeSearch(in.Search)

	var total int64
	// pageKeys is the ordering value the page query sorted each row by, captured
	// from that query's own result set. The +1 fetch means it can describe one
	// more row than the trimmed page; cardOrderKeys reads it per returned card,
	// so the extra entry is simply never looked up.
	var pageKeys map[string]time.Time
	cards, hasNext, hasPrev, err := assemblePage(first, last, after != nil, before != nil,
		func(wantFirst, wantLast int) ([]*domain.Card, error) {
			rows, t, k, e := u.cardRepo.FindPageByCardgroupForUser(
				ctx, user.Sub, in.CardgroupID, after, before, wantFirst, wantLast, orderBy, dir, search,
			)
			if e != nil {
				return nil, eris.Wrap(e, "usecase: card: list by cardgroup: find page")
			}
			total = t
			pageKeys = k
			return rows, nil
		},
	)
	if err != nil {
		return nil, err
	}

	// OrderKeys snapshots the ordering key of every row in this page so the
	// resolver can embed it in the cursor it emits. The values come from the page
	// query itself rather than a follow-up read: the DUE ordering keys off the
	// viewer's FSRS row, and re-reading it here would resolve a later snapshot,
	// so a review landing between the two reads would mint a cursor pointing at a
	// boundary the page never used. Capturing at serve time — not re-reading when
	// the cursor comes back — is what makes the bookmark survive a later edit.
	keys, err := cardOrderKeys(orderBy, cards, pageKeys)
	if err != nil {
		return nil, err
	}

	out := &CardConnectionOutput{
		TotalCount: total,
		HasNext:    hasNext,
		HasPrev:    hasPrev,
		Cards:      cards,
		Ordering:   ordering,
		OrderKeys:  keys,
	}
	out.StartCur, out.EndCur = firstLastCursor(cards, func(c *domain.Card) string { return c.ID })
	return out, nil
}

// dueOrderValues resolves the DUE ordering key of every supplied card in ONE
// batched repository round-trip, keyed by card id.
//
// Its ONLY consumer is the v1 / legacy re-hydration path, which is handed a
// bare card id and must recover the ordering key from the current row. The emit
// path deliberately does NOT use it: a v2 cursor's key comes from the page
// query's own result set, because resolving it here would read a later snapshot
// than the one that ordered the page and could anchor the bookmark to a position
// that page never served.
//
// It therefore re-implements, in Go, the COALESCE(user_card_fsrs.due,
// cards.created_at) fallback the page query expresses in SQL. The two must stay
// in step: a key recovered here under one fallback and compared in SQL under
// another lands the bookmark on the wrong row. Do not re-derive the fallback
// anywhere else.
//
// The len(cards) == 0 early return is mandatory, not an optimisation: GORM
// silently drops a `WHERE card_id IN ?` clause built from an empty slice and
// returns every row.
//
// Two degraded paths resolve every card to its CreatedAt, matching the
// COALESCE's NULL branch exactly: a nil userFSRSRepo (logged, because it means
// the usecase was wired without the FSRS repository) and an unauthenticated
// context (there is no user whose FSRS rows could be joined). Context
// cancellation passes through unwrapped; any other repository failure is
// wrapped.
func (u *cardUsecase) dueOrderValues(ctx context.Context, cards []*domain.Card) (map[string]time.Time, error) {
	due := make(map[string]time.Time, len(cards))
	if len(cards) == 0 {
		return due, nil
	}
	ids := make([]string, 0, len(cards))
	for _, card := range cards {
		due[card.ID] = card.CreatedAt
		ids = append(ids, card.ID)
	}

	if u.userFSRSRepo == nil {
		u.logger.WarnContext(ctx, "card: falling back to createdAt for OrderByDue because userFSRSRepo is nil or not configured")
		return due, nil
	}
	user := auth.UserFrom(ctx)
	if user == nil {
		return due, nil
	}

	byCardID, err := u.userFSRSRepo.FindByUserAndCardIDs(ctx, user.Sub, ids)
	if err != nil {
		if isContextDone(err) {
			return nil, err
		}
		return nil, eris.Wrap(err, "usecase: card: resolve due order values")
	}
	for id, ucs := range byCardID {
		if ucs == nil {
			continue
		}
		if _, wanted := due[id]; wanted {
			due[id] = ucs.State.Due
		}
	}
	return due, nil
}

// cardOrderKeys serializes, for every card in a served page, the ordering value
// the page query sorted it by. pageKeys is that query's own per-row report,
// keyed by card id; it is nil for the ID ordering, which needs no key.
//
// It performs no I/O by design. An earlier version re-derived the DUE key with a
// second FSRS query, which read a different snapshot than the one that ordered
// the page — a review of the boundary card landing between the two reads minted
// a cursor keyed to a position the page never used, reintroducing exactly the
// skip/duplicate the v2 envelope exists to prevent. Every key now has exactly
// one source: the query that produced the row.
//
// A hydrating ordering whose key is absent from pageKeys is a repository bug,
// surfaced as INTERNAL. Defaulting to the zero time instead would emit a cursor
// anchored at year 1 and silently restart paging from the top of the deck.
func cardOrderKeys(
	orderBy repository.CardOrderBy, cards []*domain.Card, pageKeys map[string]time.Time,
) (map[string]string, error) {
	keys := make(map[string]string, len(cards))
	for _, card := range cards {
		key, ok := pageKeys[card.ID]
		if !ok && orderBy != repository.CardOrderByID {
			return nil, eris.Errorf("usecase: card: page query returned no %q ordering key for card %q", orderBy, card.ID)
		}
		k, err := cardOrderKey(orderBy, key)
		if err != nil {
			return nil, err
		}
		keys[card.ID] = k
	}
	return keys, nil
}

// cardOrderKey serializes one card's ordering key for embedding in a v2 cursor.
// Ordering by ID needs no key — the id is already carried by the cursor — so it
// returns the empty string and ignores key.
//
// key is the value the page query sorted the row by, passed in rather than read
// off the card so that every ordering has one source and this function stays
// pure. The three hydrating orderings share a branch deliberately: DUE keys off
// COALESCE(user_card_fsrs.due, cards.created_at), which is on no card column, so
// deriving CREATED_AT / UPDATED_AT from the row while deriving DUE from the query
// would leave two sources that can disagree. The default arm mirrors
// resolveCardCursor's: an orderBy the switch does not handle is a caller bug,
// surfaced as INTERNAL rather than a silently unanchored cursor.
func cardOrderKey(orderBy repository.CardOrderBy, key time.Time) (string, error) {
	switch orderBy {
	case repository.CardOrderByID:
		return "", nil
	case repository.CardOrderByCreatedAt, repository.CardOrderByUpdatedAt, repository.CardOrderByDue:
		return encodeTimeOrderKey(key), nil
	default:
		return "", eris.Errorf("usecase: card: unhandled orderBy %q", orderBy)
	}
}

// applyCardOrderKey populates the repository cursor column the active orderBy
// needs from the value a v2 cursor carried. A key that does not parse into the
// column type returns errCursorKeyMalformed unchanged so the caller maps it to
// BAD_USER_INPUT; an unhandled orderBy stays INTERNAL.
func applyCardOrderKey(c *repository.CardCursor, orderBy repository.CardOrderBy, key string) error {
	switch orderBy {
	case repository.CardOrderByID:
		// No extra column needed; the id in the cursor is the ordering key.
		return nil
	case repository.CardOrderByCreatedAt:
		t, err := decodeTimeOrderKey(key)
		if err != nil {
			return err
		}
		c.CreatedAt = &t
		return nil
	case repository.CardOrderByUpdatedAt:
		t, err := decodeTimeOrderKey(key)
		if err != nil {
			return err
		}
		c.UpdatedAt = &t
		return nil
	case repository.CardOrderByDue:
		t, err := decodeTimeOrderKey(key)
		if err != nil {
			return err
		}
		c.Due = &t
		return nil
	default:
		return eris.Errorf("usecase: card: unhandled orderBy %q", orderBy)
	}
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

// resolveCardCursor decodes an opaque cursor string into a
// *repository.CardCursor with the column required by the active orderBy
// populated. The cursor may be a v2 envelope ("v2:" + base64 JSON), a v1
// envelope ("v1:" + base64), or a legacy bare UUID; all three are accepted.
// Returns BAD_USER_INPUT when the cursor cannot be decoded, was taken under a
// different ordering, carries an ordering-key value that does not parse, the
// card cannot be found, or the card belongs to a different cardgroup.
//
// A v2 cursor supplies the ordering-key value captured when its page was
// served, so an edit to the row between two fetches cannot move the bookmark.
// On the DUE ordering that also spares the per-cursor
// FindByUserAndCardIDs round-trip the re-read needs. A v1 or legacy cursor
// carries no such value and falls back to re-reading the ordering key off the
// CURRENT row — via dueOrderValues, whose Go-side COALESCE mirrors the one the
// page query expresses in SQL. That re-read is what
// duplicates or skips rows when the ordering key is mutable, and it exists only
// so cursors persisted by older clients keep paging.
//
// On every ordering where v2 matters, both paths run the same FindByID lookup
// and the same cardgroup guard, and they run it BEFORE the embedded key is
// consumed. Without the guard an attacker could probe for the existence of
// cards outside the requested cardgroup by paging past a guessed cursor and
// observing whether rows come back — a v2 cursor must not bypass that gate just
// because it can hydrate itself. The ID ordering returns before the lookup: it
// has no ordering column to hydrate, and the page query is already scoped to
// the cardgroup the caller was authorized for upstream.
func (u *cardUsecase) resolveCardCursor(
	ctx context.Context,
	cursorStr *string,
	cardgroupID string,
	orderBy repository.CardOrderBy,
	ordering PageOrdering,
	field string,
) (*repository.CardCursor, error) {
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
	id := p.ID
	c := &repository.CardCursor{ID: id}
	if orderBy == repository.CardOrderByID {
		return c, nil
	}
	card, err := u.cardRepo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ucerr.NewValidationError(field, "cursor not found")
		}
		if isContextDone(err) {
			return nil, err
		}
		return nil, eris.Wrap(err, "usecase: card: resolve cursor: find by id")
	}
	if !card.BelongsToCardgroup(domain.CardgroupID(cardgroupID)) {
		return nil, ucerr.NewValidationError(field, "cursor not found")
	}

	if p.HasOrdering {
		if err := applyCardOrderKey(c, orderBy, p.OrderKey); err != nil {
			if errors.Is(err, errCursorKeyMalformed) {
				return nil, ucerr.NewValidationError(field, "invalid cursor")
			}
			return nil, err
		}
		return c, nil
	}

	// v1 / legacy bare-UUID fallback: re-hydrate from the current row.
	switch orderBy {
	case repository.CardOrderByDue:
		byCardID, err := u.dueOrderValues(ctx, []*domain.Card{card})
		if err != nil {
			return nil, err
		}
		due := byCardID[card.ID]
		c.Due = &due
	case repository.CardOrderByCreatedAt:
		ca := card.CreatedAt
		c.CreatedAt = &ca
	case repository.CardOrderByUpdatedAt:
		ua := card.UpdatedAt
		c.UpdatedAt = &ua
	default:
		return nil, eris.Errorf("usecase: card: unhandled orderBy %q", orderBy)
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
	if err := checkBulkDeleteCap(ids); err != nil {
		return 0, err
	}
	if len(ids) == 0 {
		return 0, nil
	}

	if u.tx == nil {
		return 0, eris.New("usecase: card: tx runner not configured")
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
		return 0, eris.Wrap(err, "usecase: card: bulk delete: transaction")
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

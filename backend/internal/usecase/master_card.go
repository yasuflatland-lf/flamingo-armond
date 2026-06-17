package usecase

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/rotisserie/eris"
	"gorm.io/gorm"

	"backend/internal/cursor"
	"backend/internal/domain"
	"backend/internal/repository"
	"backend/internal/textdic"
	"backend/internal/usecase/ucerr"
)

// MasterCardUsecase is the admin-only surface for master cards. Reads: a single
// master deck (incl. DRAFT) with its card count, and the deck's cards as a
// Relay-style paginated connection. Writes: create, update, delete, bulk-delete,
// and batch import. Master cards carry no per-viewer / FSRS state, so no method
// takes a user-card argument. Every method requires the AdminGate to pass;
// non-admin callers receive FORBIDDEN, anonymous callers receive UNAUTHENTICATED.
type MasterCardUsecase interface {
	// AdminMaster returns the master cardgroup with the given id INCLUDING DRAFT
	// decks, bundled with its current card count. Admin-only. A missing row is a
	// validation error on "id".
	AdminMaster(ctx context.Context, id string) (*MasterWithCount, error)
	// ListMasterCards paginates a master deck's cards with Relay-style forward
	// (first/after) or backward (last/before) cursors. Admin-only.
	ListMasterCards(ctx context.Context, in MasterCardConnectionInput) (*MasterCardConnectionOutput, error)
	// CreateMasterCard persists a new master card. Admin-only. A duplicate
	// (case-insensitive) front is returned as data via the outcome's Duplicate
	// field, not as an error.
	CreateMasterCard(ctx context.Context, in CreateMasterCardInput) (CreateMasterCardOutcome, error)
	// UpdateMasterCard patches a master card's front/back. Admin-only. A field
	// that fails validation is returned as data via the outcome's Validation
	// field.
	UpdateMasterCard(ctx context.Context, id string, in UpdateMasterCardInput) (UpdateMasterCardOutcome, error)
	// DeleteMasterCard hard-deletes a master card by id. Admin-only.
	DeleteMasterCard(ctx context.Context, id string) error
	// DeleteMasterCards bulk hard-deletes master cards by id, returning the
	// number of rows deleted. Admin-only.
	DeleteMasterCards(ctx context.Context, ids []string) (int64, error)
	// ImportMasterCards parses a base64 text payload and upserts the parsed cards
	// into a master deck by (master_cardgroup_id, front). Admin-only. Per-line
	// parse diagnostics are returned in the output's Errors slice.
	ImportMasterCards(ctx context.Context, in ImportMasterCardsInput) (ImportMasterCardsOutput, error)
}

// MasterCardOrderBy mirrors the schema MasterCardOrderBy enum but stays in the
// usecase layer so the repository remains independent of the GraphQL model
// package. The string values are identical to model.MasterCardOrderBy so the
// resolver can convert with a direct cast.
type MasterCardOrderBy string

const (
	MasterCardOrderByID        MasterCardOrderBy = "ID"
	MasterCardOrderByPosition  MasterCardOrderBy = "POSITION"
	MasterCardOrderByCreatedAt MasterCardOrderBy = "CREATED_AT"
	MasterCardOrderByUpdatedAt MasterCardOrderBy = "UPDATED_AT"
)

// MasterCardConnectionInput captures the GraphQL pagination arguments for
// adminMasterCardsConnection. Pointer fields preserve "absent" semantics from
// the schema so the usecase can default unset values explicitly.
type MasterCardConnectionInput struct {
	MasterCardgroupID string
	First, Last       *int
	After, Before     *string // raw GraphQL ID strings (cursor = master card UUID)
	// Search is optional; nil disables the filter. The usecase normalizes
	// whitespace-only strings to nil before reaching the repository.
	Search         *string
	OrderBy        *MasterCardOrderBy
	OrderDirection *SortOrder
}

// MasterCardConnectionOutput is the usecase-level page result. The resolver
// wraps it into a model.MasterCardConnection.
type MasterCardConnectionOutput struct {
	Cards      []*domain.MasterCard
	TotalCount int64
	HasNext    bool
	HasPrev    bool
	StartCur   string
	EndCur     string
}

type masterCardUsecase struct {
	masterCardRepo      repository.MasterCardRepository
	masterCardgroupRepo repository.MasterCardgroupRepository
	adminGate           *AdminGate
	tx                  txRunner
	processCardImport   func(string) ([]textdic.ParsedWord, []textdic.ValidationError, error)
	logger              *slog.Logger
}

// NewMasterCardUsecase constructs a MasterCardUsecase. db is the gorm handle used
// to open the transaction that backs ImportMasterCards; passing a nil db defers
// transaction wiring (Import then returns INTERNAL when invoked without a tx
// runner). adminGate gates every method. Panics when any required dependency
// (other than db) is nil — a nil required dependency is a wiring bug that must
// fail at startup, not at first use.
func NewMasterCardUsecase(
	db *gorm.DB,
	masterCard repository.MasterCardRepository,
	masterCardgroup repository.MasterCardgroupRepository,
	adminGate *AdminGate,
	logger *slog.Logger,
) MasterCardUsecase {
	if masterCard == nil {
		panic("usecase: master card: masterCard repository is required")
	}
	if masterCardgroup == nil {
		panic("usecase: master card: masterCardgroup repository is required")
	}
	if adminGate == nil {
		panic("usecase: master card: adminGate is required")
	}
	if logger == nil {
		panic("usecase: master card: logger is required")
	}
	uc := &masterCardUsecase{
		masterCardRepo:      masterCard,
		masterCardgroupRepo: masterCardgroup,
		adminGate:           adminGate,
		processCardImport:   textdic.Process,
		logger:              logger,
	}
	if db != nil {
		uc.tx = func(ctx context.Context, fn func(tx *gorm.DB) error) error {
			return db.WithContext(ctx).Transaction(fn)
		}
	}
	return uc
}

// NewMasterCardUsecaseWithTx constructs a MasterCardUsecase with an explicit
// transaction runner. Intended for unit tests that exercise ImportMasterCards
// without a real database. Production code must use NewMasterCardUsecase.
func NewMasterCardUsecaseWithTx(
	masterCard repository.MasterCardRepository,
	masterCardgroup repository.MasterCardgroupRepository,
	tx txRunner,
	adminGate *AdminGate,
	logger *slog.Logger,
) MasterCardUsecase {
	if masterCard == nil {
		panic("usecase: master card: masterCard repository is required")
	}
	if masterCardgroup == nil {
		panic("usecase: master card: masterCardgroup repository is required")
	}
	if adminGate == nil {
		panic("usecase: master card: adminGate is required")
	}
	if logger == nil {
		panic("usecase: master card: logger is required")
	}
	return &masterCardUsecase{
		masterCardRepo:      masterCard,
		masterCardgroupRepo: masterCardgroup,
		adminGate:           adminGate,
		tx:                  tx,
		processCardImport:   textdic.Process,
		logger:              logger,
	}
}

// CreateMasterCardInput is the wire-shape consumed by CreateMasterCard.
type CreateMasterCardInput struct {
	MasterCardgroupID string
	Front             string
	Back              string
}

// CreateMasterCardOutcome is the result of CreateMasterCard. Exactly one of Card
// or Duplicate is non-nil on a nil-error return: the happy path carries the new
// Card; a (master_cardgroup_id, front) unique collision surfaces the existing
// card's identity via Duplicate so the resolver maps it to the
// MasterCardDuplicateFrontError union variant. DuplicateCardInfo is shared with
// the user-card create path (card.go).
type CreateMasterCardOutcome struct {
	// Card is the newly persisted master card on the happy path. Non-nil iff Duplicate is nil.
	Card *domain.MasterCard
	// Duplicate carries the existing card's identity when the (master_cardgroup_id,
	// front) unique index is violated. Non-nil iff Card is nil.
	Duplicate *DuplicateCardInfo
}

// UpdateMasterCardInput is the wire-shape consumed by UpdateMasterCard. A nil
// pointer means "leave unchanged".
type UpdateMasterCardInput struct {
	Front *string
	Back  *string
}

// UpdateMasterCardOutcome is the result of UpdateMasterCard. Exactly one of Card
// or Validation is non-nil on a nil-error return: a successful patch carries the
// updated Card; a front/back that fails validation surfaces via Validation so
// the resolver maps it to the UpdateMasterCardResult union's InputValidationError
// variant.
type UpdateMasterCardOutcome struct {
	Card       *domain.MasterCard
	Validation *InputValidationInfo
}

// ImportMasterCardsInput is the wire-shape consumed by ImportMasterCards. Payload
// reuses the validateCardImport base64-encoded text format.
type ImportMasterCardsInput struct {
	MasterCardgroupID string
	Payload           string // standard base64-encoded plain-text card import payload
}

// ImportMasterCardsOutput mirrors ImportCardsOutput: Inserted + Updated equals the
// number of cards persisted; Errors carries the per-line parser diagnostics that
// did not block the import. CardImportError is shared with the user-card import
// path (card_import.go).
type ImportMasterCardsOutput struct {
	Inserted int64
	Updated  int64
	Errors   []CardImportError
}

// CreateMasterCard persists a new master card and returns the duplicate-front
// case as data (outcome.Duplicate) rather than as an error. master_cards.front is
// citext, so the duplicate check is case-insensitive at the DB. Admin-only.
func (u *masterCardUsecase) CreateMasterCard(ctx context.Context, in CreateMasterCardInput) (CreateMasterCardOutcome, error) {
	if _, err := u.adminGate.Require(ctx, "usecase: master card: create"); err != nil {
		return CreateMasterCardOutcome{}, err
	}
	// Gate the FK column at the boundary: an empty masterCardgroupId would
	// otherwise reach the INSERT and surface the FK violation as INTERNAL instead
	// of BAD_USER_INPUT (mirrors the explicit guard in ImportMasterCards).
	if in.MasterCardgroupID == "" {
		return CreateMasterCardOutcome{}, ucerr.NewValidationError("masterCardgroupId", "masterCardgroupId is required")
	}
	card, err := domain.NewMasterCard(in.MasterCardgroupID, in.Front, in.Back, 0)
	if err != nil {
		return CreateMasterCardOutcome{}, translateCardErr(err)
	}
	if err := u.masterCardRepo.Create(ctx, card); err != nil {
		if errors.Is(err, repository.ErrCardDuplicateFront) {
			existing, lookupErr := u.masterCardRepo.FindByMasterCardgroupAndFront(ctx, in.MasterCardgroupID, string(card.Front))
			if lookupErr != nil {
				if isContextDone(lookupErr) {
					return CreateMasterCardOutcome{}, lookupErr
				}
				// The lookup may race with a concurrent delete (the duplicate row
				// vanished between the failed INSERT and this SELECT) or fail for an
				// unrelated DB reason. Either way, surface as Internal so the client
				// can retry.
				return CreateMasterCardOutcome{}, eris.Wrap(lookupErr, "usecase: master card: lookup duplicate after 23505")
			}
			return CreateMasterCardOutcome{Duplicate: &DuplicateCardInfo{
				ExistingID:   existing.ID,
				ExistingBack: string(existing.Back),
			}}, nil
		}
		if isContextDone(err) {
			return CreateMasterCardOutcome{}, err
		}
		return CreateMasterCardOutcome{}, eris.Wrap(err, "usecase: master card: create: repo create")
	}
	return CreateMasterCardOutcome{Card: card}, nil
}

// UpdateMasterCard patches a master card's front/back. A field that fails
// validation is returned as data (outcome.Validation); a missing id is a
// validation error on "id". Admin-only.
func (u *masterCardUsecase) UpdateMasterCard(ctx context.Context, id string, in UpdateMasterCardInput) (UpdateMasterCardOutcome, error) {
	if _, err := u.adminGate.Require(ctx, "usecase: master card: update"); err != nil {
		return UpdateMasterCardOutcome{}, err
	}

	patch := repository.MasterCardUpdate{}
	if in.Front != nil {
		front, err := domain.ParseCardText(*in.Front, domain.ErrCardFrontRequired, domain.ErrCardFrontTooLong)
		if err != nil {
			info, perr := liftValidationErr(translateCardErr(err))
			if perr != nil {
				return UpdateMasterCardOutcome{}, perr
			}
			return UpdateMasterCardOutcome{Validation: info}, nil
		}
		s := front.String()
		patch.Front = &s
	}
	if in.Back != nil {
		back, err := domain.ParseCardText(*in.Back, domain.ErrCardBackRequired, domain.ErrCardBackTooLong)
		if err != nil {
			info, perr := liftValidationErr(translateCardErr(err))
			if perr != nil {
				return UpdateMasterCardOutcome{}, perr
			}
			return UpdateMasterCardOutcome{Validation: info}, nil
		}
		s := back.String()
		patch.Back = &s
	}

	updated, err := u.masterCardRepo.Update(ctx, id, patch)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return UpdateMasterCardOutcome{}, ucerr.NewValidationError("id", "master card not found")
		}
		if isContextDone(err) {
			return UpdateMasterCardOutcome{}, err
		}
		return UpdateMasterCardOutcome{}, eris.Wrap(err, "usecase: master card: update: repo update")
	}
	return UpdateMasterCardOutcome{Card: updated}, nil
}

// DeleteMasterCard hard-deletes a master card by id. A missing id is a validation
// error on "id". Admin-only.
func (u *masterCardUsecase) DeleteMasterCard(ctx context.Context, id string) error {
	if _, err := u.adminGate.Require(ctx, "usecase: master card: delete"); err != nil {
		return err
	}
	if err := u.masterCardRepo.Delete(ctx, id); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return ucerr.NewValidationError("id", "master card not found")
		}
		if isContextDone(err) {
			return err
		}
		return eris.Wrap(err, "usecase: master card: delete: repo delete")
	}
	return nil
}

// DeleteMasterCards bulk hard-deletes master cards by id and returns the number of
// rows deleted. At most maxBulkDelete ids may be supplied; exceeding the cap is a
// validation error on "ids". Admin-only.
func (u *masterCardUsecase) DeleteMasterCards(ctx context.Context, ids []string) (int64, error) {
	if _, err := u.adminGate.Require(ctx, "usecase: master card: bulk delete"); err != nil {
		return 0, err
	}
	if len(ids) > maxBulkDelete {
		return 0, ucerr.NewValidationError("ids", fmt.Sprintf("at most %d ids per call", maxBulkDelete))
	}
	if len(ids) == 0 {
		return 0, nil
	}
	n, err := u.masterCardRepo.DeleteMany(ctx, ids)
	if err != nil {
		if isContextDone(err) {
			return 0, err
		}
		return 0, eris.Wrap(err, "usecase: master card: bulk delete: repo delete many")
	}
	if n < int64(len(ids)) {
		u.logger.LogAttrs(ctx, slog.LevelInfo, "master card bulk delete: partial match",
			slog.Int("requested", len(ids)),
			slog.Int64("deleted", n),
		)
	}
	return n, nil
}

// ImportMasterCards parses a base64-encoded card import payload and upserts the
// parsed cards into the target master deck by (master_cardgroup_id, front). Admin-
// only. Mirrors cardImportUsecase.Import: an empty (well-formed) parse persists
// nothing and surfaces the parser diagnostics via Output.Errors.
func (u *masterCardUsecase) ImportMasterCards(ctx context.Context, in ImportMasterCardsInput) (ImportMasterCardsOutput, error) {
	if _, err := u.adminGate.Require(ctx, "usecase: master card: import"); err != nil {
		return ImportMasterCardsOutput{}, err
	}
	if in.MasterCardgroupID == "" {
		return ImportMasterCardsOutput{}, ucerr.NewValidationError("masterCardgroupId", "masterCardgroupId is required")
	}
	if in.Payload == "" {
		return ImportMasterCardsOutput{}, ucerr.NewValidationError("payload", "payload must not be empty")
	}
	decoded, err := base64.StdEncoding.DecodeString(in.Payload)
	if err != nil {
		return ImportMasterCardsOutput{}, ucerr.NewValidationError("payload", "payload must be standard base64-encoded text")
	}

	process := u.processCardImport
	if process == nil {
		process = textdic.Process
	}
	words, parseErrs, perr := process(string(decoded))
	if perr != nil {
		return ImportMasterCardsOutput{}, eris.Wrap(perr, "usecase: master card: import: parse")
	}

	if len(words) > cardImportParsedRowCap {
		return ImportMasterCardsOutput{}, ucerr.NewValidationError("payload", "payload exceeds 5000 row cap")
	}

	mappedErrs := cardImportErrorsFromTextdic(parseErrs)

	// Deduplicate parsed words by front within this payload (last occurrence
	// wins); earlier occurrences are dropped and reported. Postgres error 21000
	// fires when a conflict key repeats in one INSERT, so the dedupe must happen
	// before UpsertManyTx. master_cards.front is citext, so the conflict key is
	// case-insensitive — the map key is therefore case-folded (unlike the
	// plain-text cards.front mirror in card_import.go) so "Apple" and "apple"
	// collapse to one row rather than both reaching the ON CONFLICT INSERT.
	lastIndex := make(map[string]int, len(words))
	for i, w := range words {
		lastIndex[strings.ToLower(w.Front)] = i
	}
	deduped := make([]textdic.ParsedWord, 0, len(words))
	for i, w := range words {
		if lastIndex[strings.ToLower(w.Front)] != i {
			winningBack := words[lastIndex[w.Front]].Back
			mappedErrs = append(mappedErrs, CardImportError{
				Line:    w.Line,
				Message: fmt.Sprintf("duplicated front (%s) was overridden with the new back (%s)", w.Front, winningBack),
				Kind:    CardImportErrKindDuplicate,
				Front:   w.Front,
				Back:    w.Back,
			})
			continue
		}
		deduped = append(deduped, w)
	}
	words = deduped

	// Empty (but well-formed) parse: nothing to persist; surface the parser's
	// per-line diagnostics so the caller can act on them.
	if len(words) == 0 {
		return ImportMasterCardsOutput{Errors: mappedErrs}, nil
	}

	now := time.Now().UTC()
	cards := make([]*domain.MasterCard, 0, len(words))
	for _, w := range words {
		// Build through the enforcing constructor so an over-length front/back
		// cannot reach the repository. textdic guarantees both fields are present,
		// so the realistic failure is the length cap; surface it as a typed
		// validation error.
		c, err := domain.NewMasterCard(in.MasterCardgroupID, w.Front, w.Back, 0)
		if err != nil {
			return ImportMasterCardsOutput{}, translateCardErr(err)
		}
		// NewMasterCard stamps per-card timestamps; pin the whole batch to one now.
		c.CreatedAt = now
		c.UpdatedAt = now
		cards = append(cards, c)
	}

	if u.tx == nil {
		return ImportMasterCardsOutput{}, eris.New("usecase: master card: import tx runner not configured")
	}

	var result repository.UpsertManyTxResult
	if err := u.tx(ctx, func(tx *gorm.DB) error {
		r, err := u.masterCardRepo.UpsertManyTx(ctx, tx, cards)
		if err != nil {
			return eris.Wrap(err, "usecase: master card: import: repo")
		}
		result = r
		return nil
	}); err != nil {
		if isContextDone(err) {
			return ImportMasterCardsOutput{}, err
		}
		return ImportMasterCardsOutput{}, eris.Wrap(err, "usecase: master card: import: tx")
	}

	return ImportMasterCardsOutput{
		Inserted: result.Inserted,
		Updated:  result.Updated,
		Errors:   mappedErrs,
	}, nil
}

// AdminMaster returns the master cardgroup (incl. DRAFT) with the given id plus
// its card count. Admin-only: the gate rejects non-admin / anonymous callers
// before any repository access. FindByID returns ANY status, so DRAFT decks are
// included. A missing row surfaces as a validation error on "id".
func (u *masterCardUsecase) AdminMaster(ctx context.Context, id string) (*MasterWithCount, error) {
	if _, err := u.adminGate.Require(ctx, "usecase: master card: admin master"); err != nil {
		return nil, err
	}
	master, err := u.masterCardgroupRepo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ucerr.NewValidationError("id", "master cardgroup not found")
		}
		if isContextDone(err) {
			return nil, err
		}
		return nil, eris.Wrap(err, "usecase: master card: admin master: find by id")
	}
	count, err := u.masterCardgroupRepo.CountCards(ctx, id)
	if err != nil {
		if isContextDone(err) {
			return nil, err
		}
		return nil, eris.Wrap(err, "usecase: master card: admin master: count cards")
	}
	return &MasterWithCount{Master: master, CardCount: count}, nil
}

// ListMasterCards paginates a master deck's cards using Relay-style forward
// (first/after) or backward (last/before) cursors. Admin-only. The mixed
// direction combinations are rejected with BAD_USER_INPUT before any repository
// access. totalCount is the search-aware count returned by
// FindPageByMasterCardgroup (its internal COUNT(*) applies the active search
// filter); the repository computes it before the first==0 && last==0
// short-circuit so a totalCount-only request still observes the real count.
func (u *masterCardUsecase) ListMasterCards(
	ctx context.Context, in MasterCardConnectionInput,
) (*MasterCardConnectionOutput, error) {
	if _, err := u.adminGate.Require(ctx, "usecase: master card: list"); err != nil {
		return nil, err
	}

	first, last, err := resolveRelayPage(in.First, in.Last, in.After, in.Before, resolveStandardPageSize)
	if err != nil {
		return nil, err
	}

	orderBy, dir, err := resolveMasterCardOrderBy(in.OrderBy, in.OrderDirection)
	if err != nil {
		return nil, err
	}

	after, err := u.resolveMasterCardCursor(ctx, in.After, in.MasterCardgroupID, orderBy, "after")
	if err != nil {
		return nil, err
	}
	before, err := u.resolveMasterCardCursor(ctx, in.Before, in.MasterCardgroupID, orderBy, "before")
	if err != nil {
		return nil, err
	}

	// Normalize: nil and whitespace-only both mean "no filter". After this block
	// a non-nil search pointer holds a non-empty, trimmed string — the repository
	// relies on this invariant.
	search := in.Search
	if search != nil {
		trimmed := strings.TrimSpace(*search)
		if trimmed == "" {
			search = nil
		} else {
			search = &trimmed
		}
	}

	// totalCount is the search-aware count returned by FindPageByMasterCardgroup
	// (captured inside the assemblePage closure). The repository computes it
	// before its own no-rows short-circuit, so a totalCount-only request
	// (first==0 && last==0) still observes the real, search-filtered count.
	var total int64
	cards, hasNext, hasPrev, err := assemblePage(first, last, after != nil, before != nil,
		func(wantFirst, wantLast int) ([]*domain.MasterCard, error) {
			rows, t, e := u.masterCardRepo.FindPageByMasterCardgroup(
				ctx, in.MasterCardgroupID, after, before, wantFirst, wantLast, orderBy, dir, search,
			)
			if e != nil {
				if isContextDone(e) {
					return nil, e
				}
				return nil, eris.Wrap(e, "usecase: master card: list: find page")
			}
			total = t
			return rows, nil
		},
	)
	if err != nil {
		return nil, err
	}

	// StartCur / EndCur carry the RAW node id; the resolver's connection layer
	// applies the cursor encoder once. Encoding here would double-encode.
	out := &MasterCardConnectionOutput{TotalCount: total, HasNext: hasNext, HasPrev: hasPrev, Cards: cards}
	if len(cards) > 0 {
		out.StartCur = cards[0].ID
		out.EndCur = cards[len(cards)-1].ID
	}
	return out, nil
}

// resolveMasterCardOrderBy maps the typed usecase enums to the repository
// allowlist. Defaults match the schema (POSITION, ASC) when both inputs are
// nil. The default switch arm is defense in depth — gqlgen UnmarshalGQL already
// rejects invalid enum strings upstream.
func resolveMasterCardOrderBy(
	orderBy *MasterCardOrderBy, dir *SortOrder,
) (repository.MasterCardOrderBy, repository.SortOrder, error) {
	field := repository.MasterCardOrderByPosition
	if orderBy != nil {
		switch *orderBy {
		case MasterCardOrderByID:
			field = repository.MasterCardOrderByID
		case MasterCardOrderByPosition:
			field = repository.MasterCardOrderByPosition
		case MasterCardOrderByCreatedAt:
			field = repository.MasterCardOrderByCreatedAt
		case MasterCardOrderByUpdatedAt:
			field = repository.MasterCardOrderByUpdatedAt
		default:
			return "", "", ucerr.NewValidationError("orderBy", "invalid")
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
			return "", "", ucerr.NewValidationError("orderDirection", "invalid")
		}
	}
	return field, d, nil
}

// resolveMasterCardCursor decodes an opaque cursor string into a
// *repository.MasterCardCursor with the column required by the active orderBy
// populated. Returns BAD_USER_INPUT when the cursor cannot be decoded, the
// master card cannot be found, or it belongs to a different master cardgroup.
//
// For MasterCardOrderByID no column hydration is needed — the decoded id is the
// full cursor. For the time/position orderings the column value is hydrated via
// a single-row FindByID lookup. FindByID is group-agnostic, so the cross-group
// guard is explicit: a card whose MasterCardgroupID differs from the requested
// group is treated as cursor-not-found, never leaked into the page query. A
// missing column for the active orderBy is a caller/internal bug surfaced as an
// error, never a silent zero-value (which would generate a wrong-but-valid SQL
// predicate and quietly skip rows).
func (u *masterCardUsecase) resolveMasterCardCursor(
	ctx context.Context,
	cursorStr *string,
	masterCardgroupID string,
	orderBy repository.MasterCardOrderBy,
	field string,
) (*repository.MasterCardCursor, error) {
	if cursorStr == nil || *cursorStr == "" {
		return nil, nil
	}
	id, err := cursor.Decode(*cursorStr)
	if err != nil {
		return nil, ucerr.NewValidationError(field, "invalid cursor")
	}
	c := &repository.MasterCardCursor{ID: id}
	if orderBy == repository.MasterCardOrderByID {
		return c, nil
	}

	card, err := u.masterCardRepo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ucerr.NewValidationError(field, "cursor not found")
		}
		if isContextDone(err) {
			return nil, err
		}
		return nil, eris.Wrap(err, "usecase: master card: resolve cursor: find by id")
	}
	// FindByID is group-agnostic — reject a cursor whose card belongs to a
	// different master cardgroup so the cursor cannot reference rows outside the
	// requested deck.
	if card.MasterCardgroupID != masterCardgroupID {
		return nil, ucerr.NewValidationError(field, "cursor not found")
	}

	switch orderBy {
	case repository.MasterCardOrderByPosition:
		pos := card.Position
		c.Position = &pos
	case repository.MasterCardOrderByCreatedAt:
		ca := card.CreatedAt
		c.CreatedAt = &ca
	case repository.MasterCardOrderByUpdatedAt:
		ua := card.UpdatedAt
		c.UpdatedAt = &ua
	default:
		return nil, eris.Errorf("usecase: master card: unhandled orderBy %q", orderBy)
	}
	return c, nil
}

package usecase

import (
	"context"
	"errors"
	"log/slog"
	"strconv"
	"time"

	"github.com/rotisserie/eris"
	"gorm.io/gorm"

	"backend/internal/auth"
	"backend/internal/domain"
	"backend/internal/repository"
	"backend/internal/textdic"
	"backend/internal/usecase/ucerr"
)

// MasterCardUsecase owns master-card CRUD and listing. Master cards carry no
// per-viewer / FSRS state, so no method takes a user-card argument. Admin
// methods require AdminGate; the public listing requires authentication and a
// catalog-visible deck.
type MasterCardUsecase interface {
	// ListMasterCards paginates a master deck's cards with Relay-style forward
	// (first/after) or backward (last/before) cursors. Admin-only.
	ListMasterCards(ctx context.Context, in MasterCardConnectionInput) (*MasterCardConnectionOutput, error)
	// ListPublicMasterCards paginates a PUBLISHED master deck's cards for any
	// authenticated caller (no admin gate). The deck must be catalog-visible
	// (published AND non-empty) — a DRAFT, card-less or unknown id is rejected as a
	// validation error on "masterCardgroupId" (non-disclosure gate). Anonymous
	// callers receive UNAUTHENTICATED.
	ListPublicMasterCards(ctx context.Context, in MasterCardConnectionInput) (*MasterCardConnectionOutput, error)
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
//
// Ordering and OrderKeys exist so the resolver can emit v2 cursors: the
// master-card listing defaults to the POSITION column, which an admin batch
// import rewrites for every conflicting row, so a cursor that carried only an
// id would move whenever the row it points at is repositioned. Ordering is the
// (orderBy, direction) this page was served under; OrderKeys maps each returned
// master card id to the serialized value its ordering column held at serve time
// (empty string when the ordering key IS the id). Both are consumed only at the
// resolver→model boundary — the output itself still carries RAW ids, never
// pre-encoded cursors.
type MasterCardConnectionOutput struct {
	Cards      []*domain.MasterCard
	TotalCount int64
	HasNext    bool
	HasPrev    bool
	StartCur   string
	EndCur     string
	Ordering   PageOrdering
	OrderKeys  map[string]string
}

// masterCardRepoForMasterCard is the narrow consumer interface for master-card
// persistence used by masterCardUsecase — only the methods this usecase calls,
// not the full repository.MasterCardRepository surface. Satisfied implicitly by
// repository.MasterCardRepository.
type masterCardRepoForMasterCard interface {
	Create(ctx context.Context, c *domain.MasterCard) error
	FindByMasterCardgroupAndFront(ctx context.Context, masterCardgroupID, front string) (*domain.MasterCard, error)
	Update(ctx context.Context, id string, patch repository.MasterCardUpdate) (*domain.MasterCard, error)
	Delete(ctx context.Context, id string) error
	DeleteMany(ctx context.Context, ids []string) (int64, error)
	UpsertManyTx(ctx context.Context, tx *gorm.DB, cards []*domain.MasterCard) (repository.UpsertManyTxResult, error)
	FindPageByMasterCardgroup(
		ctx context.Context,
		masterCardgroupID string,
		after, before *repository.MasterCardCursor,
		first, last int,
		orderBy repository.MasterCardOrderBy,
		dir repository.SortOrder,
		search *string,
	) (cards []*domain.MasterCard, totalCount int64, err error)
	FindByID(ctx context.Context, id string) (*domain.MasterCard, error)
}

// masterCardgroupRepoForMasterCard is the catalog-visibility gate used by
// masterCardUsecase. Satisfied implicitly by repository.MasterCardgroupRepository.
type masterCardgroupRepoForMasterCard interface {
	FindPublishedByID(ctx context.Context, id string) (*domain.MasterCardgroup, error)
}

type masterCardUsecase struct {
	masterCardRepo      masterCardRepoForMasterCard
	masterCardgroupRepo masterCardgroupRepoForMasterCard
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
	masterCard masterCardRepoForMasterCard,
	masterCardgroup masterCardgroupRepoForMasterCard,
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
	uc.tx = newTxRunner(db)
	return uc
}

// newMasterCardUsecaseWithTx constructs a MasterCardUsecase with an explicit
// transaction runner. Intended for unit tests that exercise ImportMasterCards
// without a real database. Production code must use NewMasterCardUsecase.
func newMasterCardUsecaseWithTx(
	masterCard masterCardRepoForMasterCard,
	masterCardgroup masterCardgroupRepoForMasterCard,
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
// reuses the base64-encoded text format of the `validateCardImport` GraphQL query.
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
	// Gate the FK column at the boundary: an empty masterCardgroupId is rejected
	// here as a fast-path (no DB round-trip) with a clear "required" message. A
	// non-empty but nonexistent id is caught later by classifyMasterCardFKError
	// (Postgres FK violation 23503) and mapped to the same BAD_USER_INPUT field.
	if in.MasterCardgroupID == "" {
		return CreateMasterCardOutcome{}, ucerr.NewValidationError("masterCardgroupId", "masterCardgroupId is required")
	}
	card, err := domain.NewMasterCard(in.MasterCardgroupID, in.Front, in.Back, 0)
	if err != nil {
		return CreateMasterCardOutcome{}, translateCardErr(err)
	}
	if err := u.masterCardRepo.Create(ctx, card); err != nil {
		if errors.Is(err, repository.ErrCardDuplicateFront) {
			dup, recoverErr := recoverDuplicateFront(func() (string, string, error) {
				existing, lookupErr := u.masterCardRepo.FindByMasterCardgroupAndFront(ctx, in.MasterCardgroupID, string(card.Front))
				if lookupErr != nil {
					return "", "", lookupErr
				}
				return existing.ID, string(existing.Back), nil
			}, "usecase: master card: lookup duplicate after 23505")
			if recoverErr != nil {
				return CreateMasterCardOutcome{}, recoverErr
			}
			return CreateMasterCardOutcome{Duplicate: dup}, nil
		}
		if errors.Is(err, repository.ErrMasterCardgroupNotFound) {
			return CreateMasterCardOutcome{}, ucerr.NewValidationError("masterCardgroupId", "master cardgroup not found")
		}
		if translated := translateTextLengthViolation(err); translated != nil {
			return CreateMasterCardOutcome{}, translated
		}
		return CreateMasterCardOutcome{}, wrapInfraErr(err, "usecase: master card: create: repo create")
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

	// Validate and stage each requested field through the shared stageCardText helper
	// rather than writing the patch DTO inline (mirrors cardUsecase.Update). The
	// patch-build path is preserved — no FindByID read round-trip: a transient
	// MasterCard carries the parsed value into UpdateFront/UpdateBack and the resulting
	// field is copied into the repository patch. UpdateFront/UpdateBack errors are
	// routed through eris.Wrap inside each apply closure, not translateCardErr:
	// ParseCardText (run inside stageCardText) already returns the sentinel for
	// empty/zero input on the validation channel. If UpdateFront/UpdateBack still
	// rejects the parsed VO, the invariant has been violated by a programmer error,
	// not bad user input — INTERNAL is the honest classification.
	patch := repository.MasterCardUpdate{}
	staged := &domain.MasterCard{}
	frontStaged, frontInfo, err := stageCardText(in.Front,
		domain.ErrCardFrontRequired, domain.ErrCardFrontTooLong,
		func(text domain.CardText) (string, error) {
			if err := staged.UpdateFront(text); err != nil {
				return "", eris.Wrap(err, "usecase: master card: update front")
			}
			return staged.Front.String(), nil
		})
	if err != nil {
		return UpdateMasterCardOutcome{}, err
	}
	if frontInfo != nil {
		return UpdateMasterCardOutcome{Validation: frontInfo}, nil
	}
	if frontStaged != nil {
		patch.Front = frontStaged
	}
	backStaged, backInfo, err := stageCardText(in.Back,
		domain.ErrCardBackRequired, domain.ErrCardBackTooLong,
		func(text domain.CardText) (string, error) {
			if err := staged.UpdateBack(text); err != nil {
				return "", eris.Wrap(err, "usecase: master card: update back")
			}
			return staged.Back.String(), nil
		})
	if err != nil {
		return UpdateMasterCardOutcome{}, err
	}
	if backInfo != nil {
		return UpdateMasterCardOutcome{Validation: backInfo}, nil
	}
	if backStaged != nil {
		patch.Back = backStaged
	}

	updated, err := u.masterCardRepo.Update(ctx, id, patch)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return UpdateMasterCardOutcome{}, ucerr.NewValidationError("id", "master card not found")
		}
		if translated := translateTextLengthViolation(err); translated != nil {
			return UpdateMasterCardOutcome{}, translated
		}
		return UpdateMasterCardOutcome{}, wrapInfraErr(err, "usecase: master card: update: repo update")
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
		return wrapInfraErr(err, "usecase: master card: delete: repo delete")
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
	if err := checkBulkDeleteCap(ids); err != nil {
		return 0, err
	}
	if len(ids) == 0 {
		return 0, nil
	}
	n, err := u.masterCardRepo.DeleteMany(ctx, ids)
	if err != nil {
		return 0, wrapInfraErr(err, "usecase: master card: bulk delete: repo delete many")
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
	// No ownership check: master decks are owner-less and the admin gate above
	// already authorizes the write against every one of them.

	res, err := runCardImport(ctx, in.Payload, cardImportPipeline[*domain.MasterCard]{
		wrap:    "usecase: master card: import",
		process: u.processCardImport,
		// master_cards.front is citext, so the conflict key is case-insensitive —
		// frontMatchKey case-folds it (unlike the plain-text cards.front mirror)
		// so "Apple" and "apple" collapse to one row rather than both reaching the
		// ON CONFLICT INSERT (which would trip Postgres error 21000).
		dedupeKey: frontMatchKey,
		newRow: func(front, back domain.CardText, now time.Time) (*domain.MasterCard, error) {
			c, err := domain.NewMasterCardFromValidated(in.MasterCardgroupID, front, back, 0)
			if err != nil {
				return nil, err
			}
			// NewMasterCardFromValidated stamps per-card timestamps; pin the whole
			// batch to one created_at. updated_at is database-owned, so the
			// constructor's value is neither sent nor pinned here.
			c.CreatedAt = now
			return c, nil
		},
		tx: u.tx,
		upsert: func(ctx context.Context, tx *gorm.DB, cards []*domain.MasterCard) (repository.UpsertManyTxResult, error) {
			return u.masterCardRepo.UpsertManyTx(ctx, tx, cards)
		},
		translateTxErr: translateMasterCardgroupNotFound,
	})
	if err != nil {
		return ImportMasterCardsOutput{}, err
	}
	return ImportMasterCardsOutput(res), nil
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
	return u.listMasterCardsCore(ctx, in, "usecase: master card: list", func(ctx context.Context) error {
		_, err := u.adminGate.Require(ctx, "usecase: master card: list")
		return err
	})
}

// ListPublicMasterCards paginates a PUBLISHED master deck's cards for any
// authenticated caller (no admin gate). The body from the page assembly onward
// mirrors ListMasterCards; only the gate differs — the admin gate is replaced by
// an authentication check plus the catalog-visibility gate (published AND
// non-empty). totalCount is the search-aware count captured inside the
// assemblePage closure (same as the admin path).
func (u *masterCardUsecase) ListPublicMasterCards(
	ctx context.Context, in MasterCardConnectionInput,
) (*MasterCardConnectionOutput, error) {
	return u.listMasterCardsCore(ctx, in, "usecase: master card: public list", func(ctx context.Context) error {
		if err := requireCallerSub(auth.UserFrom(ctx)); err != nil {
			return err
		}
		// Catalog-visibility gate. FindPublishedByID returns ErrNotFound for
		// unknown ids, DRAFT ids AND published decks holding zero cards,
		// collapsing them into one not-found so the
		// endpoint cannot be used as a draft-existence oracle (non-disclosure gate).
		if _, err := u.masterCardgroupRepo.FindPublishedByID(ctx, in.MasterCardgroupID); err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				return ucerr.NewValidationError("masterCardgroupId", "master deck not found")
			}
			return wrapInfraErr(err, "usecase: master card: public list: find published by id")
		}
		return nil
	})
}

// listMasterCardsCore holds the shared page-assembly body for ListMasterCards
// and ListPublicMasterCards. The gate closure runs first and supplies the
// per-caller authorization / visibility check (admin gate vs. authentication +
// catalog-visibility gate); everything from cursor resolution onward is identical.
// opPrefix is the caller's two-segment module prefix, supplied so the shared
// find-page eris wrap carries the correct attribution (error-wrapping rule:
// shared helpers take the caller prefix as an argument, never hardcode it).
func (u *masterCardUsecase) listMasterCardsCore(
	ctx context.Context, in MasterCardConnectionInput, opPrefix string, gate func(context.Context) error,
) (*MasterCardConnectionOutput, error) {
	if err := gate(ctx); err != nil {
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

	ordering := PageOrdering{OrderBy: string(orderBy), Direction: string(dir)}

	after, err := u.resolveMasterCardCursor(ctx, in.After, in.MasterCardgroupID, orderBy, ordering, "after")
	if err != nil {
		return nil, err
	}
	before, err := u.resolveMasterCardCursor(ctx, in.Before, in.MasterCardgroupID, orderBy, ordering, "before")
	if err != nil {
		return nil, err
	}

	// Normalize: nil and whitespace-only both mean "no filter". After this call
	// a non-nil search pointer holds a non-empty, trimmed string — the repository
	// relies on this invariant.
	search := normalizeSearch(in.Search)

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
				return nil, wrapInfraErr(e, opPrefix+": find page")
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
	// bookmark survive a repositioning of the boundary row.
	keys, err := masterCardOrderKeys(orderBy, cards)
	if err != nil {
		return nil, err
	}

	// StartCur / EndCur carry the RAW node id; the resolver's connection layer
	// applies the cursor encoder once. Encoding here would double-encode.
	out := &MasterCardConnectionOutput{
		TotalCount: total,
		HasNext:    hasNext,
		HasPrev:    hasPrev,
		Cards:      cards,
		Ordering:   ordering,
		OrderKeys:  keys,
	}
	out.StartCur, out.EndCur = firstLastCursor(cards, func(c *domain.MasterCard) string { return c.ID })
	return out, nil
}

// masterCardOrderKeys serializes the active ordering column of every row in a
// served page, keyed by master card id. An orderBy outside the allowlist is a
// caller bug and surfaces as INTERNAL, matching masterCardOrderKey.
func masterCardOrderKeys(orderBy repository.MasterCardOrderBy, cards []*domain.MasterCard) (map[string]string, error) {
	keys := make(map[string]string, len(cards))
	for _, c := range cards {
		if c == nil {
			continue
		}
		k, err := masterCardOrderKey(orderBy, c)
		if err != nil {
			return nil, err
		}
		keys[c.ID] = k
	}
	return keys, nil
}

// masterCardOrderKey serializes one master card's ordering column for embedding
// in a v2 cursor. Ordering by ID needs no key — the id is already carried by the
// cursor — so it returns the empty string. The default arm mirrors
// resolveMasterCardCursor's: an orderBy the switch does not handle is a caller
// bug, surfaced as INTERNAL rather than a silently unanchored cursor.
func masterCardOrderKey(orderBy repository.MasterCardOrderBy, card *domain.MasterCard) (string, error) {
	switch orderBy {
	case repository.MasterCardOrderByID:
		return "", nil
	case repository.MasterCardOrderByPosition:
		return strconv.Itoa(card.Position), nil
	case repository.MasterCardOrderByCreatedAt:
		return encodeTimeOrderKey(card.CreatedAt), nil
	case repository.MasterCardOrderByUpdatedAt:
		return encodeTimeOrderKey(card.UpdatedAt), nil
	default:
		return "", eris.Errorf("usecase: master card: unhandled orderBy %q", orderBy)
	}
}

// applyMasterCardOrderKey populates the repository cursor column the active
// orderBy needs from the value a v2 cursor carried. A key that does not parse
// into the column type returns errCursorKeyMalformed so the caller maps it to
// BAD_USER_INPUT; an unhandled orderBy stays INTERNAL.
func applyMasterCardOrderKey(c *repository.MasterCardCursor, orderBy repository.MasterCardOrderBy, key string) error {
	switch orderBy {
	case repository.MasterCardOrderByID:
		// No extra column needed; the id in the cursor is the ordering key.
		return nil
	case repository.MasterCardOrderByPosition:
		n, err := decodeIntOrderKey(key)
		if err != nil {
			return err
		}
		c.Position = &n
		return nil
	case repository.MasterCardOrderByCreatedAt:
		t, err := decodeTimeOrderKey(key)
		if err != nil {
			return err
		}
		c.CreatedAt = &t
		return nil
	case repository.MasterCardOrderByUpdatedAt:
		t, err := decodeTimeOrderKey(key)
		if err != nil {
			return err
		}
		c.UpdatedAt = &t
		return nil
	default:
		return eris.Errorf("usecase: master card: unhandled orderBy %q", orderBy)
	}
}

// masterCardOrderByColumns is the usecase→repository orderBy allowlist for master cards.
var masterCardOrderByColumns = map[MasterCardOrderBy]repository.MasterCardOrderBy{
	MasterCardOrderByID:        repository.MasterCardOrderByID,
	MasterCardOrderByPosition:  repository.MasterCardOrderByPosition,
	MasterCardOrderByCreatedAt: repository.MasterCardOrderByCreatedAt,
	MasterCardOrderByUpdatedAt: repository.MasterCardOrderByUpdatedAt,
}

// resolveMasterCardOrderBy maps the typed usecase enums to the repository
// allowlist. Defaults match the schema (POSITION, ASC) when both inputs are
// nil. The default switch arm is defense in depth — gqlgen UnmarshalGQL already
// rejects invalid enum strings upstream.
func resolveMasterCardOrderBy(
	orderBy *MasterCardOrderBy, dir *SortOrder,
) (repository.MasterCardOrderBy, repository.SortOrder, error) {
	return resolveOrderByColumn(orderBy, dir, masterCardOrderByColumns, repository.MasterCardOrderByPosition, repository.SortAsc)
}

// resolveMasterCardCursor decodes an opaque cursor string into a
// *repository.MasterCardCursor with the column required by the active orderBy
// populated. The cursor may be a v2 envelope ("v2:" + base64 JSON), a v1
// envelope ("v1:" + base64), or a legacy bare UUID; all three are accepted.
// Returns BAD_USER_INPUT when the cursor cannot be decoded, was taken under a
// different ordering, carries an ordering-key value that does not parse, the
// master card cannot be found, or it belongs to a different master cardgroup —
// the last would otherwise let a cursor reference rows outside the requested
// deck.
//
// A v2 cursor supplies the ordering-key value captured when its page was
// served, so an admin repositioning the row between two fetches cannot move the
// bookmark. A v1 or legacy cursor carries no such value and falls back to
// re-reading the ordering column off the CURRENT row; that fallback is what
// duplicates or skips rows when the ordering column is mutable, and it exists
// only so cursors persisted by older clients keep paging.
//
// On every ordering where v2 actually matters — POSITION, CREATED_AT,
// UPDATED_AT — the FindByID lookup and the cross-deck guard both run, on the v2
// path as well as the v1 one: a v2 cursor must not skip the scope check just
// because it can hydrate itself. MasterCardOrderByID returns before the lookup
// because there is no ordering column to hydrate and the page query is already
// deck-scoped, so no cross-deck row can be reached through it.
func (u *masterCardUsecase) resolveMasterCardCursor(
	ctx context.Context,
	cursorStr *string,
	masterCardgroupID string,
	orderBy repository.MasterCardOrderBy,
	ordering PageOrdering,
	field string,
) (*repository.MasterCardCursor, error) {
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
	c := &repository.MasterCardCursor{ID: id}
	if orderBy == repository.MasterCardOrderByID {
		return c, nil
	}

	card, err := u.masterCardRepo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ucerr.NewValidationError(field, "cursor not found")
		}
		return nil, wrapInfraErr(err, "usecase: master card: resolve cursor: find by id")
	}
	// FindByID is group-agnostic — reject a cursor whose card belongs to a
	// different master cardgroup so the cursor cannot reference rows outside the
	// requested deck.
	if !card.BelongsToMasterCardgroup(masterCardgroupID) {
		return nil, ucerr.NewValidationError(field, "cursor not found")
	}

	if p.HasOrdering {
		if err := applyMasterCardOrderKey(c, orderBy, p.OrderKey); err != nil {
			if errors.Is(err, errCursorKeyMalformed) {
				return nil, ucerr.NewValidationError(field, "invalid cursor")
			}
			return nil, err
		}
		return c, nil
	}

	// v1 / legacy bare-UUID fallback: re-hydrate from the current row.
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

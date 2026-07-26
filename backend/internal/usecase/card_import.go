package usecase

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/rotisserie/eris"

	"backend/internal/auth"
	"backend/internal/domain"
	"backend/internal/repository"
	"backend/internal/textdic"
	"backend/internal/usecase/ucerr"
)

// CardImportCardRepository is the subset of repository.CardRepository the
// card import usecase consumes. Declaring a narrow interface here lets the
// existing mockCardRepository in card_test.go satisfy it without a separate
// double.
type CardImportCardRepository interface {
	UpsertManyTx(ctx context.Context, tx repository.Tx, cards []*domain.Card) (repository.UpsertManyTxResult, error)
}

// CardImportUsecase exposes authenticated card import validation and owner-only
// batch import into a cardgroup.
type CardImportUsecase interface {
	Import(ctx context.Context, input ImportCardsInput) (ImportCardsOutput, error)
	Validate(ctx context.Context, payload string) (ValidateCardImportOutcome, error)
}

// ImportCardsInput is the wire-shape consumed by CardImportUsecase.Import.
// Payload reuses the base64-encoded text format of the `validateCardImport`
// GraphQL query.
type ImportCardsInput struct {
	CardgroupID string
	Payload     string // standard base64-encoded plain-text card import payload
}

// CardImportErrorKind is the wire-aligned classifier for CardImportError.Kind.
// Values mirror the GraphQL CardImportErrorKind enum literals exactly, so the
// resolver can cast string(e.Kind) to model.CardImportErrorKind without translation.
//
// Callers MUST branch on these constants in switches; never substring-match Message.
type CardImportErrorKind string

const (
	CardImportErrKindUnknown      CardImportErrorKind = "UNKNOWN" // programming-error sentinel
	CardImportErrKindHard         CardImportErrorKind = "HARD"
	CardImportErrKindFrontOnly    CardImportErrorKind = "FRONT_ONLY"
	CardImportErrKindBackOnly     CardImportErrorKind = "BACK_ONLY"
	CardImportErrKindUnrecognized CardImportErrorKind = "UNRECOGNIZED"
	CardImportErrKindDuplicate    CardImportErrorKind = "DUPLICATE"
)

// IsSoftSkip reports whether a row with this error kind may be silently skipped.
func (k CardImportErrorKind) IsSoftSkip() bool {
	return k == CardImportErrKindFrontOnly || k == CardImportErrKindBackOnly
}

// CardImportError mirrors textdic.ValidationError so callers in the resolver
// layer can reshape it into the GraphQL model without importing the textdic
// package directly.
//
// Kind distinguishes payload-level / hard errors, grammar-recovered skips, and
// dedupe warnings. Snippet carries parser-extracted text. Front / Back are
// populated only for dedupe duplicates. Callers MUST branch on Kind rather than
// substring-matching Message; Message stays as the UI-facing description.
type CardImportError struct {
	Line    int                 `json:"line"`
	Message string              `json:"message"`
	Kind    CardImportErrorKind `json:"kind"`
	Snippet string              `json:"snippet,omitempty"` // parser-cut content
	Front   string              `json:"front,omitempty"`   // dedupe-only
	Back    string              `json:"back,omitempty"`    // dedupe-only
}

type ParsedCard struct {
	Front string
	Back  string
	Line  int
}

type ValidateCardImportOutcome struct {
	Valid       bool
	ParsedCards []ParsedCard
	Errors      []CardImportError
}

// ImportCardsOutput is the result returned to the caller. Inserted + Updated
// equals the number of cards persisted; Errors carries the per-line parser
// diagnostics that did not block the import.
type ImportCardsOutput struct {
	Inserted int64
	Updated  int64
	Errors   []CardImportError
}

// cardImportParsedRowCap is the maximum number of parsed rows accepted in a
// single import. The limit applies to the parser's *output* — a payload with
// more lines but fewer parsed rows (after errors are filtered) is still
// allowed.
const cardImportParsedRowCap = 5000

// cardImportPayloadByteCap caps a single import payload at 1 MiB. The limit
// applies to the DECODED text, not to the base64 envelope; 1 MiB already
// represents tens of thousands of entries.
const cardImportPayloadByteCap = 1 << 20

// cardImportUsecase wires auth, ownership, the textdic parser, the card
// repository, and the transaction runner that persists the import.
type cardImportUsecase struct {
	cardgroupRepo     CardgroupOwnershipFinder
	cardRepo          CardImportCardRepository
	tx                txRunner
	processCardImport func(string) ([]textdic.ParsedWord, []textdic.ValidationError, error)
	logger            *slog.Logger
}

// NewCardImportUsecase constructs a CardImportUsecase. db is the database handle
// used to open transactions; production must pass a non-nil db — with a nil db,
// runInTx hands the import closure a nil handle, which panics inside GORM at the
// first Import. Panics on nil cardgroupRepo or logger; cardRepo and db stay
// unguarded because tests without a database use NewCardImportUsecaseWithTx.
func NewCardImportUsecase(cardgroupRepo CardgroupOwnershipFinder, cardRepo CardImportCardRepository, db repository.Tx, logger *slog.Logger) *cardImportUsecase {
	if cardgroupRepo == nil {
		panic("usecase: card import: cardgroupRepo is required")
	}
	if logger == nil {
		panic("usecase: card import: logger is required")
	}
	uc := &cardImportUsecase{cardgroupRepo: cardgroupRepo, cardRepo: cardRepo, processCardImport: textdic.Process, logger: logger}
	uc.tx = newTxRunner(db)
	return uc
}

// NewCardImportUsecaseWithTx is the test-time constructor that injects an
// explicit transaction runner. Production callers must use NewCardImportUsecase.
func NewCardImportUsecaseWithTx(cardgroupRepo CardgroupOwnershipFinder, cardRepo CardImportCardRepository, tx txRunner, logger *slog.Logger) *cardImportUsecase {
	if cardgroupRepo == nil {
		panic("usecase: card import: cardgroupRepo is required")
	}
	if logger == nil {
		panic("usecase: card import: logger is required")
	}
	return &cardImportUsecase{cardgroupRepo: cardgroupRepo, cardRepo: cardRepo, tx: tx, processCardImport: textdic.Process, logger: logger}
}

func (u *cardImportUsecase) Validate(ctx context.Context, payload string) (ValidateCardImportOutcome, error) {
	if err := requireCallerSub(auth.UserFrom(ctx)); err != nil {
		return ValidateCardImportOutcome{}, err
	}

	if payload == "" {
		return ValidateCardImportOutcome{}, ucerr.NewValidationError("payload", "payload must not be empty")
	}
	decoded, err := base64.StdEncoding.DecodeString(payload)
	if err != nil {
		return ValidateCardImportOutcome{}, ucerr.NewValidationError("payload", "payload must be standard base64-encoded text")
	}
	// Whole-payload reject (byte cap): same channel as the row cap below — the
	// call succeeds, Valid is false, and the violation is a HARD line-0 entry
	// with no preview rows. An over-size payload is never parsed.
	if v := validateImportPayloadSize(len(decoded)); v != nil {
		return ValidateCardImportOutcome{Valid: false, Errors: []CardImportError{{Line: v.Line, Message: v.Message, Kind: CardImportErrKindHard}}}, nil
	}

	process := u.processCardImport
	if process == nil {
		process = textdic.Process
	}
	words, parseErrs, perr := process(string(decoded))
	if perr != nil {
		return ValidateCardImportOutcome{}, eris.Wrap(perr, "usecase: card import validate: parse")
	}

	errs := cardImportErrorsFromTextdic(parseErrs)
	_, caps := validateImportRows(words)
	for _, v := range caps {
		errs = append(errs, CardImportError{Line: v.Line, Message: v.Message, Kind: CardImportErrKindHard})
	}
	// Whole-payload reject (row cap): mirror Import's all-or-nothing reject and
	// echo an empty ParsedCards rather than the full parsed set. validateImportRows
	// short-circuits the row cap to a single payload-level violation ahead of any
	// per-row scan, so an over-cap payload cannot be previewed row-by-row anyway
	// and shipping the full parsed set is wasted allocation plus wasted bytes on
	// the wire. Below the row cap the full preview is retained so per-line errors
	// stay actionable.
	if len(words) > cardImportParsedRowCap {
		return ValidateCardImportOutcome{Valid: false, Errors: errs}, nil
	}
	parsed := make([]ParsedCard, 0, len(words))
	for _, w := range words {
		parsed = append(parsed, ParsedCard{Front: w.Front, Back: w.Back, Line: w.Line})
	}
	return ValidateCardImportOutcome{
		Valid:       len(errs) == 0 && len(parsed) > 0,
		ParsedCards: parsed,
		Errors:      errs,
	}, nil
}

// Import ingests a base64-encoded card import payload, parses it, and persists
// the parsed cards into the target cardgroup. The target cardgroup must be
// owned by the authenticated user. Empty parser output (e.g. a non-empty
// payload whose every line failed to parse — note that an empty payload is
// rejected with BAD_USER_INPUT before reaching the parser) returns a zero-valued
// result with the parser error surfaced via Output.Errors.
func (u *cardImportUsecase) Import(ctx context.Context, input ImportCardsInput) (ImportCardsOutput, error) {
	caller := auth.UserFrom(ctx)
	if err := requireCallerSub(caller); err != nil {
		return ImportCardsOutput{}, err
	}

	if input.CardgroupID == "" {
		return ImportCardsOutput{}, ucerr.NewValidationError("cardgroupId", "cardgroupId is required")
	}
	if err := authorizeCardgroupOrBadInput(ctx, u.cardgroupRepo, domain.CardgroupID(input.CardgroupID), domain.UserID(caller.Sub)); err != nil {
		return ImportCardsOutput{}, err
	}

	res, err := runCardImport(ctx, input.Payload, cardImportPipeline[*domain.Card]{
		wrap:    "usecase: card import",
		process: u.processCardImport,
		// cards.front is plain text, so the in-payload conflict key is the front
		// verbatim.
		dedupeKey: identityKey,
		newRow: func(front, back domain.CardText, now time.Time) (*domain.Card, error) {
			c, err := domain.NewCardFromValidated(domain.CardgroupID(input.CardgroupID), front, back, 0, now)
			if err != nil {
				return nil, err
			}
			return c, nil
		},
		tx: u.tx,
		upsert: func(ctx context.Context, tx repository.Tx, cards []*domain.Card) (repository.UpsertManyTxResult, error) {
			return u.cardRepo.UpsertManyTx(ctx, tx, cards)
		},
		translateTxErr: translateCardCardgroupNotFound,
	})
	if err != nil {
		return ImportCardsOutput{}, err
	}
	return ImportCardsOutput(res), nil
}

func cardImportErrorsFromTextdic(errs []textdic.ValidationError) []CardImportError {
	out := make([]CardImportError, 0, len(errs))
	for _, e := range errs {
		out = append(out, CardImportError{
			Line:    e.Line,
			Message: e.Message,
			Kind:    CardImportErrorKind(e.Kind.String()),
			Snippet: e.Snippet,
		})
	}
	return out
}

// capViolation is the internal result of validateImportRows. Each consumer maps it
// into its own output shape: Validate -> CardImportError{Kind: HARD}; Import ->
// ucerr.NewValidationError. Field carries the validation field name explicitly so
// no caller has to substring-match Message.
type capViolation struct {
	Line    int    // 0 for the payload-level row cap; w.Line for a per-row length error
	Field   string // "payload" | "front" | "back"
	Message string
}

// validatedCard bundles the trimmed, grapheme-bounded CardText VOs that
// validateImportRows parses for one import row. Returning them lets Import build the
// Card via domain.NewCardFromValidated instead of re-running domain.ParseCardText
// on the same strings — one grapheme scan per row rather than two.
type validatedCard struct {
	front domain.CardText
	back  domain.CardText
}

// validateImportPayloadSize enforces the decoded-payload byte cap
// (cardImportPayloadByteCap), the one import cap that can be checked without
// parsing. It returns nil when the payload fits, so callers use it as a
// pre-filter immediately after base64 decoding and before textdic.Process.
//
// The violation is payload-scoped (Line 0) and shaped exactly like the row cap's,
// so each consumer routes both whole-payload rejects through one channel: Import
// returns a top-level ucerr.ValidationError on "payload"; Validate reports a HARD
// line-0 entry with an empty preview.
func validateImportPayloadSize(decodedBytes int) *capViolation {
	if decodedBytes <= cardImportPayloadByteCap {
		return nil
	}
	return &capViolation{Line: 0, Field: "payload", Message: fmt.Sprintf("payload exceeds %d bytes", cardImportPayloadByteCap)}
}

// validateImportRows enforces the two per-parse import caps shared by Validate and
// Import: the parsed-row cap (cardImportParsedRowCap) and the per-side grapheme cap
// (domain.CardTextMax). Together with validateImportPayloadSize — which covers the
// third cap, on the decoded payload's byte length — it is the single source of cap
// logic for both paths, so they cannot re-diverge.
//
// The row cap takes precedence and short-circuits: an over-cap payload returns a
// nil VO slice and only the row-cap violation (the caller must cut rows before any
// per-row error is actionable), mirroring Import's "row cap is a top-level reject"
// ordering. Below the row cap, every row's front and back are parsed via
// domain.ParseCardText and the resulting VOs are returned parallel to words. When
// the returned violation slice is empty every row passed and validated[i] holds
// row i's front/back VOs for reuse; when it is non-empty the caller aborts the
// whole batch, so the VO slice is unused.
func validateImportRows(words []textdic.ParsedWord) ([]validatedCard, []capViolation) {
	if len(words) > cardImportParsedRowCap {
		return nil, []capViolation{{Line: 0, Field: "payload", Message: fmt.Sprintf("payload exceeds %d row cap", cardImportParsedRowCap)}}
	}
	validated := make([]validatedCard, len(words))
	var out []capViolation
	for i, w := range words {
		front, ferr := domain.ParseCardText(w.Front, domain.ErrCardFrontRequired, domain.ErrCardFrontTooLong)
		if errors.Is(ferr, domain.ErrCardFrontTooLong) {
			out = append(out, capViolation{Line: w.Line, Field: "front", Message: fmt.Sprintf("front must be at most %d characters", domain.CardTextMax)})
		}
		back, berr := domain.ParseCardText(w.Back, domain.ErrCardBackRequired, domain.ErrCardBackTooLong)
		if errors.Is(berr, domain.ErrCardBackTooLong) {
			out = append(out, capViolation{Line: w.Line, Field: "back", Message: fmt.Sprintf("back must be at most %d characters", domain.CardTextMax)})
		}
		validated[i] = validatedCard{front: front, back: back}
	}
	return validated, out
}

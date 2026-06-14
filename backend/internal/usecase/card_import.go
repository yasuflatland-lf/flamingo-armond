package usecase

import (
	"context"
	"encoding/base64"
	"fmt"
	"log/slog"
	"time"

	"github.com/rotisserie/eris"
	"gorm.io/gorm"

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
	UpsertManyTx(ctx context.Context, tx *gorm.DB, cards []*domain.Card) (repository.UpsertManyTxResult, error)
}

// CardImportUsecase exposes authenticated card import validation and owner-only
// batch import into a cardgroup.
type CardImportUsecase interface {
	Import(ctx context.Context, input ImportCardsInput) (ImportCardsOutput, error)
	Validate(ctx context.Context, payload string) (ValidateCardImportOutcome, error)
}

// ImportCardsInput is the wire-shape consumed by CardImportUsecase.Import.
// Payload reuses the validateCardImport base64-encoded text format.
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

// cardImportUsecase wires auth, ownership, the textdic parser, the card
// repository, and the transaction runner that persists the import.
type cardImportUsecase struct {
	cardgroupRepo     CardgroupOwnershipFinder
	cardRepo          CardImportCardRepository
	tx                txRunner
	processCardImport func(string) ([]textdic.ParsedWord, []textdic.ValidationError, error)
	logger            *slog.Logger
}

// NewCardImportUsecase constructs a CardImportUsecase. db is the gorm handle
// used to open transactions; pass the same *gorm.DB used by the other usecase
// constructors. Passing a nil db defers transaction wiring; the usecase will
// return INTERNAL when Import is invoked without a tx runner.
func NewCardImportUsecase(cardgroupRepo CardgroupOwnershipFinder, cardRepo CardImportCardRepository, db *gorm.DB, logger *slog.Logger) *cardImportUsecase {
	if cardgroupRepo == nil {
		panic("usecase: card import: cardgroupRepo is required")
	}
	if logger == nil {
		panic("usecase: card import: logger is required")
	}
	uc := &cardImportUsecase{cardgroupRepo: cardgroupRepo, cardRepo: cardRepo, processCardImport: textdic.Process, logger: logger}
	if db != nil {
		uc.tx = func(ctx context.Context, fn func(tx *gorm.DB) error) error {
			return db.WithContext(ctx).Transaction(fn)
		}
	}
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

	process := u.processCardImport
	if process == nil {
		process = textdic.Process
	}
	words, parseErrs, perr := process(string(decoded))
	if perr != nil {
		return ValidateCardImportOutcome{}, eris.Wrap(perr, "usecase: card import validate: parse")
	}

	parsed := make([]ParsedCard, 0, len(words))
	for _, w := range words {
		parsed = append(parsed, ParsedCard{Front: w.Front, Back: w.Back, Line: w.Line})
	}
	errs := cardImportErrorsFromTextdic(parseErrs)
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
	if err := authorizeCardgroupOrBadInput(ctx, u.cardgroupRepo, input.CardgroupID, caller.Sub); err != nil {
		return ImportCardsOutput{}, err
	}

	if input.Payload == "" {
		return ImportCardsOutput{}, ucerr.NewValidationError("payload", "payload must not be empty")
	}
	decoded, err := base64.StdEncoding.DecodeString(input.Payload)
	if err != nil {
		return ImportCardsOutput{}, ucerr.NewValidationError("payload", "payload must be standard base64-encoded text")
	}

	process := u.processCardImport
	if process == nil {
		process = textdic.Process
	}
	words, parseErrs, perr := process(string(decoded))
	if perr != nil {
		return ImportCardsOutput{}, eris.Wrap(perr, "usecase: card import: parse")
	}

	if len(words) > cardImportParsedRowCap {
		return ImportCardsOutput{}, ucerr.NewValidationError("payload", "payload exceeds 5000 row cap")
	}

	mappedErrs := cardImportErrorsFromTextdic(parseErrs)

	// Deduplicate parsed words by front within this payload. Postgres error 21000
	// ("ON CONFLICT DO UPDATE command cannot affect row a second time") fires when
	// the same conflict key appears more than once in a single INSERT statement.
	// Last occurrence wins; earlier occurrences are dropped and reported in Errors.
	lastIndex := make(map[string]int, len(words))
	for i, w := range words {
		lastIndex[w.Front] = i
	}
	deduped := make([]textdic.ParsedWord, 0, len(words))
	for i, w := range words {
		if lastIndex[w.Front] != i {
			// w is the earlier (dropped) occurrence; the row that survives is at
			// lastIndex, so its Back is the value that overrode this one.
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
		return ImportCardsOutput{Errors: mappedErrs}, nil
	}

	now := time.Now().UTC()
	cards := make([]*domain.Card, 0, len(words))
	for _, w := range words {
		// Build through the enforcing constructor so an over-length front/back
		// cannot reach the repository. textdic guarantees both fields are
		// present, so the realistic failure is the length cap; surface it as a
		// typed validation error (the DB CHECK would otherwise abort the tx with
		// an opaque constraint violation).
		c, err := domain.NewCard(domain.CardgroupID(input.CardgroupID), w.Front, w.Back, 0)
		if err != nil {
			return ImportCardsOutput{}, translateCardErr(err)
		}
		// NewCard stamps per-card timestamps; pin the whole batch to one now.
		c.CreatedAt = now
		c.UpdatedAt = now
		cards = append(cards, c)
	}

	if u.tx == nil {
		return ImportCardsOutput{}, eris.New("usecase: card import tx runner not configured")
	}

	var result repository.UpsertManyTxResult
	if err := u.tx(ctx, func(tx *gorm.DB) error {
		r, err := u.cardRepo.UpsertManyTx(ctx, tx, cards)
		if err != nil {
			return eris.Wrap(err, "usecase: card import: repo")
		}
		result = r
		return nil
	}); err != nil {
		if isContextDone(err) {
			return ImportCardsOutput{}, err
		}
		return ImportCardsOutput{}, eris.Wrap(err, "usecase: card import: tx")
	}

	return ImportCardsOutput{
		Inserted: result.Inserted,
		Updated:  result.Updated,
		Errors:   mappedErrs,
	}, nil
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

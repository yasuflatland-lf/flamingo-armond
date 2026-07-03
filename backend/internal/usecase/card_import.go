package usecase

import (
	"context"
	"encoding/base64"
	"errors"
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

	process := u.processCardImport
	if process == nil {
		process = textdic.Process
	}
	words, parseErrs, perr := process(string(decoded))
	if perr != nil {
		return ValidateCardImportOutcome{}, eris.Wrap(perr, "usecase: card import validate: parse")
	}

	errs := cardImportErrorsFromTextdic(parseErrs)
	_, caps := checkImportCaps(words)
	for _, v := range caps {
		errs = append(errs, CardImportError{Line: v.Line, Message: v.Message, Kind: CardImportErrKindHard})
	}
	// Whole-payload reject (row cap): mirror Import's all-or-nothing reject and
	// echo an empty ParsedCards rather than the full parsed set. checkImportCaps
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

	// Enforce the same caps the Validate preview reports, via the shared
	// checker, so the two paths cannot diverge. Checked on the raw parsed words
	// (before dedup) so Import and Validate agree exactly. The first violation
	// aborts the whole batch (all-or-nothing); the row cap short-circuits ahead
	// of any per-row length error inside the helper. On the pass path the checker
	// hands back the already-parsed CardText VOs so the build loop below can reuse
	// them instead of grapheme-scanning every row a second time.
	validated, caps := checkImportCaps(words)
	if len(caps) > 0 {
		return ImportCardsOutput{}, ucerr.NewValidationError(caps[0].Field, caps[0].Message)
	}

	mappedErrs := cardImportErrorsFromTextdic(parseErrs)

	// validated is parallel to the raw words; key each row's VOs by the dedup key
	// (last occurrence wins, matching dedupeParsedWords' survivor) so the post-dedup
	// build loop can look up the already-parsed VOs for the surviving rows.
	identityKey := func(s string) string { return s }
	voByKey := make(map[string]validatedCard, len(words))
	for i, w := range words {
		voByKey[identityKey(w.Front)] = validated[i]
	}

	// Deduplicate parsed words by front within this payload. cards.front is
	// plain text, so the conflict key is the front verbatim (identity key).
	deduped, dupErrs := dedupeParsedWords(words, identityKey)
	words = deduped
	mappedErrs = append(mappedErrs, dupErrs...)

	// Empty (but well-formed) parse: nothing to persist; surface the parser's
	// per-line diagnostics so the caller can act on them.
	if len(words) == 0 {
		return ImportCardsOutput{Errors: mappedErrs}, nil
	}

	now := time.Now().UTC()
	cards := make([]*domain.Card, 0, len(words))
	for _, w := range words {
		// Build from the CardText VOs checkImportCaps already parsed for this row
		// (single grapheme scan per row). NewCardFromValidated skips the re-scan
		// domain.NewCard would perform; the per-side length cap was enforced
		// upstream by checkImportCaps, and the realistic remaining failure is ID
		// generation. Surface any error as a typed validation error rather than
		// letting a bad value reach the repository / DB CHECK as an opaque
		// constraint violation.
		vc := voByKey[identityKey(w.Front)]
		c, err := domain.NewCardFromValidated(domain.CardgroupID(input.CardgroupID), vc.front, vc.back, 0)
		if err != nil {
			return ImportCardsOutput{}, translateCardErr(err)
		}
		// NewCardFromValidated stamps per-card timestamps; pin the whole batch to one now.
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

// capViolation is the internal result of checkImportCaps. Each consumer maps it
// into its own output shape: Validate -> CardImportError{Kind: HARD}; Import ->
// ucerr.NewValidationError. Field carries the validation field name explicitly so
// no caller has to substring-match Message.
type capViolation struct {
	Line    int    // 0 for the payload-level row cap; w.Line for a per-row length error
	Field   string // "payload" | "front" | "back"
	Message string
}

// validatedCard bundles the trimmed, grapheme-bounded CardText VOs that
// checkImportCaps parses for one import row. Returning them lets Import build the
// Card via domain.NewCardFromValidated instead of re-running domain.ParseCardText
// on the same strings — one grapheme scan per row rather than two.
type validatedCard struct {
	front domain.CardText
	back  domain.CardText
}

// checkImportCaps enforces the two import caps shared by Validate and Import: the
// parsed-row cap (cardImportParsedRowCap) and the per-side grapheme cap
// (domain.CardTextMax). It is the single source of cap logic for both paths so
// they cannot re-diverge.
//
// The row cap takes precedence and short-circuits: an over-cap payload returns a
// nil VO slice and only the row-cap violation (the caller must cut rows before any
// per-row error is actionable), mirroring Import's "row cap is a top-level reject"
// ordering. Below the row cap, every row's front and back are parsed via
// domain.ParseCardText and the resulting VOs are returned parallel to words. When
// the returned violation slice is empty every row passed and validated[i] holds
// row i's front/back VOs for reuse; when it is non-empty the caller aborts the
// whole batch, so the VO slice is unused.
func checkImportCaps(words []textdic.ParsedWord) ([]validatedCard, []capViolation) {
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

// dedupeParsedWords drops earlier duplicates by key(front), last occurrence wins,
// and reports each dropped row as a CardImportErrKindDuplicate error.
//
// Postgres error 21000 ("ON CONFLICT DO UPDATE command cannot affect row a second
// time") fires when the same conflict key appears more than once in a single
// INSERT statement, so this dedupe must run before UpsertManyTx. The key function
// adapts the conflict-key semantics to the target column: identity for plain-text
// cards.front, frontMatchKey (case-fold) for citext master_cards.front so case
// variants collapse to one row.
func dedupeParsedWords(words []textdic.ParsedWord, key func(string) string) ([]textdic.ParsedWord, []CardImportError) {
	lastIndex := make(map[string]int, len(words))
	for i, w := range words {
		lastIndex[key(w.Front)] = i
	}
	deduped := make([]textdic.ParsedWord, 0, len(words))
	var dropErrors []CardImportError
	for i, w := range words {
		if lastIndex[key(w.Front)] != i {
			// w is the earlier (dropped) occurrence; the row that survives is at
			// lastIndex, so its Back is the value that overrode this one.
			winningBack := words[lastIndex[key(w.Front)]].Back
			dropErrors = append(dropErrors, CardImportError{
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
	return deduped, dropErrors
}

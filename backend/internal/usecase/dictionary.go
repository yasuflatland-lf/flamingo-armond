package usecase

import (
	"context"
	"encoding/base64"
	"log/slog"
	"time"

	"github.com/rotisserie/eris"
	"gorm.io/gorm"

	"backend/internal/domain"
	"backend/internal/repository"
	"backend/internal/textdic"
	"backend/internal/usecase/ucerr"
)

// DictionaryCardRepository is the subset of repository.CardRepository the
// dictionary usecase consumes. Declaring a narrow interface here lets the
// existing mockCardRepository in card_test.go satisfy it without a separate
// double.
type DictionaryCardRepository interface {
	UpsertManyTx(ctx context.Context, tx *gorm.DB, cards []*domain.Card) (repository.UpsertManyTxResult, error)
}

// DictionaryUsecase exposes the admin-only bulk dictionary upsert.
type DictionaryUsecase interface {
	Upsert(ctx context.Context, input UpsertDictionaryInput) (UpsertDictionaryOutput, error)
}

// UpsertDictionaryInput is the wire-shape consumed by DictionaryUsecase.Upsert.
// Payload reuses the validateDictionary base64-encoded text format.
type UpsertDictionaryInput struct {
	CardgroupID string
	Payload     string // standard base64-encoded plain-text dictionary
}

// DictionaryErrorKind is the wire-aligned classifier for DictionaryValidationError.Kind.
// Values mirror the GraphQL DictionaryValidationKind enum literals exactly, so the
// resolver can cast string(e.Kind) → model.DictionaryValidationKind without translation.
//
// Callers MUST branch on these constants in switches; never substring-match Message.
type DictionaryErrorKind string

const (
	DictErrKindUnknown      DictionaryErrorKind = "UNKNOWN" // programming-error sentinel
	DictErrKindHard         DictionaryErrorKind = "HARD"
	DictErrKindFrontOnly    DictionaryErrorKind = "FRONT_ONLY"
	DictErrKindBackOnly     DictionaryErrorKind = "BACK_ONLY"
	DictErrKindUnrecognized DictionaryErrorKind = "UNRECOGNIZED"
	DictErrKindDuplicate    DictionaryErrorKind = "DUPLICATE"
)

// DictionaryValidationError mirrors textdic.ValidationError so callers in the
// resolver layer can reshape it into the GraphQL model without importing the
// textdic package directly.
//
// Kind distinguishes payload-level / hard errors, grammar-recovered skips, and
// dedupe warnings. Snippet carries parser-extracted text. Front / Back are
// populated only for dedupe duplicates. Callers MUST branch on Kind rather than
// substring-matching Message; Message stays as the UI-facing description.
type DictionaryValidationError struct {
	Line    int                 `json:"line"`
	Message string              `json:"message"`
	Kind    DictionaryErrorKind `json:"kind"`
	Snippet string              `json:"snippet,omitempty"` // parser-cut content
	Front   string              `json:"front,omitempty"`   // dedupe-only
	Back    string              `json:"back,omitempty"`    // dedupe-only
}

// UpsertDictionaryOutput is the result returned to the caller. Inserted +
// Updated equals the number of cards persisted; Errors carries the per-line
// parser diagnostics that did not block the upsert.
type UpsertDictionaryOutput struct {
	Inserted int64
	Updated  int64
	Errors   []DictionaryValidationError
}

// dictionaryParsedRowCap is the maximum number of parsed rows accepted in a
// single upsert. The limit applies to the parser's *output* — a payload with
// more lines but fewer parsed rows (after errors are filtered) is still
// allowed.
const dictionaryParsedRowCap = 5000

// dictionaryUsecase wires the auth check, the textdic parser, the card
// repository, and the transaction runner that persists the upsert.
type dictionaryUsecase struct {
	auth     AdminChecker
	cardRepo DictionaryCardRepository
	tx       txRunner
	logger   *slog.Logger
}

// NewDictionaryUsecase constructs a DictionaryUsecase. db is the gorm handle
// used to open transactions; pass the same *gorm.DB used by the other
// usecase constructors. Passing a nil db defers transaction wiring; the
// usecase will return INTERNAL when Upsert is invoked without a tx runner.
func NewDictionaryUsecase(authSvc AdminChecker, cardRepo DictionaryCardRepository, db *gorm.DB, logger *slog.Logger) *dictionaryUsecase {
	if logger == nil {
		panic("usecase: dictionary: logger is required")
	}
	uc := &dictionaryUsecase{auth: authSvc, cardRepo: cardRepo, logger: logger}
	if db != nil {
		uc.tx = func(ctx context.Context, fn func(tx *gorm.DB) error) error {
			return db.WithContext(ctx).Transaction(fn)
		}
	}
	return uc
}

// NewDictionaryUsecaseWithTx is the test-time constructor that injects an
// explicit transaction runner. Production callers must use
// NewDictionaryUsecase.
func NewDictionaryUsecaseWithTx(authSvc AdminChecker, cardRepo DictionaryCardRepository, tx txRunner, logger *slog.Logger) *dictionaryUsecase {
	if logger == nil {
		panic("usecase: dictionary: logger is required")
	}
	return &dictionaryUsecase{auth: authSvc, cardRepo: cardRepo, tx: tx, logger: logger}
}

// Upsert ingests a base64-encoded dictionary payload, parses it, and persists
// the parsed cards into the target cardgroup. Authorization is admin-only:
// administrators may target any cardgroup (no ownership check). Empty parser
// output (e.g. a non-empty payload whose every line failed to parse — note
// that an empty payload is rejected with BAD_USER_INPUT before reaching the
// parser) returns a zero-valued result with the parser error surfaced via
// Output.Errors.
func (u *dictionaryUsecase) Upsert(ctx context.Context, input UpsertDictionaryInput) (UpsertDictionaryOutput, error) {
	if _, err := requireAdmin(ctx, u.auth); err != nil {
		return UpsertDictionaryOutput{}, err
	}

	if input.CardgroupID == "" {
		return UpsertDictionaryOutput{}, ucerr.NewValidationError("cardgroupId", "cardgroupId is required")
	}

	if input.Payload == "" {
		return UpsertDictionaryOutput{}, ucerr.NewValidationError("payload", "payload must not be empty")
	}
	decoded, err := base64.StdEncoding.DecodeString(input.Payload)
	if err != nil {
		return UpsertDictionaryOutput{}, ucerr.NewValidationError("payload", "payload must be standard base64-encoded text")
	}

	words, parseErrs, perr := textdic.Process(string(decoded))
	if perr != nil {
		return UpsertDictionaryOutput{}, eris.Wrap(perr, "usecase: dictionary upsert: parse")
	}

	if len(words) > dictionaryParsedRowCap {
		return UpsertDictionaryOutput{}, ucerr.NewValidationError("payload", "payload exceeds 5000 row cap")
	}

	mappedErrs := make([]DictionaryValidationError, 0, len(parseErrs))
	for _, e := range parseErrs {
		mappedErrs = append(mappedErrs, DictionaryValidationError{
			Line:    e.Line,
			Message: e.Message,
			Kind:    DictionaryErrorKind(e.Kind.String()),
			Snippet: e.Snippet,
		})
	}

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
			mappedErrs = append(mappedErrs, DictionaryValidationError{
				Line:    w.Line,
				Message: "duplicate front in payload (later occurrence wins)",
				Kind:    DictErrKindDuplicate,
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
		return UpsertDictionaryOutput{Errors: mappedErrs}, nil
	}

	now := time.Now().UTC()
	cards := make([]*domain.Card, 0, len(words))
	for _, w := range words {
		c := &domain.Card{
			CardgroupID: input.CardgroupID,
			Front:       w.Front,
			Back:        w.Back,
			CreatedAt:   now,
			UpdatedAt:   now,
		}
		cards = append(cards, c)
	}

	if u.tx == nil {
		return UpsertDictionaryOutput{}, eris.New("usecase: dictionary tx runner not configured")
	}

	var result repository.UpsertManyTxResult
	if err := u.tx(ctx, func(tx *gorm.DB) error {
		r, err := u.cardRepo.UpsertManyTx(ctx, tx, cards)
		if err != nil {
			return eris.Wrap(err, "usecase: dictionary upsert: repo")
		}
		result = r
		return nil
	}); err != nil {
		if isContextDone(err) {
			return UpsertDictionaryOutput{}, err
		}
		return UpsertDictionaryOutput{}, eris.Wrap(err, "usecase: dictionary upsert: tx")
	}

	return UpsertDictionaryOutput{
		Inserted: result.Inserted,
		Updated:  result.Updated,
		Errors:   mappedErrs,
	}, nil
}

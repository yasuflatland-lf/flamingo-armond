package usecase

import (
	"context"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/rotisserie/eris"
	"gorm.io/gorm"

	"backend/internal/domain"
	"backend/internal/repository"
	"backend/internal/textdic"
	"backend/internal/usecase/ucerr"
)

// cardImportResult is the shared import pipeline's return shape.
// ImportCardsOutput and ImportMasterCardsOutput are field-identical to it, so
// each path converts with a direct struct conversion; a divergence in either
// public type breaks the build instead of silently dropping a field.
type cardImportResult struct {
	Inserted int64
	Updated  int64
	Errors   []CardImportError
}

// cardImportPipeline parameterises the three differences between the user-deck
// and the master-deck import paths — the in-payload conflict key, the row
// constructor, and the path-specific transaction-error translation. Every step
// from the payload down lives in runCardImport, so the two paths cannot drift
// apart again. R is the persistence row type (*domain.Card for a user deck,
// *domain.MasterCard for a master deck).
//
// Authorization is deliberately NOT a pipeline parameter: each caller runs its
// own actor gate and target-deck checks before delegating, so a reader of either
// public method sees the whole authorization story without following a callback
// into shared code.
type cardImportPipeline[R any] struct {
	// wrap is this path's two-segment error prefix. Every eris wrap the
	// pipeline emits extends it (": parse", ": repo", ": tx").
	wrap string
	// process is the textdic entrypoint. nil falls back to textdic.Process;
	// tests inject a stub.
	process func(string) ([]textdic.ParsedWord, []textdic.ValidationError, error)
	// dedupeKey adapts the in-payload conflict key to the target column:
	// identityKey for plain-text cards.front, frontMatchKey for citext
	// master_cards.front.
	dedupeKey func(string) string
	// newRow builds one persistence row from the CardText VOs
	// validateImportRows already parsed, stamping the batch-wide created_at.
	newRow func(front, back domain.CardText, now time.Time) (R, error)
	// tx is the transaction runner. A nil runner is a wiring bug surfaced as an
	// internal error rather than a panic.
	tx txRunner
	// upsert performs the batch write inside the transaction.
	upsert func(ctx context.Context, tx *gorm.DB, rows []R) (repository.UpsertManyTxResult, error)
	// translateTxErr is the path-specific transaction-error translation, applied
	// before the shared classifiers. It returns nil for an error it does not
	// recognise. nil when the path has no extra translation.
	translateTxErr func(error) error
}

// runCardImport is the single implementation of the card-import recipe shared by
// the user-deck and master-deck paths: decode and size-check the payload, parse
// it, enforce the row and per-side caps, dedupe by the target column's conflict
// key, build rows from the already-parsed VOs, and upsert them in one
// transaction. Callers authorize the write first and convert the returned
// cardImportResult into their public output type.
//
// An empty (but well-formed) parse persists nothing and returns the parser
// diagnostics; every whole-payload cap violation is a top-level validation error.
func runCardImport[R any](ctx context.Context, payload string, p cardImportPipeline[R]) (cardImportResult, error) {
	if payload == "" {
		return cardImportResult{}, ucerr.NewValidationError("payload", "payload must not be empty")
	}
	decoded, err := base64.StdEncoding.DecodeString(payload)
	if err != nil {
		return cardImportResult{}, ucerr.NewValidationError("payload", "payload must be standard base64-encoded text")
	}
	// Byte cap first: it is the only cap that can be enforced without parsing,
	// and it shares the row cap's channel so both whole-payload rejects fail the
	// same way.
	if v := validateImportPayloadSize(len(decoded)); v != nil {
		return cardImportResult{}, ucerr.NewValidationError(v.Field, v.Message)
	}

	process := p.process
	if process == nil {
		process = textdic.Process
	}
	words, parseErrs, perr := process(string(decoded))
	if perr != nil {
		return cardImportResult{}, eris.Wrap(perr, p.wrap+": parse")
	}

	// Enforce the caps the Validate preview reports, via the shared checker, so
	// preview and commit cannot diverge. Checked on the raw parsed words (before
	// dedup) so both consumers see identical input; the first violation aborts
	// the whole batch (all-or-nothing) and the row cap short-circuits ahead of
	// any per-row length error inside the helper. On the pass path the checker
	// hands back the already-parsed CardText VOs so the build loop below reuses
	// them instead of grapheme-scanning every row a second time.
	validated, caps := validateImportRows(words)
	if len(caps) > 0 {
		return cardImportResult{}, ucerr.NewValidationError(caps[0].Field, caps[0].Message)
	}

	mappedErrs := cardImportErrorsFromTextdic(parseErrs)

	// validated is parallel to the raw words; key each row's VOs by the dedup key
	// (last occurrence wins, matching the dedupe survivor) so the post-dedup build
	// loop can look up the already-parsed VOs for the surviving rows.
	voByKey := make(map[string]validatedCard, len(words))
	for i, w := range words {
		voByKey[p.dedupeKey(w.Front)] = validated[i]
	}

	deduped, dupErrs := dedupeByKey(words, func(w textdic.ParsedWord) string { return p.dedupeKey(w.Front) }, duplicateFrontDiagnostic)
	words = deduped
	mappedErrs = append(mappedErrs, dupErrs...)

	// Empty (but well-formed) parse: nothing to persist; surface the parser's
	// per-line diagnostics so the caller can act on them.
	if len(words) == 0 {
		return cardImportResult{Errors: mappedErrs}, nil
	}

	now := time.Now().UTC()
	rows := make([]R, 0, len(words))
	for _, w := range words {
		// Build from the VOs validateImportRows already parsed for this row
		// (single grapheme scan per row). The per-side length cap was enforced
		// upstream, so the realistic remaining failure is ID generation; surface
		// it as a typed validation error rather than letting a bad value reach
		// the repository and the DB CHECK as an opaque constraint violation.
		vc := voByKey[p.dedupeKey(w.Front)]
		row, err := p.newRow(vc.front, vc.back, now)
		if err != nil {
			return cardImportResult{}, translateCardErr(err)
		}
		rows = append(rows, row)
	}

	var result repository.UpsertManyTxResult
	if err := runInTx(ctx, p.tx, func(tx *gorm.DB) error {
		r, err := p.upsert(ctx, tx, rows)
		if err != nil {
			return eris.Wrap(err, p.wrap+": repo")
		}
		result = r
		return nil
	}); err != nil {
		if p.translateTxErr != nil {
			if translated := p.translateTxErr(err); translated != nil {
				return cardImportResult{}, translated
			}
		}
		if isContextDone(err) {
			return cardImportResult{}, err
		}
		if translated := translateTextLengthViolation(err); translated != nil {
			return cardImportResult{}, translated
		}
		return cardImportResult{}, eris.Wrap(err, p.wrap+": tx")
	}

	return cardImportResult{Inserted: result.Inserted, Updated: result.Updated, Errors: mappedErrs}, nil
}

// identityKey is the in-payload conflict key for plain-text cards.front: the
// front verbatim. Its citext counterpart is frontMatchKey, which case-folds so
// master_cards.front variants collapse to one row.
func identityKey(front string) string { return front }

// duplicateFrontDiagnostic reports a parsed row dropped by the in-payload dedupe.
// survivor is the row that kept the conflict key, so its Back is the value that
// overrode the dropped row's.
func duplicateFrontDiagnostic(dropped, survivor textdic.ParsedWord) CardImportError {
	return CardImportError{
		Line:    dropped.Line,
		Message: fmt.Sprintf("duplicated front (%s) was overridden with the new back (%s)", dropped.Front, survivor.Back),
		Kind:    CardImportErrKindDuplicate,
		Front:   dropped.Front,
		Back:    dropped.Back,
	}
}

// dedupeByKey drops earlier duplicates by key(item) — last occurrence wins — and
// reports each dropped item through the caller-supplied diagnostic builder, which
// receives both the dropped item and the survivor that displaced it.
//
// Postgres error 21000 ("ON CONFLICT DO UPDATE command cannot affect row a second
// time") fires when the same conflict key appears more than once in a single
// INSERT statement, so this dedupe must run before any multi-row upsert. The key
// function adapts the conflict-key semantics to the target column: identityKey for
// plain-text cards.front, frontMatchKey (case-fold) for citext master_cards.front
// so case variants collapse to one row.
func dedupeByKey[T any](items []T, key func(T) string, diagnostic func(dropped, survivor T) CardImportError) ([]T, []CardImportError) {
	lastIndex := make(map[string]int, len(items))
	for i, item := range items {
		lastIndex[key(item)] = i
	}
	kept := make([]T, 0, len(items))
	var dropErrors []CardImportError
	for i, item := range items {
		if survivor := lastIndex[key(item)]; survivor != i {
			dropErrors = append(dropErrors, diagnostic(item, items[survivor]))
			continue
		}
		kept = append(kept, item)
	}
	return kept, dropErrors
}

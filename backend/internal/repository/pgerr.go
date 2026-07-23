package repository

import (
	"errors"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"
)

// pgConstraintViolation reports whether err unwraps to a *pgconn.PgError whose
// SQLSTATE Code equals code and whose ConstraintName contains constraintSubstr.
// It returns false for any non-pg error, a code mismatch, or a constraint-name
// miss, so callers can use it as a pre-filter before falling through to
// eris.Wrap. The SQLSTATE constant ("23503" FK, "23505" unique) and the
// sentinel each caller returns stay at the call site; this helper only performs
// the shared errors.As + Code + ConstraintName predicate every classify* wrapper
// repeated.
func pgConstraintViolation(err error, code, constraintSubstr string) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == code && strings.Contains(pgErr.ConstraintName, constraintSubstr)
}

// pgInvalidTextRepresentation reports whether err unwraps to a
// *pgconn.PgError for SQLSTATE 22P02 (invalid_text_representation).
func pgInvalidTextRepresentation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "22P02"
}

// textLengthConstraintSuffix is the naming convention every text-length CHECK
// constraint in the schema follows: "<table>_<column>_length", e.g.
// cards_front_length, cardgroups_name_length, master_cards_back_length.
const textLengthConstraintSuffix = "_length"

// TextLengthViolationError reports a Postgres CHECK violation (SQLSTATE 23514)
// on one of the "<table>_<column>_length" constraints that back the card and
// cardgroup text columns.
//
// The domain layer already enforces the user-visible length rule in grapheme
// clusters, and the database bound is deliberately far wider, so this is a
// backstop that fires only for pathological input -- a grapheme cluster admits
// an unbounded combining-mark run, so no finite code-point bound closes the gap.
// When it fires, the input is still user-supplied text that is too long for the
// column -- classifying it as an internal error would hide a fixable input
// problem behind a generic failure.
// The usecase layer maps this to a field-scoped BAD_USER_INPUT via
// translateTextLengthViolation.
//
// Field is the column the constraint guards ("front", "back", "name"), derived
// from the constraint name so callers do not need a per-table lookup table.
// Pointer receiver on Error() so callers recover it with
// errors.AsType[*TextLengthViolationError] even after eris.Wrap.
type TextLengthViolationError struct {
	Constraint string
	Field      string
}

func (e *TextLengthViolationError) Error() string {
	return "repository: text length constraint violated: " + e.Constraint
}

// classifyTextLengthViolation maps a Postgres CHECK violation (code 23514) on a
// "<table>_<column>_length" constraint to a *TextLengthViolationError. It
// returns nil for any other error -- including a 23514 on a differently-named
// CHECK, which carries no field attribution and stays an internal error -- so
// callers can use it as a pre-filter before falling through to eris.Wrap
// (mirrors classifyMasterCardFKError).
func classifyTextLengthViolation(err error) error {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23514" {
		return nil
	}
	field, ok := textLengthConstraintField(pgErr.ConstraintName)
	if !ok {
		return nil
	}
	return &TextLengthViolationError{Constraint: pgErr.ConstraintName, Field: field}
}

// textLengthConstraintField extracts the guarded column from a
// "<table>_<column>_length" constraint name: "cards_front_length" -> "front",
// "master_cardgroups_name_length" -> "name". Reports false when the name does
// not follow the convention or carries no column segment.
func textLengthConstraintField(name string) (string, bool) {
	stem, ok := strings.CutSuffix(name, textLengthConstraintSuffix)
	if !ok {
		return "", false
	}
	i := strings.LastIndex(stem, "_")
	if i < 0 || i == len(stem)-1 {
		return "", false
	}
	return stem[i+1:], true
}

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

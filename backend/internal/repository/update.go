package repository

import "github.com/rotisserie/eris"

// refetchAfterUpdate is the shared tail of the aggregate Update methods. Given the
// RowsAffected count of a completed Updates call, it returns notFoundErr when no
// row matched, otherwise re-fetches the fresh row via refetch (so callers observe
// trigger-refreshed columns such as updated_at). A non-empty refetchErrPrefix wraps
// a refetch failure with that caller-supplied prefix; pass "" to return the refetch
// error unwrapped. The caller runs Updates and classifies/wraps res.Error before
// calling this helper — that step diverges per aggregate (e.g. role classifies a
// unique violation) and is deliberately NOT owned here (see
// .claude/rules/error-wrapping.md: shared helpers take a caller-supplied prefix,
// never a fixed one).
func refetchAfterUpdate[T any](rowsAffected int64, notFoundErr error, refetch func() (*T, error), refetchErrPrefix string) (*T, error) {
	if rowsAffected == 0 {
		return nil, notFoundErr
	}
	row, err := refetch()
	if err != nil {
		if refetchErrPrefix != "" {
			return nil, eris.Wrap(err, refetchErrPrefix)
		}
		return nil, err
	}
	return row, nil
}

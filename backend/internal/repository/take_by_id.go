package repository

import (
	"context"
	"errors"

	"github.com/rotisserie/eris"
	"gorm.io/gorm"
)

// takeByID runs a single-row lookup (db.Where(where, args...).Take) and
// classifies the result into one of three outcomes: the populated row, the
// caller-supplied not-found sentinel, or an infrastructure error wrapped with
// the caller-supplied prefix.
//
// Both the sentinel and the wrap prefix are parameters rather than constants
// because they vary per aggregate: most repos use ErrNotFound, roleRepo uses
// ErrRoleNotFound, and each call site owns a distinct "repository: <aggregate>:
// ..." wrap prefix. Passing the prefix in (instead of hardcoding one here) keeps
// the error_chain attribution pointed at the calling aggregate's module rather
// than this shared helper, per .claude/rules/error-wrapping.md § "Shared helpers
// must not embed a layer prefix wrap".
//
// The helper deliberately stops at the Take + not-found classification and
// returns the raw GORM row; the caller runs its own *toDomain conversion. The
// per-aggregate converters diverge (some return a bare pointer, masterCardgroup
// returns (*domain.MasterCardgroup, error)), so folding conversion in here would
// not collapse cleanly.
func takeByID[Row any](
	ctx context.Context,
	db *gorm.DB,
	where string,
	args []any,
	notFound error,
	wrap string,
) (Row, error) {
	var row Row
	if err := db.WithContext(ctx).Where(where, args...).Take(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return row, notFound
		}
		return row, eris.Wrap(err, wrap)
	}
	return row, nil
}

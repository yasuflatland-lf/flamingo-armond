package repository

import "strings"

// PageCap is the repository-level limit that lets the +1 fetch trick survive a
// request at the documented maximum page size. See
// .claude/rules/pagination.md § "Page-size cap asymmetry".
const PageCap = 101 // maxPageSize (100) + 1

// ClampPageSize clamps want to [0, PageCap]. A negative want returns 0 so the
// caller can short-circuit (totalCount-only query with no rows). A want above
// PageCap is capped at PageCap.
func ClampPageSize(want int) int {
	if want < 0 {
		return 0
	}
	if want > PageCap {
		return PageCap
	}
	return want
}

// InvertDir returns the opposite SortOrder. Used by backward pagination to
// flip the ORDER BY direction before LIMIT.
func InvertDir(d SortOrder) SortOrder {
	if d == SortDesc {
		return SortAsc
	}
	return SortDesc
}

// ReverseSlice reverses xs in place and returns it. Generic over T so card /
// cardgroup / user pagination all share one implementation.
func ReverseSlice[T any](xs []T) []T {
	for i, j := 0, len(xs)-1; i < j; i, j = i+1, j-1 {
		xs[i], xs[j] = xs[j], xs[i]
	}
	return xs
}

// paginateSetup computes the backward-paging direction-flip preamble shared by
// every cursor-paginated repository (card / master_card / cardgroup /
// master_catalog / user). Forward paging (last == 0) returns the requested direction,
// `first` as the limit, the `after` cursor, and reverse == false. Backward
// paging (last > 0) inverts the ORDER BY direction, uses `last` as the limit,
// the `before` cursor, and reverse == true so the caller reverses the fetched
// slice in memory to restore the natural order. Generic over the per-aggregate
// cursor type C because the cursor value types differ (int position vs
// *time.Time). See .claude/rules/pagination.md § "Backward pagination via
// direction-flip + reverse".
func paginateSetup[C any](dir SortOrder, first, last int, after, before *C) (effectiveDir SortOrder, limit int, cursor *C, reverse bool) {
	if last > 0 {
		return InvertDir(dir), last, before, true
	}
	return dir, first, after, false
}

// cursorTupleWhere builds the portable tuple-comparison WHERE clause shared by
// every cursor-paginated repository. The expanded form
// `field op ? OR (field = ? AND <alias>.id op ?)` is dialect-portable (the
// Postgres-only row-constructor `(a, b) > (?, ?)` is avoided). `field` is the
// already-qualified primary sort expression (e.g. `cards.created_at`,
// `COALESCE(ucs.due, cards.created_at)`, `mcg.sort_order`), `alias` is the table
// alias prefix for the id tie-break column (`cards`, `master_cards`, `mcg`, or
// `""` for an unaliased `id`), and the operators are `>` for ASC or `<` for
// DESC.
func cursorTupleWhere(alias, field, fieldOp, idOp string, fieldVal, idVal any) (string, []any) {
	idCol := "id"
	if alias != "" {
		idCol = alias + ".id"
	}
	return "(" + field + " " + fieldOp + " ? OR (" + field + " = ? AND " + idCol + " " + idOp + " ?))",
		[]any{fieldVal, fieldVal, idVal}
}

// cursorSpec describes one aggregate's cursor geometry to the generic
// where/order-clause builders. It collapses the repeated per-aggregate triplet
// (XxxOrderClause / XxxCursorWhere / XxxCursorFieldValue) into data: each repo
// builds a spec from its orderBy switch and delegates to buildOrderClause /
// buildCursorWhere, which emit the same SQL the hand-written triplets did.
//
//   - alias: the table-alias prefix for the id tie-break column (`cards`,
//     `master_cards`, `mcg`, or `""` for an unaliased `id`). Threaded straight
//     into cursorTupleWhere and used to qualify the id column in the ORDER BY
//     and id-only WHERE forms.
//   - orderCol: the already-qualified primary sort expression
//     (`cards.created_at`, `COALESCE(ucs.due, cards.created_at)`,
//     `mcg.sort_order`, …). Ignored when isIDOrder is true.
//   - idDir: an optional id tie-break direction. Empty uses the primary
//     direction.
//   - isIDOrder: the active orderBy resolves to the id column, so only the id
//     column is emitted (no tuple). Aggregates whose orderBy enum has no id
//     member (master_catalog) leave this false.
//   - fieldValue: returns the hydrated cursor value for orderCol, or the
//     cursor-missing error. Only invoked when isIDOrder is false.
type cursorSpec struct {
	alias      string
	orderCol   string
	idDir      SortOrder
	isIDOrder  bool
	fieldValue func() (any, error)
}

// idColumn returns the id tie-break column qualified by the spec's alias, or a
// bare `id` when the alias is empty (cardgroup's unaliased form).
func (s cursorSpec) idColumn() string {
	if s.alias == "" {
		return "id"
	}
	return s.alias + ".id"
}

// buildOrderClause renders the SQL ORDER BY tail for a cursorSpec and direction.
// An id-order spec emits only the id column; otherwise the primary sort column
// is followed by the id column so the ordering is always total.
func buildOrderClause(spec cursorSpec, dir SortOrder) string {
	d := string(dir)
	idCol := spec.idColumn()
	if spec.isIDOrder {
		return idCol + " " + d
	}
	idDir := d
	if spec.idDir != "" {
		idDir = string(spec.idDir)
	}
	return spec.orderCol + " " + d + ", " + idCol + " " + idDir
}

// buildCursorWhere builds the tuple-comparison WHERE for a cursorSpec, cursor id,
// and direction. ASC yields `>`, DESC yields `<`. An id-order spec emits the
// single `id op ?` form; otherwise it hydrates the cursor field value and
// delegates to cursorTupleWhere. Returns the cursor-missing error from
// spec.fieldValue unchanged.
func buildCursorWhere(spec cursorSpec, dir SortOrder, idVal any) (string, []any, error) {
	fieldOp := ">"
	if dir == SortDesc {
		fieldOp = "<"
	}
	if spec.isIDOrder {
		return spec.idColumn() + " " + fieldOp + " ?", []any{idVal}, nil
	}
	idOp := fieldOp
	if spec.idDir == SortAsc {
		idOp = ">"
	} else if spec.idDir == SortDesc {
		idOp = "<"
	}
	val, err := spec.fieldValue()
	if err != nil {
		return "", nil, err
	}
	clause, args := cursorTupleWhere(spec.alias, spec.orderCol, fieldOp, idOp, val, idVal)
	return clause, args, nil
}

// searchLikePattern wraps a non-empty trimmed search term in `%...%` after
// escaping LIKE/ILIKE metacharacters so user-supplied `%` and `_` match
// literally. Returns (pattern, false) when search is nil or trims to empty so
// callers can skip the predicate. Shared by every paginated repository that
// accepts a search argument (card / cardgroup / master_catalog / user).
func searchLikePattern(search *string) (string, bool) {
	if search == nil {
		return "", false
	}
	trimmed := strings.TrimSpace(*search)
	if trimmed == "" {
		return "", false
	}
	return "%" + escapeLikePattern(trimmed) + "%", true
}

package repository

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

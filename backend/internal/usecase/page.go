package usecase

// TrimAndDetect trims one trailing item from items when len(items) > want and
// returns (trimmed, true) so the caller can set hasNextPage / hasPreviousPage.
// Used after the repository's "+1 fetch" trick for forward pagination: ask for
// want+1 rows, pass the returned slice in, get back (page, hasMore).
//
// For backward pagination, use TrimAndDetectBackward — it trims the leading
// (not trailing) element when len > want.
func TrimAndDetect[T any](items []T, want int) (out []T, hasMore bool) {
	if want > 0 && len(items) > want {
		return items[:want], true
	}
	return items, false
}

// TrimAndDetectBackward trims one leading item when len(items) > want and
// returns (trimmed, true). Used by backward pagination where the repository
// already reversed the slice; the extra row sits at the head, not the tail.
func TrimAndDetectBackward[T any](items []T, want int) (out []T, hasMore bool) {
	if want > 0 && len(items) > want {
		return items[len(items)-want:], true
	}
	return items, false
}

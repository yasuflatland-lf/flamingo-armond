package usecase

// TrimAndDetect trims one trailing item from items when len(items) > want and
// returns (trimmed, true) so the caller can set hasNextPage / hasPreviousPage.
// Used after the repository's "+1 fetch" trick: ask for want+1 rows, pass the
// returned slice in, get back (page, hasMore).
//
// For backward pagination (last cursor), the repository already returned the
// rows with the extra leading row at index 0 (because the SQL order was
// inverted and then reversed in memory). Callers handle that direction by
// passing the slice and using out[len(out)-want:] when hasMore — the trim
// here covers the forward direction.
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

// ResolvePageSize clamps a (first, last) pair to [1, max], defaulting to def.
// When both are nil, returns def. When first is set, returns clamp(*first, 1, max).
// When last is set, returns clamp(*last, 1, max). When both are set, the caller
// is expected to have already rejected the combo (see
// .claude/rules/pagination.md § "Reject mixed-direction argument combos").
func ResolvePageSize(first, last *int, def, max int) int {
	switch {
	case first != nil:
		return clamp(*first, 1, max)
	case last != nil:
		return clamp(*last, 1, max)
	default:
		return def
	}
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

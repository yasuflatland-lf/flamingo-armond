package usecase

import "backend/internal/usecase/ucerr"

// resolveStandardPageSize clamps first/last to [0, maxPageSize] and rejects
// passing both. Defaults first=defaultPageSize (20) when neither is provided,
// matching the schema's documented default. maxPageSize/defaultPageSize are the
// package-wide page-size caps (declared in card.go) shared by the card,
// cardgroup, and master-catalog connection resolvers; the repository-level cap
// (repository.PageCap = maxPageSize + 1) is one greater so the "+1 fetch" trick
// survives a maximum-sized request. The admin connections deliberately do NOT
// use this resolver (resolveAdminPageSize defaults to maxPageSize and rejects
// rather than clamps out-of-range values).
func resolveStandardPageSize(first, last *int) (int, int, error) {
	if first != nil && last != nil {
		return 0, 0, ucerr.NewValidationError("first", "specify either first or last, not both")
	}
	if first == nil && last == nil {
		return defaultPageSize, 0, nil
	}
	clamp := func(v int) int {
		if v < 0 {
			return 0
		}
		if v > maxPageSize {
			return maxPageSize
		}
		return v
	}
	if first != nil {
		return clamp(*first), 0, nil
	}
	return 0, clamp(*last), nil
}

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

// resolveRelayPage validates Relay argument coherence, then clamps the page
// size via the per-aggregate clamp closure. Bundling validation with the
// mandatory page-size step makes validateRelayArgs structurally impossible to
// skip — every connection method needs the (first, last) return — while keeping
// validation ahead of cursor resolution, so a mixed-direction combo is reported
// before a malformed-cursor decode error (preserving error precedence).
func resolveRelayPage(
	first, last *int,
	after, before *string,
	clamp func(first, last *int) (int, int, error),
) (pageFirst, pageLast int, err error) {
	if err := validateRelayArgs(first, last, after, before); err != nil {
		return 0, 0, err
	}
	return clamp(first, last)
}

// assemblePage performs the Relay "+1 fetch" trick shared by every connection
// list method: it inflates the requested page size by one, calls fetch, then
// trims the extra row and reports hasNext/hasPrev. Forward paging (first>0)
// trims the trailing row and derives hasPrev from hasAfter; backward paging
// (last>0) trims the leading row and derives hasNext from hasBefore. A
// total-count-only request (first==0 && last==0) calls fetch with (0,0), trims
// nothing, and reports both flags false.
//
// hasAfter/hasBefore MUST be the post-decode cursor presence — i.e. pass
// (resolvedAfter != nil) / (resolvedBefore != nil) using the value returned
// by the per-aggregate resolveCursor step, NOT the raw request *string. They
// supply the "other" page-edge flag the +1 trim cannot derive: forward paging
// sets hasPrev from hasAfter, backward paging sets hasNext from hasBefore.
func assemblePage[T any](
	first, last int,
	hasAfter, hasBefore bool,
	fetch func(wantFirst, wantLast int) ([]T, error),
) (items []T, hasNext, hasPrev bool, err error) {
	wantFirst, wantLast := first, last
	if wantFirst > 0 {
		wantFirst++
	}
	if wantLast > 0 {
		wantLast++
	}
	items, err = fetch(wantFirst, wantLast)
	if err != nil {
		return nil, false, false, err
	}
	switch {
	case first > 0:
		items, hasNext = TrimAndDetect(items, first)
		hasPrev = hasAfter
	case last > 0:
		items, hasPrev = TrimAndDetectBackward(items, last)
		hasNext = hasBefore
	}
	return items, hasNext, hasPrev, nil
}

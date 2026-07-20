package usecase

import (
	"errors"
	"strconv"
	"time"

	"backend/internal/cursor"
	"backend/internal/repository"
	"backend/internal/usecase/ucerr"
)

// resolveSortDir maps the typed usecase SortOrder enum to the repository sort
// direction, defaulting to def when dir is nil. The default switch arm is
// defense in depth — gqlgen UnmarshalGQL already rejects invalid enum strings
// upstream. Shared by every aggregate's resolve*OrderBy; only the per-aggregate
// default direction (def) differs.
func resolveSortDir(dir *SortOrder, def repository.SortOrder) (repository.SortOrder, error) {
	if dir == nil {
		return def, nil
	}
	switch *dir {
	case SortOrderAsc:
		return repository.SortAsc, nil
	case SortOrderDesc:
		return repository.SortDesc, nil
	default:
		return "", ucerr.NewValidationError("orderDirection", "invalid")
	}
}

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
// by the per-aggregate resolve*Cursor step, NOT the raw request *string. They
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

// decodeCursorOrBadInput decodes an opaque cursor string, returning present=false
// for a nil/empty cursor and a BAD_USER_INPUT validation error for a malformed one.
// It covers only the decode+guard prefix shared by every aggregate's resolve*Cursor
// method; the per-aggregate hydration (FindByID / FindPublishedByID) and the
// repository cursor-struct population stay inline in each method.
//
// The returned payload carries the raw entity id for every envelope version.
// For a v2 cursor it additionally carries the ordering the page was served
// under plus the ordering-key value captured at that time; aggregates ordering
// on a mutable column consume those via requireCursorOrdering and their
// apply*OrderKey helper instead of re-reading the column off the current row.
func decodeCursorOrBadInput(cursorStr *string, field string) (p cursor.Payload, present bool, err error) {
	if cursorStr == nil || *cursorStr == "" {
		return cursor.Payload{}, false, nil
	}
	p, err = cursor.Decode(*cursorStr)
	if err != nil {
		return cursor.Payload{}, false, ucerr.NewValidationError(field, "invalid cursor")
	}
	return p, true, nil
}

// PageOrdering is the effective ordering a connection page was served under.
// It is carried on the usecase connection output so the resolver can embed it
// in the v2 cursors it emits, and compared against an incoming v2 cursor so a
// bookmark taken under one ordering is never silently re-interpreted under
// another. Both fields are server-internal tokens (the repository column name
// and sort direction); they are opaque to clients.
type PageOrdering struct {
	OrderBy   string
	Direction string
}

// requireCursorOrdering rejects a v2 cursor whose embedded ordering disagrees
// with the ordering the current request resolved to. Serving such a cursor
// would compare the stored ordering-key value against a different column (or
// the same column in the opposite direction) and silently return a wrong page,
// so it is a BAD_USER_INPUT — the same shape as "cursor not found".
//
// v1 envelopes and legacy bare ids carry no ordering and pass through: they
// fall back to the re-hydration path, which is ordering-agnostic by
// construction.
func requireCursorOrdering(p cursor.Payload, ord PageOrdering, field string) error {
	if !p.HasOrdering {
		return nil
	}
	if p.OrderBy != ord.OrderBy || p.Direction != ord.Direction {
		return ucerr.NewValidationError(field, "cursor does not match the requested ordering")
	}
	return nil
}

// errCursorKeyMalformed marks a v2 ordering-key value that does not parse back
// into the column type the active orderBy needs. Every apply*OrderKey helper
// wraps its parse failures in it so the caller can map it to BAD_USER_INPUT;
// any other error from those helpers is an internal caller bug (an orderBy the
// helper does not handle) and must stay INTERNAL.
var errCursorKeyMalformed = errors.New("usecase: malformed cursor ordering key")

// encodeTimeOrderKey serializes a timestamp ordering key. RFC3339 with
// nanosecond precision round-trips the microsecond resolution Postgres stores,
// so the tuple comparison lands on exactly the same boundary row the page ended
// on. UTC normalisation keeps the encoded form stable regardless of the
// session time zone; the comparison is by instant, so it does not shift rows.
func encodeTimeOrderKey(t time.Time) string {
	return t.UTC().Format(time.RFC3339Nano)
}

// decodeTimeOrderKey parses a timestamp ordering key produced by
// encodeTimeOrderKey. A value that does not parse is a client-supplied
// malformed cursor, not an internal fault.
func decodeTimeOrderKey(s string) (time.Time, error) {
	t, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		return time.Time{}, errCursorKeyMalformed
	}
	return t, nil
}

// decodeIntOrderKey parses an integer ordering key (the master catalog's
// sort_order). A value that does not parse is a client-supplied malformed
// cursor, not an internal fault.
func decodeIntOrderKey(s string) (int, error) {
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, errCursorKeyMalformed
	}
	return n, nil
}

// resolveOrderByColumn maps the typed usecase orderBy enum to the repository
// column via the supplied allowlist, defaulting to def when orderBy is nil, and
// delegates the direction half to the shared resolveSortDir. An orderBy outside
// the allowlist returns a BAD_USER_INPUT validation error; the default arm is
// defense in depth — gqlgen UnmarshalGQL already rejects invalid enum strings
// upstream. Each aggregate's resolve*OrderBy is a thin wrapper supplying its
// own map + (default column, default direction).
func resolveOrderByColumn[K comparable, V any](
	orderBy *K,
	dir *SortOrder,
	allow map[K]V,
	def V,
	defDir repository.SortOrder,
) (V, repository.SortOrder, error) {
	field := def
	if orderBy != nil {
		col, ok := allow[*orderBy]
		if !ok {
			var zero V
			return zero, "", ucerr.NewValidationError("orderBy", "invalid")
		}
		field = col
	}
	d, err := resolveSortDir(dir, defDir)
	if err != nil {
		var zero V
		return zero, "", err
	}
	return field, d, nil
}

// firstLastCursor returns the id() of the first and last rows, or "","" when the
// slice is empty. Shared by every connection usecase's StartCur / EndCur tail;
// the id accessor extracts the raw node id (the resolver applies the cursor
// encoder once downstream).
func firstLastCursor[T any](rows []T, id func(T) string) (start, end string) {
	if len(rows) == 0 {
		return "", ""
	}
	return id(rows[0]), id(rows[len(rows)-1])
}

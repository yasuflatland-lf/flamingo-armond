package usecase

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"backend/internal/cursor"
	"backend/internal/usecase/ucerr"
)

// resolveStandardPageSize clamps first to [0, maxPageSize]. Defaults
// first=defaultPageSize (20) when it is omitted, matching the schema's
// documented default. maxPageSize/defaultPageSize are the
// package-wide page-size caps (declared in card.go) shared by the card,
// cardgroup, master-catalog and master-card connection resolvers; the
// repository-level cap (repository.PageCap = maxPageSize + 1) is one greater so
// the "+1 fetch" trick survives a maximum-sized request. Admin gating does not
// imply the admin page-size contract: the admin master-card and master-catalog
// connections use this resolver too. The admin user list is the sole opt-out
// (resolveAdminPageSize defaults to maxPageSize and rejects rather than clamps
// out-of-range values).
func resolveStandardPageSize(first *int) (int, error) {
	if first == nil {
		return defaultPageSize, nil
	}
	if *first < 0 {
		return 0, nil
	}
	if *first > maxPageSize {
		return maxPageSize, nil
	}
	return *first, nil
}

// resolveAdminPageSize rejects (rather than clamps) a first outside
// [0, maxPageSize]. When first is nil it defaults to maxPageSize — unlike
// resolveStandardPageSize (which defaults to defaultPageSize=20) — so
// single-page admin views stay simple. The admin user list is its only caller;
// every other connection, admin-gated or not, uses resolveStandardPageSize.
// maxPageSize is the package-wide cap shared with resolveStandardPageSize.
func resolveAdminPageSize(first *int) (int, error) {
	if first == nil {
		return maxPageSize, nil
	}
	if *first < 0 {
		return 0, ucerr.NewValidationError("first", "first must be >= 0")
	}
	if *first > maxPageSize {
		return 0, ucerr.NewValidationError("first", fmt.Sprintf("first must be <= %d", maxPageSize))
	}
	return *first, nil
}

// TrimAndDetect trims one trailing item from items when len(items) > want and
// returns (trimmed, true) so the caller can set hasNextPage.
// Used after the repository's "+1 fetch" trick for forward pagination: ask for
// want+1 rows, pass the returned slice in, get back (page, hasMore).
func TrimAndDetect[T any](items []T, want int) (out []T, hasMore bool) {
	if want > 0 && len(items) > want {
		return items[:want], true
	}
	return items, false
}

// resolveRelayPage validates Relay argument coherence, then clamps the page
// size via the per-aggregate clamp closure. Bundling validation with the
// mandatory page-size step makes validateRelayArgs structurally impossible to
// skip — every connection method needs the page-size return — while keeping
// validation ahead of cursor resolution, so an `after` without `first` is
// reported before a malformed-cursor decode error (preserving error precedence).
func resolveRelayPage(
	first *int,
	after *string,
	clamp func(first *int) (int, error),
) (int, error) {
	if err := validateRelayArgs(first, after); err != nil {
		return 0, err
	}
	return clamp(first)
}

// assemblePage performs the Relay "+1 fetch" trick shared by every connection
// list method: it inflates the requested page size by one, calls fetch, trims
// the trailing extra row to derive hasNext, and derives hasPrev from hasAfter.
// A total-count-only request (first==0) calls fetch with 0, trims nothing, and
// reports both flags false.
//
// hasAfter MUST be the post-decode cursor presence — i.e. pass
// (resolvedAfter != nil) using the value returned by the per-aggregate
// resolve*Cursor step, NOT the raw request *string. It supplies the
// hasPreviousPage flag the +1 trim cannot derive.
func assemblePage[T any](
	first int,
	hasAfter bool,
	fetch func(want int) ([]T, error),
) (items []T, hasNext, hasPrev bool, err error) {
	want := first
	if want > 0 {
		want++
	}
	items, err = fetch(want)
	if err != nil {
		return nil, false, false, err
	}
	if first > 0 {
		items, hasNext = TrimAndDetect(items, first)
		hasPrev = hasAfter
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
// with the connection's fixed ordering. With one ordering per connection, a
// mismatch means a bookmark an older client took under an ordering the server
// no longer serves. Serving such a cursor would compare the stored
// ordering-key value against a different column (or the same column in the
// opposite direction) and silently return a wrong page, so it is a
// BAD_USER_INPUT — the same shape as "cursor not found".
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

// rejectOrderedCursor is the counterpart requireCursorOrdering for connections
// that do not emit v2: it rejects any inbound cursor carrying ordering
// metadata, because such a cursor cannot have come from this connection.
//
// The guard exists because Decode is shared. A connection that ignores the
// embedded ordering would accept a v2 cursor and page by the raw id alone —
// serving a bookmark under an ordering that was never validated against the
// request. Rejecting is also the behaviour these connections had before Decode
// learned the v2 envelope: a "v2:" string then fell through the bare-id branch
// and failed the row lookup as cursor-not-found.
//
// Call it at every resolve*Cursor that consumes only p.ID. Once a connection
// migrates to v2, swap the call for requireCursorOrdering.
func rejectOrderedCursor(p cursor.Payload, field string) error {
	if p.HasOrdering {
		return ucerr.NewValidationError(field, "cursor does not match the requested ordering")
	}
	return nil
}

// errCursorKeyMalformed marks a v2 ordering-key value that does not parse back
// into the type of the connection's ordering column. Every apply*OrderKey helper
// returns it in place of the underlying parse failure so the caller can map it
// to BAD_USER_INPUT; the parse cause is deliberately dropped because no caller
// surfaces it (each one answers with a fresh ucerr validation error).
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

// derefOr returns *p when p is non-nil, otherwise def.
func derefOr[T any](p *T, def T) T {
	if p != nil {
		return *p
	}
	return def
}

// normalizeSearch collapses nil and whitespace-only search inputs to nil and
// trims a non-empty search. After this the repository receives either nil (no
// filter) or a non-empty, trimmed string — the same invariant ListMasterCards
// relies on. Normalizing at the usecase boundary keeps totalCount and the page
// query in agreement instead of depending on the repository to trim.
func normalizeSearch(search *string) *string {
	if search == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*search)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

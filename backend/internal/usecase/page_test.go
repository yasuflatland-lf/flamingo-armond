package usecase

import (
	"errors"
	"reflect"
	"testing"
	"time"

	"backend/internal/cursor"
	"backend/internal/repository"
	"backend/internal/usecase/ucerr"
)

// TestResolveSortDir covers the shared SortOrder -> repository.SortOrder helper
// extracted from the four resolve*OrderBy functions: nil falls back to the
// caller-supplied default, ASC/DESC map to repository.SortAsc/SortDesc, and an
// out-of-range value is a field-level orderDirection validation error.
func TestResolveSortDir(t *testing.T) {
	t.Parallel()

	asc := SortOrderAsc
	desc := SortOrderDesc
	bogus := SortOrder("SIDEWAYS")

	tests := []struct {
		name    string
		dir     *SortOrder
		def     repository.SortOrder
		want    repository.SortOrder
		wantErr bool
	}{
		{name: "nil -> default ASC", dir: nil, def: repository.SortAsc, want: repository.SortAsc},
		{name: "nil -> default DESC", dir: nil, def: repository.SortDesc, want: repository.SortDesc},
		{name: "ASC -> repository.SortAsc", dir: &asc, def: repository.SortDesc, want: repository.SortAsc},
		{name: "DESC -> repository.SortDesc", dir: &desc, def: repository.SortAsc, want: repository.SortDesc},
		{name: "invalid -> validation error", dir: &bogus, def: repository.SortAsc, wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := resolveSortDir(tc.dir, tc.def)
			if tc.wantErr {
				var ve *ucerr.ValidationError
				if !errors.As(err, &ve) {
					t.Fatalf("want ValidationError, got %v", err)
				}
				if ve.Field != "orderDirection" {
					t.Fatalf("want field orderDirection, got %q", ve.Field)
				}
				if got != "" {
					t.Fatalf("want empty direction on error, got %q", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestTrimAndDetect(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		items    []int
		want     int
		wantOut  []int
		wantMore bool
	}{
		{
			name:     "want == 0 returns input unchanged, no more",
			items:    []int{1, 2, 3},
			want:     0,
			wantOut:  []int{1, 2, 3},
			wantMore: false,
		},
		{
			name:     "len < want returns input unchanged, no more",
			items:    []int{1, 2},
			want:     5,
			wantOut:  []int{1, 2},
			wantMore: false,
		},
		{
			name:     "len == want returns input unchanged, no more",
			items:    []int{1, 2, 3},
			want:     3,
			wantOut:  []int{1, 2, 3},
			wantMore: false,
		},
		{
			name:     "len == want+1 trims trailing element, hasMore=true",
			items:    []int{1, 2, 3, 4},
			want:     3,
			wantOut:  []int{1, 2, 3},
			wantMore: true,
		},
		{
			name:     "len > want+1 trims to items[:want] only, hasMore=true",
			items:    []int{1, 2, 3, 4, 5, 6},
			want:     3,
			wantOut:  []int{1, 2, 3},
			wantMore: true,
		},
		{
			name:     "empty slice returns empty, no more",
			items:    []int{},
			want:     5,
			wantOut:  []int{},
			wantMore: false,
		},
		{
			name:     "nil input returns nil, no more",
			items:    nil,
			want:     3,
			wantOut:  nil,
			wantMore: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			gotOut, gotMore := TrimAndDetect(tc.items, tc.want)
			if gotMore != tc.wantMore {
				t.Fatalf("hasMore: got %v, want %v", gotMore, tc.wantMore)
			}
			if !reflect.DeepEqual(gotOut, tc.wantOut) {
				t.Fatalf("out: got %v, want %v", gotOut, tc.wantOut)
			}
		})
	}
}

func TestTrimAndDetectBackward(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		items    []int
		want     int
		wantOut  []int
		wantMore bool
	}{
		{
			name:     "want == 0 returns input unchanged, no more",
			items:    []int{1, 2, 3},
			want:     0,
			wantOut:  []int{1, 2, 3},
			wantMore: false,
		},
		{
			name:     "len < want returns input unchanged, no more",
			items:    []int{1, 2},
			want:     5,
			wantOut:  []int{1, 2},
			wantMore: false,
		},
		{
			name:     "len == want returns input unchanged, no more",
			items:    []int{1, 2, 3},
			want:     3,
			wantOut:  []int{1, 2, 3},
			wantMore: false,
		},
		{
			name:     "len == want+1 trims leading element, hasMore=true",
			items:    []int{1, 2, 3, 4},
			want:     3,
			wantOut:  []int{2, 3, 4},
			wantMore: true,
		},
		{
			name:     "len > want+1 keeps trailing want items, hasMore=true",
			items:    []int{1, 2, 3, 4, 5, 6},
			want:     3,
			wantOut:  []int{4, 5, 6},
			wantMore: true,
		},
		{
			name:     "empty slice returns empty, no more",
			items:    []int{},
			want:     5,
			wantOut:  []int{},
			wantMore: false,
		},
		{
			name:     "nil input returns nil, no more",
			items:    nil,
			want:     3,
			wantOut:  nil,
			wantMore: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			gotOut, gotMore := TrimAndDetectBackward(tc.items, tc.want)
			if gotMore != tc.wantMore {
				t.Fatalf("hasMore: got %v, want %v", gotMore, tc.wantMore)
			}
			if !reflect.DeepEqual(gotOut, tc.wantOut) {
				t.Fatalf("out: got %v, want %v", gotOut, tc.wantOut)
			}
		})
	}
}

// TestTrimAndDetect_DirectionDistinction explicitly verifies that forward and
// backward trimming produce different results from the same input, confirming
// that TrimAndDetect removes the tail while TrimAndDetectBackward removes the
// head.
func TestTrimAndDetect_DirectionDistinction(t *testing.T) {
	t.Parallel()

	in := []int{1, 2, 3, 4} // len == want+1 == 4, want == 3
	want := 3

	forward, fMore := TrimAndDetect(in, want)
	backward, bMore := TrimAndDetectBackward(in, want)

	if !fMore {
		t.Fatal("TrimAndDetect: expected hasMore=true")
	}
	if !bMore {
		t.Fatal("TrimAndDetectBackward: expected hasMore=true")
	}

	wantForward := []int{1, 2, 3}
	wantBackward := []int{2, 3, 4}

	if !reflect.DeepEqual(forward, wantForward) {
		t.Fatalf("TrimAndDetect: got %v, want %v", forward, wantForward)
	}
	if !reflect.DeepEqual(backward, wantBackward) {
		t.Fatalf("TrimAndDetectBackward: got %v, want %v", backward, wantBackward)
	}
	if reflect.DeepEqual(forward, backward) {
		t.Fatal("forward and backward results must differ")
	}
}

func TestAssemblePage_ForwardTrimsTrailingAndSetsHasNext(t *testing.T) {
	t.Parallel()
	// first=2 → fetch must be asked for 3 (+1 trick). Return 3 → trim to 2, hasNext=true.
	items, hasNext, hasPrev, err := assemblePage(2, 0, true, false,
		func(wantFirst, wantLast int) ([]int, error) {
			if wantFirst != 3 || wantLast != 0 {
				t.Fatalf("want fetch(3,0) for first=2, got fetch(%d,%d)", wantFirst, wantLast)
			}
			return []int{1, 2, 3}, nil
		})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 2 || items[0] != 1 || items[1] != 2 {
		t.Fatalf("want [1 2] after trailing trim, got %v", items)
	}
	if !hasNext {
		t.Fatal("want hasNext=true (extra row trimmed)")
	}
	if !hasPrev {
		t.Fatal("want hasPrev=true (hasAfter passed true)")
	}
}

func TestAssemblePage_ForwardNoExtraRowNoNext(t *testing.T) {
	t.Parallel()
	items, hasNext, hasPrev, err := assemblePage(5, 0, false, false,
		func(wantFirst, wantLast int) ([]int, error) {
			return []int{1, 2}, nil // fewer than first → no extra row
		})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("want 2 items, got %d", len(items))
	}
	if hasNext {
		t.Fatal("want hasNext=false when no extra row")
	}
	if hasPrev {
		t.Fatal("want hasPrev=false when hasAfter=false")
	}
}

func TestAssemblePage_BackwardTrimsLeadingAndSetsHasPrev(t *testing.T) {
	t.Parallel()
	// last=2 → fetch asked for 3. Repo returns 3 (already reversed) → trim leading.
	items, hasNext, hasPrev, err := assemblePage(0, 2, false, true,
		func(wantFirst, wantLast int) ([]int, error) {
			if wantFirst != 0 || wantLast != 3 {
				t.Fatalf("want fetch(0,3) for last=2, got fetch(%d,%d)", wantFirst, wantLast)
			}
			return []int{1, 2, 3}, nil
		})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 2 || items[0] != 2 || items[1] != 3 {
		t.Fatalf("want [2 3] after leading trim, got %v", items)
	}
	if !hasPrev {
		t.Fatal("want hasPrev=true (extra leading row trimmed)")
	}
	if !hasNext {
		t.Fatal("want hasNext=true (hasBefore passed true)")
	}
}

func TestAssemblePage_TotalCountOnlyRequestNoTrim(t *testing.T) {
	t.Parallel()
	// first=0 && last=0 → fetch called with (0,0); switch matches neither arm.
	called := false
	items, hasNext, hasPrev, err := assemblePage(0, 0, false, false,
		func(wantFirst, wantLast int) ([]int, error) {
			called = true
			if wantFirst != 0 || wantLast != 0 {
				t.Fatalf("want fetch(0,0), got fetch(%d,%d)", wantFirst, wantLast)
			}
			return []int{}, nil
		})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Fatal("want fetch invoked even for total-only request")
	}
	if len(items) != 0 || hasNext || hasPrev {
		t.Fatalf("want empty/no-flags, got items=%v hasNext=%v hasPrev=%v", items, hasNext, hasPrev)
	}
}

func TestAssemblePage_FetchErrorPropagates(t *testing.T) {
	t.Parallel()
	sentinel := errors.New("boom")
	items, hasNext, hasPrev, err := assemblePage(2, 0, false, false,
		func(wantFirst, wantLast int) ([]int, error) {
			return nil, sentinel
		})
	if !errors.Is(err, sentinel) {
		t.Fatalf("want sentinel error propagated, got %v", err)
	}
	if items != nil || hasNext || hasPrev {
		t.Fatalf("want nil/no-flags on error, got items=%v hasNext=%v hasPrev=%v", items, hasNext, hasPrev)
	}
}

func TestResolveRelayPage_ValidArgsCallsClamp(t *testing.T) {
	t.Parallel()
	clampCalled := false
	first := 7
	pageFirst, pageLast, err := resolveRelayPage(&first, nil, nil, nil,
		func(f, l *int) (int, int, error) {
			clampCalled = true
			return 7, 0, nil
		})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !clampCalled {
		t.Fatal("want clamp invoked for valid args")
	}
	if pageFirst != 7 || pageLast != 0 {
		t.Fatalf("want (7,0) from clamp, got (%d,%d)", pageFirst, pageLast)
	}
}

func TestResolveRelayPage_InvalidComboSkipsClamp(t *testing.T) {
	t.Parallel()
	// after + last is a mixed-direction combo → validateRelayArgs rejects it
	// BEFORE the clamp runs (preserving the validate-before-cursor ordering).
	last := 5
	after := "cursor"
	clampCalled := false
	_, _, err := resolveRelayPage(nil, &last, &after, nil,
		func(f, l *int) (int, int, error) {
			clampCalled = true
			return 0, 0, nil
		})
	var ve *ucerr.ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("want ValidationError for after+last, got %v", err)
	}
	if clampCalled {
		t.Fatal("want clamp NOT called when validation fails")
	}
}

// TestDecodeCursorOrBadInput covers the shared decode+guard prefix extracted
// from the five resolve*Cursor methods: a nil/empty cursor yields present=false
// with no error, a malformed envelope is a field-level BAD_USER_INPUT
// validation error, a well-formed v1 cursor decodes to the raw id with no
// ordering metadata, and a v2 cursor additionally carries the ordering it was
// served under plus the captured ordering-key value.
func TestDecodeCursorOrBadInput(t *testing.T) {
	t.Parallel()

	empty := ""
	valid := cursor.Encode("abc123")
	ordered := cursor.EncodeV2(cursor.Payload{ID: "abc123", OrderBy: "updated_at", Direction: "DESC", OrderKey: "2026-07-20T00:00:00Z"})
	// A v1 envelope with a base64 payload that cannot be decoded.
	bad := "v1:!!!not-base64!!!"
	badV2 := "v2:!!!not-base64!!!"

	tests := []struct {
		name        string
		cursorStr   *string
		want        cursor.Payload
		wantPresent bool
		wantErr     bool
	}{
		{name: "nil -> not present, no error", cursorStr: nil, wantPresent: false},
		{name: "empty -> not present, no error", cursorStr: &empty, wantPresent: false},
		{name: "v1 -> decoded id, no ordering", cursorStr: &valid, want: cursor.Payload{ID: "abc123"}, wantPresent: true},
		{
			name:        "v2 -> decoded id plus ordering",
			cursorStr:   &ordered,
			want:        cursor.Payload{ID: "abc123", HasOrdering: true, OrderBy: "updated_at", Direction: "DESC", OrderKey: "2026-07-20T00:00:00Z"},
			wantPresent: true,
		},
		{name: "malformed v1 -> validation error", cursorStr: &bad, wantErr: true},
		{name: "malformed v2 -> validation error", cursorStr: &badV2, wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, present, err := decodeCursorOrBadInput(tc.cursorStr, "after")
			if tc.wantErr {
				var ve *ucerr.ValidationError
				if !errors.As(err, &ve) {
					t.Fatalf("want ValidationError, got %v", err)
				}
				if ve.Field != "after" {
					t.Fatalf("want field after, got %q", ve.Field)
				}
				if present || got != (cursor.Payload{}) {
					t.Fatalf("want zero/not-present on error, got payload=%+v present=%v", got, present)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if present != tc.wantPresent {
				t.Fatalf("present: got %v, want %v", present, tc.wantPresent)
			}
			if got != tc.want {
				t.Fatalf("payload: got %+v, want %+v", got, tc.want)
			}
		})
	}
}

// TestRequireCursorOrdering covers the shared v2 ordering guard: a cursor that
// carries no ordering (v1 / legacy bare id) always passes so it can fall back
// to the re-hydration path, a v2 cursor whose ordering matches the request
// passes, and a v2 cursor taken under a different column or direction is a
// field-level BAD_USER_INPUT rather than a silent mis-page.
func TestRequireCursorOrdering(t *testing.T) {
	t.Parallel()

	req := PageOrdering{OrderBy: "updated_at", Direction: "DESC"}

	tests := []struct {
		name    string
		payload cursor.Payload
		wantErr bool
	}{
		{name: "no ordering passes through", payload: cursor.Payload{ID: "a"}},
		{
			name:    "matching ordering accepted",
			payload: cursor.Payload{ID: "a", HasOrdering: true, OrderBy: "updated_at", Direction: "DESC"},
		},
		{
			name:    "different column rejected",
			payload: cursor.Payload{ID: "a", HasOrdering: true, OrderBy: "name", Direction: "DESC"},
			wantErr: true,
		},
		{
			name:    "different direction rejected",
			payload: cursor.Payload{ID: "a", HasOrdering: true, OrderBy: "updated_at", Direction: "ASC"},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := requireCursorOrdering(tc.payload, req, "after")
			if !tc.wantErr {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			var ve *ucerr.ValidationError
			if !errors.As(err, &ve) {
				t.Fatalf("want ValidationError, got %v", err)
			}
			if ve.Field != "after" {
				t.Fatalf("want field after, got %q", ve.Field)
			}
		})
	}
}

// TestOrderKeyCodecs covers the ordering-key serializers shared by the
// mutable-key aggregates: a timestamp round-trips at the microsecond
// resolution Postgres stores, and both the timestamp and integer parsers
// classify an unparseable client-supplied value as errCursorKeyMalformed so
// the caller maps it to BAD_USER_INPUT rather than INTERNAL.
func TestOrderKeyCodecs(t *testing.T) {
	t.Parallel()

	want := time.Date(2026, 7, 20, 4, 5, 6, 789012000, time.UTC)
	got, err := decodeTimeOrderKey(encodeTimeOrderKey(want))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !got.Equal(want) {
		t.Fatalf("timestamp round-trip mismatch: got %v, want %v", got, want)
	}

	if _, err := decodeTimeOrderKey("not-a-timestamp"); !errors.Is(err, errCursorKeyMalformed) {
		t.Fatalf("want errCursorKeyMalformed, got %v", err)
	}

	n, err := decodeIntOrderKey("-42")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != -42 {
		t.Fatalf("int round-trip mismatch: got %d, want -42", n)
	}
	if _, err := decodeIntOrderKey("not-an-int"); !errors.Is(err, errCursorKeyMalformed) {
		t.Fatalf("want errCursorKeyMalformed, got %v", err)
	}
}

// TestResolveOrderByColumn covers the generic table-driven orderBy resolver each
// aggregate's resolve*OrderBy now wraps: nil orderBy yields the default column,
// every mapped enum yields its repository column, an unmapped enum is a
// BAD_USER_INPUT validation error, and the direction half is delegated to
// resolveSortDir (an invalid direction surfaces its orderDirection error).
func TestResolveOrderByColumn(t *testing.T) {
	t.Parallel()

	allow := map[CardOrderBy]repository.CardOrderBy{
		CardOrderByID:        repository.CardOrderByID,
		CardOrderByCreatedAt: repository.CardOrderByCreatedAt,
		CardOrderByUpdatedAt: repository.CardOrderByUpdatedAt,
		CardOrderByDue:       repository.CardOrderByDue,
	}

	createdAt := CardOrderByCreatedAt
	due := CardOrderByDue
	bogus := CardOrderBy("BOGUS")
	descDir := SortOrderDesc
	bogusDir := SortOrder("SIDEWAYS")

	t.Run("nil orderBy -> default column and direction", func(t *testing.T) {
		t.Parallel()
		field, dir, err := resolveOrderByColumn(nil, nil, allow, repository.CardOrderByID, repository.SortAsc)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if field != repository.CardOrderByID {
			t.Fatalf("field: got %q, want %q", field, repository.CardOrderByID)
		}
		if dir != repository.SortAsc {
			t.Fatalf("dir: got %q, want %q", dir, repository.SortAsc)
		}
	})

	t.Run("mapped enum -> repository column", func(t *testing.T) {
		t.Parallel()
		field, _, err := resolveOrderByColumn(&createdAt, nil, allow, repository.CardOrderByID, repository.SortAsc)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if field != repository.CardOrderByCreatedAt {
			t.Fatalf("field: got %q, want %q", field, repository.CardOrderByCreatedAt)
		}
	})

	t.Run("mapped enum + explicit direction", func(t *testing.T) {
		t.Parallel()
		field, dir, err := resolveOrderByColumn(&due, &descDir, allow, repository.CardOrderByID, repository.SortAsc)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if field != repository.CardOrderByDue {
			t.Fatalf("field: got %q, want %q", field, repository.CardOrderByDue)
		}
		if dir != repository.SortDesc {
			t.Fatalf("dir: got %q, want %q", dir, repository.SortDesc)
		}
	})

	t.Run("unmapped enum -> orderBy validation error", func(t *testing.T) {
		t.Parallel()
		field, dir, err := resolveOrderByColumn(&bogus, nil, allow, repository.CardOrderByID, repository.SortAsc)
		var ve *ucerr.ValidationError
		if !errors.As(err, &ve) {
			t.Fatalf("want ValidationError, got %v", err)
		}
		if ve.Field != "orderBy" {
			t.Fatalf("want field orderBy, got %q", ve.Field)
		}
		if field != "" || dir != "" {
			t.Fatalf("want empty field/dir on error, got field=%q dir=%q", field, dir)
		}
	})

	t.Run("invalid direction -> orderDirection validation error", func(t *testing.T) {
		t.Parallel()
		field, dir, err := resolveOrderByColumn(&createdAt, &bogusDir, allow, repository.CardOrderByID, repository.SortAsc)
		var ve *ucerr.ValidationError
		if !errors.As(err, &ve) {
			t.Fatalf("want ValidationError, got %v", err)
		}
		if ve.Field != "orderDirection" {
			t.Fatalf("want field orderDirection, got %q", ve.Field)
		}
		if field != "" || dir != "" {
			t.Fatalf("want empty field/dir on error, got field=%q dir=%q", field, dir)
		}
	})
}

func TestFirstLastCursor(t *testing.T) {
	t.Parallel()

	id := func(s string) string { return s }

	tests := []struct {
		name      string
		rows      []string
		wantStart string
		wantEnd   string
	}{
		{name: "empty -> empty, empty", rows: []string{}, wantStart: "", wantEnd: ""},
		{name: "nil -> empty, empty", rows: nil, wantStart: "", wantEnd: ""},
		{name: "single -> start == end", rows: []string{"only"}, wantStart: "only", wantEnd: "only"},
		{name: "multi -> first and last", rows: []string{"a", "b", "c"}, wantStart: "a", wantEnd: "c"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			start, end := firstLastCursor(tc.rows, id)
			if start != tc.wantStart {
				t.Fatalf("start: got %q, want %q", start, tc.wantStart)
			}
			if end != tc.wantEnd {
				t.Fatalf("end: got %q, want %q", end, tc.wantEnd)
			}
		})
	}
}

// TestFirstLastCursor_IDAccessorAppliesCast confirms the id accessor is applied
// to extract the raw string from a typed newtype element (mirroring the
// string(cg.ID) / string(u.ID) call shape at the cardgroup and admin-user sites).
func TestFirstLastCursor_IDAccessorAppliesCast(t *testing.T) {
	t.Parallel()

	type idNewtype string
	rows := []idNewtype{"first", "mid", "last"}
	start, end := firstLastCursor(rows, func(v idNewtype) string { return string(v) })
	if start != "first" || end != "last" {
		t.Fatalf("got (%q, %q), want (first, last)", start, end)
	}
}

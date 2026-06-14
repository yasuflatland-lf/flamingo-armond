package usecase

import (
	"errors"
	"reflect"
	"testing"

	"backend/internal/usecase/ucerr"
)

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

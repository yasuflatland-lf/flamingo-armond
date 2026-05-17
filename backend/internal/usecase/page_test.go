package usecase

import (
	"reflect"
	"testing"
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

package domain

import (
	"errors"
	"strings"
	"testing"
)

func TestParseBoundedText(t *testing.T) {
	const zwjEmoji = "👨‍👩‍👧‍👦"
	const max = 5

	tests := []struct {
		name    string
		input   string
		max     int
		wantVal BoundedText
		wantErr error
	}{
		{name: "happy ascii", input: "hello", max: max, wantVal: BoundedText("hello")},
		{name: "trims surrounding whitespace", input: "  hi  ", max: max, wantVal: BoundedText("hi")},
		{name: "empty is allowed (no required bound)", input: "   ", max: max, wantVal: BoundedText("")},
		{name: "exactly max graphemes", input: strings.Repeat(zwjEmoji, max), max: max, wantVal: BoundedText(strings.Repeat(zwjEmoji, max))},
		{name: "max+1 ascii", input: strings.Repeat("a", max+1), max: max, wantErr: ErrTextTooLong},
		{name: "max+1 zwj emojis", input: strings.Repeat(zwjEmoji, max+1), max: max, wantErr: ErrTextTooLong},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseBoundedText(tc.input, tc.max)
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("err = %v, want %v", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected err: %v", err)
			}
			if got != tc.wantVal {
				t.Fatalf("got %q, want %q", got, tc.wantVal)
			}
		})
	}
}

func TestParseBoundedText_PanicsOnNonPositiveMax(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic on max < 1")
		}
	}()
	_, _ = ParseBoundedText("x", 0)
}

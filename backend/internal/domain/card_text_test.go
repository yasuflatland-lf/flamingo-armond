package domain

import (
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseCardText(t *testing.T) {
	t.Parallel()

	errReq := errors.New("required-sentinel")
	errLong := errors.New("toolong-sentinel")

	const zwjEmoji = "👨‍👩‍👧‍👦"

	cases := []struct {
		name    string
		input   string
		wantVal CardText
		wantErr error
	}{
		{
			name:    "happy ascii",
			input:   "hello",
			wantVal: CardText("hello"),
		},
		{
			name:    "happy trims whitespace",
			input:   "  hello  ",
			wantVal: CardText("hello"),
		},
		{
			name:    "happy exactly CardTextMax grapheme clusters",
			input:   strings.Repeat(zwjEmoji, CardTextMax),
			wantVal: CardText(strings.Repeat(zwjEmoji, CardTextMax)),
		},
		{
			name:    "empty string returns requiredErr",
			input:   "",
			wantErr: errReq,
		},
		{
			name:    "whitespace only returns requiredErr",
			input:   "   ",
			wantErr: errReq,
		},
		{
			name:    "501 ascii chars returns tooLongErr",
			input:   strings.Repeat("a", CardTextMax+1),
			wantErr: errLong,
		},
		{
			name:    "501 ZWJ emojis returns tooLongErr",
			input:   strings.Repeat(zwjEmoji, CardTextMax+1),
			wantErr: errLong,
		},
		{
			name:    "sentinel propagation required branch",
			input:   "",
			wantErr: errReq,
		},
		{
			name:    "sentinel propagation toolong branch",
			input:   strings.Repeat("b", CardTextMax+1),
			wantErr: errLong,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := ParseCardText(tc.input, errReq, errLong)
			if tc.wantErr != nil {
				require.True(t, errors.Is(err, tc.wantErr), "got %v, want %v", err, tc.wantErr)
				require.Empty(t, got)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.wantVal, got)
		})
	}
}

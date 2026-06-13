package domain

import (
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMasterCardValidate(t *testing.T) {
	t.Parallel()

	const zwjEmoji = "👨‍👩‍👧‍👦"

	cases := []struct {
		name        string
		card        MasterCard
		sentinelErr error
	}{
		{
			name: "valid front and back",
			card: MasterCard{
				MasterCardgroupID: "mcg",
				Front:             CardText("front"),
				Back:              CardText("back"),
			},
		},
		{
			name: "empty front returns ErrCardFrontRequired",
			card: MasterCard{
				MasterCardgroupID: "mcg",
				Front:             CardText(""),
				Back:              CardText("back"),
			},
			sentinelErr: ErrCardFrontRequired,
		},
		{
			name: "whitespace-only front returns ErrCardFrontRequired",
			card: MasterCard{
				MasterCardgroupID: "mcg",
				Front:             CardText("   "),
				Back:              CardText("back"),
			},
			sentinelErr: ErrCardFrontRequired,
		},
		{
			name: "front over CardTextMax returns ErrCardFrontTooLong",
			card: MasterCard{
				MasterCardgroupID: "mcg",
				Front:             CardText(strings.Repeat("a", CardTextMax+1)),
				Back:              CardText("back"),
			},
			sentinelErr: ErrCardFrontTooLong,
		},
		{
			name: "front over CardTextMax with graphemes returns ErrCardFrontTooLong",
			card: MasterCard{
				MasterCardgroupID: "mcg",
				Front:             CardText(strings.Repeat(zwjEmoji, CardTextMax+1)),
				Back:              CardText("back"),
			},
			sentinelErr: ErrCardFrontTooLong,
		},
		{
			name: "empty back returns ErrCardBackRequired",
			card: MasterCard{
				MasterCardgroupID: "mcg",
				Front:             CardText("front"),
				Back:              CardText(""),
			},
			sentinelErr: ErrCardBackRequired,
		},
		{
			name: "back over CardTextMax returns ErrCardBackTooLong",
			card: MasterCard{
				MasterCardgroupID: "mcg",
				Front:             CardText("front"),
				Back:              CardText(strings.Repeat("b", CardTextMax+1)),
			},
			sentinelErr: ErrCardBackTooLong,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := tc.card.Validate()
			if tc.sentinelErr == nil {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			require.True(t, errors.Is(err, tc.sentinelErr), "got %v", err)
		})
	}
}

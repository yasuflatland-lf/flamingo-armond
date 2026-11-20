package domain

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"
)

// cardSideOracle is the aggregate law: the front sentinel wins, then the back
// sentinel, else both sides are the ParseCardText values.
func cardSideOracle(front, back string) (CardText, CardText, error) {
	f, err := ParseCardText(front, ErrCardFrontRequired, ErrCardFrontTooLong)
	if err != nil {
		return "", "", err
	}
	b, err := ParseCardText(back, ErrCardBackRequired, ErrCardBackTooLong)
	if err != nil {
		return "", "", err
	}
	return f, b, nil
}

// TestCardConstructors_Property_MatchParseCardText: NewCard and NewMasterCard
// accept (front, back) iff both sides parse, report the front sentinel first,
// and store the parsed values with CreatedAt == UpdatedAt == now.
func TestCardConstructors_Property_MatchParseCardText(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 4, 26, 0, 0, 0, 0, time.UTC)
	rapid.Check(t, func(t *rapid.T) {
		front := genText(8, CardTextMax).Draw(t, "front")
		back := genText(8, CardTextMax).Draw(t, "back")
		pos := rapid.IntRange(-3, 1000).Draw(t, "pos")
		wf, wb, werr := cardSideOracle(front, back)

		c, err := NewCard("cg", front, back, pos, now)
		mc, merr := NewMasterCard("mcg", front, back, pos, now)
		if werr != nil {
			require.ErrorIs(t, err, werr)
			require.ErrorIs(t, merr, werr)
			require.Nil(t, c)
			require.Nil(t, mc)
			return
		}
		require.NoError(t, err)
		require.NoError(t, merr)
		require.Equal(t, [2]CardText{wf, wb}, [2]CardText{c.Front, c.Back})
		require.Equal(t, [2]CardText{wf, wb}, [2]CardText{mc.Front, mc.Back})
		require.Equal(t, pos, c.Position)
		require.Equal(t, pos, mc.Position)
		require.NotEmpty(t, c.ID)
		require.NotEmpty(t, mc.ID)
		require.Equal(t, [2]time.Time{now, now}, [2]time.Time{c.CreatedAt, c.UpdatedAt})
		require.Equal(t, [2]time.Time{now, now}, [2]time.Time{mc.CreatedAt, mc.UpdatedAt})
	})
}

// TestCardUpdate_Property_TouchesOnlyItsSide: UpdateFront/UpdateBack on Card and
// MasterCard reject exactly the zero CardText with the side's required sentinel,
// leave the receiver unchanged on error, and never modify the other side.
func TestCardUpdate_Property_TouchesOnlyItsSide(t *testing.T) {
	t.Parallel()
	genCardText := rapid.Custom(func(t *rapid.T) CardText {
		if rapid.IntRange(0, 4).Draw(t, "zero") == 0 {
			return ""
		}
		return CardText(genText(6, 0).Filter(func(s string) bool { return s != "" }).Draw(t, "raw"))
	})
	rapid.Check(t, func(t *rapid.T) {
		f0, b0 := CardText("front"), CardText("back")
		v := genCardText.Draw(t, "v")

		for _, side := range []string{"front", "back"} {
			c := Card{ID: "c", CardgroupID: "cg", Front: f0, Back: b0}
			m := MasterCard{ID: "m", MasterCardgroupID: "mcg", Front: f0, Back: b0}
			var cerr, merr error
			wantSentinel := ErrCardFrontRequired
			if side == "front" {
				cerr, merr = c.UpdateFront(v), m.UpdateFront(v)
			} else {
				cerr, merr = c.UpdateBack(v), m.UpdateBack(v)
				wantSentinel = ErrCardBackRequired
			}
			wantF, wantB := f0, b0
			if v == "" {
				require.ErrorIs(t, cerr, wantSentinel)
				require.ErrorIs(t, merr, wantSentinel)
			} else {
				require.NoError(t, cerr)
				require.NoError(t, merr)
				if side == "front" {
					wantF = v
				} else {
					wantB = v
				}
			}
			require.Equal(t, [2]CardText{wantF, wantB}, [2]CardText{c.Front, c.Back}, side)
			require.Equal(t, [2]CardText{wantF, wantB}, [2]CardText{m.Front, m.Back}, side)
		}
	})
}

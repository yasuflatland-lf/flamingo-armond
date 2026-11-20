package domain

import (
	"errors"
	"strings"
	"testing"

	"github.com/rivo/uniseg"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"
)

// requiredTextVO describes one trimmed, grapheme-bounded, required text VO:
// parse returns the stored string or a sentinel.
type requiredTextVO struct {
	name              string
	max               int
	required, tooLong error
	parse             func(string) (string, error)
	extra             func(trimmed string) error // additional rule after the length checks, nil if none
}

var (
	errCardTextRequired = errors.New("card text required")
	errCardTextTooLong  = errors.New("card text too long")
)

var requiredTextVOs = []requiredTextVO{
	{
		name: "CardText", max: CardTextMax, required: errCardTextRequired, tooLong: errCardTextTooLong,
		parse: func(s string) (string, error) {
			v, err := ParseCardText(s, errCardTextRequired, errCardTextTooLong)
			return v.String(), err
		},
	},
	{
		name: "CardgroupName", max: CardgroupNameMax, required: ErrCardgroupNameRequired, tooLong: ErrCardgroupNameTooLong,
		parse: func(s string) (string, error) { v, err := ParseCardgroupName(s); return v.String(), err },
	},
	{
		name: "DisplayName", max: DisplayNameMax, required: ErrDisplayNameRequired, tooLong: ErrDisplayNameTooLong,
		parse: func(s string) (string, error) { v, err := ParseDisplayName(s); return string(v), err },
		extra: func(trimmed string) error {
			if _, ok := reservedDisplayNames[strings.ToLower(trimmed)]; ok {
				return ErrDisplayNameReserved
			}
			return nil
		},
	},
}

// TestRequiredTextVO_Property_TrimAndGraphemeCap: with trimmed = TrimSpace(s) and
// n = graphemes(trimmed), parse fails with required iff n == 0, with tooLong iff
// n > max, otherwise (subject to extra) stores exactly trimmed; the stored value
// is a fixed point of parse.
func TestRequiredTextVO_Property_TrimAndGraphemeCap(t *testing.T) {
	t.Parallel()
	for _, vo := range requiredTextVOs {
		t.Run(vo.name, func(t *testing.T) {
			t.Parallel()
			rapid.Check(t, func(t *rapid.T) {
				s := genText(12, vo.max).Draw(t, "s")
				trimmed := strings.TrimSpace(s)
				n := uniseg.GraphemeClusterCount(trimmed)
				var want error
				switch {
				case n == 0:
					want = vo.required
				case n > vo.max:
					want = vo.tooLong
				case vo.extra != nil:
					want = vo.extra(trimmed)
				}
				got, err := vo.parse(s)
				if want != nil {
					require.ErrorIs(t, err, want, "input %q", s)
					require.Empty(t, got)
					return
				}
				require.NoError(t, err, "input %q", s)
				require.Equal(t, trimmed, got)
				again, err := vo.parse(got)
				require.NoError(t, err)
				require.Equal(t, got, again)
			})
		})
	}
}

// TestTrinaryText_Property_BioAndDescription: nil stays unset (Ptr is nil); a non-nil input
// stores TrimSpace(s) (whitespace-only becomes the explicit empty value) unless
// it exceeds max graphemes; Ptr is always a fresh copy; FromPtr keeps the value
// verbatim.
func TestTrinaryText_Property_BioAndDescription(t *testing.T) {
	t.Parallel()
	type trinaryVO struct {
		name    string
		max     int
		tooLong error
		parse   func(*string) (trinaryText, error)
		fromPtr func(*string) trinaryText
	}
	vos := []trinaryVO{
		{"Bio", BioMax, ErrBioTooLong,
			func(p *string) (trinaryText, error) { v, err := ParseBio(p); return v.trinaryText, err },
			func(p *string) trinaryText { return BioFromPtr(p).trinaryText }},
		{"Description", DescriptionMax, ErrDescriptionTooLong,
			func(p *string) (trinaryText, error) { v, err := ParseDescription(p); return v.trinaryText, err },
			func(p *string) trinaryText { return DescriptionFromPtr(p).trinaryText }},
	}
	for _, vo := range vos {
		t.Run(vo.name, func(t *testing.T) {
			t.Parallel()
			rapid.Check(t, func(t *rapid.T) {
				var in *string
				if rapid.Bool().Draw(t, "set") {
					s := genText(12, vo.max).Draw(t, "s")
					in = &s
				}
				got, err := vo.parse(in)
				if in == nil {
					require.NoError(t, err)
					require.Nil(t, got.Ptr())
					require.Nil(t, vo.fromPtr(nil).Ptr())
					return
				}
				trimmed := strings.TrimSpace(*in)
				if uniseg.GraphemeClusterCount(trimmed) > vo.max {
					require.ErrorIs(t, err, vo.tooLong, "input %q", *in)
					return
				}
				require.NoError(t, err)
				p1, p2 := got.Ptr(), got.Ptr()
				require.NotNil(t, p1)
				require.NotSame(t, p1, p2)
				require.Equal(t, trimmed, *p1)

				raw := *in
				fp := vo.fromPtr(&raw)
				require.NotNil(t, fp.Ptr())
				require.Equal(t, *in, *fp.Ptr(), "FromPtr must not trim")
				raw = "mutated"
				require.Equal(t, *in, *fp.Ptr(), "FromPtr must copy its input")
			})
		})
	}
}

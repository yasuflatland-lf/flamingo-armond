package cursor_test

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"

	"backend/internal/cursor"
)

// genField draws valid UTF-8 including the JSON and base64 delimiter characters.
var genField = rapid.OneOf(
	rapid.String(),
	rapid.StringOf(rapid.RuneFrom([]rune(`"\:,{}[]/+= `+"\x00\n\u2028\U0001F600"))),
)

// TestEncode_Property_V1RoundTrip: for any byte string id, Decode(Encode(id))
// returns id with HasOrdering=false, and the envelope is "v1:" + unpadded
// URL-safe base64.
func TestEncode_Property_V1RoundTrip(t *testing.T) {
	t.Parallel()
	rapid.Check(t, func(t *rapid.T) {
		id := string(rapid.SliceOf(rapid.Byte()).Draw(t, "id"))
		enc := cursor.Encode(id)
		require.True(t, strings.HasPrefix(enc, "v1:"))
		require.NotContains(t, enc[3:], "+")
		require.NotContains(t, enc[3:], "/")
		require.NotContains(t, enc[3:], "=")
		got, err := cursor.Decode(enc)
		require.NoError(t, err)
		require.Equal(t, cursor.Payload{ID: id}, got)
	})
}

// TestEncodeV2_Property_RoundTrip: for any valid-UTF-8 payload, Decode(EncodeV2(p))
// returns p with HasOrdering=true regardless of the input flag.
func TestEncodeV2_Property_RoundTrip(t *testing.T) {
	t.Parallel()
	rapid.Check(t, func(t *rapid.T) {
		p := cursor.Payload{
			ID: genField.Draw(t, "id"), OrderBy: genField.Draw(t, "orderBy"),
			Direction: genField.Draw(t, "direction"), OrderKey: genField.Draw(t, "orderKey"),
			HasOrdering: rapid.Bool().Draw(t, "hasOrdering"),
		}
		enc := cursor.EncodeV2(p)
		require.True(t, strings.HasPrefix(enc, "v2:"))
		require.False(t, strings.ContainsAny(enc[3:], "+/="))
		got, err := cursor.Decode(enc)
		require.NoError(t, err)
		p.HasOrdering = true
		require.Equal(t, p, got)
	})
}

// TestDecode_Property_V2RejectsTrailingData: a valid v2 body followed by any
// suffix that is not pure JSON whitespace is rejected.
func TestDecode_Property_V2RejectsTrailingData(t *testing.T) {
	t.Parallel()
	rapid.Check(t, func(t *rapid.T) {
		body, err := json.Marshal(map[string]string{
			"i": genField.Draw(t, "i"), "o": genField.Draw(t, "o"), "d": genField.Draw(t, "d"), "k": genField.Draw(t, "k"),
		})
		require.NoError(t, err)
		suffix := rapid.StringOf(rapid.RuneFrom([]rune(" \t\r\n}]{[x\"0,:"))).Draw(t, "suffix")
		_, err = cursor.Decode("v2:" + base64.RawURLEncoding.EncodeToString(append(body, suffix...)))
		if strings.Trim(suffix, " \t\r\n") == "" {
			require.NoError(t, err, "whitespace suffix %q", suffix)
		} else {
			require.Error(t, err, "suffix %q must be rejected", suffix)
		}
	})
}

// FuzzDecode: Decode never panics, a bare (unprefixed) input passes through as
// the id, and an accepted v2 cursor re-encodes to a cursor that decodes to the
// same payload.
func FuzzDecode(f *testing.F) {
	for _, s := range []string{"", "v1:", "v2:", "v1:c2hvcnQ", cursor.EncodeV2(cursor.Payload{ID: "a", OrderBy: "name", Direction: "ASC", OrderKey: "k"}), "f47ac10b-58cc-4372-a567-0e02b2c3d479"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		p, err := cursor.Decode(s)
		if err != nil {
			return
		}
		switch {
		case strings.HasPrefix(s, "v2:"):
			require.True(t, p.HasOrdering)
			if utf8.ValidString(p.ID + p.OrderBy + p.Direction + p.OrderKey) {
				again, err := cursor.Decode(cursor.EncodeV2(p))
				require.NoError(t, err)
				require.Equal(t, p, again)
			}
		case strings.HasPrefix(s, "v1:"):
			require.False(t, p.HasOrdering)
		default:
			require.Equal(t, cursor.Payload{ID: s}, p)
		}
	})
}

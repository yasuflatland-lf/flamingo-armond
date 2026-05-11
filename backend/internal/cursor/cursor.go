// Package cursor provides helpers for encoding and decoding Relay-style
// Connection cursors. Cursors are opaque to clients: the v1 envelope wraps
// the raw entity UUID in base64 so the server can change its internal ID
// scheme without breaking existing clients.
package cursor

import (
	"encoding/base64"
	"strings"

	"github.com/rotisserie/eris"
)

const v1Prefix = "v1:"

// Encode wraps a raw entity ID in the v1 opaque cursor envelope.
// The encoded form is "v1:" + RawURLBase64(id) — padding-free and URL-safe.
// Clients must treat the result as an opaque string and pass it back unchanged
// as an after/before pagination argument.
func Encode(id string) string {
	return v1Prefix + base64.RawURLEncoding.EncodeToString([]byte(id))
}

// Decode accepts either the v1 envelope ("v1:" + base64) or a bare entity ID
// (legacy backward-compat path). It returns the raw entity ID extracted from
// the cursor, or an error if the v1 envelope is present but the base64
// payload cannot be decoded.
//
// The bare-ID branch does not validate UUID shape — that is the caller's
// responsibility. The v1 branch returns a hard error on a malformed base64
// payload; callers should map this to BAD_USER_INPUT.
func Decode(c string) (string, error) {
	if strings.HasPrefix(c, v1Prefix) {
		payload := c[len(v1Prefix):]
		decoded, err := base64.RawURLEncoding.DecodeString(payload)
		if err != nil {
			return "", eris.Wrap(err, "cursor: invalid v1 base64 payload")
		}
		return string(decoded), nil
	}
	// Legacy bare-ID path: pass through unchanged.
	return c, nil
}

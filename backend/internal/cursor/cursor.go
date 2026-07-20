// Package cursor provides helpers for encoding and decoding Relay-style
// Connection cursors. Cursors are opaque to clients: the envelope wraps the
// server-side payload in base64 so the server can change its internal ID
// scheme without breaking existing clients.
//
// Two envelopes exist. The v1 envelope carries only the raw entity UUID; the
// serving code must therefore re-read the row at page time to recover the
// ordering-key value, which moves the bookmark when the row is edited between
// two page fetches. The v2 envelope additionally carries the ordering-key
// value captured when the page was served, together with the ordering it was
// served under, so an edit to the row cannot move the bookmark. Connections
// whose ordering key is mutable emit v2; v1 and legacy bare ids stay decodable
// so cursors persisted by older clients keep working.
package cursor

import (
	"encoding/base64"
	"encoding/json"
	"strings"

	"github.com/rotisserie/eris"
)

const (
	v1Prefix = "v1:"
	v2Prefix = "v2:"
)

// Payload is the decoded content of a cursor string, and the input to EncodeV2.
//
// ID is always populated. HasOrdering reports whether the cursor carried
// ordering metadata: it is true only for a v2 envelope, and OrderBy, Direction
// and OrderKey are meaningful only then. v1 envelopes and legacy bare ids
// decode with HasOrdering false and the remaining fields empty, which is the
// signal for the caller to fall back to re-reading the ordering column off the
// current row.
//
// OrderBy and Direction are server-internal tokens (the repository column name
// and sort direction). They are never interpreted by clients; the serving code
// compares them verbatim against the ordering the current request resolved to
// and rejects a mismatch. OrderKey is the serialized ordering-key value of the
// row the cursor points at — empty when the ordering key IS the id, since the
// id is already carried.
type Payload struct {
	ID          string
	HasOrdering bool
	OrderBy     string
	Direction   string
	OrderKey    string
}

// v2Body is the JSON body wrapped by the v2 envelope. The keys are terse
// because the marshalled form is base64-wrapped into every edge cursor of
// every page.
type v2Body struct {
	ID        string `json:"i"`
	OrderBy   string `json:"o"`
	Direction string `json:"d"`
	OrderKey  string `json:"k"`
}

// Encode wraps a raw entity ID in the v1 opaque cursor envelope.
// The encoded form is "v1:" + RawURLBase64(id) — padding-free and URL-safe.
// Clients must treat the result as an opaque string and pass it back unchanged
// as an after/before pagination argument.
//
// Use Encode for connections whose ordering key is immutable (the admin-users
// listing orders by created_at, for example). Connections that order by a
// mutable column must use EncodeV2 instead, or an edit to the boundary row
// duplicates or skips rows on the next page fetch.
func Encode(id string) string {
	return v1Prefix + base64.RawURLEncoding.EncodeToString([]byte(id))
}

// EncodeV2 wraps a raw entity ID plus the ordering context the page was served
// under in the v2 opaque cursor envelope: "v2:" + RawURLBase64(JSON body).
// The payload's HasOrdering field is ignored — EncodeV2 always emits a v2
// envelope, so Decode always reports HasOrdering true for its output.
func EncodeV2(p Payload) string {
	body, err := json.Marshal(v2Body{
		ID:        p.ID,
		OrderBy:   p.OrderBy,
		Direction: p.Direction,
		OrderKey:  p.OrderKey,
	})
	if err != nil {
		// Unreachable: v2Body is a flat struct of strings, which encoding/json
		// always marshals. Degrade to the v1 envelope rather than serving a
		// corrupt cursor — the page still works, it is merely edit-sensitive.
		return Encode(p.ID)
	}
	return v2Prefix + base64.RawURLEncoding.EncodeToString(body)
}

// Decode accepts a v2 envelope, a v1 envelope, or a bare entity ID (legacy
// backward-compat path), and returns the decoded Payload.
//
// The bare-ID branch does not validate UUID shape — that is the caller's
// responsibility. The v1 and v2 branches return a hard error on a malformed
// base64 payload (and, for v2, on a body that is not the expected JSON);
// callers should map this to BAD_USER_INPUT.
func Decode(c string) (Payload, error) {
	switch {
	case strings.HasPrefix(c, v2Prefix):
		raw, err := base64.RawURLEncoding.DecodeString(c[len(v2Prefix):])
		if err != nil {
			return Payload{}, eris.Wrap(err, "cursor: invalid v2 base64 payload")
		}
		var body v2Body
		if err := json.Unmarshal(raw, &body); err != nil {
			return Payload{}, eris.Wrap(err, "cursor: invalid v2 json payload")
		}
		return Payload{
			ID:          body.ID,
			HasOrdering: true,
			OrderBy:     body.OrderBy,
			Direction:   body.Direction,
			OrderKey:    body.OrderKey,
		}, nil
	case strings.HasPrefix(c, v1Prefix):
		decoded, err := base64.RawURLEncoding.DecodeString(c[len(v1Prefix):])
		if err != nil {
			return Payload{}, eris.Wrap(err, "cursor: invalid v1 base64 payload")
		}
		return Payload{ID: string(decoded)}, nil
	default:
		// Legacy bare-ID path: pass through unchanged.
		return Payload{ID: c}, nil
	}
}

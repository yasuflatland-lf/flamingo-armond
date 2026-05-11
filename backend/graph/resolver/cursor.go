package resolver

import "backend/internal/cursor"

// EncodeCursor wraps a raw entity ID in the v1 opaque cursor envelope.
// See backend/internal/cursor for the encoding contract.
func EncodeCursor(id string) string { return cursor.Encode(id) }

// DecodeCursor accepts either the v1 envelope ("v1:" + base64) or a bare
// entity ID (legacy backward-compat). See backend/internal/cursor for the
// decoding contract.
func DecodeCursor(c string) (string, error) { return cursor.Decode(c) }

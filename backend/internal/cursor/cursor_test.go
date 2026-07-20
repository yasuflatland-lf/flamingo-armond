package cursor_test

import (
	"encoding/base64"
	"strings"
	"testing"

	"backend/internal/cursor"
)

// TestEncode_RoundTrip verifies that Decode(Encode(id)) == id for typical UUIDs.
func TestEncode_RoundTrip(t *testing.T) {
	t.Parallel()

	ids := []string{
		"01JABE00000000000000000001",
		"00000000-0000-0000-0000-000000000000",
		"f47ac10b-58cc-4372-a567-0e02b2c3d479",
		"short",
		"",
	}
	for _, id := range ids {
		t.Run(id, func(t *testing.T) {
			t.Parallel()

			encoded := cursor.Encode(id)
			got, err := cursor.Decode(encoded)
			if err != nil {
				t.Fatalf("Decode(Encode(%q)) returned unexpected error: %v", id, err)
			}
			if got.ID != id {
				t.Fatalf("round-trip mismatch: Decode(Encode(%q)).ID = %q", id, got.ID)
			}
			if got.HasOrdering {
				t.Fatalf("v1 cursor must decode with HasOrdering=false, got %+v", got)
			}
		})
	}
}

// TestEncode_V1Prefix verifies that encoded cursors start with "v1:".
func TestEncode_V1Prefix(t *testing.T) {
	t.Parallel()

	encoded := cursor.Encode("some-uuid")
	if !strings.HasPrefix(encoded, "v1:") {
		t.Fatalf("expected v1: prefix, got %q", encoded)
	}
}

// TestEncode_RawURLBase64 verifies the payload is RawURLEncoding (no padding,
// no standard-base64 characters like '+' or '/').
func TestEncode_RawURLBase64(t *testing.T) {
	t.Parallel()

	id := "f47ac10b-58cc-4372-a567-0e02b2c3d479"
	encoded := cursor.Encode(id)
	payload := strings.TrimPrefix(encoded, "v1:")

	// Must round-trip via RawURLEncoding.
	raw, err := base64.RawURLEncoding.DecodeString(payload)
	if err != nil {
		t.Fatalf("payload is not valid RawURLEncoding: %v", err)
	}
	if string(raw) != id {
		t.Fatalf("decoded payload mismatch: got %q, want %q", raw, id)
	}

	// Must not contain standard base64 padding or '+' / '/'.
	if strings.ContainsAny(payload, "+/=") {
		t.Fatalf("payload contains padding or non-URL-safe characters: %q", payload)
	}
}

// TestDecode_BareUUIDPassthrough verifies that a bare UUID (legacy path) is
// returned unchanged without error.
func TestDecode_BareUUIDPassthrough(t *testing.T) {
	t.Parallel()

	bareID := "f47ac10b-58cc-4372-a567-0e02b2c3d479"
	got, err := cursor.Decode(bareID)
	if err != nil {
		t.Fatalf("Decode(bare UUID) returned unexpected error: %v", err)
	}
	if got.ID != bareID {
		t.Fatalf("expected bare ID to pass through unchanged: got %q, want %q", got.ID, bareID)
	}
	if got.HasOrdering {
		t.Fatalf("legacy bare id must decode with HasOrdering=false, got %+v", got)
	}
}

// TestDecode_MalformedV1_ReturnsError verifies that a "v1:" prefix with an
// invalid base64 payload returns a hard error.
func TestDecode_MalformedV1_ReturnsError(t *testing.T) {
	t.Parallel()

	cases := []string{
		"v1:!!!not-base64!!!",
		"v1:====",     // standard padding that RawURL rejects
		"v1:\x00\x01", // non-printable bytes
	}
	for _, c := range cases {
		t.Run(c, func(t *testing.T) {
			t.Parallel()
			_, err := cursor.Decode(c)
			if err == nil {
				t.Fatalf("expected error for malformed v1 cursor %q, got nil", c)
			}
		})
	}
}

// TestDecode_EmptyInput verifies that an empty string is passed through
// unchanged (the bare-ID branch), returning no error. The usecase guards
// nil/empty separately, but an empty string reaching Decode should not panic.
func TestDecode_EmptyInput(t *testing.T) {
	t.Parallel()

	got, err := cursor.Decode("")
	if err != nil {
		t.Fatalf("Decode(\"\") returned unexpected error: %v", err)
	}
	if got.ID != "" {
		t.Fatalf("expected empty string, got %q", got.ID)
	}
}

// TestDecode_V1EmptyPayload verifies that "v1:" with no payload decodes to an
// empty string (the base64 of empty is empty).
func TestDecode_V1EmptyPayload(t *testing.T) {
	t.Parallel()

	got, err := cursor.Decode("v1:")
	if err != nil {
		t.Fatalf("Decode(\"v1:\") returned unexpected error: %v", err)
	}
	if got.ID != "" {
		t.Fatalf("expected empty decoded string, got %q", got.ID)
	}
}

// ---------------------------------------------------------------------------
// v2 envelope
// ---------------------------------------------------------------------------

// TestEncodeV2_RoundTrip verifies that Decode(EncodeV2(p)) returns the same
// id, ordering, and ordering-key value, and reports HasOrdering. The ordering
// key is deliberately awkward (a name containing the JSON and base64 delimiter
// characters) so the envelope is proved delimiter-safe.
func TestEncodeV2_RoundTrip(t *testing.T) {
	t.Parallel()

	cases := []cursor.Payload{
		{ID: "f47ac10b-58cc-4372-a567-0e02b2c3d479", OrderBy: "updated_at", Direction: "DESC", OrderKey: "2026-07-20T04:05:06.789012Z"},
		{ID: "cg-1", OrderBy: "name", Direction: "ASC", OrderKey: `a"b:c,{}/+= deck`},
		{ID: "mcg-1", OrderBy: "sort_order", Direction: "ASC", OrderKey: "-3"},
		{ID: "cg-2", OrderBy: "id", Direction: "ASC", OrderKey: ""},
	}
	for _, want := range cases {
		t.Run(want.ID+"/"+want.OrderBy, func(t *testing.T) {
			t.Parallel()

			encoded := cursor.EncodeV2(want)
			if !strings.HasPrefix(encoded, "v2:") {
				t.Fatalf("expected v2: prefix, got %q", encoded)
			}
			if strings.ContainsAny(strings.TrimPrefix(encoded, "v2:"), "+/=") {
				t.Fatalf("payload must be RawURLEncoding, got %q", encoded)
			}
			got, err := cursor.Decode(encoded)
			if err != nil {
				t.Fatalf("Decode(EncodeV2(%+v)) returned unexpected error: %v", want, err)
			}
			if !got.HasOrdering {
				t.Fatalf("v2 cursor must decode with HasOrdering=true, got %+v", got)
			}
			want.HasOrdering = true
			if got != want {
				t.Fatalf("round-trip mismatch: got %+v, want %+v", got, want)
			}
		})
	}
}

// TestDecode_MalformedV2_ReturnsError verifies that a "v2:" prefix whose
// payload is not valid base64, or whose decoded bytes are not the expected
// JSON body, returns a hard error the usecase maps to BAD_USER_INPUT.
func TestDecode_MalformedV2_ReturnsError(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"invalid base64": "v2:!!!not-base64!!!",
		"base64 padding": "v2:====",
		"not json":       "v2:" + base64.RawURLEncoding.EncodeToString([]byte("not json at all")),
		"json array":     "v2:" + base64.RawURLEncoding.EncodeToString([]byte(`["i","cg-1"]`)),
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if _, err := cursor.Decode(c); err == nil {
				t.Fatalf("expected error for malformed v2 cursor %q, got nil", c)
			}
		})
	}
}

// TestEncodeV2_IgnoresHasOrdering verifies that EncodeV2 always emits a v2
// envelope regardless of the input payload's HasOrdering flag, so a caller
// cannot accidentally downgrade a cursor by leaving the flag false.
func TestEncodeV2_IgnoresHasOrdering(t *testing.T) {
	t.Parallel()

	got, err := cursor.Decode(cursor.EncodeV2(cursor.Payload{ID: "cg-1", OrderBy: "updated_at", Direction: "DESC", OrderKey: "k"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !got.HasOrdering {
		t.Fatalf("expected HasOrdering=true, got %+v", got)
	}
}

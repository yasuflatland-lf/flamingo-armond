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
		id := id
		t.Run(id, func(t *testing.T) {
			t.Parallel()

			encoded := cursor.Encode(id)
			got, err := cursor.Decode(encoded)
			if err != nil {
				t.Fatalf("Decode(Encode(%q)) returned unexpected error: %v", id, err)
			}
			if got != id {
				t.Fatalf("round-trip mismatch: Decode(Encode(%q)) = %q", id, got)
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
	if got != bareID {
		t.Fatalf("expected bare ID to pass through unchanged: got %q, want %q", got, bareID)
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
		c := c
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
	if got != "" {
		t.Fatalf("expected empty string, got %q", got)
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
	if got != "" {
		t.Fatalf("expected empty decoded string, got %q", got)
	}
}

package cursor_test

import (
	"encoding/base64"
	"testing"

	"backend/internal/cursor"
)

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

// TestDecode_StructurallyIncompleteV2_ReturnsError pins the hand-crafted-body
// cases that a plain-string v2Body would have accepted. A missing or null "k"
// used to decode to "" — a legal ordering-key value for the ID orderings — so
// the body was served as a real bookmark whose text boundary was the empty
// string, silently returning the wrong page. Every field is now required to be
// present and non-null, unknown keys are rejected, and a second JSON value
// after the object is rejected.
func TestDecode_StructurallyIncompleteV2_ReturnsError(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"missing order key":      `{"i":"cg-1","o":"name","d":"ASC"}`,
		"null order key":         `{"i":"cg-1","o":"name","d":"ASC","k":null}`,
		"missing id":             `{"o":"name","d":"ASC","k":"Alpha"}`,
		"null id":                `{"i":null,"o":"name","d":"ASC","k":"Alpha"}`,
		"missing order by":       `{"i":"cg-1","d":"ASC","k":"Alpha"}`,
		"null order by":          `{"i":"cg-1","o":null,"d":"ASC","k":"Alpha"}`,
		"missing direction":      `{"i":"cg-1","o":"name","k":"Alpha"}`,
		"null direction":         `{"i":"cg-1","o":"name","d":null,"k":"Alpha"}`,
		"empty object":           `{}`,
		"json null":              `null`,
		"unknown field":          `{"i":"cg-1","o":"name","d":"ASC","k":"Alpha","x":"extra"}`,
		"trailing json value":    `{"i":"cg-1","o":"name","d":"ASC","k":"Alpha"}{"i":"cg-2"}`,
		"trailing close brace":   `{"i":"cg-1","o":"name","d":"ASC","k":"Alpha"}}`,
		"trailing close bracket": `{"i":"cg-1","o":"name","d":"ASC","k":"Alpha"}]garbage`,
		"trailing newline brace": "{\"i\":\"cg-1\",\"o\":\"name\",\"d\":\"ASC\",\"k\":\"Alpha\"}\n}",
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			c := "v2:" + base64.RawURLEncoding.EncodeToString([]byte(body))
			if _, err := cursor.Decode(c); err == nil {
				t.Fatalf("expected error for structurally incomplete v2 body %s, got nil", body)
			}
		})
	}
}

// TestDecode_V2EmptyOrderKeyIsPreserved verifies that an explicitly-present
// empty "k" still decodes — the ID orderings carry no separate ordering key, so
// "" is a legal value and must not be conflated with the absent/null cases
// rejected above.
func TestDecode_V2EmptyOrderKeyIsPreserved(t *testing.T) {
	t.Parallel()

	c := "v2:" + base64.RawURLEncoding.EncodeToString([]byte(`{"i":"cg-1","o":"id","d":"ASC","k":""}`))
	got, err := cursor.Decode(c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !got.HasOrdering {
		t.Fatalf("expected HasOrdering=true, got %+v", got)
	}
	if got.ID != "cg-1" || got.OrderBy != "id" || got.Direction != "ASC" || got.OrderKey != "" {
		t.Fatalf("unexpected payload: %+v", got)
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

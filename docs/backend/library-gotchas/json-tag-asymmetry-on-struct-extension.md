# Extending a JSON-marshaled struct: tag all fields, not just the new ones

> Part of the [Go library gotchas](../../../.claude/rules/go-library-gotchas.md) rules.

When a new field is added to a struct that is already serialized via `c.JSON` (or any `encoding/json.Marshal` call), tagging only the new field while leaving existing fields untagged produces a mixed-case wire shape: the untagged fields render as their Go field names (e.g. `"Line"`, `"Message"`) while the new tagged fields render as lowercase (`"front"`, `"back"`). Consumers that expect all lowercase keys — an external cron, a REST client, a monitoring script — silently receive the wrong shape for the existing fields and may misparse or drop them.

Go's `encoding/json` uses the struct field name verbatim when no `json:` tag is present. Adding tags to some fields but not others is valid Go; the encoder applies the override only to the tagged fields, leaving the rest capitalized. The result is an inconsistent wire shape that is invisible to the GraphQL path (gqlgen has its own marshaler and ignores `json:` tags on resolver types) but breaks REST-over-JSON consumers immediately.

The fix is to tag every field when any field needs a tag, normalizing all keys to lowercase camelCase:

```go
// Before (mixed casing on the wire when this struct is c.JSON'd)
type DictionaryValidationError struct {
    Line    int    // -> "Line"
    Message string // -> "Message"
    Front   string `json:"front,omitempty"` // -> "front"
    Back    string `json:"back,omitempty"`  // -> "back"
}

// After
type DictionaryValidationError struct {
    Line    int    `json:"line"`
    Message string `json:"message"`
    Front   string `json:"front,omitempty"`
    Back    string `json:"back,omitempty"`
}
```

Pair the fix with a wire-shape test whenever the struct flows through `c.JSON`. The test should decode the response body into `map[string]json.RawMessage` and assert key presence and absence — this catches both the tag-renaming and the `omitempty` branch in one pass. See `backend/internal/handler/notionsync/handler_test.go` `TestHandlerSuccess_ParseErrorsJSONShape` for the asserted example: it verifies that `"line"` and `"message"` are present and that `"front"` and `"back"` are absent when the error carries no card text. Without this test, a future edit that drops a tag goes undetected — the GraphQL path is unaffected, so only the REST consumers see the breakage.

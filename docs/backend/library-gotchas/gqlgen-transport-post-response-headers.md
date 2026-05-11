# gqlgen `transport.POST` response headers must be set at construction time

> Part of the [Go library gotchas](../../../.claude/rules/go-library-gotchas.md) rules.

`transport.POST` is a struct, not an interface. Its `ResponseHeaders http.Header` field
overrides the default `Content-Type: application/json` header that gqlgen emits for every
GraphQL POST response. There is no setter method — the field must be populated when the
struct is constructed and passed to `srv.AddTransport`:

```go
srv.AddTransport(transport.POST{
    ResponseHeaders: http.Header{
        "Content-Type": []string{"application/graphql-response+json; charset=utf-8"},
    },
})
```

When `ResponseHeaders` is nil (the zero value), gqlgen falls back to
`Content-Type: application/json`.

## Why the spec media type matters

The [GraphQL over HTTP draft spec](https://graphql.github.io/graphql-over-http/) defines
`application/graphql-response+json` as the content type for responses to well-formed GraphQL
requests. Clients that send `Accept: application/graphql-response+json` use this type to
distinguish a spec-compliant server from a generic JSON API:

- A `200 OK` with `application/graphql-response+json` means the envelope is always present
  and `errors` carries typed extensions (e.g. `extensions.code`).
- A `200 OK` with `application/json` is ambiguous — the client cannot assume the same
  envelope shape.

Setting the override tells conformance-aware clients (and test tooling) that this server
follows the spec.

## Where to set it

Set it once in the server constructor, not per-request. For this project that is
`newGraphQLServer` in `backend/cmd/server/main.go`. Tests that spin up a minimal server
for panic-recovery or content-type checks must mirror the same field so the production
configuration stays authoritative (see `newPanicGraphQLServer` in
`backend/cmd/server/main_test.go`).

## Integration test assertion

Assert the response header in a dedicated test rather than relying on ad-hoc inspection:

```go
ct := res.Header.Get("Content-Type")
if !strings.Contains(ct, "application/graphql-response+json") {
    t.Errorf("Content-Type = %q, want it to contain application/graphql-response+json", ct)
}
```

`strings.Contains` is intentional: the header value may include `; charset=utf-8`, and
a strict equality check would break when charset is added or removed.

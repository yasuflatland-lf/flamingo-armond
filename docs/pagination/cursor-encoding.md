# Cursor encoding

> Part of the [pagination](../../.claude/rules/pagination.md) rules. Cross-referenced by `docs/backend.md` and `docs/frontend.md`.

Cursors are opaque to clients. The schema declares `cursor: ID!` and `pageInfo.startCursor`/`endCursor: ID`. Clients must treat these values as opaque handles and pass them back unchanged as `after`/`before` arguments — do NOT decode, inspect, or construct them.

## v1 envelope format

The server encodes each cursor as:

```
v1:<RawURLBase64(entityUUID)>
```

where `RawURLBase64` is `encoding/base64.RawURLEncoding` — no padding characters (`=`), URL-safe alphabet (`-` and `_` instead of `+` and `/`).

Example: entity UUID `f47ac10b-58cc-4372-a567-0e02b2c3d479` encodes to
`v1:ZjQ3YWMxMGItNThjYy00MzcyLWE1NjctMGUwMmIyYzNkNDc5`.

## Encoding site

The resolver boundary is the single encoding site. The helpers
`toCardConnectionModel` and `toCardgroupConnectionModel` in
`backend/graph/resolver/helpers.go` call `EncodeCursor(id)` for every edge
cursor and for `startCursor`/`endCursor` in `PageInfo`. The usecase layer
operates on raw entity UUIDs internally and is unaware of the encoding.

## Decoding site

The usecase functions `resolveCursor` (`backend/internal/usecase/card.go`) and
`resolveCardgroupCursor` (`backend/internal/usecase/cardgroup.go`) decode an
incoming `after`/`before` argument by calling `cursor.Decode` from
`backend/internal/cursor`. A malformed `v1:` payload (invalid base64) is
surfaced as `BAD_USER_INPUT`, matching existing UUID-parse failures.

## Backward compatibility

`cursor.Decode` accepts both the v1 envelope and a legacy bare UUID:

- Input starts with `"v1:"` — strip the prefix, base64-decode the rest. A
  failed decode is a hard `BAD_USER_INPUT` error.
- Input does not start with `"v1:"` — pass through unchanged as the raw ID.
  UUID shape validation is the caller's responsibility.

This lets the server change internal ID encoding in a future version without
breaking clients that persisted an earlier cursor form.

## Frontend rule

The frontend must never decode, inspect, or construct cursor strings. Cursor
values are read from API responses and passed back verbatim. The audit grep
`grep -rn "atob\|btoa\|startsWith.*v1" frontend/src/` must return no matches
for cursor-related code.

# Cursor encoding

> Part of the [pagination](../../.claude/rules/pagination.md) rules. Cross-referenced by `docs/backend.md` and `frontend/CLAUDE.md`.

Cursors are opaque to clients. The schema declares `cursor: ID!` and `pageInfo.startCursor`/`endCursor: ID`. Clients must treat these values as opaque handles and pass them back unchanged as `after`/`before` arguments — do NOT decode, inspect, or construct them.

Two envelopes exist. Which one a connection emits depends on whether its ordering key can change while a client is paging.

## v1 envelope format

The v1 envelope carries only the entity id:

```
v1:<RawURLBase64(entityUUID)>
```

where `RawURLBase64` is `encoding/base64.RawURLEncoding` — no padding characters (`=`), URL-safe alphabet (`-` and `_` instead of `+` and `/`).

Example: entity UUID `f47ac10b-58cc-4372-a567-0e02b2c3d479` encodes to
`v1:ZjQ3YWMxMGItNThjYy00MzcyLWE1NjctMGUwMmIyYzNkNDc5`.

Because the payload is id-only, the server must re-read the row at serve time to recover the value of its ordering column. That is safe only when the ordering column is immutable — the admin-users listing orders by `created_at` and stays on v1 for that reason.

## v2 envelope format

The v2 envelope additionally carries the ordering the page was served under plus the ordering-key value the row held at that moment:

```
v2:<RawURLBase64(JSON{"i":id,"o":orderBy,"d":direction,"k":orderKey})>
```

- `i` — the raw entity id, as in v1.
- `o` / `d` — server-internal tokens: the repository column name (`updated_at`, `sort_order`, …) and sort direction (`ASC` / `DESC`). Never interpreted by clients.
- `k` — the serialized ordering-key value. Timestamps use RFC3339 with nanosecond precision (`encodeTimeOrderKey`), which round-trips the microsecond resolution Postgres stores; integers use base-10; text columns are carried verbatim. When the ordering key IS the id, `k` is empty because `i` already carries it.

`backend/internal/cursor` implements both: `Encode(id)` emits v1, `EncodeV2(Payload)` emits v2, and `Decode(string)` accepts v1, v2, and a legacy bare id, returning a `Payload` whose `HasOrdering` field reports whether the ordering metadata is meaningful.

## Which connections emit which envelope

| Connection | Default ordering key | Envelope |
| --- | --- | --- |
| `myCardgroupsConnection` | `updated_at` (mutable) | v2 |
| `masterCatalog` / admin master catalog | `sort_order` (admin-mutable) | v2 |
| cards by cardgroup | `id` (immutable) | v1 |
| master cards | `position` (admin-mutable) | v2 |
| admin users | `created_at` (immutable) | v1 |

The column named is the one the connection orders by when the client sends no `orderBy` — `resolveCardOrderBy` defaults to `(ID, ASC)`, `resolveMasterCardOrderBy` to `(POSITION, ASC)`, matching the schema defaults. In every case the ordering is made total by appending `id` as the tiebreaker, so a v1 row whose default key is `id` is safe by construction.

The one remaining v1 row is not quite a clean bill of health. The cards connection is safe on its `ID` default but not on the opt-in `DUE` / `UPDATED_AT` orderings, both of which move under normal review activity; it has not been migrated.

## What v2 guarantees, and what it does not

Guaranteed once a connection is on v2:

- **No duplicates when the boundary row's ordering key is edited upward.** The bookmark compares against the value captured at serve time, so the next page starts exactly where the previous one ended instead of after the row's new position.
- **No skipped rows when the boundary row's ordering key is edited downward.** Under v1 the re-read moved the bookmark past every remaining row, emptying the rest of the walk; the captured value keeps the walk anchored.
- **No silent mis-page across an ordering change.** A cursor whose embedded `orderBy` / `direction` disagrees with the current request is rejected as `BAD_USER_INPUT` rather than compared against a different column.
- **No weakening of the scope checks.** A v2 cursor can hydrate its ordering column without the repository, but the owner lookup (cardgroups), the published-scope lookup (catalog) and the cross-deck guard (master cards) still run, so the endpoint never becomes an existence oracle.

Not guaranteed — these are inherent to cursor pagination over a mutable column, and no envelope format fixes them:

- **A row deleted at the page boundary.** Its cursor no longer resolves; the request is rejected as `cursor not found`.
- **A row whose ordering key crosses the bookmark.** An unseen row edited so it sorts above the cursor has moved into a region already served and is skipped; the boundary row edited so it sorts below the cursor has moved into the region not yet served and is met again. The edit moved the row across the bookmark, not the bookmark across the rows.
- **`totalCount` drift.** The count is a separate query and reflects the moment it ran.

## Encoding site

The resolver boundary is the single encoding site. `backend/graph/resolver/connection.go` passes the per-connection encoder into the shared `buildEdges` generic, so every edge cursor and both `PageInfo` boundary cursors go through the same function exactly once:

- `cursor.Encode` for the v1 connections.
- `orderedCursorEncoder(out.Ordering, out.OrderKeys)` for the v2 connections. The usecase output carries `Ordering` (the effective `orderBy` / direction) and `OrderKeys` (node id → serialized ordering-key value); the resolver reads them and never touches the repository enums.

The usecase layer operates on raw entity UUIDs internally and never calls an encoder — see [`.claude/rules/pagination.md` § "Server-side design"](../../.claude/rules/pagination.md#server-side-design).

## Decoding site

Every aggregate's `resolve*Cursor` decodes its incoming `after`/`before` argument through the shared `decodeCursorOrBadInput` helper in `backend/internal/usecase/page.go`, which wraps `cursor.Decode`. A malformed payload (invalid base64, or a v2 body that is not the expected JSON) is surfaced as `BAD_USER_INPUT`, matching existing UUID-parse failures.

The v2 connections then run two extra steps before hydration:

1. `requireCursorOrdering` rejects a cursor taken under a different column or direction (`BAD_USER_INPUT`).
2. `applyCardgroupOrderKey` / `applyMasterCatalogOrderKey` / `applyMasterCardOrderKey` populates the repository cursor column from the embedded value. A value that does not parse into the column's type is `BAD_USER_INPUT`; an `orderBy` the helper does not handle is a caller bug and stays `INTERNAL`.

## Backward compatibility

`cursor.Decode` accepts all three forms, so cursors persisted by an older client keep paging:

- Input starts with `"v2:"` — strip the prefix, base64-decode, JSON-unmarshal. A failed decode is a hard `BAD_USER_INPUT` error.
- Input starts with `"v1:"` — strip the prefix, base64-decode the rest. A failed decode is a hard `BAD_USER_INPUT` error. `HasOrdering` is false, so the caller falls back to re-reading the ordering column off the current row.
- Input does not start with either prefix — pass through unchanged as the raw ID, same fallback as v1. UUID shape validation is the caller's responsibility.

The fallback path is deliberately the pre-v2 behaviour, defect included: a v1 cursor on a mutable-key connection can still duplicate or skip rows. It exists so no client is hard-broken by the migration, not because it is correct.

## Frontend rule

The frontend must never decode, inspect, or construct cursor strings. Cursor
values are read from API responses and passed back verbatim. The audit grep
`grep -rn "atob\|btoa\|startsWith.*v1" frontend/src/` must return no matches
for cursor-related code.

## "Opaque envelope" vs plain UUID — terminology by aggregate

The repo carries two distinct cursor encodings depending on the aggregate:

- **Opaque envelope** (`v1:base64(uuid)` or `v2:base64(json)`): `Card`, `Cardgroup`, `MasterCardgroup`, `MasterCard`, and `User` connection cursors are all wrapped by an encoder in the resolver helpers (`toCardConnectionModel`, `toCardgroupConnectionModel`, … in `backend/graph/resolver/connection.go`). The "opaque" label is meaningful — clients treat the envelope as a black box and must not parse it.
- **Plain UUID** (raw entity ID): the usecase connection *outputs* carry raw ids in `StartCur` / `EndCur`, and each edge keys off the raw `node.id`. Those are never handed to clients unwrapped.

When documenting or commenting on a Connection helper, use "opaque envelope" only for the encoded form. Use "plain UUID" (or "raw entity ID") for the unwrapped form. Mixing the two terms in the same context confuses readers who grep for the encoding convention.

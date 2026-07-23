# Cursor encoding

> Part of the [pagination](../../.claude/rules/pagination.md) rules. Cross-referenced by [`docs/pagination/resolve-connection-edge-by-node-id.md`](resolve-connection-edge-by-node-id.md).

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

### The `DUE` ordering key is not a plain column read

Every other ordering key is a column on the row being served. The card connection's `DUE` key is `COALESCE(user_card_fsrs.due, cards.created_at)` for the requesting user — the same expression the page query sorts on — serialized with `encodeTimeOrderKey`.

The emit path takes that value **out of the page query's own result set**: the repository reports the key it sorted each returned row by, and `cardOrderKeys` merely serializes what it was handed. It performs no I/O by design. A post-page FSRS read would resolve a *later* snapshot than the one that ordered the page, so a review landing between the two reads would mint a cursor keyed to a boundary the page never served — reintroducing exactly the skip the v2 envelope exists to prevent. `TestCardCursorWalk_Due_EmitIssuesNoFSRSLookup` is the structural guard: it asserts zero FSRS calls on emit, so any future re-introduction of a post-page read fails even when the value it recovers happens to agree.

`cardUsecase.dueOrderValues` — a single batched FSRS lookup — exists for the **v1 / legacy re-hydration path only**, which is handed a bare row id and has no captured key to fall back on. It re-implements the `COALESCE` fallback in Go, and the two expressions must stay in step: a key recovered there under one fallback and compared in SQL under another lands the bookmark on the wrong row.

## Which connections emit which envelope

| Connection | Default ordering key | Envelope |
| --- | --- | --- |
| `myCardgroupsConnection` | `updated_at` (mutable) | v2 |
| `masterCatalog` / admin master catalog | `sort_order` (admin-mutable) | v2 |
| cards by cardgroup | `id` (immutable); opt-in `DUE` / `UPDATED_AT` are mutable | v2 |
| master cards | `position` (admin-mutable) | v2 |
| admin users | `created_at` (immutable) | v1 |

The column named is the one the connection orders by when the client sends no `orderBy` — `resolveCardOrderBy` defaults to `(ID, ASC)`, `resolveMasterCardOrderBy` to `(POSITION, ASC)`, matching the schema defaults. In every case the ordering is made total by appending `id` as the tiebreaker, so a connection whose default key is `id` is anchored by an immutable value on that default.

`admin users` is the only connection left on v1, and it is safe by construction: `created_at` is never updated after insert. Every other connection emits v2, including the card connection whose *default* key is immutable — it would otherwise regress the moment a screen adopts `orderBy: DUE`.

## What v2 guarantees, and what it does not

Guaranteed once a connection is on v2:

- **No duplicates or skips of unmutated rows when the boundary row's ordering key is edited.** The bookmark compares against the captured value, so the edit never moves the bookmark. The boundary row itself follows the "row whose ordering key crosses the bookmark" non-guarantee below: an edit that moves it into the not-yet-served region (upward under ASC, downward under DESC) means it is met again; an edit into the already-served region means it is not.
- **No silent mis-page across an ordering change.** A cursor whose embedded `orderBy` / `direction` disagrees with the current request is rejected as `BAD_USER_INPUT` rather than compared against a different column.
- **No weakening of the scope checks.** A v2 cursor can hydrate its ordering column without the repository, but the owner lookup (cardgroups), the published-scope lookup (catalog) and the cross-deck / cross-cardgroup guards (master cards, cards) still run, so the endpoint never becomes an existence oracle.

Not guaranteed — these are inherent to cursor pagination over a mutable column, and no envelope format fixes them:

- **A row deleted at the page boundary.** Under hydrating orderings, and on connections that decline the [`orderBy: ID` shortcut](../../.claude/rules/pagination.md#server-side-design), its cursor no longer resolves and the request is rejected as `cursor not found`. On `card` / `master-card` under `orderBy: ID`, the cursor is accepted and the request serves the correct next page.
- **A row whose ordering key crosses the bookmark.** An unseen row edited so it sorts above the cursor has moved into a region already served and is skipped; the boundary row edited so it sorts below the cursor has moved into the region not yet served and is met again. The edit moved the row across the bookmark, not the bookmark across the rows.
- **`totalCount` drift.** The count is a separate query and reflects the moment it ran.

## Encoding site

The resolver boundary is the single encoding site. `backend/graph/resolver/connection.go` passes the per-connection encoder into the shared `buildEdges` generic, so every edge cursor and both `PageInfo` boundary cursors go through the same function exactly once:

- `cursor.Encode` for the one v1 connection (admin users).
- `orderedCursorEncoder(out.Ordering, out.OrderKeys)` for the v2 connections. The usecase output carries `Ordering` (the effective `orderBy` / direction) and `OrderKeys` (node id → serialized ordering-key value); the resolver reads them and never touches the repository enums.

The usecase layer operates on raw entity UUIDs internally and never calls an encoder — see [`.claude/rules/pagination.md` § "Server-side design"](../../.claude/rules/pagination.md#server-side-design).

## Decoding site

All five connections decode their incoming `after`/`before` argument through the shared `decodeCursorOrBadInput` helper in `backend/internal/usecase/page.go`, which wraps `cursor.Decode`. A malformed payload (invalid base64, or a v2 body that is not the expected JSON) is surfaced as `BAD_USER_INPUT`, matching existing UUID-parse failures.

Where that call sits differs by envelope. The four v2 connections each own a per-aggregate method — `resolveCardCursor`, `resolveCardgroupCursor`, `resolveMasterCardCursor`, `resolveMasterCatalogCursor` — because they have post-decode work to do. The v1 admin-users connection has no such method: it calls the helper inline from `adminUserUsecase.List`, so do not grep for a `resolveAdminUserCursor`.

The v2 connections then run two extra steps before hydration:

1. `requireCursorOrdering` rejects a cursor taken under a different column or direction (`BAD_USER_INPUT`).
2. `applyCardgroupOrderKey` / `applyMasterCatalogOrderKey` / `applyMasterCardOrderKey` / `applyCardOrderKey` populates the repository cursor column from the embedded value. A value that does not parse into the column's type is `BAD_USER_INPUT`; an `orderBy` the helper does not handle is a caller bug and stays `INTERNAL`.

The v1 connection runs the symmetric guard instead: `rejectOrderedCursor` refuses any inbound cursor that carries ordering metadata, since a v2 cursor cannot have been issued by the admin-users connection (`BAD_USER_INPUT`).

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

## Terminology: wire form vs internal form

Both envelopes above are the **wire form** — what a client receives and hands back. The **internal form** is the raw entity UUID, and the two are separated by one layer boundary, not by aggregate:

- Above the boundary (client-facing), every connection emits an envelope. `Card`, `Cardgroup`, `MasterCardgroup`, `MasterCard`, and `User` cursors are all produced by an encoder passed into `buildEdges` in `backend/graph/resolver/connection.go`. No connection hands a client a bare UUID.
- Below the boundary (usecase and repository), nothing holds an envelope. The connection outputs carry raw ids in `StartCur` / `EndCur`, each edge keys off the raw `node.id`, and `decodeCursorOrBadInput` unwraps inbound cursors back to raw ids before any repository call.

When documenting or commenting on a Connection helper, say "cursor" (or name the `v1:` / `v2:` envelope) only for the wire form, and "raw entity id" for the internal form. Calling the internal form a cursor invites the id-equals-cursor assumption that the shared `buildEdges` helper exists to prevent.

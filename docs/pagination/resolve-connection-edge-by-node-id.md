# Resolve a connection edge by `node.id`, never by `edge.cursor`

> Part of the [pagination rules](../../.claude/rules/pagination.md) (§ "Frontend cache patterns").
> Closely related to the [cursor-opaqueness invariant](../../.claude/rules/pagination.md#why-relay-connection-not-offsetlimit)
> and [cursor encoding](cursor-encoding.md).

## Why

A Relay-style `Connection` edge carries two distinct identifiers, and they are
**not interchangeable**:

- `edge.node.id` — the entity's stable primary key (`"01J…"`). This is what the
  rest of the app uses to address the entity (open its editor, delete it, read
  it from the normalized cache as `<Type>:<id>`).
- `edge.cursor` — an **opaque pagination handle**. The backend emits either
  `"v1:" + base64(id)` or, on the connections whose ordering column is mutable,
  `"v2:" + base64(json)` (see [cursor-encoding.md](cursor-encoding.md)).
  Cursor opaqueness is a deliberate Relay invariant: clients treat the cursor as
  a black box and pass it back unchanged via `after` / `before`. The server may
  change the encoding at any time without breaking clients.

Because the cursor is `"v1:base64(id)"` and never the raw id, **any client code
that matches an entity by comparing a raw id against `edge.cursor` always
misses**. The two failure shapes:

1. **Edit-target resolution.** `edges.find((e) => e.cursor === editId)` returns
   `undefined` for every edge when `editId` is a raw node id → the editor panel
   renders its not-found state (`"Master not found."`) for *every* row.
2. **Cache-`modify` delete filter.** `conn.edges.filter((edge) => edge.cursor !== id)`
   never removes anything → the deleted edge lingers in the connection and the
   row stays on screen until a full refetch.

Both bugs are silent: the code compiles, the types line up (`string === string`),
and the happy path of *loading* the list works — only the *identity match* fails.

## What

The fix is to match on the node's id on both sides. Worked example —
`frontend/src/app/admin/masters/admin-masters-client.tsx`:

```ts
// Edit-target resolution: match by node.id, NOT by the opaque cursor.
const editEdge = editId ? edges.find((e) => e.node.id === editId) : undefined;

// Cache-modify delete filter: in the normalized cache the edge's `node` is a
// Reference, so read its id via readField — `edge.node.id` is not directly
// available on a Reference.
cache.modify({
  fields: {
    adminMasters(existing, { readField }) {
      const conn = existing as { edges?: ReadonlyArray<{ node: Reference }>; totalCount?: number };
      if (!conn.edges) return existing;
      const next = conn.edges.filter((edge) => readField<string>("id", edge.node) !== id);
      if (next.length === conn.edges.length) return existing;
      return { ...conn, edges: next, totalCount: Math.max(0, (conn.totalCount ?? 0) - 1) };
    },
  },
});
```

The two sites read the id differently because they operate on different
representations of the same edge:

- The **render path** (`editEdge`) sees the live query result — `edges` from the
  shared `useConnectionPagination` hook — where `edge.node` is a plain object, so
  `edge.node.id` is a direct field read.
- The **cache path** (`cache.modify`) sees the *normalized* store, where
  `edge.node` is an Apollo `Reference` (a pointer into the entity table) — the
  id is read with `readField<string>("id", edge.node)`.

## How to apply

1. Never compare a raw entity id against `edge.cursor`. Use `edge.node.id` in
   render-path code and `readField<string>("id", edge.node)` inside
   `cache.modify`.
2. **Keep the regression-test fixture faithful: the mock edge's `cursor` MUST be
   the opaque encoded value, not the raw id.** A fixture that sets
   `cursor: master.id` lets a (wrong) cursor-based match pass and masks exactly
   the bug this rule prevents. Mirror the backend encoder in the test:

   ```ts
   // Mirror cursor.Encode (Go): "v1:" + base64(id). A faithful cursor is what
   // makes the node.id-vs-cursor regression detectable.
   function encodeCursor(id: string): string {
     return `v1:${btoa(id)}`;
   }
   ```

   Worked example: the narrow test `admin-masters-client.test.tsx` uses
   `encodeCursor` and pins both halves of the fix — "opens the edit drawer
   resolved by node.id even when the edge cursor is opaque" and "removes the
   deleted deck from the list". A broad page test whose `makeEdge` sets
   `cursor: master.id` is a latent footgun: it does not break today (it never
   opens the drawer or deletes), but any future edit/delete assertion added to
   it would pass against the raw-id cursor while production resolves by
   `node.id`, re-hiding the regression.

## See also

- [Cursor encoding](cursor-encoding.md) — the opaque `v1:` / `v2:` envelopes this rule depends on.
- [`.claude/rules/pagination.md` § "Frontend cache patterns"](../../.claude/rules/pagination.md#frontend-cache-patterns) — the Connection create/delete/update bullets.

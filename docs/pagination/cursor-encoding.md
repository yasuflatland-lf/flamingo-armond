# Cursor encoding

> Part of the [pagination](../../.claude/rules/pagination.md) rules. Cross-referenced by `docs/backend.md` and `docs/frontend.md`.

The cursor is the bare entity UUID — not base64-encoded. UUID is already opaque; double-encoding adds no security and forces clients to decode before comparing. The schema declares `cursor: ID!` and the resolver assigns the entity ID directly into edges.

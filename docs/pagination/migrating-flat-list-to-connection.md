# Migrating a flat list to a Connection: write to BOTH cached shapes during the deprecation window

> Part of the [pagination](../../.claude/rules/pagination.md) rules. Cross-referenced by `docs/backend.md` and `docs/frontend.md`.

The `@deprecated` schema migration (see [`pagination.md` § "Migration via `@deprecated`"](../../.claude/rules/pagination.md#schema-side-conventions)) leaves the old flat-list query and the new Connection query coexisting in the codebase. A mutation `update` callback that creates an entity must write to BOTH cached shapes for the entire window where any consumer still reads the deprecated query — otherwise the consumer of the old query (often a sibling component like a picker sheet that has not yet migrated) shows stale data after a successful create.

> **Note:** The `myCardgroups` → `myCardgroupsConnection` migration in this codebase is now complete. The example below uses generic placeholder names (`LegacyDocument` / `NewConnectionDocument`) to illustrate the dual-write pattern for future migrations.

```ts
update(cache, { data }) {
  if (!data?.createEntity?.entity) return;
  const created = data.createEntity.entity;
  // Deprecated flat list — keep updating until all readers move over.
  const flat = cache.readQuery({ query: LegacyDocument });
  cache.writeQuery({
    query: LegacyDocument,
    data: { legacyEntities: [created, ...(flat?.legacyEntities ?? [])] },
  });
  // New Connection — readQuery + writeQuery with the shared default vars.
  const conn = cache.readQuery({
    query: NewConnectionDocument,
    variables: DEFAULT_VARS,
  });
  cache.writeQuery({ /* prepend edge, bump totalCount, or build cold-cache shape */ });
}
```

Removing the deprecated branch is the last step of the migration, after a grep confirms zero callers of the old document. Reference: `frontend/src/app/cardgroups/new/new-cardgroup-client.tsx` (the completed `MyCardgroupsConnectionDocument` write after the migration).

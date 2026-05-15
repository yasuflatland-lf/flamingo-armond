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

## Completeness checklist for the consumer audit

The grep that decides "are we done?" is the highest-leverage step of the migration. A single-pattern grep — e.g. `grep -rn "useQuery(MyCardgroups" frontend/src/` — routinely misses consumers and lets the deprecation removal land while live code still references the old shape. The `MyCardgroups` → `myCardgroupsConnection` migration in this codebase missed 5 of 7 consumers on the first audit because the grep was scoped to one pattern and one tree.

The completeness audit runs **multi-vector grep across every source tree** before any deprecation removal:

- **Operation name** (the named GraphQL document, PascalCase): `grep -rn "MyCardgroups\b" frontend/src/ backend/`.
- **Field name** (the schema field, camelCase): `grep -rn "\bmyCardgroups\b" frontend/src/ backend/ schema/`.
- **Generated document/type names** (codegen output): `grep -rn "MyCardgroupsDocument\|MyCardgroupsQuery\|MyCardgroupsQueryVariables" frontend/src/`.
- **Backend Go consumers** (resolvers, usecases, integration tests): `grep -rn "MyCardgroups\|myCardgroups" backend/` — backend Go code reads the schema directly via gqlgen and is invisible to a frontend-only grep.
- **Test fixtures** (mocks, MSW handlers, MockedProvider setup): `grep -rn "MyCardgroups" frontend/src/**/*.test.* frontend/src/**/__mocks__/`.

Each axis is run on each source tree. Any non-empty result is a consumer that must be migrated before the deprecation removal lands. The audit is **finished** only when all five greps return no results outside the operation's own definition file.

## Test coverage migrates with the production code

Tests for the deprecated shape (unit tests for a flat-list-to-model converter, frontend `update` callback tests for the legacy cache write, backend integration tests asserting the flat-list response) must be **replaced** with Connection-equivalent tests before the deprecation removal — not deleted in a separate pass. Deleting `TestToCardgroupModels_FiltersNil` without adding `TestToCardgroupConnectionModel_FiltersNilNodes` silently drops the coverage of the equivalent code path; the deletion-only diff looks safe in review because no new failures appear, but the property the test was protecting is now unverified. The replacement test must exercise the same code path against the new shape (e.g. nil-edge filtering, cache update with the new variables, warm-cache vs cold-cache branches). Run the production-code audit grep with `--include='*_test.*'` added to confirm every test reference to the old shape has a counterpart in the new shape's test file.

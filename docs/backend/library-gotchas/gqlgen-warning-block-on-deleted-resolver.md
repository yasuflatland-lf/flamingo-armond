# gqlgen wraps deleted-field resolvers in a `// !!! WARNING !!!` block — they are not auto-removed

> Part of the [Go library gotchas](../../../.claude/rules/go-library-gotchas.md) rules.

When a field is deleted from `schema/schema.graphql` and `gqlgen` is regenerated, the corresponding resolver function in `backend/graph/resolver/schema.resolvers.go` is **not** removed. Instead, gqlgen wraps the orphaned resolver in a `// !!! WARNING !!!` comment block that survives every subsequent regeneration. The build still passes because the function is no longer referenced from the generated interface, but the code stays in the repo as dead body until a human deletes the block by hand.

The same applies to model receivers and any helper that was uniquely keyed to the deleted field (e.g. a `toFooModel` converter): if the helper has no other callers after the field is gone, it becomes unreferenced but is not auto-removed by gqlgen.

## Why this matters

The orphaned resolver is dead code that:

- **Survives regen.** Re-running `gqlgen generate` does not strip the WARNING block. Every future regen preserves it. A "did anyone clean up after the schema change?" audit cannot rely on the generator.
- **Imports may go stale.** The orphaned function still references types, sentinels, and helper packages. If a downstream cleanup removes an import that only the orphan used, the build fails on the WARNING block — at which point the cleanup engineer is debugging code they did not write and were not expecting to see.
- **Confuses future readers.** A reader greps for the resolver name to understand a usecase entrypoint and lands inside a WARNING block whose code path is unreachable. The comment header reads as scary ("`WARNING`") without explaining the resolver is dead, not broken.

## Removal procedure

After running `gqlgen generate` on a schema change that deletes a field:

1. Search the generated file for the warning marker: `grep -n "!!! WARNING !!!" backend/graph/resolver/schema.resolvers.go`.
2. For each match, confirm the function corresponds to a deleted schema field (compare against the schema diff).
3. Delete the entire block — the leading comment, the function body, the trailing `}`.
4. Run `go vet ./... && go build ./...` from `backend/` to catch any helper, type, or import that was only reachable from the deleted resolver. Helpers that are now unreferenced must also be deleted (gqlgen does not track these).
5. Run `go test ./...` — tests targeting the orphan must also be removed (see [`migrating-flat-list-to-connection.md` § "Test coverage migrates with the production code"](../../pagination/migrating-flat-list-to-connection.md#test-coverage-migrates-with-the-production-code) for the analogous frontend audit).

## Detection grep

Add the warning marker grep to the schema-deletion checklist:

```bash
grep -n "!!! WARNING !!!" backend/graph/resolver/schema.resolvers.go
```

A clean schema-deletion PR returns no matches. A leftover match is a hard failure of the cleanup pass, not a stylistic note.

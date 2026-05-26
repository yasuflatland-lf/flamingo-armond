# gqlgen `follow-schema` layout orphans the old resolver file when a schema FILE is renamed

> Part of the [Go library gotchas](../../../.claude/rules/go-library-gotchas.md) rules.

`backend/gqlgen.yml` sets `resolver.layout: follow-schema` with `filename_template: "{name}.resolvers.go"`, so each resolver file's name derives from the **schema file** that declared its operations. Renaming a whole schema file — e.g. `schema/dictionary.graphql` → `schema/card_import.graphql` — therefore makes gqlgen emit a **new** resolver file named after the new schema file (`backend/graph/resolver/card_import.resolvers.go`) while leaving the **old** `backend/graph/resolver/dictionary.resolvers.go` on disk untouched.

The old file is not cleaned up and is not wrapped in a `// !!! WARNING !!!` block (that mechanism only applies to a deleted *field* inside a still-existing resolver file — see the related links). It is simply orphaned: a separate file whose resolver-method receivers still match the `Resolver` interface. The result is a **duplicate-method build failure** — the same resolver method is now declared in both the new and the old file:

```
backend/graph/resolver/card_import.resolvers.go:NN: method Resolver.ImportCards already declared at backend/graph/resolver/dictionary.resolvers.go:MM
```

The old file must be **deleted by hand**. gqlgen will not remove it on any future regen because, from the generator's point of view, it owns a schema file (`dictionary.graphql`) that no longer exists — so it is never re-touched.

## Removal procedure

After running `gqlgen generate` on a schema **file rename** (as opposed to a field drop):

1. Confirm the new resolver file was created: `ls backend/graph/resolver/<new-name>.resolvers.go`.
2. Delete the orphaned old file: `rm backend/graph/resolver/<old-name>.resolvers.go`.
3. Run `go vet ./... && go build ./...` from `backend/` to confirm the duplicate-method error is gone and no helper that was unique to the old file is left dangling.
4. Run `go test ./...` — any test file keyed to the old resolver (`<old-name>_resolver_test.go`) is now dead and must be deleted alongside it, per the code-path-deletion-obliges-test-deletion rule in [`.claude/rules/scope-discipline.md`](../../../.claude/rules/scope-discipline.md).

## Why the distinction from the field-drop case matters

The detection grep for the field-drop case (`grep -n "!!! WARNING !!!"`) returns **nothing** for a file rename — there is no WARNING block to find. The orphaned-file failure surfaces only at `go build` as a duplicate-method error, not as a survivable dead-code block. The two failure modes need different cleanup steps: a field drop edits one resolver file in place; a file rename deletes a whole file.

## Related

- [gqlgen wraps deleted-field resolvers in a `// !!! WARNING !!!` block — they are not auto-removed](gqlgen-warning-block-on-deleted-resolver.md) — the field-drop sibling: an orphaned resolver *within* a still-existing file, build still passes.
- [Codegen two-pass failure mode: schema field drop blocks regen until production code is fixed](codegen-two-pass-schema-field-drop.md) — a field drop that blocks regen *before* it can run; a file rename does not block regen, it leaves a stale file *after* a successful regen.

# Codegen two-pass failure mode: schema field drop blocks regen until production code is fixed

> Part of the [Go library gotchas](../../../.claude/rules/go-library-gotchas.md) rules.

When a field is removed from `schema/*.graphql`, the **schema is the source of truth that both generators validate against**. Neither `gqlgen generate` (backend) nor `pnpm codegen` (frontend) will succeed until every reference to the dropped field is removed from production code first. The regen step and the production-code fix are mutually dependent: regen cannot proceed without the fix, and writing the fix by reading regen output is impossible because regen has not run yet.

## Why both generators fail

### gqlgen (backend)

`gqlgen generate` reads the schema and then validates any hand-written mapper or model code against it. If `backend/graph/resolver/mapper.go` still references `model.SwipeResponse.NextCards` after `nextCards` is removed from the schema, `gqlgen` emits a compile-time error on the struct literal:

```
unknown field NextCards in struct literal of type model.SwipeResponse
```

The generator cannot proceed because the struct literal is invalid Go after the generated `model.SwipeResponse` type is updated to drop the field.

### graphql-codegen (frontend)

`pnpm codegen` validates GraphQL operation documents (`.graphql` files under `frontend/src/`) against the schema. If a query or mutation still selects `nextCards` on `SwipeResponse`, the validator reports:

```
Cannot query field "nextCards" on type "SwipeResponse".
```

The codegen tool fails before writing any output file.

## Prescribed ordering

```
1. Edit schema/*.graphql  — remove the field declaration
2. Fix production code    — remove the reference BY HAND, without regenerating
   a. backend/graph/resolver/mapper.go  — drop the struct field assignment
   b. frontend/src/**/*.graphql          — drop the field from the selection set
   c. Any other file that references the dropped field (grep first)
3. Run gqlgen generate   — now succeeds; model type no longer has the field
4. Run pnpm codegen      — now succeeds; selection set matches schema
```

The key constraint is step 2: the hand-edit must happen **before** either generator runs. There is no way to read the "new" generated types to guide the fix because the generators refuse to produce output until the fix is already in place.

### Pre-flight grep

Before fixing anything, enumerate every reference to the dropped field across both layers:

```bash
# Backend: mapper, resolvers, usecases
grep -rn 'NextCards\|nextCards' backend/ --include='*.go'

# Frontend: operation documents and generated client code (generated files
# will be rewritten by codegen; focus on hand-written .graphql files)
grep -rn 'nextCards' frontend/src/ --include='*.graphql'
```

Fix every hit returned by the grep before attempting either regen.

## Worked example: `nextCards` removal (issue #229)

The `nextCards` field was removed from the `SwipeResponse` type in `schema/schema.graphql`.

**First `gqlgen generate` attempt (before fixing mapper.go):**

```
backend/graph/resolver/mapper.go:42:3: unknown field NextCards in struct literal of type model.SwipeResponse
exit status 1
```

**First `pnpm codegen` attempt (before fixing the frontend operation document):**

```
[FAILED] documents: frontend/src/generated/graphql.ts
  Cannot query field "nextCards" on type "SwipeResponse". (line 7, column 5)
```

After `mapper.go` was updated to remove the `NextCards` field assignment and the frontend operation document was updated to drop `nextCards` from the selection set, both generators ran to completion without errors.

## Anti-pattern: trying to fix by reading regen output

A tempting but unworkable approach is:

1. Run `gqlgen generate` hoping to read the new `model.SwipeResponse` definition.
2. Use that definition to update `mapper.go`.
3. Run `gqlgen generate` again.

This is a chicken-and-egg loop. Step 1 always fails because the struct literal in the current `mapper.go` is invalid against the updated schema. The generator exits before writing the new model types. The fix must be authored by reading the **schema diff** directly, not by reading generator output.

## Related

- [gqlgen wraps deleted-field resolvers in a `// !!! WARNING !!!` block — they are not auto-removed](gqlgen-warning-block-on-deleted-resolver.md) — a different gqlgen failure mode that applies after a successful regen (orphaned resolver bodies) rather than before regen (mapper struct literal mismatch).
- [gqlgen `follow-schema` layout orphans the old resolver file when a schema FILE is renamed](gqlgen-schema-file-rename-orphans-resolver.md) — the file-rename sibling: a whole orphaned resolver file (duplicate-method build break) rather than a dropped field within a file.

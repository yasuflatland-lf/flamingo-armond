# gqlgen acronym enum: TYPE keeps the acronym, METHOD lowercases its tail

> Part of the [Go library gotchas](../../../.claude/rules/go-library-gotchas.md) rules.

A GraphQL enum whose name starts with an all-caps acronym produces **two
different casings** in gqlgen-generated Go, and hand-written mapper/resolver
code that assumes a single consistent spelling will not compile.

For `enum CEFRLevel { A1 A2 ... }` exposed as the field `cefrLevel`, gqlgen
v0.17.90 generates:

- The Go **type** as `model.CEFRLevel` — the acronym stays uppercase.
- The **constants** as `model.CEFRLevelA1` .. `model.CEFRLevelC2` — acronym
  uppercase preserved.
- The resolver-interface **method** for the field as `CefrLevel` — gqlgen
  lowercases the acronym tail when deriving the method name from the field
  name (`cefrLevel` → `CefrLevel`), so the acronym is *not* preserved here.

The same concept therefore carries two spellings in generated code: type
`CEFRLevel`, method `CefrLevel`. A mapper that returns `model.CEFRLevel` and a
resolver method named `func (r *cardResolver) CefrLevel(...)` are both correct
and must coexist.

## Why it matters

The divergence is invisible until compile time, and guessing the method name as
`CEFRLevel(...)` to "match the type" silently produces a method that does not
satisfy the generated `CardResolver` interface — the resolver stays unwired and
the field always returns null. The only safe path is to **regenerate first,
then grep the generated identifiers** before writing the mapper or resolver:

```bash
# Type + constants (acronym preserved)
grep -nE 'type CEFRLevel|CEFRLevelA1' backend/graph/model/models_gen.go
# Resolver-interface method (acronym tail lowercased)
grep -n 'CefrLevel(ctx' backend/graph/generated/generated.go
```

Verify with: `backend/graph/resolver/mapper.go` (`toCEFRLevelModel` returns
`model.CEFRLevel`) and `backend/graph/resolver/card.resolvers.go` (method
`func (r *cardResolver) CefrLevel(...)`).

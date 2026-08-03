# go-arch-lint deepScan attributes concrete-type value-flow to the producing component

> Part of the [Go library gotchas](../../../.claude/rules/go-library-gotchas.md) rules.

With `deepScan: true` (the v3 default, made explicit in
`backend/.go-arch-lint.yml`), go-arch-lint does more than read the import graph
at a wiring site: it tracks the **concrete type of a value** flowing into a
constructor and attributes a dependency edge to the component that *produces*
that value — even when every individual import is legal.

A composition-root line that passes a concrete value straight into a
constructor is the trigger:

```go
// Fails: deepScan infers a `cefr --> domain_service` edge because the concrete
// *cefr.WordList value flows into service.NewCEFRClassifier, and that edge is
// attributed to the cefr component (which may depend only on domain).
cefrClassifier := service.NewCEFRClassifier(cefr.NewWordList())
// (a concrete-typed local — `var w *cefr.WordList = cefr.NewWordList()` — fails the same way)
```

```
Dependency cefr --> domain_service not allowed
```

The fix is to **widen the value to the consumer-defined port interface at the
wiring site**, so the inferred edge becomes the allowed
`domain_service --> domain` instead of `cefr --> domain_service`:

```go
// Passes: the value is typed as the domain port, so deepScan attributes
// the edge to domain_service -> domain (allowed), not cefr -> domain_service.
var cefrWords domain.CEFRWordList = cefr.NewWordList()
cefrClassifier := service.NewCEFRClassifier(cefrWords)
```

## Why it matters

This is invisible at the import-graph level: `cmd/server` legally imports both
`cefr` and `domain/service`, and each constructor's imports are individually
allowed. Only deepScan's value-flow analysis surfaces the forbidden edge. The
fix is a one-line interface-typed local at the composition root — **not** an
archfile change. Reaching for a `mayDependOn` widening would instead punch a
permanent hole in the layer model to paper over a fixable wiring detail.

The `auth` wiring also shows why a consumer-defined port may need to be
exported. `RoleChecker` and `RoleAssigner` are named by `cmd/server` so concrete
`repository.UserRoleRepository` values can be widened before entering auth
constructors, avoiding an `auth.mayDependOn: repository` exception.

This refines the blanket "import statement only" framing in
[`go-arch-lint-violation-output-and-scope.md`](go-arch-lint-violation-output-and-scope.md):
with `deepScan: true`, a *value-flow* edge can appear where no import-level
violation exists.

Verify with: the wiring block and its comment in
`backend/cmd/server/main.go` (the `var cefrWords domain.CEFRWordList =
cefr.NewWordList()` line) and `deepScan: true` in
`backend/.go-arch-lint.yml`.

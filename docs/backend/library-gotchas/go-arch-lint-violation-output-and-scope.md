# go-arch-lint violation output format and import-graph scope

> Part of the [Go library gotchas](../../../.claude/rules/go-library-gotchas.md) rules.

## Violation output format

`go-arch-lint check` emits plain ASCII text, one line per violation:

```
Component resolver shouldn't depend on backend/internal/gqlerr in /abs/path/graph/resolver/schema.resolvers.go:12
```

The format is: `Component <name> shouldn't depend on <import-path> in <abs-file>:<line>`.
There is no column number, no surrounding quotes, and no "not in allowedList" suffix.

For machine-parseable output pass `--output-type=json`. In v1.15.0 violations
are nested under `.Payload.ArchWarningsDeps`. This path is internal and may
change between binary releases — do not pin scripts to it; use it only for
one-off local debugging.

## Binary version vs. archfile version

`go-arch-lint --version` (or the `Linter version:` line printed by
`go tool go-arch-lint version`) reports the binary version (`v1.15.0`).
The `version: 3` at the top of `.go-arch-lint.yml` is the **archfile schema
version**, not the binary version. The two are independent. Do not conflate
them when reading release notes or filing bug reports.

## What go-arch-lint covers — and what it does not

`go-arch-lint` operates at the **import-graph level only**. Even with
`deepScan: true` (the v3 default, made explicit in `backend/.go-arch-lint.yml`
for clarity), it answers only "does package A have an `import` statement for
package B?" — it does not inspect:

- Which functions, methods, or types from B are actually called
- Whether the import is used via a composite literal (`&T{}`)
- Whether a string literal matches a hardcoded constant from B
- Call sequences across package boundaries

This boundary determines which CI gates can be retired in favor of
`go-arch-lint` and which must stay as grep or AST-aware tools. The table from
[`.claude/rules/backend-layering.md` § "What go-arch-lint covers vs. doesn't"](../../../.claude/rules/backend-layering.md#what-go-arch-lint-covers-vs-doesnt)
captures the current split:

| Shape | Tool |
|---|---|
| Import graph | `go-arch-lint` |
| Function-call + string-arg (`fmt.Errorf("%w")`) | grep / `forbidigo` |
| Composite literal (`&ucerr.ValidationError{}`) | grep / `ast-grep` |
| Call-sequence (resolver → FromUsecaseError path) | grep / `ast-grep` |
| String literal (hardcoded `extensions.code`) | grep / `ast-grep` |

`depguard` (a `golangci-lint` linter) duplicates the import-graph check and
adds no coverage over `go-arch-lint`; do not add it as a complementary tool.

## Cross-reference

See [`.claude/rules/backend-layering.md`](../../../.claude/rules/backend-layering.md)
for the authoritative layer config and the full gate-coverage table.

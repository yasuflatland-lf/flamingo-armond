# gqlgen-managed resolver imports must match `gqlgen generate` output — a post-gen reorder fails CI

> Part of the [Go library gotchas](../../../.claude/rules/go-library-gotchas.md) rules.

The backend CI "Generate gqlgen artifacts" step regenerates the gqlgen output and then
`git diff`-checks it: the committed `backend/graph/resolver/*.resolvers.go` (and the
`backend/graph/{generated,model}/` files) must match what `go tool gqlgen generate` produces
**byte-for-byte**. Any divergence fails the job with `exit code 1` and a diff dump — before
the test step even runs.

gqlgen emits the resolver file's `import (...)` block in its own grouping: project-local
packages (`backend/graph/model`, `backend/internal/...`) first, then stdlib (`context`) and
third-party (`github.com/rotisserie/eris`). This is **not** the goimports/gofmt-with-`-local`
grouping (stdlib first). So if a formatter or import-sorter runs on the generated resolver
**after** `gqlgen generate` — a `goimports -w`, a `biome`-style "organize imports" reach into
Go, or a code-simplifier pass — it reorders the block to the stdlib-first convention and the
committed file diverges from gqlgen's canonical output.

## Why it is invisible locally

Import order does not affect compilation. `go build ./...`, `go vet ./...`, and the full test
suite all pass on the reordered file. The divergence surfaces **only** in the CI regen-and-diff
gate, which a normal local `go test` run does not reproduce. A green local run is not evidence
the gqlgen gate will pass.

## The rule

- After editing a resolver, run `go tool gqlgen generate` from `backend/` and commit its output
  **as-is**. Do not run a separate import-sorter/formatter over `*.resolvers.go` or the generated
  `graph/{generated,model}/` files.
- Treat the gqlgen-managed `import (...)` block in `*.resolvers.go` as generated code: hand edits
  to resolver *bodies* are fine (gqlgen preserves them), but the import block belongs to the
  generator. A code-simplifier / formatter agent must be told explicitly to leave it alone.
- To reproduce the CI check locally before pushing: run `go tool gqlgen generate` then
  `git diff --exit-status -- backend/graph/` — a non-empty diff is exactly what CI will fail on.

## Worked example

Adding the `seedDefaultStarterCardgroups` mutation, a post-generation format step reordered
`master_catalog.resolvers.go`'s imports to `context` + `eris` **before** the `backend/*` group.
Local `go build`/`go vet`/`go test` were all green, but the CI "Generate gqlgen artifacts" step
failed with a diff on the import block (`exit code 1`). The fix was to re-run
`go tool gqlgen generate` (which restored gqlgen's `backend/*`-first order) and commit that
output verbatim — no behavior change, only the import grouping.

# go-arch-lint: exclude testdata Go files and regenerate generated packages before checking

> Part of the [Go library gotchas](../../../.claude/rules/go-library-gotchas.md) rules.

## testdata `.go` files declare real package names

Go source files under a `testdata/` directory are skipped by the Go toolchain
by convention — `go build` and `go test` never compile them. However,
`go-arch-lint` reads the import graph from the file system, not from the
compiled packages, and it will attempt to assign a `testdata/*.go` file to a
component based on its package declaration.

A testdata file that declares `package resolver` or `package usecase` will be
flagged as "not attached to any component" because the `go-arch-lint` component
definition for `resolver` points at `graph/resolver`, not at
`cmd/schema-lint/testdata/`. This is not a real violation — the file is a
fixture for the schema-lint AST walker, not production code.

Exclude testdata directories in `excludeFiles`:

```yaml
excludeFiles:
  - '^.*_test\.go$'                        # tests legitimately cross layers
  - '.*schema-lint/testdata/.*\.go$'       # AST-walker fixtures; not compiled into the binary
```

The second pattern is directory-specific rather than a blanket
`.*testdata/.*\.go$` to avoid excluding legitimate testdata in other tools that
might be added later. Broaden the pattern only if a second tool's testdata
triggers the same issue.

## Generated packages must exist before go-arch-lint runs

`go-arch-lint` reads the actual import graph from source files on disk. If a
generated package (`graph/generated/`, etc.) is gitignored and does not exist
in a fresh clone or worktree, `go-arch-lint` silently skips it — components
that import it will appear to have no dependency on it, and any `commonComponents`
entries for it become dead references.

In this repo, `backend/graph/generated/` is gitignored (gqlgen output).
CI handles this by running `go tool gqlgen generate` **before** the
`go-arch-lint check` step. When running the tool locally on a fresh worktree,
run the generator first:

```bash
cd backend
go tool gqlgen generate
go tool go-arch-lint check --project-path .
```

The same principle applies to any tool that reads the import graph (e.g.,
`go vet`, `staticcheck`): if a generated package is missing, the tool may
report false "package not found" errors or silently omit the package from its
analysis.

## Cross-reference

See [`.claude/rules/backend-layering.md` § "Operating notes"](../../../.claude/rules/backend-layering.md#operating-notes)
for the full local-run sequence, including the generator step.

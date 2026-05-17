# `go get -tool`: installing Go tools as module dependencies (Go 1.24+)

> Part of the [Go library gotchas](../../../.claude/rules/go-library-gotchas.md) rules.

Go 1.24 introduced the `tool` directive in `go.mod`. A tool declared this way
is pinned in `go.sum`, reproducible across environments, and invokable as
`go tool <name>` without a separate installation step. This repo uses it for
`go-arch-lint`, `gqlgen`, and `goyacc`.

## Adding a tool

```bash
# From the backend/ directory:
go get -tool github.com/fe3dback/go-arch-lint@latest
go mod tidy
```

This appends to `go.mod`:

```
tool (
    ...
    github.com/fe3dback/go-arch-lint
)
```

The tool's transitive dependencies appear in `go.sum` as indirect requirements.
No separate `go install` step is needed; `go mod download` fetches and
checksum-verifies the tool binary as part of the normal module bootstrap.

## Invoking a tool

```bash
go tool go-arch-lint check --project-path .
go tool gqlgen generate
go tool goyacc -o parser.go grammar.y
```

The `go tool <name>` form uses the short name (last path segment of the module
path). If two tools share the same last segment, use the full module path
instead.

## Updating a tool

```bash
go get -tool github.com/fe3dback/go-arch-lint@<new-tag>
go mod tidy
```

Commit both `go.mod` and `go.sum`. CI uses the exact pinned version, so the
update is safe to review in the diff.

## Why this matters

Before the `tool` directive, teams used a separate `tools.go` file with a blank
`_` import under a build tag to pin tool versions, or relied on `go install`
calls with version suffixes that silently used the locally installed binary
rather than the pinned version. The `tool` directive removes both workarounds
and makes the tool version as auditable as any other dependency.

## Cross-reference

See [`.claude/rules/backend-layering.md` § "Operating notes"](../../../.claude/rules/backend-layering.md#operating-notes)
for `go tool go-arch-lint` usage. For `gqlgen` tool usage, see the generator
regeneration step in CI (`.github/workflows/backend.yml`).

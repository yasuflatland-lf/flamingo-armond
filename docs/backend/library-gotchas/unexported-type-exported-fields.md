# Unexported types must not have exported fields

> Part of the [Go library gotchas](../../../.claude/rules/go-library-gotchas.md) rules.

An unexported struct type whose fields are capitalized (exported) is
internally inconsistent. Within a `cmd/` binary package the inconsistency is
harmless — no outside package can reference the type by name — but the
exported-field capitalization implies a public API contract that does not exist.
Readers unfamiliar with the package may introduce linting exceptions or assume
cross-package use is intended, which erodes the design signal of unexported types.

```go
// WRONG — unexported type with exported fields
type serverConfig struct {
    ShutdownTimeout time.Duration // exported, but type is unexported
}

// CORRECT — field visibility matches type visibility
type serverConfig struct {
    shutdownTimeout time.Duration
}
```

**When this matters most:** binary-only packages (`cmd/`) where the type is never
embedded or accessed via reflection by external code. Library packages
(`internal/`, exported packages) use exported fields on unexported types in two
legitimate scenarios: unexported embedding of an exported struct, or JSON/YAML
unmarshal using `encoding/json` or `gopkg.in/yaml.v3` (the decoder accesses
fields via reflection regardless of package visibility). For a standalone config
struct that is only ever accessed by code in the same package, use unexported
fields.

**Reference:** `backend/cmd/server/main.go` — `serverConfig.shutdownTimeout` uses
an unexported field because `serverConfig` itself is unexported and both live in
`package main`.

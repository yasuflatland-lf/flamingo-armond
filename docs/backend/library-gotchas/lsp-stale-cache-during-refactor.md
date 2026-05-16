# LSP stale-cache diagnostics during refactor — verify with `go build` before treating as real

> Part of the [Go library gotchas](../../../.claude/rules/go-library-gotchas.md) rules.

During a multi-file refactor — package rename, type extraction, type re-export — `gopls` (and the editor LSP layer that talks to it) may report errors against symbols that the actual Go toolchain compiles cleanly. Typical false positives: `X undefined` for a type that *does* exist in the workspace, "no field or method" against a method that was just moved to a different file in the same package, and import-cycle warnings against a file whose imports no longer close a cycle. The cache lags because gopls indexes incrementally and a write under one file may not propagate to dependent files until the editor triggers a re-analysis.

**The rule:** when a refactor produces LSP diagnostics whose subject is a symbol that the refactor touched, treat them as **unverified** until `go build ./...` and `go vet ./...` (run from `backend/`, with no caching shortcuts) reproduce the failure. If the toolchain compiles cleanly, the diagnostic is stale and the fix is to restart the LSP server or re-open the workspace, not to "fix" production code. The cost of trusting the LSP is real: hunting a phantom undefined-symbol error wastes time, and any "fix" that satisfies the LSP without satisfying the compiler will be reverted as soon as the cache catches up.

This applies most often during bulk type migrations where dozens of files reference a moved or re-exported type — every file whose import list changed is a candidate for a stale diagnostic.

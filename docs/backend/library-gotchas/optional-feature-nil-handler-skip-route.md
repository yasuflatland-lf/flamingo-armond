# Optional feature: pass `nil` handler and let the router skip route registration

> Part of the [Go library gotchas](../../../.claude/rules/go-library-gotchas.md) rules.

For a feature that should be disabled when its env config is incomplete (graceful skip rather than startup failure), keep the *router builder signature stable* and pass `nil` for the handler when the feature is disabled:

```go
// startup
var notionSyncHandler *notionsync.Handler
if !notionSyncDisabled {
    notionSyncHandler = notionsync.New(...)
}
e := newRouter(..., notionSyncHandler /* may be nil */, ...)

// router
if notionSyncHandler != nil {
    e.POST("/internal/notion-sync", notionSyncHandler.Handle)
}
```

The route is simply not registered when the handler is nil; an HTTP request to that path returns the framework's normal 404. No `if-disabled-then-stub-handler` indirection is needed.

**Why this shape:** the alternative — a stub handler that returns 404 — looks symmetric but adds a dispatcher entry, complicates middleware ordering, and obscures *why* the route is unavailable. Skipping registration entirely makes the disabled state structurally observable: a curl + grep against route mounts will not find the path at all.

**Why nil through the constructor (not a flag):** passing `notionSyncDisabled` as a separate boolean means the router builder has to encode the same disabled-vs-enabled logic twice (once to skip the route, once to maybe-call constructors with bad inputs). A nil handler collapses the decision to one branch and keeps every other call site unaware of the optionality.

**Reference:** `backend/cmd/server/main.go` constructs `notionSyncHandler` only when env is complete; `newRouter` registers `/internal/notion-sync` only when the handler is non-nil.

# Fire-and-forget goroutine: detach context from request lifecycle

> Part of the [Go library gotchas](../../../.claude/rules/go-library-gotchas.md) rules.

When launching a goroutine from inside an HTTP handler or GraphQL resolver for a side effect that must outlive the request, derive the goroutine's context from `context.Background()` with an explicit `WithTimeout` — never from the request context. The request context is cancelled the moment the handler returns; any in-flight network call inside the goroutine would fail with `context canceled` before it completes.

## Pattern

```go
// Capture closure-captured locals BEFORE go func() to avoid races
// with later mutation of the receiver's fields.
text   := card.Front + " " + card.Back
cardID := card.ID
pageID := u.notionPageID
writer := u.notionWriter

go func() {
    ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
    defer cancel()
    if err := writer.AppendParagraph(ctx, pageID, text); err != nil {
        slog.WarnContext(ctx, "card create: notion writeback failed",
            "card_id", cardID, "page_id", pageID, "err", err)
    }
}()
```

## Why capture before `go func()`

Copying the relevant fields into local variables before the goroutine starts prevents data races. If the receiver (`u`) is later mutated — by a test, a hot-reload, or concurrent use — the goroutine still holds the values that were current at launch time. The Go race detector flags closure captures of struct fields that are written after the goroutine starts; the local-copy idiom eliminates the race at the source.

## Timeout budget

The timeout must be short enough to avoid leaking goroutines under sustained request load, and long enough for the side-effect call to succeed under normal network conditions. 15 seconds is the budget used for the Notion write-back. Size the timeout for the specific downstream call; do not reuse this constant across unrelated goroutines.

## Panic policy

An unrecovered panic in a detached goroutine crashes the process. This is intentional — detached goroutines do not have a caller that can handle the panic, so crashing is the safest failure mode. Do not add a blanket `recover()` to the goroutine body. For background on the recover policy applied to third-party-generated code (parsers, etc.) where a blanket recover is used, see [`defer-recover-must-rethrow-runtime-error.md`](defer-recover-must-rethrow-runtime-error.md).

## Reference

The Notion card write-back in `backend/internal/usecase/card.go` (`CardUsecase.Create`) is the primary example. The design rationale is documented in [`docs/superpowers/specs/2026-05-12-notion-card-writeback-design.md`](../../superpowers/specs/2026-05-12-notion-card-writeback-design.md).

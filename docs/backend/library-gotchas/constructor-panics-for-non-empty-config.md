# Constructor panics are the right tool for "non-empty config requires non-nil deps"

> Part of the [Go library gotchas](../../../.claude/rules/go-library-gotchas.md) rules.

When a constructor accepts a feature-flag-shaped configuration plus the dependencies that are required *only* when the flag is non-empty, returning an error is awkward (every caller has to plumb an extra error through `run()`) and a silent half-configured struct is a per-request nil-deref hazard. Panic at construction is the right level of force: the misconfiguration is an operator-visible programming error, not a runtime input, and `run()` has not yet started the HTTP server when it fires — Echo's `Recover` middleware is not in the path, so the panic crashes the process at boot.

```go
func NewSuperUserPromoter(emails map[string]struct{}, adminRoleID string,
    checker adminChecker, assigner roleAssigner) *SuperUserPromoter {
    if len(emails) > 0 {
        if checker == nil      { panic("auth: ... checker must not be nil when emails is non-empty") }
        if assigner == nil     { panic("auth: ... assigner must not be nil when emails is non-empty") }
        if adminRoleID == ""   { panic("auth: ... adminRoleID must not be empty when emails is non-empty") }
    }
    // empty-emails path: nil deps are intentional; Middleware() returns a pass-through.
}
```

The pattern only applies to config-shaped constructors where one branch (here, the OFF branch) legitimately accepts zero values. For constructors whose contract is "always need these deps" (no OFF branch), panic is equally correct — a nil required dep is still an operator-visible wiring bug, not a runtime input, and the process should crash at boot before accepting requests. See `NewAdminGate` (`backend/internal/usecase/admin_gate.go`) and `NewLearnUsecase` (`backend/internal/usecase/learn.go`) for examples of unconditional nil-guards that panic.

## A dependency exercised by only some interface methods is still required

A subtler case sits between the two above: a constructor dependency that every caller supplies unconditionally, but that only *some* of the interface's methods exercise at runtime. The trap is to reason "method X never touches this dep, so a nil is harmless for an X-only caller" and leave it nil-tolerant. That makes a nil dep — always a wiring bug — fail lazily at the first call to a method that *does* use it (an unattributed nil-interface panic deep inside a request), instead of loudly at boot.

The deciding question is the same as the OFF-branch case: *is there a runtime branch where nil is the intended, correct configuration?* For a config-flag OFF branch the answer is yes (nil deps are deliberate; the feature is disabled). For a dep that is merely unused by some methods the answer is no — nil never means "configured correctly", only "this subset of methods will crash". Guard it at construction.

Worked example: `usecase.NewMasterCatalogUsecase(repo, copyUC, logger)` (`backend/internal/usecase/master_catalog.go`). `copyUC` (`CopyMasterToUserUsecase`) is exercised only by `ImportMaster`; the pre-existing `ListPublishedConnection` never touches it, and its tests legitimately pass a placeholder. An initial implementation panicked only on a nil `logger` and accepted nil `repo`/`copyUC` silently, deferring a nil-interface panic to the first `ImportMaster` call. Three independent review agents flagged it; the fix panics on all three deps at construction. Test code exercising only `ListPublishedConnection` now passes a non-nil placeholder stub rather than `nil`, which also self-documents that the dep is required.

## Asymmetric guards: panic only when the wire consequence is unrecoverable

When applying panic guards to typed-error constructors, the decision to panic must trace back to the wire-format consequence of the missing value, not to a uniform "non-empty / non-nil" policy.

`ucerr.NewValidationError(field, message string) *ValidationError` panics when `field == ""` because the struct is serialized to `extensions.field` in the GraphQL error response. An empty `field` value is unrenderable on the frontend — the client cannot attach the validation message to any input element. The missing value has no valid fallback at the consumer; panicking at construction forces the caller to supply a real field name.

`ucerr.NewForbiddenError(message string) *ForbiddenError` does **not** panic when `message == ""` because the frontend has fallback copy for a forbidden state. An empty message degrades gracefully; it is not unrenderable.

Both decisions trace to the same question: *can the downstream consumer recover from the zero value?* Panic when the answer is no; allow the zero value when the answer is yes.

See also [`docs/backend/error-wrapping/pointer-receiver-for-errors-as.md`](../error-wrapping/pointer-receiver-for-errors-as.md) for the pointer-receiver discipline required by `errors.As` on these same typed-error structs.

# Inject `AdminChecker` (bool) for admin-exempt business rules, not `AdminGate.Require`

> Part of the [Go library gotchas](../../../.claude/rules/go-library-gotchas.md) rules.

The backend has two distinct patterns for involving admin status in a usecase. Choosing
the wrong one produces either an overly restrictive gate or a missed exemption.

## The critical distinction: JWT database role vs. application role

The JWT carries a **Supabase/Postgres database role** claim (`authenticated`,
`anon`, `service_role`) that is distinct from the application-level
`admin` / `general` role stored in `public.user_roles`. `auth.AuthUser`
(`Sub`, `Email`, `EmailVerified`) no longer surfaces a `Role` field — it was
removed (issue #832) because it was never the application role: reading it to
decide whether the caller is an admin silently compared two unrelated
concepts and always returned false for admin users. The application role is
determined by `auth.Service.IsAdmin(ctx, userID)`, which performs a database
membership check against `domain.AdminRoleName`.

## The two patterns and when to use each

### Admin-required gate: `AdminGate.Require`

Use this when **only admins may call the operation at all** (typically admin mutations).
`AdminGate.Require` confirms the bearer is an admin and returns their user ID; a
non-admin receives `ForbiddenError("admin only")` and the operation is not attempted.
See [`error-classifier-helper-pass-through-with-caller-prefix.md`](../error-wrapping/error-classifier-helper-pass-through-with-caller-prefix.md)
for the wrap convention.

### Admin-exempt rule: inject `AdminChecker` directly

Use this when **everyone may call the operation, but admins bypass a restriction**.
Inject the `AdminChecker` interface as a usecase struct field and branch on `IsAdmin`
inside the business-logic path. `*auth.Service` satisfies `AdminChecker` in production;
tests substitute a stub. The constructor panics when the injected checker is nil —
required dependencies are enforced at construction time, not per-call.

```go
// AdminChecker is the surface both AdminGate and direct admin-exempt callers use.
// Declared in usecase/admin_gate.go; *auth.Service satisfies it.
type AdminChecker interface {
    IsAdmin(ctx context.Context, userID string) (bool, error)
}
```

`userUsecase` already follows this pattern: it holds an `auth AdminChecker` field
and calls `IsAdmin` to conditionally skip operations that only apply to non-admins.
`cardgroupUsecase` uses the same pattern for the per-user cardgroup creation cap.

## Worked example: per-user cardgroup creation cap

`domain.GeneralUserCardgroupLimit = 5` (`backend/internal/domain/cardgroup_quota.go`)
caps the number of cardgroups a non-admin owner may hold. Admins are exempt.

`cardgroupUsecase` holds an `admin AdminChecker` field. Its constructor panics when
the checker is nil:

```go
type cardgroupUsecase struct {
    repo   CardgroupRepository
    admin  AdminChecker
    tx     txRunner
    logger *slog.Logger
}

func NewCardgroupUsecase(db repository.Tx, repo CardgroupRepository, admin AdminChecker, logger *slog.Logger) CardgroupUsecase {
    if admin == nil {
        panic("usecase: cardgroup: admin checker is required")
    }
    // ...
    return &cardgroupUsecase{repo: repo, admin: admin, tx: newTxRunner(db), logger: logger}
}
```

The admin exemption is decided by the caller: `Create` calls `IsAdmin` itself and
admins skip the quota entirely (no lock, no count). For non-admins the quota lives in
the free function `lockCardgroupQuotaTx`, which runs inside the caller's transaction:
it takes the per-owner advisory lock (`AcquireUserCardgroupLockTx`), counts on the same
transaction via the narrow `cardgroupQuotaTxRepo` interface (`CountByOwnerTx`), and
returns a non-nil `*CardgroupLimitInfo` when `domain.GeneralUserCardgroupQuotaReached`
reports the cap. The insert then runs on that same transaction, so two concurrent
creates for one owner cannot both read a count below the limit:

```go
// cardgroupQuotaTxRepo is the narrow surface lockCardgroupQuotaTx needs.
type cardgroupQuotaTxRepo interface {
    AcquireUserCardgroupLockTx(ctx context.Context, tx repository.Tx, userID string) error
    CountByOwnerTx(ctx context.Context, tx repository.Tx, ownerID string) (int64, error)
}

func lockCardgroupQuotaTx(ctx context.Context, repo cardgroupQuotaTxRepo, tx repository.Tx, ownerID string) (*CardgroupLimitInfo, error) {
    if err := repo.AcquireUserCardgroupLockTx(ctx, tx, ownerID); err != nil {
        return nil, wrapInfraErr(err, "acquire owner cardgroup lock")
    }
    count, err := repo.CountByOwnerTx(ctx, tx, ownerID)
    if err != nil {
        return nil, wrapInfraErr(err, "count owner cardgroups")
    }
    if domain.GeneralUserCardgroupQuotaReached(count) {
        return &CardgroupLimitInfo{Limit: domain.GeneralUserCardgroupLimit, Current: int(count)}, nil
    }
    return nil, nil
}
```

The helper's wrap messages carry no layer prefix because it is shared between
`cardgroup.go` and `master_deck.go`; each caller adds its own prefix.

When `lockCardgroupQuotaTx` returns a non-nil `*CardgroupLimitInfo`, `Create` surfaces
the rejection via the `CardgroupLimitReachedError` GraphQL union variant (errors-as-data),
carrying `limit` and `current`. Existing users already over the cap are rejected for
new creates only — `domain.GeneralUserCardgroupQuotaReached` (`count >= limit`) rejects
at-or-above the limit without deleting existing cardgroups.

## Call ordering in `Create`

The `Create` method runs its checks in this order:

1. Auth guard (`user == nil` → UNAUTHENTICATED) — no DB.
2. Name validation (`domain.ParseCardgroupName`) — no DB.
3. `IsAdmin`.
4. Inside one transaction: non-admins run `AcquireUserCardgroupLockTx` → `CountByOwnerTx` → `CreateTx`; admins run `CreateTx` only.

`ImportMaster` (via `CopyMasterToUser` with `enforceQuota`) and `SeedForNewUser` take the
same per-owner advisory lock first in their own transactions, so every write that adds
cardgroups to an owner serializes on one key.

Name validation runs before the limit check because it requires no DB round-trips.
An invalid name fails immediately, avoiding the `IsAdmin`, lock and count queries
on the common invalid-input path.

## References

- [Result union "errors as data"](../error-wrapping/result-union-errors-as-data.md) — `CardgroupLimitReachedError` as the rejection union variant.
- [XOR-invariant outcome struct](xor-invariant-outcome-struct.md) — `CreateCardgroupOutcome` is a three-variant XOR (`Cardgroup` / `Validation` / `LimitReached`).
- [Consumer-defined narrow interface](consumer-defined-narrow-repo-interface.md) — `cardgroupQuotaTxRepo` is the helper-site narrow interface.
- [Pin unwrapped context error identity](../error-wrapping/pin-unwrapped-context-error-with-identity-check.md) — both infra calls in `lockCardgroupQuotaTx` (`AcquireUserCardgroupLockTx` and `CountByOwnerTx`) guard context errors via `wrapInfraErr` before wrapping.
- [AdminGate error classifier](../error-wrapping/error-classifier-helper-pass-through-with-caller-prefix.md) — the admin-required counterpart that returns `ForbiddenError` for non-admins.

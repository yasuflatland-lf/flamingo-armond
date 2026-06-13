# Inject `AdminChecker` (bool) for admin-exempt business rules, not `AdminGate.Require`

> Part of the [Go library gotchas](../../../.claude/rules/go-library-gotchas.md) rules.

The backend has two distinct patterns for involving admin status in a usecase. Choosing
the wrong one produces either an overly restrictive gate or a missed exemption.

## The critical distinction: JWT database role vs. application role

`auth.AuthUser.Role` carries the **Supabase/Postgres database role** set in the JWT
(`authenticated`, `anon`, `service_role`). It is NOT the application-level
`admin` / `general` role stored in `public.user_roles`. The application role is
determined by `auth.Service.IsAdmin(ctx, userID)`, which performs a database membership
check against `domain.AdminRoleName`. Reading `AuthUser.Role` to decide whether the
caller is an admin silently compares two unrelated concepts and always returns false
for admin users.

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

`const generalUserCardgroupLimit = 5` (package-private in `backend/internal/usecase/`)
caps the number of cardgroups a non-admin owner may create. Admins are exempt.

`cardgroupUsecase` holds an `admin AdminChecker` field. Its constructor panics when
the checker is nil:

```go
type cardgroupUsecase struct {
    repo   CardgroupRepository
    admin  AdminChecker
    logger *slog.Logger
}

func NewCardgroupUsecase(repo CardgroupRepository, admin AdminChecker, logger *slog.Logger) CardgroupUsecase {
    if admin == nil {
        panic("usecase: cardgroup: admin checker is required")
    }
    // ...
}
```

The limit check lives in the free function `checkCardgroupLimit`. It calls `IsAdmin`
first; admins return `(nil, nil)` immediately without a count query. For non-admins
it counts via the narrow `cardgroupOwnerCounter` interface and returns a non-nil
`*CardgroupLimitInfo` when `count >= generalUserCardgroupLimit`:

```go
// cardgroupOwnerCounter is the narrow surface checkCardgroupLimit needs.
type cardgroupOwnerCounter interface {
    CountByOwner(ctx context.Context, ownerID string, search *string) (int64, error)
}

func checkCardgroupLimit(
    ctx context.Context,
    counter cardgroupOwnerCounter,
    admin AdminChecker,
    ownerID string,
) (*CardgroupLimitInfo, error) {
    isAdmin, err := admin.IsAdmin(ctx, ownerID)
    if err != nil {
        if isContextDone(err) {
            return nil, err
        }
        return nil, eris.Wrap(err, "usecase: cardgroup: check admin")
    }
    if isAdmin {
        return nil, nil // admin exempt — no count query
    }
    count, err := counter.CountByOwner(ctx, ownerID, nil)
    if err != nil {
        if isContextDone(err) {
            return nil, err
        }
        return nil, eris.Wrap(err, "usecase: cardgroup: count by owner")
    }
    if count >= generalUserCardgroupLimit {
        return &CardgroupLimitInfo{Limit: generalUserCardgroupLimit, Current: int(count)}, nil
    }
    return nil, nil
}
```

When `checkCardgroupLimit` returns a non-nil `*CardgroupLimitInfo`, `Create` surfaces
the rejection via the `CardgroupLimitReachedError` GraphQL union variant (errors-as-data),
carrying `limit` and `current`. Existing users already over the cap are rejected for
new creates only — the `>=` comparison rejects at-or-above the limit without deleting
existing cardgroups.

## Call ordering in `Create`

The `Create` method runs its checks in this order:

1. Auth guard (`user == nil` → UNAUTHENTICATED) — no DB.
2. Name validation (`domain.ParseCardgroupName`) — no DB.
3. Limit check (`checkCardgroupLimit` → `IsAdmin` + optionally `CountByOwner`).
4. ID generation and insert.

Name validation runs before the limit check because it requires no DB round-trips.
An invalid name fails immediately, avoiding the `IsAdmin` and `CountByOwner` queries
on the common invalid-input path.

## References

- [Result union "errors as data"](../error-wrapping/result-union-errors-as-data.md) — `CardgroupLimitReachedError` as the rejection union variant.
- [XOR-invariant outcome struct](xor-invariant-outcome-struct.md) — `CreateCardgroupOutcome` is a three-variant XOR (`Cardgroup` / `Validation` / `LimitReached`).
- [Consumer-defined narrow interface](consumer-defined-narrow-repo-interface.md) — `cardgroupOwnerCounter` is the helper-site narrow interface.
- [Pin unwrapped context error identity](../error-wrapping/pin-unwrapped-context-error-with-identity-check.md) — both infra calls in `checkCardgroupLimit` (`IsAdmin` and `CountByOwner`) guard context errors via `isContextDone` before wrapping.
- [AdminGate error classifier](../error-wrapping/error-classifier-helper-pass-through-with-caller-prefix.md) — the admin-required counterpart that returns `ForbiddenError` for non-admins.

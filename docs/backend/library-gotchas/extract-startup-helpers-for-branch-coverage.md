# Extract startup helpers to make branch coverage testable without a live server

> Part of the [Go library gotchas](../../../.claude/rules/go-library-gotchas.md) rules.

When `run()` or `main()` contains branching logic (e.g. "if `SUPER_USER_EMAILS`
is set, build a promoter; otherwise build a no-op"), testing each branch requires
standing up the full HTTP server unless the logic lives in a separate helper. A
thin helper that accepts its external dependencies as parameters can be exercised
with stub repos without starting any network listener.

```go
// bootstrapSuperUserPromoter returns a configured SuperUserPromoter or an
// error. It is a standalone function so tests can inject stub dependencies.
func bootstrapSuperUserPromoter(
    ctx context.Context,
    logger *slog.Logger,
    authSvc authService,
    roleRepo repository.RoleRepository,
    emailsEnv string,
) (*auth.SuperUserPromoter, error) { ... }
```

**Why:** inlining the branch in `run()` means every test of that branch must
start the real Echo server and real DB client. The startup cost is high, the test
is slow, and flaky network conditions can make coverage non-deterministic. A
helper with injected deps turns five branches into five fast, deterministic unit
tests.

**How to apply:** when a startup function acquires a concrete dependency (DB pool,
HTTP client, config value) and then branches on it, split the acquisition step
from the branching step. Pass the already-acquired dep into the helper so the test
can substitute a stub. Reference: `backend/cmd/server/main.go`
`bootstrapSuperUserPromoter` (5 branches × stub-driven unit test).

# Testable startup helpers

> Part of the [Go library gotchas](../../../.claude/rules/go-library-gotchas.md) rules.

Startup code that inlines its branching logic forces tests to either stand up a live server or re-implement the branch inside the test body. The first is slow and flaky; the second is an anti-pattern because the test asserts its own copy of the logic, not the production path. The fix is to extract a thin startup helper that accepts its external dependencies as parameters, so each branch can be exercised by a fast, deterministic unit test that calls the real production code.

## Anti-pattern: inline copy of production logic in tests

A test that re-implements a branch from the code under test — e.g. an
`if len(emails) > 0 { logger.Warn(...) }` block inside the test body — asserts
its own copy of the logic, not the production path. When the production code
is refactored away from that branch, the test still matches its own copy and
CI stays green. The regression ships.

**How to apply:** the tipping point for a refactor is "can the test call the
production helper directly instead of re-implementing it?" If yes, expose the
helper (or extract it), pass stubs in, and assert on the helper's actual
output. If the logic is truly untestable at the unit level without a live
server, extract a helper first (see the next section). The
`bootstrapSuperUserPromoter` extraction replaced inline test copies with direct
calls that exercise real production code paths.

## Pattern: extract startup helpers for branch coverage

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

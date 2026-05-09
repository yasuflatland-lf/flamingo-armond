# Inline copy of production logic in tests is an anti-pattern

> Part of the [Go library gotchas](../../../.claude/rules/go-library-gotchas.md) rules.

A test that re-implements a branch from the code under test — e.g. an
`if len(emails) > 0 { logger.Warn(...) }` block inside the test body — will stay
green even after the production code is refactored away from that branch. The test
is asserting its own copy of the logic, not the production path, so the two can
diverge silently.

**Why:** the test's "copy" and the production code are two independent sources of
truth. When the production code changes, the test still matches its own copy and
CI stays green. The regression ships.

**How to apply:** the tipping point for a refactor is "can the test call the
production helper directly instead of re-implementing it?" If yes, refactor: expose
the helper (or extract it), pass stubs in, and assert on the helper's actual
output. If the logic is truly untestable at the unit level without a live server,
that is a signal to extract a helper first (see [Extract startup helpers to make branch coverage testable without a live server](extract-startup-helpers-for-branch-coverage.md)).
The `bootstrapSuperUserPromoter` extraction replaced inline test copies with direct
calls that exercise real production code paths.

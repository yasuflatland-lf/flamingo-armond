# Drop `optimisticResponse` for mutations that can fail with typed GraphQL errors

> Part of the [pagination](../../.claude/rules/pagination.md) rules. Cross-referenced by `docs/backend.md` and `docs/frontend.md`.

`@apollo/client` v3.x rolls back optimistic writes on **network** errors but not consistently on typed GraphQL errors (`FORBIDDEN`, `BAD_USER_INPUT`, etc.). For a mutation that can plausibly return one of those — e.g. a role assignment that fails authorization, or a self-demotion blocked by a server-side guard — the optimistic write persists and the cache lies until the next mount. Two acceptable postures:

1. **Drop optimistic** for mutations that can fail typed. The user pays a single round-trip of latency, but the cache stays truthful.
2. **Manual rollback in the catch branch** — `cache.evict({ id: ... })` followed by `cache.gc()` to undo whatever the optimistic update wrote.

Posture 1 is the default; reach for posture 2 only when the perceived latency cost is measurable. Never leave a typed-error-capable mutation with `optimisticResponse` and no rollback.

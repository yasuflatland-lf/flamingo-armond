# Backward pagination via direction-flip + reverse

> Part of the [pagination](../../.claude/rules/pagination.md) rules. Cross-referenced by `docs/backend.md` and `docs/frontend.md`.

Natural SQL "give me N rows before X" is awkward. The repository inverts the `ORDER BY` direction, applies `LIMIT N+1`, then reverses the returned slice in memory. The usecase mirrors the trim logic on the leading edge so the page boundary stays at the tail.

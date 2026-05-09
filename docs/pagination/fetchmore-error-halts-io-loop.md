# `fetchMoreError != null` halts the IO loop

> Part of the [pagination](../../.claude/rules/pagination.md) rules. Cross-referenced by `docs/backend.md` and `docs/frontend.md`.

Without an error halt gate, the IO keeps firing on the same failed cursor and loops invisibly with only `console.error` noise. Set state to a banner string in the `fetchMore` `.catch`, render a Retry button that clears the state and re-invokes the request, and short-circuit the IO `useEffect` while the error is set.

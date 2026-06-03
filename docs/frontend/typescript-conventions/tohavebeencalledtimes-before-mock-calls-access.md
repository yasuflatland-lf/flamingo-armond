# `toHaveBeenCalledTimes(N)` before indexing into `mock.calls[N]`

> Part of the [frontend TypeScript conventions](../typescript-conventions.md). See the index for related chapters.

Tests that inspect a specific argument of a recorded call routinely reach into `spy.mock.calls[N]`:

```ts
expect(consoleWarnSpy).toHaveBeenCalledWith(
  "[layout] getClaims() failed",
  expect.objectContaining({ user_id: userId }),
);
// PII absence — keys check on the actual payload.
const warnPayload = consoleWarnSpy.mock.calls[0][1] as Record<string, unknown>;
expect(Object.keys(warnPayload)).not.toContain("email");
```

`mock.calls[0]` silently returns the **first** recorded call without bounding the number of calls. A regression that fires the warn twice — e.g. a refactor that runs the branch once during render and once during an effect, or a guard that no longer short-circuits — leaves the first call unchanged and adds a second call. The `toHaveBeenCalledWith` assertion is satisfied by either call, and the PII-keys assertion runs against the intended first call, so the regression slips through.

Add a `toHaveBeenCalledTimes(N)` assertion immediately before any `mock.calls[N]` access:

```ts
expect(consoleWarnSpy).toHaveBeenCalledTimes(1);
const warnPayload = consoleWarnSpy.mock.calls[0][1] as Record<string, unknown>;
expect(Object.keys(warnPayload)).not.toContain("email");
expect(Object.keys(warnPayload)).not.toContain("display_name");
```

The same rule applies to any spy indexed by position: `mock.results[0]`, `mock.instances[0]`, `mock.lastCall`. `lastCall` is special — it returns the **last** call rather than the first, so a regression that fires once before the intended call also slips through unless `toHaveBeenCalledTimes` bounds the count.

## How this composes with the discriminating-key rule

The discriminating-key rule (see [`expect-objectcontaining-message-is-not-enough.md`](expect-objectcontaining-message-is-not-enough.md)) guards against a bare `Error` regression by adding a non-`Error.prototype` key to `expect.objectContaining(...)`. `toHaveBeenCalledTimes` is the count-bound complement: the discriminating key says "the right shape was logged at least once"; `toHaveBeenCalledTimes` says "and nothing else was logged in this branch". Apply both on warn / error assertions that carry PII-redaction or structured-shape guarantees.

Reference: `frontend/src/components/nav/logo-drawer.test.tsx` (multiple event-listener assertions that call `toHaveBeenCalledTimes(1)` before accessing `listener.mock.calls[0]` to inspect the event payload).

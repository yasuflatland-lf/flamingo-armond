# One-shot mount-effect mutation guard via `useRef<string | null>`

> Part of the [pagination](../../.claude/rules/pagination.md) rules. Cross-referenced by `docs/backend.md` and `docs/frontend.md`.

The same async-state hazard appears whenever a client component fires a mutation **once per discriminator value** from a `useEffect`. React 18 Strict Mode double-mounts dev-time, and any future re-render that re-runs the effect re-fires the mutation — a `useState` "did we send it yet" flag updates async and both passes read the previous value. The fix is the same shape as the IO guard, but the ref holds the **discriminating identity** the mutation was last dispatched for, not a boolean:

```ts
const lastDispatchedRef = useRef<string | null>(null);
useEffect(() => {
  if (serverCurrentValue === incomingValue) return;     // already in sync
  if (lastDispatchedRef.current === incomingValue) return; // already dispatched this run
  lastDispatchedRef.current = incomingValue;
  client.mutate({ mutation: PersistMutation, variables: { incomingValue }, update: ... })
    .catch((err) => console.warn("[scope] persist failed", { incomingValue, err }));
}, [incomingValue, serverCurrentValue, client]);
```

Two non-obvious points: (1) the SSR-seeded "current server value" lets the effect skip the network entirely when nothing changed — keep that prop, do not collapse the check to "always fire once"; (2) the mutation should run via the imperative `client.mutate(...)` (not `useMutation`) so the cache update runs regardless of caller render state and the effect's dependency surface stays narrow. Reference: `frontend/src/app/learn/[cardgroupId]/learn-client.tsx` persisting `lastViewedCardgroup`.

The [drop `optimisticResponse` rule](drop-optimistic-response-typed-errors.md) applies here too: a mutation that can plausibly return `BAD_USER_INPUT` (e.g. cardgroup deleted between page render and the effect firing) should not carry an optimistic write.

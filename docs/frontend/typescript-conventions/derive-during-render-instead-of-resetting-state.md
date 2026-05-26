# Derive during render instead of resetting state via a `useEffect` keyed on the trigger

> Part of [`docs/frontend/typescript-conventions.md`](../typescript-conventions.md). See the index for related rules.

When a piece of derived state depends on a snapshot of an input that may drift (e.g. "the user has not edited the payload since the last validation"), the obvious shape is a `useEffect([input])` that nulls every dependent piece of state when the input changes. The cleaner shape is to capture the snapshot the input had when the derivation last ran, and compare it to the current input during render:

```tsx
const [validationResult, setValidationResult] = useState<ValidationResult | null>(null);
// validatedPayload tracks the payloadText value that was in effect when the last
// successful validate call completed. canImport checks this against the current
// payloadText to prevent importing a stale/edited payload without re-validating.
const [validatedPayload, setValidatedPayload] = useState<string | null>(null);

async function handleValidate() {
  /* ... */
  setValidationResult(result.data.validateCardImport);
  setValidatedPayload(payloadText); // record snapshot at validation time
}

// Derived during render — no resetting useEffect needed.
const canImport =
  validationResult?.valid === true &&
  validationResult.parsedCards.length > 0 &&
  !!cardgroupId &&
  validatedPayload === payloadText; // becomes false the moment the user edits
```

**Why:** the resetting-effect shape requires three things to land together — (a) the effect itself, (b) a `biome-ignore lint/correctness/useExhaustiveDependencies` comment to acknowledge that the dep array drives a side effect rather than a synchronization, and (c) operator-mental-model debt because a reader has to trace the effect to understand when each piece of state goes back to `null`. The derive-during-render shape collapses all three into a single equality check. The double-render the effect produces (render with stale state → effect fires → re-render with nulled state) is also gone — the comparison runs in the same render the input changed.

**How to apply:** when introducing derived state that should "invalidate when X changes", first ask "can I capture a snapshot of X at the moment the derivation was last valid, and compare during render?" If yes, store the snapshot in a `useState<typeof X | null>` alongside the derived value, set the snapshot in the same handler that produced the derived value, and read the snapshot during render via `snapshot === currentX`. Reach for the resetting `useEffect` only when the invalidation also has to fire a side effect that cannot be expressed as a render-time comparison (a network round-trip, a DOM measurement). This rule pairs with § "Audit collapsed helpers for branches that lose all side effects" above: both push more work into the synchronous render path and out of post-render effects.

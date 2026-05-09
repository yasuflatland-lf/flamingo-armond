# Pair every `try { ... } finally { setLoading(false) }` with a `catch` for transport rejections

> Part of the [frontend RSC error handling](../../../.claude/rules/frontend-rsc-error-handling.md) rules.

Client components that toggle a loading flag around a third-party SDK call typically write `try { await sdk.doThing(); ... } finally { setLoading(false); }`. The `finally` releases the UI lock so the button re-enables, but it does **not** observe the rejection. The Supabase JS pattern is the canonical example: `await supabase.auth.updateUser({ ... })` returns API-level errors via the resolved `{ error }` object, but **transport-level** failures (network unreachable, DNS failure, request timeout) come out as a rejected promise. Without a `catch`, the rejection escapes the React event handler — no banner renders, no log fires, the user just sees the form return to the idle state with no feedback.

```tsx
// AVOID: API errors handled, transport rejections silently lost.
try {
  setLoading(true);
  const { error } = await supabase.auth.updateUser({ email });
  if (error) { setError(classify(error.message)); return; }
  setSuccess(true);
} finally {
  setLoading(false);
}

// PREFER: catch the rejected promise, log the error name only, surface a generic banner.
try {
  setLoading(true);
  const { error } = await supabase.auth.updateUser({ email });
  if (error) { setError(classify(error.message)); return; }
  setSuccess(true);
} catch (err) {
  // Transport-level failure (network, timeout). The API-shaped failure goes via { error } above.
  // Do NOT log err.message — SDK exception messages can echo user-typed input (the email here).
  console.warn("[change-email] updateUser threw:", err instanceof Error ? err.name : "unknown");
  setError("Network error. Please check your connection and try again.");
} finally {
  setLoading(false);
}
```

**Why:** the two failure modes (API-shaped `{ error }` resolve and rejected promise) reach the call site through different channels. A try/finally without a catch handles only the resolve channel; the reject channel surfaces as an unhandled-promise warning at most, with no UI signal. The user is stuck — the form looks idle, the action did nothing, and nothing tells them why.

**How to apply:** every async event handler that wraps a third-party SDK call in `try/finally` for a UI lock release MUST have a paired `catch`. The catch logs `err.name` only (not `err.message` — SDK exceptions can echo user input, including the very value the user typed into the form), and sets a generic user-facing banner. Test the rejection path with `mockRejectedValue(Object.assign(new Error("network down"), { name: "FetchError" }))` and assert both the banner copy AND the structural log call (`expect(consoleWarnSpy).toHaveBeenCalledWith("[scope] action threw:", "FetchError")`) — the structural assertion is what guarantees `err.message` is not in the log payload, per [`docs/frontend/typescript-conventions.md` § "`expect.objectContaining({ message })` is not enough — add a discriminating key"](../typescript-conventions/expect-objectcontaining-message-is-not-enough.md). Reference: `frontend/src/app/profile/change-email/change-email-client.tsx` `handleSubmit` (the `catch (err)` arm and test S7 in the sibling test file).

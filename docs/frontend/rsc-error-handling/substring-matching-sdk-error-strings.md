# Substring-matching SDK error strings: pair mapped copy with raw-message warn for unmapped paths

> Part of the [frontend RSC error handling](../../../.claude/rules/frontend-rsc-error-handling.md) rules.

When mapping a third-party SDK's error.message strings to user-facing copy via `.includes(...)` (e.g. classifying Supabase's `"Email rate limit exceeded"` to `"Too many requests..."`), three risks compose:

1. **Echo risk** — an unmapped error falls through to the UI and exposes the raw upstream string, leaking error names, request IDs, or technical wording the user has no context for.
2. **Operator-blind risk** — if the unmapped path silently shows a generic banner, operators get no signal to extend the classifier when a new upstream message starts firing.
3. **PII risk** — the raw message goes into a `console.warn`. Some SDKs (e.g. browser `fetch` exceptions, third-party validators) echo user-typed input verbatim into the message, so blindly logging `err.message` re-leaks the input.

The compliant shape is a classifier returning `string | null` (mapped copy or `null` to mean "unmapped") plus a two-branch call site:

```ts
function classifySupabaseError(message: string): string | null {
  const lower = message.toLowerCase();
  if (lower.includes("rate limit")) return "Too many requests. Please wait a moment and try again.";
  if (lower.includes("already registered")) return "That email address is already in use.";
  return null;
}

const classified = classifySupabaseError(err.message);
if (classified !== null) {
  // Classified: operators know what happened from the user copy + err.name; no raw needed.
  console.warn("[scope] updateUser failed:", err.name);
  setError(classified);
} else {
  // Unmapped: log the raw upstream message so operators can extend classifySupabaseError.
  // PII gate: this is safe ONLY when the upstream's API error messages are server-generated
  // and do not echo user-typed input. Verify per SDK before applying.
  console.warn("[scope] updateUser failed (unmapped):", err.name, err.message);
  setError("Could not send confirmation link. Please try again.");
}
```

The `string | null` shape collapses indirection at the call site — a wrapper return type like `{ userMessage, classified }` was tried and rejected during review because every caller had to read both fields, and the `userMessage` slot duplicated the generic-fallback copy already living at the call site. Returning `null` lets the call site own the generic copy and the unmapped log together.

**Why:** the unmapped path is what catches new upstream message variants. Without a raw-message log there, the classifier silently falls behind every SDK update — the user keeps seeing the generic copy, and operators have no telemetry to know which upstream message was the one that needs a new mapping. Conversely, logging the raw message on the **mapped** path is redundant and adds log noise — the mapped copy + `err.name` are enough for operator triage.

**How to apply:** any classifier that turns SDK strings into user-facing copy returns `string | null`, and the unmapped branch emits a 3-arg `console.warn("[scope] action failed (unmapped):", err.name, err.message)`. Document the PII gate explicitly in a code comment ("Supabase API error messages are server-generated and do not echo user-typed input, so this is PII-safe"). Pair with two tests: one that asserts the mapped path logs `(prefix, err.name)` only (2-arg shape), and one that asserts the unmapped path logs `(prefix, err.name, err.message)` (3-arg shape). The structural-call assertions guarantee no future contributor adds `err.message` to the mapped path's warn (which would re-introduce the leak this rule prevents). Reference: `frontend/src/app/profile/change-email/change-email-client.tsx` (`classifyUpdateUserError` returning `string | null`) and the S3/S5/S6 test cases in the sibling test file.

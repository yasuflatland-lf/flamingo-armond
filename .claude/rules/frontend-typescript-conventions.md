# Frontend TypeScript conventions

> Applies to: `frontend/src/**/*.{ts,tsx}`. Cross-cutting type-design rules that affect correctness, security, or testability and are non-obvious from the TypeScript docs alone.

## Required `string | null` over optional `?: string | null` for security-relevant or caller-deliberate props

`prop?: string | null` and `prop: string | null` are not interchangeable. The optional form (`?`) collapses three distinct caller states into two observable outcomes — callers may omit the prop entirely, which is indistinguishable at runtime from an explicit `null` and means the type system does not force the caller to acknowledge the prop's existence. When a prop has security implications (e.g. a redirect destination, a sanitized user-supplied value) or when the calling component must make an explicit choice (pass a value or acknowledge absence), use the required form:

```ts
// AVOID: callers can omit entirely; the prop's existence is unacknowledged.
interface Props { returnTo?: string | null; }

// PREFER: callers must pass something, even if it is null.
interface Props { returnTo: string | null; }
```

The required form surfaces callers that forgot to wire the prop (compile error: "returnTo is missing") rather than silently defaulting to `undefined`. Reference: `frontend/src/app/cardgroups/new/new-cardgroup-client.tsx` (`returnTo: string | null`) after a review finding that the optional form allowed callers to skip the prop and lose the sanitized redirect value without any error.

## JSDoc as the enforcer of "pre-sanitized" invariants when branded types are not used

When a function or component accepts a value that must have crossed a security boundary before being passed (e.g. "this path has been validated as an internal path"), and the project style does not use branded/nominal types, a load-bearing JSDoc comment is the only compile-time signal available. The comment must:

1. State what the caller is responsible for (e.g. "must be a pre-sanitized internal path").
2. State what the receiving side does defensively (e.g. "the receiving page also calls `sanitizeReturnTo`").
3. State what callers must NOT pass (e.g. "do not pass arbitrary user input here").

```ts
/**
 * Path to return to after creating a new cardgroup. Must be a **pre-sanitized
 * internal path** (e.g. `/cards/new`). The receiving page applies
 * `sanitizeReturnTo` defensively, but callers are responsible for not passing
 * arbitrary user input here.
 */
createReturnTo: string;
```

The JSDoc documents a two-layer defence: the caller sanitizes before passing, the receiver sanitizes again on arrival. Both layers are intentional — the "defensive" layer in the receiver is the last-resort guard against a future caller that skips pre-sanitization. Reference: `frontend/src/components/cardgroups/cardgroup-picker-sheet.tsx` (`createReturnTo` prop).

If the project style evolves to allow branded types, replace the JSDoc with a nominal type (e.g. `type InternalPath = string & { readonly __brand: "InternalPath" }`) and a constructor function that calls `sanitizeReturnTo`. Until then, treat the JSDoc as load-bearing — do not remove it during refactoring without adding the branded type.

## `expect.objectContaining({ message })` is not enough — add a discriminating key

`Error.prototype.message` is an own (though non-enumerable) property on every `Error` instance. Vitest's `expect.objectContaining` uses `hasOwnProperty` for key checks. Therefore, asserting `expect.objectContaining({ message: expect.any(String) })` against a `console.error` second argument will pass whether the argument is the intended structured object `{ message, err }` OR a bare `Error` regression. The test is tautologically green and the regression ships silently.

The fix is to include a discriminating own-property key that exists in the structured object but NOT on `Error` instances — e.g. `err: expect.anything()`.

```ts
// AVOID — passes for both `{ message: "...", err }` AND a bare Error regression.
expect(consoleSpy).toHaveBeenCalledWith(
  "[scope] msg",
  expect.objectContaining({ message: expect.any(String) }),
);

// PREFER — the `err` key is absent on a bare Error instance, so a regression fails.
expect(consoleSpy).toHaveBeenCalledWith(
  "[scope] msg",
  expect.objectContaining({
    message: expect.any(String),
    err: expect.anything(),
  }),
);
```

**Why:** structured log arguments (`{ message, err }`) are deliberately different from a raw `Error`. The assertion must verify the structure matches what was intentionally logged, not just that some string-ish property exists.

**How to apply:** whenever a `console.error` / `console.warn` call passes a structured object as the second argument, pair the `message` matcher with at least one additional key that distinguishes the object from a bare `Error`. Reference: `frontend/src/app/cards/new/cards-new-client.test.tsx` `handleCreate` log assertion.

## TanStack Form `_handleSubmit` re-throws — chain `.catch()` on `form.handleSubmit()`

`@tanstack/form-core` v1.x's `_handleSubmit` wraps the user-supplied `onSubmit` in a `try { ... } catch (err) { ...done(); throw err; }` block. The re-throw is necessary for `formState.isSubmitSuccessful` to remain `false` when the submission fails. The typical JSX call shape `void form.handleSubmit()` discards the resulting rejected promise and produces a browser "Uncaught (in promise)" warning.

Chain a no-op `.catch()` at the JSX call site with an explanatory comment:

```tsx
<form onSubmit={(e) => {
  e.preventDefault();
  e.stopPropagation();
  form.handleSubmit().catch(() => {
    // The inner submit handler's .catch already logged; swallow here so the
    // re-thrown rejection (which keeps formState.isSubmitSuccessful=false correct)
    // does not surface as an unhandled browser promise rejection.
  });
}}>
```

**Why:** only the user's `onSubmit` callback's rejection propagates through `_handleSubmit` — validation failures hit `return`, not `throw`. The inner `.catch` in `onSubmit` already logs the failure, so the outer `.catch` is a deliberate, documented swallow.

**How to apply:** replace every `void form.handleSubmit()` call site with this pattern. The comment is load-bearing documentation — do not omit it. This rule pairs with the re-throw rule below; they must land together. Reference: `frontend/src/components/cardgroups/card-form.tsx`.

## Re-throw inside TanStack Form `useForm.onSubmit` to keep `isSubmitSuccessful` correct

When a parent passes a `submit` callback to a form component, swallowing a rejection inside `useForm.onSubmit` without re-throwing collapses two error states: TanStack Form sees the `onSubmit` as resolved-success and updates `isSubmitSuccessful = true`, even though the underlying mutation failed. Re-throw after logging:

```ts
onSubmit: async ({ value }) => {
  await submit(value).catch((err) => {
    console.error("[scope] submit rejected", err);
    throw err; // keep formState.isSubmitSuccessful correct
  });
},
```

**Why:** `_handleSubmit` gates `formState.isSubmitSuccessful` on whether `onSubmit` resolves or rejects. A swallowed rejection makes the form believe the submission succeeded, which can unblock navigation, clear state, or show a success banner while the mutation actually failed.

**How to apply:** every `useForm.onSubmit` that calls an external `submit` prop must re-throw after the catch/log. The re-throw is forward-safe even when the current caller wraps `submit` in its own `try/catch` — the rejection does not bubble past that boundary today. This rule pairs with the `.catch()` rule above; they must land together. Reference: `frontend/src/components/cardgroups/card-form.tsx`.

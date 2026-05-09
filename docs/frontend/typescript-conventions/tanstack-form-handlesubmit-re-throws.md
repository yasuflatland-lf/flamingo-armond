# TanStack Form `_handleSubmit` re-throws — chain `.catch()` on `form.handleSubmit()`

> Part of [`docs/frontend/typescript-conventions.md`](../typescript-conventions.md). See the index for related rules.

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

# Re-throw inside TanStack Form `useForm.onSubmit` to keep `isSubmitSuccessful` correct

> Part of [`.claude/rules/frontend-typescript-conventions.md`](../frontend-typescript-conventions.md). See the index for related rules.

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

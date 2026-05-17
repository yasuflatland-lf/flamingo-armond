# Do not reuse one mutation's Apollo-managed error state for a sibling mutation's failure surface

> Part of the [pagination](../../.claude/rules/pagination.md) rules. Cross-referenced by `docs/backend.md` and `frontend/CLAUDE.md`.

`useMutation` returns a managed `error` field that clears automatically on the next call to the same mutation. If a component runs two independent mutations (e.g. `createCard` and `updateCard`), using `createError` from `useMutation(CreateCardMutation)` to display a failure message from `updateCard` causes the banner to vanish the moment the user retries `createCard` — even before the user dismisses the error. The clearing is silent; the user sees the banner disappear with no explanation.

Use a sibling `useState<string | null>` for any error that belongs to a second mutation (or any flow not managed by the primary hook):

```ts
// correct: each mutation gets its own error surface
const [createCard, { error: createError }] = useMutation(CreateCardMutation);
const [updateCard] = useMutation(UpdateCardMutation);
const [overwriteError, setOverwriteError] = useState<string | null>(null);

// in the overwrite handler:
updateCard({ ... }).catch((err) => {
  setOverwriteError(getBackendErrorBanner(err) ?? "Overwrite failed");
});
```

The `useState` error lives until the user explicitly dismisses it or the component unmounts — it does not clear on unrelated mutation interactions. Reference: `frontend/src/app/cards/new/cards-new-client.tsx` (`overwriteError` state alongside `createError`).

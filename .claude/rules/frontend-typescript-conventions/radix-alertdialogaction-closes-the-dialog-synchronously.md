# Radix `AlertDialogAction` closes the dialog synchronously — call `e.preventDefault()` to keep it open on failure

> Part of [`.claude/rules/frontend-typescript-conventions.md`](../frontend-typescript-conventions.md). See the index for related rules.

Radix UI's `AlertDialogAction` calls `onOpenChange(false)` synchronously as soon as its `onClick` handler resolves, regardless of whether the action succeeded or failed. For a confirm action that may fail and must keep the dialog mounted (e.g. an overwrite mutation that returns a typed `BAD_USER_INPUT`), this means a failed action still closes the dialog and the user loses any inline error message.

The fix: call `e.preventDefault()` at the start of the `onClick` handler and let the consuming component decide when to close the dialog based on the outcome of the operation.

```tsx
<AlertDialogAction
  onClick={async (e) => {
    e.preventDefault(); // prevent Radix from closing on click — we close on success only
    try {
      await onConfirm();
      // caller closes the dialog (e.g. setDuplicate(null)) on success
    } catch {
      // dialog stays open; surface error inline
    }
  }}
>
  Confirm
</AlertDialogAction>
```

The cancel path uses the standard `AlertDialogCancel` component, which closes correctly without `preventDefault`. The cancel path is the catch-all close path for any non-action close (backdrop click, Escape key, explicit cancel).

**How to apply:** any `AlertDialogAction` whose `onClick` fires an async operation that can fail and must keep the dialog mounted MUST call `e.preventDefault()` at the top of the handler. Reference: `frontend/src/app/cards/new/cards-new-client.tsx` `DuplicateOverwriteDialog`.

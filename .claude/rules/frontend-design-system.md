---
paths:
  - "frontend/**"
---

# Frontend design system

> Applies to: `frontend/src/**/*.tsx` (UI components, dialogs). Cross-cutting because the
> color-intent rules below govern every action button and confirm dialog. Source of truth for
> the token table and rationale: [`docs/frontend/design-system.md`](../../docs/frontend/design-system.md).

The app is **light-only**; the flamingo coral brand and a deep-red danger color are both present.
The two rules below keep them from colliding and keep destructive actions unmistakable.

## Red is reserved for danger

`destructive` (deep red) means "stop / irreversible". `brand` (flamingo coral) is the primary
constructive CTA color. They are different tokens doing different jobs:

- **Never style a constructive action** (Save, Create, Login, Import, Publish) with `destructive`.
  A constructive primary action is `variant="brand"`.
- **Never leave a destructive action in a neutral variant.** A delete / overwrite / discard action
  is `variant="destructive"` (or the dialog form below).

Keeping both reds is deliberate — they are token-distinguishable (deep red L≈58 vs. coral L≈74)
and never render on the same surface, so the brand-red CTAs are kept, not flattened to neutral.
Full rationale: [`docs/frontend/design-system.md` § "Red is reserved for danger"](../../docs/frontend/design-system.md#red-is-reserved-for-danger).

## Destructive / data-loss confirm dialogs MUST pass `variant="destructive"`

shadcn's [`AlertDialogAction`](../../frontend/src/components/ui/alert-dialog.tsx) defaults to
`buttonVariants()` → the `default` (neutral near-black) variant. So a confirm button that deletes
or irreversibly loses data renders **neutral black by default** and reads as a safe "next" button —
a silent footgun. Every destructive / data-loss confirm MUST opt in explicitly:

```tsx
import { buttonVariants } from "@/components/ui/button";

<AlertDialogAction className={buttonVariants({ variant: "destructive" })} onClick={onConfirm}>
  {t("confirmLabel")}
</AlertDialogAction>
```

What counts as a danger confirm: permanent delete (single + bulk), overwrite of existing data,
and discard of unsaved edits. See the worked-example table in
[`docs/frontend/design-system.md` § "What counts as a danger confirm"](../../docs/frontend/design-system.md#what-counts-as-a-danger-confirm).

### Verification grep

Enumerate every `AlertDialogAction` call site and confirm each destructive one carries the
destructive variant (the cancel/keep-editing siblings stay `outline`):

```bash
grep -rn "AlertDialogAction" frontend/src --include='*.tsx' | grep -v test | grep -v "components/ui/alert-dialog.tsx"
```

Today's danger confirms that MUST carry `buttonVariants({ variant: "destructive" })`:
`admin-master-form.tsx` (Delete master), `bulk-action-bar.tsx` (bulk delete),
`cards-new-client.tsx` (Overwrite), `form-sheet.tsx` (Discard).
`cardgroup-header.tsx` (Delete group) styles its action red inline (`bg-destructive`).
A confirm with no danger color and a delete/overwrite/discard onClick is a bug, not a style choice.

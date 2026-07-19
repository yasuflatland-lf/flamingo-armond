---
paths:
  - "frontend/**"
---

# Frontend design system

> Applies to: `frontend/src/**/*.tsx` (UI components, dialogs). Cross-cutting because the
> color-intent rules below govern every action button and confirm dialog. Source of truth for
> the token table and rationale: [`docs/frontend/design-system.md`](../../docs/frontend/design-system.md).

The app is **light-only**. Coral is the brand and carries the accent roles; danger is a
separate, deepened crimson. The coral-minimal model has two enforceable conventions — the
**emphasis ladder** and the **danger two-tier** — that keep coral CTAs dominant and keep
destructive actions unmistakable.

## The coral-minimal model

Danger is `destructive` (deepened crimson, `oklch(0.51 0.21 25)`) and means "stop /
irreversible". Coral is the brand, split into a **fill** token and a **link/accent text**
token:

- **Brand fill — `--brand-primary` (L74).** Background fill under white text: primary CTA,
  progress, selected, focus ring (`--ring` is now coral, same value).
- **Link / accent text — `--brand-link` (L50, darker).** Foreground coral on white: inline
  links and the `link` button variant. The L74 fill fails WCAG AA as text on white, so coral
  *text* MUST use `--brand-link`, never `--brand-primary`.
- **Warning — `--warning` (soft amber band) / `--warning-foreground` (dark amber).** A
  non-blocking caution surface, not a button fill.

The older "two reds that never co-locate" defense is superseded: coral and crimson may share a
surface because the two conventions below disambiguate intent.

### Emphasis ladder

Exactly **one** filled coral CTA per view; everything else steps back:

- **Primary CTA (one per view)** → `variant="brand"` (filled coral).
- **Secondary** → `variant="ghost"` (recommended), or `variant="outline"` in dense lists /
  dialog footers where a visible border earns its keep.
- **Danger trigger (in a row / list / menu)** → `variant="destructiveGhost"`.
- **Danger trigger (standalone, no interactive neighbours)** → `variant="destructiveOutline"`.
- **Danger commit (confirm)** → `variant="destructive"` (filled crimson).
- **Cancel / Keep-editing** → `variant="outline"`.

Two filled coral buttons on one view is the smell — demote all but the genuine primary. Never
style a constructive action (Save, Create, Login, Import, Publish) with `destructive`.

### Danger two-tier

Danger is expressed at two emphasis levels:

- **Trigger** — `variant="destructiveGhost"` (`bg-transparent text-destructive
  hover:bg-destructive/10`) + a `Trash2` icon. Red text, transparent, faint red hover —
  **never** a filled red row. The trigger only opens the confirm. Where the trigger stands
  alone with no interactive neighbours (a Danger-zone section, a selection toolbar), use
  `variant="destructiveOutline"` — the same transparent red plus a `border-destructive/45`
  border. Border is **affordance, not emphasis**: it answers "is this tappable", never "is
  this more dangerous". Both variants sit on the same rung.
- **Commit (confirm)** — filled `variant="destructive"` on the `AlertDialogAction` that
  actually deletes / overwrites / discards. This is the **only** filled crimson in the flow.

Full rationale: [`docs/frontend/design-system.md` § "The coral-minimal model"](../../docs/frontend/design-system.md#the-coral-minimal-model).

## Destructive / data-loss confirm commits MUST pass `variant="destructive"`

shadcn's [`AlertDialogAction`](../../frontend/src/components/ui/alert-dialog.tsx) defaults to
`buttonVariants()` → the `default` (neutral near-black) variant. So a confirm button that deletes
or irreversibly loses data renders **neutral black by default** and reads as a safe "next" button —
a silent footgun. Every destructive / data-loss **commit** MUST opt in explicitly (this rule is
unchanged):

```tsx
import { buttonVariants } from "@/components/ui/button";

<AlertDialogAction className={buttonVariants({ variant: "destructive" })} onClick={onConfirm}>
  {t("confirmLabel")}
</AlertDialogAction>
```

The trigger that *opens* the dialog is the lower tier — `variant="destructiveGhost"` or
`variant="destructiveOutline"`, plus `Trash2` — never filled `destructive`.

What counts as a danger confirm: permanent delete (single + bulk), overwrite of existing data,
and discard of unsaved edits. See the worked-example table in
[`docs/frontend/design-system.md` § "What counts as a danger confirm"](../../docs/frontend/design-system.md#what-counts-as-a-danger-confirm).

### Verification grep

Two distinct surfaces, two checks:

1. **Confirm commits (filled `destructive`).** Enumerate every `AlertDialogAction` call site and
   confirm each destructive one carries the filled destructive variant (the cancel/keep-editing
   siblings stay `outline`):

   ```bash
   grep -rn "AlertDialogAction" frontend/src --include='*.tsx' | grep -v test | grep -v "components/ui/alert-dialog.tsx"
   ```

   Today's danger confirm commits that MUST carry `buttonVariants({ variant: "destructive" })`:
   `admin-master-form.tsx` (Delete master), `bulk-action-bar.tsx` (bulk delete),
   `cards-new-client.tsx` (Overwrite), `form-sheet.tsx` (Discard).
   `cardgroup-header.tsx` (Delete group) styles its commit red inline (`bg-destructive`).
   A confirm with no danger color and a delete/overwrite/discard onClick is a bug, not a style choice.

2. **Delete triggers (`destructiveGhost` / `destructiveOutline`).** A Delete affordance that
   opens a confirm is the low-emphasis tier — transparent, never a filled red row:

   ```bash
   grep -rnE "destructiveGhost|destructiveOutline" frontend/src --include='*.tsx' | grep -v '\.test\.'
   ```

   A delete trigger styled as filled `destructive` (a red row at rest) is the anti-pattern the
   two-tier replaces. A `destructiveOutline` trigger that sits next to other interactive
   controls should be demoted to `destructiveGhost` — the border is only earned by isolation.

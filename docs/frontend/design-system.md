# Frontend design system

> Color-token semantics, `Button` variant usage, and the destructive-confirm-dialog
> convention. The enforceable subset of this doc is mirrored as an auto-loaded rule in
> [`.claude/rules/frontend-design-system.md`](../../.claude/rules/frontend-design-system.md);
> this file is the source of truth for the rationale and the lookup tables.

The app is **light-only** — `globals.css` sets `color-scheme: light` so an iOS standalone
PWA on a dark-mode device does not flash a black backdrop before paint. A `.dark` token
block exists but is never activated, so the **light-mode token values are what render**.
The brand is the flamingo coral palette.

Source of truth for the values quoted below:
[`frontend/src/app/globals.css`](../../frontend/src/app/globals.css) (tokens) and
[`frontend/src/components/ui/button.tsx`](../../frontend/src/components/ui/button.tsx) (variants).

## Semantic color tokens

The action-button palette resolves to three semantically distinct fills (light mode):

| Token | oklch (L / C / H) | Renders as | Meaning |
|---|---|---|---|
| `--primary` | `0.205 / 0 / 0` | near-black, neutral | Neutral default emphasis — **not** a brand or danger signal |
| `--primary-foreground` | `0.985 / 0 / 0` | near-white | Text on `--primary` |
| `--brand-primary` | `0.7364 / 0.189 / 18.45` | bright flamingo coral | The product's primary constructive CTA color |
| `--brand-primary-foreground` | `1 / 0 / 0` | white | Text on `--brand-primary` |
| `--destructive` | `0.577 / 0.245 / 27.325` | deep blood-red | Danger / irreversible / data-loss |
| `--destructive-foreground` | `0.985 / 0 / 0` | near-white | Text on `--destructive` |
| `--secondary` | `0.97 / 0 / 0` | light gray | Low-emphasis secondary |

Adjacent palettes that are **not** part of the action-button system: `--brand-tint*`
(subtle brand-tinted surfaces) and `--cefr-a/b/c*` (domain difficulty bands, contrast-guarded
in `cefr-badge.test.tsx`). They do not participate in the red-is-danger rule below.

## Red is reserved for danger

There are two reds in the system and they do different jobs:

- `brand` (CTA) — **bright coral**, lightness ≈ 74%, chroma 0.189, hue 18.
- `destructive` (danger) — **deep red**, lightness ≈ 58%, chroma 0.245, hue 27.

The principle: **`destructive` red means "stop / irreversible". Never style a constructive
action (Save, Create, Login) with `destructive`, and never leave a destructive action in a
neutral variant.** A constructive primary action stays `brand`; a destructive action is
`destructive`.

Keeping a coral `brand` CTA *and* a danger `destructive` red does **not** create ambiguity,
for two reasons:

1. **The tokens are distinguishable** — deep red (L58, more saturated) vs. bright coral (L74)
   read as different colors when seen side by side.
2. **They never co-locate** — a delete dialog has no Save button; a form's Save has no delete
   beside it. The "two reds collide" concern is theoretical, not a surface that actually renders.

This is why the brand-red CTAs across the app were deliberately **kept** rather than flattened
to neutral: the safety goal is met by making destructive actions reliably red, not by removing
the brand from constructive ones.

## Button variants

[`Button`](../../frontend/src/components/ui/button.tsx) exposes these variants. Pick by intent,
not by appearance:

| Variant | Fill | Use for |
|---|---|---|
| `brand` | flamingo coral | **Primary constructive CTA** — Save, Create, Login, Import, Publish, Study again |
| `destructive` | deep red | **Destructive / data-loss confirmation** — see the dialog convention below |
| `outline` | bordered, transparent | Cancel, Keep-editing, toggle-off, secondary action beside a primary |
| `default` | neutral near-black | Neutral emphasis where neither brand nor danger applies |
| `secondary` | light gray | Low-emphasis secondary |
| `ghost` | none until hover | Toolbar / icon-only actions inside dense UI |
| `link` | text + underline | Inline text-link affordance |

`default` is the cva fallback (`defaultVariants.variant`). Because it is neutral near-black, an
action that *should* be brand or danger but omits its variant renders as a flat neutral button —
which is exactly the footgun the next section guards against.

## Destructive and data-loss confirm dialogs

shadcn's [`AlertDialog`](../../frontend/src/components/ui/alert-dialog.tsx) wires its two footer
buttons like this:

- `AlertDialogCancel` → `buttonVariants({ variant: "outline" })` (white, bordered) — correct default.
- `AlertDialogAction` → `buttonVariants()` with **no variant** → falls back to `default`
  (neutral near-black).

So a confirm button that performs a destructive or data-loss action renders **neutral black by
default** and reads as a safe "next" button. Every such confirm MUST opt into the danger color
explicitly:

```tsx
import { buttonVariants } from "@/components/ui/button";

<AlertDialogAction
  className={buttonVariants({ variant: "destructive" })}
  onClick={handleConfirmDelete}
>
  {t("deleteMasterConfirmButton")}
</AlertDialogAction>
```

`AlertDialogAction` already applies `cn(buttonVariants(), className)` internally, and `cn`
(tailwind-merge) lets the later `bg-destructive` win over the default `bg-primary`, so passing
the destructive classes via `className` is sufficient — no override of the component is needed.

### What counts as a "danger" confirm

Route the confirm to `destructive` whenever the action **deletes or irreversibly loses data**:

| Action | Example site | Variant |
|---|---|---|
| Permanent delete | `admin-master-form.tsx` (Delete master), `cardgroup-header.tsx` (Delete group) | `destructive` |
| Bulk delete | `bulk-action-bar.tsx` (delete selected cards) | `destructive` |
| Overwrite existing data | `cards-new-client.tsx` (Overwrite duplicate) | `destructive` |
| Discard unsaved edits | `form-sheet.tsx` (Discard changes — shared by every form sheet) | `destructive` |

Constructive confirms in the same dialogs stay non-red: `AlertDialogCancel` is `outline`,
"Keep editing" is `outline`. A constructive *primary* action elsewhere (Save / Create) is `brand`,
never `destructive`.

## Accessibility notes

- **Color is never the sole signal.** The action label carries the meaning — "Delete master",
  "Discard" — so a user who does not perceive the red still reads the consequence. Treat the
  danger color as reinforcement, not as the only cue.
- **Cancel is the safe default focus.** In a Radix `AlertDialog`, focus lands on `AlertDialogCancel`,
  so an accidental Enter dismisses rather than confirms a destructive action. Preserve that — do
  not autofocus the destructive `AlertDialogAction`.
- **Foreground contrast.** `--destructive-foreground` (near-white) on the deep-red `--destructive`
  fill, and `--brand-primary-foreground` (white) on coral, are the intended pairings; do not hand-pick
  a different text color on these fills.

## Further reading

- [`docs/frontend/shadcnui.md`](shadcnui.md) — committed `components.json` and the `cn()` helper.
- [`docs/frontend/tailwind-4-notes.md`](tailwind-4-notes.md) — CSS-first `@theme` config; where these tokens are exposed as Tailwind colors.
- [`.claude/rules/frontend-design-system.md`](../../.claude/rules/frontend-design-system.md) — the auto-loaded, enforceable subset of this doc.

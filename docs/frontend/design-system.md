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

## The coral-minimal model

Coral is the brand and it carries the accent roles. **Danger** is a separate, deepened
crimson (`--destructive`, `oklch(0.51 0.21 25)`) that reads unambiguously as "stop /
irreversible" even next to coral. The system is governed by two conventions, not by a
rule that the two colors must never co-locate:

- **Emphasis ladder** — exactly one filled coral CTA per view; every other control steps
  back to a lower-emphasis variant. This is what keeps a screen from looking like a wall
  of coral buttons.
- **Danger two-tier** — a delete *trigger* is low-emphasis red on a transparent ground
  (`destructiveGhost`, or `destructiveOutline` when it needs a visible border); the data-loss
  *commit* inside the confirm dialog is the only filled crimson. The trigger invites; the
  commit re-affirms.

These supersede the older "two reds that never co-locate" defense. Coral and crimson are
allowed to share a surface because the emphasis ladder and the two-tier already disambiguate
intent: the filled coral is the one constructive CTA, the filled crimson appears only at the
moment of irreversible commit, and everything else is neutral or ghost.

## Semantic color tokens

The action-button palette and the status/accent tokens resolve to these values (light mode):

| Token | oklch (L / C / H) | Renders as | Meaning |
|---|---|---|---|
| `--primary` | `0.205 / 0 / 0` | near-black, neutral | Neutral default emphasis — **not** a brand or danger signal |
| `--primary-foreground` | `0.985 / 0 / 0` | near-white | Text on `--primary` |
| `--brand-primary` | `0.7364 / 0.189 / 18.45` | bright flamingo coral (L74) | The brand **fill** — primary CTA, progress, selected, focus ring |
| `--brand-primary-foreground` | `1 / 0 / 0` | white | Text on `--brand-primary` |
| `--brand-link` | `0.5 / 0.15 / 20` | darker coral | Link / accent **text** — AA on white (see "Accent coral" below) |
| `--destructive` | `0.51 / 0.21 / 25` | deepened crimson | Danger / irreversible / data-loss |
| `--destructive-foreground` | `0.985 / 0 / 0` | near-white | Text on `--destructive` |
| `--warning` | `0.8 / 0.145 / 78` | soft amber band | Caution band (non-blocking warning surface), not a button fill |
| `--warning-foreground` | `0.4 / 0.1 / 72` | dark amber | Text on `--warning` (and AA on white) |
| `--success` | `0.63 / 0.17 / 149` | green | Status indicator only (e.g. published-deck dot), never a CTA fill |
| `--secondary` | `0.97 / 0 / 0` | light gray | Low-emphasis secondary |

`--ring` is now coral (`oklch(0.7364 0.189 18.45)`, the same value as `--brand-primary`), so
focus outlines read as brand rather than a neutral gray.

Adjacent palettes that are **not** part of the action-button system: `--brand-tint*`
(subtle brand-tinted surfaces) and `--cefr-a/b/c*` (domain difficulty bands, contrast-guarded
in `cefr-badge.test.tsx`). They do not participate in the danger conventions below.

## Accent coral: brand fill vs link text

Coral exists in two distinct token roles because a single coral value cannot serve both the
fill role and the text role and still pass WCAG AA:

- **Brand fill — `--brand-primary` (L74).** Used as a *background fill* under white text:
  primary CTA buttons, progress bars, selected states, and the focus ring (`--ring`). The
  white `--brand-primary-foreground` on the L74 coral fill is the intended high-contrast
  pairing.
- **Link / accent text — `--brand-link` (L50).** Used as *foreground text* on a white
  surface: inline links and the `link` button variant. The L74 brand fill is too light to
  use as text on white (it fails the WCAG-AA 4.5:1 body-text threshold), so the darker L50
  `--brand-link` carries the accent-text role and clears AA on white.

The rule of thumb: coral **on** white (text) is `--brand-link`; coral **as** the surface
(fill behind white text) is `--brand-primary`. Never use the L74 fill token as text color.

## Emphasis ladder

A view has exactly **one** filled coral CTA. Everything else steps down the ladder so the
single primary action stays visually dominant:

| Role | Variant | Renders as |
|---|---|---|
| Primary CTA (one per view) | `brand` | filled coral |
| Secondary action | `ghost` (or `outline` in dense lists / dialogs) | neutral, no fill until hover |
| Danger trigger (in a row / list) | `destructiveGhost` | red text, transparent, faint red hover |
| Danger trigger (standalone) | `destructiveOutline` | red text, transparent, red border |
| Danger commit (confirm) | `destructive` | filled crimson |
| Cancel / Keep-editing | `outline` | bordered, transparent |

`ghost` is the recommended **secondary** default; promote a secondary to `outline` only where a
visible border earns its keep — dense lists, toolbars, and dialog footers where a borderless
control would be hard to discover. Two filled coral buttons on one view is the ladder smell to
watch for: demote all but the genuine primary.

## Danger two-tier

Danger is expressed at two emphasis levels, and the level tracks how close the user is to
irreversible loss:

- **Trigger — `destructiveGhost` / `destructiveOutline` + a `Trash2` icon.** A Delete affordance
  is low-emphasis: red text on `bg-transparent`, faint red hover. It reads as red but never
  paints a filled red row. The trigger only *opens* the confirm — it does not itself destroy
  data.
- **Commit (confirm) — filled `destructive`.** The `AlertDialogAction` that actually deletes /
  overwrites / discards is the **only** filled crimson in the flow. This re-affirms the
  existing filled-destructive confirm rule below, which is **unchanged**.

A filled red row in a list is the anti-pattern this two-tier replaces: it shouts danger at rest,
before the user has expressed any delete intent. Transparent red at the trigger, filled crimson
only at the commit.

### Border is affordance, not emphasis: `destructiveGhost` vs `destructiveOutline`

The two trigger variants sit on the *same* rung of the ladder — both are transparent red. The
border answers a different question than the fill does: **is this thing tappable at all?**

- **`destructiveGhost` (borderless)** — the trigger lives inside a structure that already
  supplies the affordance: a table row, a list item, an overflow menu, a sheet section with
  sibling controls. Neighbouring hit targets tell the user this region is interactive, so the
  border would be redundant chrome.
- **`destructiveOutline` (bordered)** — the trigger stands alone on the page with no
  interactive neighbours. Borderless red text in isolation reads as a static label or an inline
  link, not a button, so the affordance has to be drawn explicitly.

Reach for `destructiveOutline` only on the isolation test. It is **not** a way to make a delete
"stand out more" — that would re-open the emphasis question the two-tier settles. The rule of
thumb: if removing the button would leave a region with no other controls in it, it needs the
border.

Worked example: the `/profile` Danger zone
([`delete-account-section.tsx`](../../frontend/src/app/profile/delete-account-section.tsx)) is a
heading, a sentence, and one control. As `destructiveGhost` the trigger rendered as a red text
row and did not read as a button; it uses `destructiveOutline`. By contrast the admin
user-profile sheet's Delete user
([`admin-user-profile-sheet.tsx`](../../frontend/src/app/admin/users/admin-user-profile-sheet.tsx))
sits below other sheet controls and stays `destructiveGhost`.

### Selection toolbars: the destructive action is the bar's primary

A contextual selection bar — e.g. the cards bulk-action bar
([`bulk-action-bar.tsx`](../../frontend/src/components/cardgroups/bulk-action-bar.tsx)) — is the
one place where a danger trigger is also the **primary** action of its view: the user entered
selection mode *in order to* delete. There the trigger uses `destructiveOutline` so it reads as
clearly tappable and findable, and its sibling **Cancel is demoted from `outline` to `ghost`** so
the single bordered control is the destructive one. This is a deliberate, local exception to the
"Cancel → `outline`" ladder row above: emphasis tracks intent, and the bar's intent is to delete.
The trigger stays transparent (border only, never a filled red row), and the real safety gate
remains the filled `destructive` commit in the confirm dialog.

## Button variants

[`Button`](../../frontend/src/components/ui/button.tsx) exposes these variants. Pick by intent,
not by appearance:

| Variant | Fill | Use for |
|---|---|---|
| `brand` | flamingo coral (`--brand-primary`) | **Primary constructive CTA** — Save, Create, Login, Import, Publish, Study again (one per view) |
| `destructive` | filled crimson (`--destructive`) | **Destructive / data-loss commit** — the confirm-dialog action; see the dialog convention below |
| `destructiveGhost` | none until hover (`text-destructive`, faint red hover) | **Low-emphasis danger trigger** — Delete inside a row / list / menu that opens a confirm |
| `destructiveOutline` | none until hover, plus a `border-destructive/45` border | **Low-emphasis danger trigger, standalone** — same rung as `destructiveGhost`; the border supplies affordance where no interactive neighbours do |
| `outline` | bordered, transparent | Cancel, Keep-editing, toggle-off, secondary action in dense lists / dialogs |
| `ghost` | none until hover | **Recommended secondary** — toolbar / icon-only actions and step-back secondaries inside dense UI |
| `link` | text + underline (coral `--brand-link`) | Inline text-link affordance |
| `default` | neutral near-black (`--primary`) | Neutral emphasis where neither brand nor danger applies |
| `secondary` | light gray | Low-emphasis secondary |

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
default** and reads as a safe "next" button. Every such confirm — the danger **commit** in the
two-tier — MUST opt into the filled danger color explicitly:

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

The trigger that *opens* this dialog is the lower tier: it uses `destructiveGhost` or
`destructiveOutline` (transparent, red text + `Trash2`), never filled `destructive`. Only the
commit above is filled.

### What counts as a "danger" confirm

Route the confirm commit to `destructive` whenever the action **deletes or irreversibly loses
data**:

| Action | Example site | Commit variant |
|---|---|---|
| Permanent delete | `admin-master-form.tsx` (Delete master), `cardgroup-header.tsx` (Delete group) | `destructive` |
| Bulk delete | `bulk-action-bar.tsx` (delete selected cards) | `destructive` |
| Overwrite existing data | `cards-new-client.tsx` (Overwrite duplicate) | `destructive` |
| Discard unsaved edits | `form-sheet.tsx` (Discard changes — shared by every form sheet) | `destructive` |

Constructive confirms in the same dialogs stay non-red: `AlertDialogCancel` is `outline`,
"Keep editing" is `outline`. A constructive *primary* action elsewhere (Save / Create) is `brand`,
never `destructive`. The trigger that opens any of these dialogs is `destructiveGhost` or
`destructiveOutline`.

## Accessibility notes

- **Color is never the sole signal.** The action label carries the meaning — "Delete master",
  "Discard" — so a user who does not perceive the red still reads the consequence. Treat the
  danger color as reinforcement, not as the only cue.
- **Cancel is the safe default focus.** In a Radix `AlertDialog`, focus lands on `AlertDialogCancel`,
  so an accidental Enter dismisses rather than confirms a destructive action. Preserve that — do
  not autofocus the destructive `AlertDialogAction`.
- **Foreground contrast.** `--destructive-foreground` (near-white) on the crimson `--destructive`
  fill, and `--brand-primary-foreground` (white) on the L74 coral fill, are the intended pairings;
  do not hand-pick a different text color on these fills. For coral *text* on white use `--brand-link`
  (L50), which clears WCAG AA — the L74 fill token fails as text on white.

## Further reading

- [`docs/frontend/shadcnui.md`](shadcnui.md) — committed `components.json` and the `cn()` helper.
- [`docs/frontend/tailwind-4-notes.md`](tailwind-4-notes.md) — CSS-first `@theme` config; where these tokens are exposed as Tailwind colors.
- [`.claude/rules/frontend-design-system.md`](../../.claude/rules/frontend-design-system.md) — the auto-loaded, enforceable subset of this doc.

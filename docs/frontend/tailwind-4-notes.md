# Tailwind 4 notes

> Part of [`frontend/CLAUDE.md`](../../frontend/CLAUDE.md). See the index for related chapters.

- Tokens and dark-mode variant live in `src/app/globals.css` via `@theme` and `@custom-variant dark (...)`. There is no `tailwind.config.ts`.
- PostCSS plugin is `@tailwindcss/postcss` (the old `tailwindcss` plugin no longer exists in v4). `autoprefixer` is not needed — Tailwind 4 handles prefixing internally.
- Tokens map into the Tailwind namespace via an **`@theme inline`** block in `globals.css` (`inline` matters — without it Tailwind emits duplicate variables). `tw-animate-css` is pulled in with `@import "tw-animate-css"` (it is a CSS package, not a JS plugin).
- When new shadcn components are added, extend the `@theme` block with the additional `--color-*` tokens the components reference.

## Semantic color-band tokens: declare in `:root`, alias in `@theme inline`

A multi-value semantic palette (e.g. the CEFR difficulty bands) follows the same two-step shape as the existing `--brand-tint*` tokens:

1. Declare the raw `oklch` triplets in `:root` — one soft fill + matching `-foreground` per band (`--cefr-a` / `--cefr-a-foreground`, `--cefr-b…`, `--cefr-c…`).
2. Alias each into the Tailwind namespace inside the `@theme inline` block as `--color-cefr-*`, so `bg-cefr-a` / `text-cefr-a-foreground` utilities exist.

The CEFR tokens are **light-only** — no `.dark` variants — because the app ships a single light theme. Do not add `.dark` overrides for a palette the design does not theme.

### Band-grouping and contrast decisions (the Why)

- **Group by band, not per level.** The six CEFR levels (A1–C2) bucket into three HUE families — A = green, B = amber, C = rose — so the color carries a coarse difficulty signal while the badge text always shows the exact level. Color is therefore never the sole signal.
- **Rose (not red) for the C band**, so an advanced word reads as "advanced", not "alarm".
- **WCAG AA on the card surface.** Foregrounds are tuned to a contrast ratio ≥ 4.5:1 against the white card surface (`--card: oklch(1 0 0)`), not just against the band fill. The amber/B band is the at-risk one (smallest luminance margin) — verify with a real contrast calculation, never by eye.

### Pin the tokens with a two-layer static guard

A unit test mirrors the `oklch` tokens as hex constants and asserts both contrast ratios (text-on-band-fill and text-on-white). Because hex is a hand-maintained mirror that can silently drift from the `oklch` source, a second `readFileSync(globals.css)` guard pins each `--cefr-*` declaration **verbatim**, so any token edit fails the suite loudly and forces the hex mirror to be re-derived and re-verified. This is the [static-source regression-guard idiom](typescript-conventions/static-grep-regression-guard-test.md), applied to design tokens rather than component source. Reference: `frontend/src/components/learn/cefr-badge.test.tsx`.

### Tailwind v4 scanner needs full literal class strings

The v4 content scanner sees source text, not runtime values: an interpolated `bg-cefr-${band}` is invisible to it and the utility is never emitted. Map the band key to a **full literal class string** instead:

```ts
// Correct — each utility appears verbatim, so the scanner emits it.
const bandClass = {
  a: "bg-cefr-a text-cefr-a-foreground",
  b: "bg-cefr-b text-cefr-b-foreground",
  c: "bg-cefr-c text-cefr-c-foreground",
} as const;

// Wrong — interpolated class names are never generated.
className={`bg-cefr-${band}`}
```

## Corner-badge-slot convention on the Learn card

The Learn card's **top-right corner** is reserved for the CEFR badge; this feature sets that precedent. Future on-card badges (FSRS state, #276 / #277) MUST claim a **different** corner so the two never collide. Two layout rules keep cumulative layout shift at zero:

- The badge is `position: absolute` (`absolute right-3 top-3`), so it never participates in flow — CLS is zero by construction. The outer card carries `relative` to anchor it.
- The centered content block reserves horizontal padding (`px-10`) so a long single-token `front` cannot slide **under** the right-pinned badge on a narrow (~320px) viewport. Padding on the flow content does not reflow the absolute badge, so CLS stays zero.

Reference: `frontend/src/components/learn/swipe-card.tsx` (`CardContent`) and `frontend/src/components/learn/cefr-badge.tsx`.

## `hover:` on touch devices — pair with `active:` for tap feedback

Tailwind v4's `hover:` variant compiles to `@media (hover: hover)` and does not fire reliably on touch devices. A list-item-style element styled with only `hover:bg-accent transition-colors` gives no feedback when a mobile user taps it — the row appears unresponsive even when the navigation works. Pair `hover:` with `active:` so the row flashes on touch:

```tsx
// Correct — desktop hover and touch tap both highlight.
<li className="... hover:bg-accent active:bg-accent transition-colors">

// Wrong — silent on mobile.
<li className="... hover:bg-accent transition-colors">
```

When the existing string carries both `hover:bg-X` and `hover:text-X`, pair both with their `active:` counterparts. Group the hover-pair before the active-pair, matching the precedent in `frontend/src/components/ui/sidebar.tsx:370`:

```tsx
// Correct — hover-pair first, active-pair second.
"hover:bg-accent hover:text-accent-foreground active:bg-accent active:text-accent-foreground"

// Wrong — interleaved.
"hover:bg-accent active:bg-accent hover:text-accent-foreground active:text-accent-foreground"
```

Tailwind class order has no cascade effect (the utilities target disjoint property/state pairs), so the rule is purely a readability convention — consistency matters for grep-ability when refactoring class strings across files.

Applies to list rows, picker-sheet items, nav-drawer links, and any other element where `hover:bg-*` is the primary affordance signal. Buttons (`components/ui/button.tsx` variants) and icon-only nav triggers are out of scope — they carry their own pressed-state conventions through the shared `Button` component, and adding `active:` there would force a wide cross-cutting change with no incremental benefit.

A static-grep regression-guard test for the presence of `active:bg-*` is deliberately NOT added — see [`typescript-conventions/static-grep-regression-guard-test.md`](typescript-conventions/static-grep-regression-guard-test.md). A missing tap highlight is a visually obvious regression caught by QA on a touch device; pinning the granular Tailwind variant choice would force test churn on any future design-token rename with no bug-catching value. The guard pattern is reserved for invisible load-bearing attributes like `optimisticResponse` absence.


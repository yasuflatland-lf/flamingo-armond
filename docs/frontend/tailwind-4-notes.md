# Tailwind 4 notes

> Part of [`frontend/CLAUDE.md`](../../frontend/CLAUDE.md). See the index for related chapters.

- Tokens and dark-mode variant live in `src/app/globals.css` via `@theme` and `@custom-variant dark (...)`. There is no `tailwind.config.ts`.
- PostCSS plugin is `@tailwindcss/postcss` (the old `tailwindcss` plugin no longer exists in v4). `autoprefixer` is not needed — Tailwind 4 handles prefixing internally.
- Tokens map into the Tailwind namespace via an **`@theme inline`** block in `globals.css` (`inline` matters — without it Tailwind emits duplicate variables). `tw-animate-css` is pulled in with `@import "tw-animate-css"` (it is a CSS package, not a JS plugin).
- When new shadcn components are added, extend the `@theme` block with the additional `--color-*` tokens the components reference.

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


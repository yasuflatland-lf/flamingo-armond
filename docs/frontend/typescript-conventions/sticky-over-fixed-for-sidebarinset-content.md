# `sticky bottom-0` over `fixed inset-x-0` for action bars inside `SidebarInset`

> Part of [`docs/frontend/typescript-conventions.md`](../typescript-conventions.md). See the index for related rules.

A bottom action bar that uses `fixed inset-x-0 flex justify-center` centers relative to the **viewport**, which on
desktop includes the `GlobalRail` sidebar (48 px collapsed, 256 px expanded). The result is a bar that is visually
off-center inside the content area.

Switching to `sticky bottom-0` places the bar in `SidebarInset`'s normal document flow. Its `flex justify-center`
then centers within the inset, automatically excluding the rail — and the bar tracks rail-collapse/expand (48 px ↔ 256 px)
without reading any `--sidebar-width` CSS variable.

## Correct class structure

```tsx
{/* outer — sticky positioning, pointer passthrough for overflow areas */}
<div className="pointer-events-none sticky bottom-0 z-40 flex justify-center px-4 pt-3 pb-safe">
  {/* inner — the interactive bar; pointer-events restored here */}
  <div className="pointer-events-auto flex items-center gap-4">
    {/* buttons */}
  </div>
</div>
```

The outer `pointer-events-none` paired with inner `pointer-events-auto` is **load-bearing**, not vestigial:

- When the sticky bar overlays content near the bottom (overflow-scroll viewport, devices with safe-area insets), touches/clicks
  aimed at content *outside* the inner element pass through the outer `pointer-events-none` zone.
- The inner `pointer-events-auto` restores interactivity exactly for the buttons.

Removing `pointer-events-none` from the outer container blocks scroll gestures and taps on any card content that
visually underlaps the padding zone.

## Why not `fixed`

| | `fixed inset-x-0` | `sticky bottom-0` |
|---|---|---|
| Centering reference | Full viewport (includes the rail) | `SidebarInset` width (excludes the rail) |
| Tracks rail collapse/expand | No — requires reading `--sidebar-width` | Yes — follows document flow |
| Scroll passthrough on overflow | Blocked unless `pointer-events-none` on the full bar | Natural — non-interactive area is transparent |

## Reference implementation

`frontend/src/components/learn/learn-action-bar.tsx` — the three-button rating bar on `/learn/[cardgroupId]`.
Tests in `frontend/src/components/learn/learn-action-bar.test.tsx` assert `toHaveClass("sticky")`,
`not.toHaveClass("fixed")`, and `not.toHaveClass("inset-x-0")` to lock in this constraint.

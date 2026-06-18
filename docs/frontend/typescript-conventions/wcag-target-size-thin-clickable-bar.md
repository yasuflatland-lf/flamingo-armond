# A thin (`h-1`) clickable bar fails the WCAG 2.5.8 target-size minimum — enlarge the hit area with transparent padding

> Part of [`docs/frontend/typescript-conventions.md`](../typescript-conventions.md). See the index for related rules.

WCAG 2.5.8 (Target Size — Minimum) requires interactive controls to have a target size of at least 24 × 24 CSS pixels. A `h-1` bar (4 px tall) passes keyboard and screen-reader paths — they activate via `Enter`/`Space` and do not depend on click area — but a 4 px touch target is impractical on mobile and makes mouse click precisely difficult on desktop.

## The fix: transparent padding with negative-margin compensation

Expand the click/tap area without moving the visual bar by adding vertical padding on the button and compensating with a negative vertical margin so the surrounding layout is unaffected:

```tsx
<button
  type="button"
  onClick={onBack}
  disabled={importing}
  aria-label="Paste & review"
  className="-my-2 py-2 disabled:cursor-not-allowed disabled:opacity-50 ..."
>
  <span className="block h-1 w-10 rounded-full bg-brand-primary" />
</button>
```

`-my-2` pulls the button's top and bottom margins inward by 8 px each; `py-2` adds 8 px of transparent padding above and below the visual `span`. The combined effect is a ~20 px tall touch/click zone centered on the 4 px bar, with no change to the bar's visual position or the surrounding flex gap.

The exact expansion (`py-2` = 8 px per side, total 20 px) brings the practical target to 20 px, which is within WCAG 2.5.8's acceptable range when the control has adequate spacing (the adjacent bar also provides separation). Increase to `py-3` (12 px padding, 28 px total) if stricter conformance is required.

## What does not work

**Increasing `h-*` on the button or the `span`** changes the visual bar height, shifting the stepper's appearance. The goal is a larger invisible target, not a larger bar.

**Adding `min-h-*` to the button** changes layout height and pushes adjacent elements, defeating the point of having a minimal-footprint stepper.

## Reference

`frontend/src/components/batch-import/batch-import-wizard.tsx` — `ImportStepper`, the step-1 back button. The non-clickable step-2 bar is a plain `<div>` with only `h-1`; only the interactive step-1 bar (when rendering step 2) requires the hit-area expansion.

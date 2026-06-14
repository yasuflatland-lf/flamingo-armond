# Auto-fit card text (`useFitText`)

> Applies to: `frontend/src/components/learn/` (the flashcard front term). The
> pattern — shrink a measured DOM element to fit, with a pure solver and a
> jsdom-safe test harness — generalizes to any layout-measuring hook.

The learn-card **front** term auto-shrinks so a long single word (e.g.
`cardiovascular`) fits on one line instead of overflowing, while multi-word
fronts wrap normally. Implemented by `computeFitFontSize` (pure solver) +
`useFitText` (hook) in `frontend/src/components/learn/use-fit-text.ts`, wired
into `CardContent` in `swipe-card.tsx`.

## Behaviour and the rules behind it

### `break-normal`, never `break-words` — and shrink only when a word overflows

The front uses Tailwind `break-normal` (`overflow-wrap: normal; word-break:
normal`): text wraps **at word boundaries** and a word is never split
mid-character. A single word longer than the line does **not** wrap under
`break-normal` — it overflows. That overflow is what `useFitText` removes by
shrinking the font, so the widest unbreakable word fits one line. Multi-word
content that already fits is left at the ceiling and simply wraps across lines.

Do **not** use `break-words` (`overflow-wrap: break-word`) on text that should
never break mid-word, and do **not** use `whitespace-nowrap` (it forbids the
multi-line wrap).

### Measure the widest word via `scrollWidth`

`computeFitFontSize` compares `intrinsicWidth` (the element's `scrollWidth` at
the reference `maxPx`) against `availableWidth` (its `clientWidth`). For a
`break-normal` element, `scrollWidth == clientWidth` **unless** a single word
overflows, in which case `scrollWidth` is that word's width. So the proportional
shrink (`maxPx * available / intrinsic`) fires exactly when — and only when — a
word is wider than the card. A `max-width: 100%` element caps its own
`clientWidth` at the parent's content box, so surrounding padding is respected
with no explicit subtraction.

### Observe the PARENT, not the element

`useFitText` attaches its `ResizeObserver` to `el.parentElement`, not `el`.
Changing the element's `font-size` changes the element's own height (wrapped
line count), which would re-fire an observer watching the element and risk a
measure loop. The parent's width tracks the viewport (the only re-fit trigger
we want), so observing the parent is both correct and loop-free.

### The apply-write after measurement is load-bearing

`measure()` writes `el.style.fontSize` twice: first to the reference `maxPx`
(to read a stable `scrollWidth`), then to the computed `next` size. The second
write is **not** redundant with `setFontPx(next)`: when `next` equals the
current React state, `setFontPx` is a no-op and React skips the re-render, which
would otherwise leave the element pinned at the reference `maxPx` from the first
write. Keep both writes.

### Trigger-only `text` dependency

The effect lists `text` in its dependency array but never reads it in the body —
it is a trigger that forces a re-measure when the card term changes. Biome's
`useExhaustiveDependencies` flags it; the fix is a load-bearing `biome-ignore`,
not removing the dep (removing it would pin the font to the previous term's
width). See [`docs/pagination/load-bearing-biome-ignore.md`](../pagination/load-bearing-biome-ignore.md).

### One ceiling across reveal — the headword keeps its size on flip

`FRONT_FIT = { maxPx: 48, minPx: 20 }` is a **single** constant used whether or
not the card is revealed. Flipping the card does not resize the headword; it
only adds the translation below it. (The earlier design shrank the revealed
front to a smaller ceiling, which read as the headword "jumping" size on flip.)
`minPx` is the floor below which an exceptionally long word is clipped by the
card's `overflow-hidden` rather than shrunk to an unreadable size.

## Testing harness — jsdom has no layout engine

jsdom reports `clientWidth`/`scrollWidth` as `0` and provides **no**
`ResizeObserver`. That shapes the test split:

- **Layout math → a pure function.** `computeFitFontSize` takes explicit width
  numbers and is unit-tested directly (`use-fit-text.test.ts`) — fits,
  exact-boundary, proportional shrink, floor-to-whole-pixel, clamp-to-`minPx`,
  and the unmeasured (`0`-width) guard. No DOM needed.
- **Hook control flow → testable in jsdom anyway.** Attach the hook's ref to a
  real element via a tiny `Harness` component and exercise (`use-fit-text.dom.test.tsx`):
  - the `typeof ResizeObserver === "undefined"` guard (jsdom default) — no throw, returns `maxPx`;
  - that the observer targets the **parent**, not the text element;
  - `disconnect()` on unmount (no leak);
  - the **re-fit on resize**: `vi.stubGlobal("ResizeObserver", Fake)` where the
    fake captures the callback, then `Object.defineProperty(el, "clientWidth"/"scrollWidth", …)`
    to non-zero values, fire the captured callback in `act()`, and assert the
    new size.
- **Assert the returned React state, not just the imperative DOM write.** The
  hook mutates `el.style.fontSize` imperatively *and* via `setFontPx`. A test
  that reads only `el.style.fontSize` passes even if `setFontPx` is dropped
  (the imperative write masks it), but production binds the **React** value, so
  a stale state would revert on the next re-render. Mirror `fontPx` onto a
  `data-*` attribute in the Harness and assert both.

The component-level test (`swipe-card.test.tsx`) pins the structural
preconditions the math depends on (front is `break-normal`, not
`break-words`/`break-all`/`whitespace-nowrap`) and the headword's constant size
across an in-place reveal — `jsdom`'s `0`-width keeps `computeFitFontSize` at
`maxPx`, so the inline `font-size` equals the ceiling and is assertable without
a layout engine.

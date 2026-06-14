import { useLayoutEffect, useRef, useState } from "react";

type FitParams = {
  /** Content-box width the text must fit into (px). */
  availableWidth: number;
  /**
   * Width the content needs at `maxPx` (px). For wrappable content this equals
   * `availableWidth` unless a single unbreakable word overflows, in which case
   * it is that word's width.
   */
  intrinsicWidth: number;
  /** Upper bound: the size used when the text already fits. */
  maxPx: number;
  /** Lower bound: the smallest size the text is allowed to shrink to. */
  minPx: number;
};

/**
 * Pure font-size solver: downscales only when the measured content width
 * exceeds the available width.
 *
 * Downscale-only: content that already fits keeps `maxPx`; when a single word
 * overflows, the size shrinks proportionally (text width scales ~linearly with
 * font-size for the same string) and is clamped to `minPx`.
 *
 * A non-positive `availableWidth` or `intrinsicWidth` means the element has
 * not been laid out yet (the common case under jsdom, which has no layout
 * engine, and on the first render before the ResizeObserver fires). In that
 * case there is nothing to measure against, so we return `maxPx` and let the
 * effect re-run once real dimensions exist.
 */
export function computeFitFontSize({
  availableWidth,
  intrinsicWidth,
  maxPx,
  minPx,
}: FitParams): number {
  if (availableWidth <= 0 || intrinsicWidth <= 0) return maxPx;
  if (intrinsicWidth <= availableWidth) return maxPx;
  const scaled = Math.floor((maxPx * availableWidth) / intrinsicWidth);
  return Math.max(minPx, scaled);
}

/**
 * Largest integer font size in `[minPx, maxPx]` for which `fits(px)` is true.
 *
 * `fits` is a monotone predicate — if a size fits, every smaller size fits too
 * (a smaller font wraps to fewer/shorter lines, so its rendered line count never
 * grows). A binary search over the monotone predicate finds the answer in
 * `O(log range)` measurements; returns `minPx` when even the smallest size does
 * not fit. Pure: the DOM-measuring `fits` closure is injected, so the search is
 * unit tested without a layout engine.
 */
export function solveFitBySearch(
  fits: (px: number) => boolean,
  minPx: number,
  maxPx: number,
): number {
  let lo = minPx;
  let hi = Math.floor(maxPx);
  let best = minPx;
  while (lo <= hi) {
    const mid = Math.floor((lo + hi) / 2);
    if (fits(mid)) {
      best = mid;
      lo = mid + 1;
    } else {
      hi = mid - 1;
    }
  }
  return best;
}

/**
 * Number of wrapped lines the element currently occupies, derived from its
 * rendered height and computed line-height. Returns 0 when the element is not
 * laid out (jsdom / pre-layout: `scrollHeight === 0`), which callers treat as
 * "unmeasured, skip the line-count constraint".
 */
function measureLineCount(el: HTMLElement, fontPx: number): number {
  if (el.scrollHeight <= 0) return 0;
  let lineHeightPx = Number.parseFloat(getComputedStyle(el).lineHeight);
  if (!Number.isFinite(lineHeightPx) || lineHeightPx <= 0) {
    // `line-height: normal` / unmeasured — approximate with the tight ratio the
    // headword uses (leading-tight = 1.25). The exact value only needs to be
    // close enough to bucket scrollHeight into the right number of lines.
    lineHeightPx = fontPx * 1.25;
  }
  return Math.round(el.scrollHeight / lineHeightPx);
}

/**
 * Shrinks a wrapping text element to fit its width and, optionally, a maximum
 * number of wrapped lines. Attach `ref` to the text element (it should wrap at
 * word boundaries, e.g. Tailwind `break-normal`) and apply the returned `fontPx`
 * as its `font-size`.
 *
 * Two downscale constraints, applied in order:
 *
 * 1. Width (proportional). Measured at `maxPx` via `scrollWidth` — for wrappable
 *    content this equals `clientWidth` unless a single unbreakable word
 *    overflows, in which case it is that word's width. `computeFitFontSize`
 *    shrinks proportionally so the widest word fits one line.
 * 2. Line count (binary search). When `maxLines` is given and the element is
 *    laid out, the size is shrunk further — via `solveFitBySearch` over
 *    `lineCount <= maxLines` — so the text wraps to at most `maxLines` lines.
 *    Callers derive `maxLines` from the word count (e.g. `ceil(words / 3)`) so a
 *    multi-word phrase wraps to fuller lines instead of stranding a lone word on
 *    its own line; pair it with `text-wrap: balance` on the element so the words
 *    spread evenly across those lines. A non-laid-out element (jsdom /
 *    pre-layout) reports 0 lines, so the constraint is skipped and only the
 *    width fit applies — preserving the downscale-only, jsdom-safe contract.
 *
 * Re-fits whenever the parent container resizes (viewport change / rotation) and
 * whenever `text` or `maxLines` changes.
 */
export function useFitText<T extends HTMLElement>(
  text: string,
  maxPx: number,
  minPx: number,
  maxLines?: number,
): { ref: React.RefObject<T | null>; fontPx: number } {
  const ref = useRef<T>(null);
  const [fontPx, setFontPx] = useState(maxPx);

  // biome-ignore lint/correctness/useExhaustiveDependencies: `text` is a trigger-only dependency; the effect re-measures `scrollWidth`/`scrollHeight` when the card term changes but does not reference `text` in its body. Removing it (Biome's offered fix) would pin the font to the previous term's dimensions.
  useLayoutEffect(() => {
    const el = ref.current;
    if (!el) return;

    const measure = () => {
      // Width fit: measure intrinsic width at a fixed reference size so the ratio
      // is stable regardless of the size currently applied.
      el.style.fontSize = `${maxPx}px`;
      const widthFit = computeFitFontSize({
        availableWidth: el.clientWidth,
        intrinsicWidth: el.scrollWidth,
        maxPx,
        minPx,
      });

      // Line-count fit: shrink further so the text wraps to at most `maxLines`
      // lines. Skipped when `maxLines` is absent or the element is not laid out
      // (jsdom / pre-layout: measureLineCount returns 0).
      let next = widthFit;
      if (maxLines && maxLines > 0) {
        el.style.fontSize = `${widthFit}px`;
        if (measureLineCount(el, widthFit) > maxLines) {
          next = solveFitBySearch(
            (px) => {
              el.style.fontSize = `${px}px`;
              return measureLineCount(el, px) <= maxLines;
            },
            minPx,
            widthFit,
          );
        }
      }

      // Apply imperatively (no paint at an intermediate size) and keep React in
      // sync for declarative re-renders.
      el.style.fontSize = `${next}px`;
      setFontPx(next);
    };

    measure();

    // jsdom has no ResizeObserver; the initial measure() above is enough there.
    if (typeof ResizeObserver === "undefined") return;

    // Observe the parent (its width tracks the viewport) rather than `el`
    // itself — changing `el`'s font-size would otherwise re-trigger the
    // observer and risk a measure loop.
    const target = el.parentElement ?? el;
    const observer = new ResizeObserver(measure);
    observer.observe(target);
    return () => observer.disconnect();
  }, [text, maxPx, minPx, maxLines]);

  return { ref, fontPx };
}

import { useLayoutEffect, useRef, useState } from "react";

type FitParams = {
  /** Content-box width the text must fit into (px). */
  availableWidth: number;
  /** Single-line width the text occupies at `maxPx` (px). */
  intrinsicWidth: number;
  /** Upper bound: the size used when the text already fits. */
  maxPx: number;
  /** Lower bound: the smallest size the text is allowed to shrink to. */
  minPx: number;
};

/**
 * Pure font-size solver for single-line auto-fit text.
 *
 * Downscale-only: a word that already fits keeps `maxPx`; a word that
 * overflows shrinks proportionally (text width scales ~linearly with
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
 * Shrinks a single-line text element so a long word fits on one line instead
 * of wrapping. Attach `ref` to the text element (it must render with
 * `white-space: nowrap`) and apply the returned `fontPx` as its `font-size`.
 *
 * The element is measured at `maxPx` (its intrinsic single-line width via
 * `scrollWidth`) against the width it is allowed to occupy (`clientWidth`,
 * which a `max-width: 100%` text element caps at its parent's content box —
 * so the surrounding padding is respected automatically). Re-fits whenever the
 * parent container resizes (viewport change / rotation) and whenever `text`
 * changes.
 */
export function useFitText<T extends HTMLElement>(
  text: string,
  maxPx: number,
  minPx: number,
): { ref: React.RefObject<T | null>; fontPx: number } {
  const ref = useRef<T>(null);
  const [fontPx, setFontPx] = useState(maxPx);

  // biome-ignore lint/correctness/useExhaustiveDependencies: `text` is a trigger-only dependency; the effect re-measures `scrollWidth` when the card term changes but does not reference `text` in its body. Removing it (Biome's offered fix) would pin the font to the previous term's width.
  useLayoutEffect(() => {
    const el = ref.current;
    if (!el) return;

    const measure = () => {
      // Measure intrinsic width at a fixed reference size so the ratio is
      // stable regardless of the size currently applied.
      el.style.fontSize = `${maxPx}px`;
      const next = computeFitFontSize({
        availableWidth: el.clientWidth,
        intrinsicWidth: el.scrollWidth,
        maxPx,
        minPx,
      });
      // Apply imperatively (no paint at the reference size) and keep React in
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
  }, [text, maxPx, minPx]);

  return { ref, fontPx };
}

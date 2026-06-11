import type { ReactNode } from "react";

// Match the manifest `background_color` and the OS/apple-touch startup images
// byte-for-byte so the hand-off from the OS launch splash to this in-app splash
// is seamless. The `--brand-primary` design token is an oklch near-equivalent,
// not this exact hex, so the backdrop is pinned to the literal instead.
const BRAND_CORAL = "#FF6F79";

// Donut mark inlined as a path literal (Material "Donut Large") so the splash
// needs no extra network request — it paints with the first HTML chunk.
const DONUT_PATH =
  "M11.2999 21.925c-2.63335 -0.2 -4.8375 -1.24585 -6.6125 -3.1375 -1.775 -1.8917 -2.6625 -4.1542 -2.6625 -6.7875 0 -2.63335 0.8875 -4.89585 2.6625 -6.7875 1.775 -1.89168 3.97915 -2.93751 6.6125 -3.13751v2.55c-1.91665 0.18333 -3.52085 0.97916 -4.8125 2.38751 -1.29165 1.4083 -1.929165 3.0708 -1.9125 4.9875 0.01667 1.91665 0.6625 3.57915 1.9375 4.9875 1.275 1.4083 2.87085 2.20415 4.7875 2.3875v2.55Zm1.5 0v-2.55c1.76665 -0.15 3.25835 -0.85 4.475 -2.1 1.21665 -1.25 1.93335 -2.75835 2.15 -4.525h2.55c-0.18335 2.4833 -1.1375 4.59165 -2.8625 6.325 -1.725 1.7333 -3.82915 2.6833 -6.3125 2.85Zm6.625 -10.675c-0.2 -1.7667 -0.9125 -3.275 -2.1375 -4.525 -1.225 -1.25 -2.72085 -1.958345 -4.4875 -2.12501v-2.55c2.46665 0.18333 4.56665 1.141665 6.3 2.875 1.73335 1.73331 2.69165 3.84166 2.875 6.32501h-2.55Z";

type BrandSplashProps = {
  /** Rotate the mark (loading) or hold it static (a terminal/error state). */
  spin?: boolean;
  /**
   * Accessible name for the splash. When provided, the container is announced as
   * a `status` region (used for the loading state). Omit for content that
   * carries its own semantics (e.g. an error heading + retry button).
   */
  label?: string;
  /** Content rendered under the mark — a message, a retry control, etc. */
  children?: ReactNode;
};

/**
 * Full-viewport coral splash with the flamingo donut mark.
 *
 * Pure markup (no hooks, no client state), so it renders in both a server
 * boundary (`app/loading.tsx`) and a client boundary (`app/error.tsx`) and ships
 * zero client JavaScript when used from a server component. The mark spins with a
 * CSS-only animation that respects `prefers-reduced-motion`.
 */
export function BrandSplash({ spin = true, label, children }: BrandSplashProps) {
  return (
    <div
      {...(label ? { role: "status", "aria-label": label } : {})}
      style={{ backgroundColor: BRAND_CORAL }}
      className="fixed inset-0 z-50 flex flex-col items-center justify-center gap-6 px-6 text-center text-white"
    >
      <svg
        viewBox="0 0 24 24"
        aria-hidden="true"
        className={`size-14 fill-white${spin ? " motion-safe:animate-spin" : ""}`}
      >
        <path d={DONUT_PATH} />
      </svg>
      {children}
    </div>
  );
}

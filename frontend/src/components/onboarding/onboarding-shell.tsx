import type { ReactNode } from "react";
import { FlamingoMark } from "@/components/brand/flamingo-mark";

interface OnboardingShellProps {
  /** First-run heading rendered as the page's single `<h1>`. */
  heading: string;
  /** Optional supporting line under the heading. */
  subline?: string;
  /** Forwarded to the `<h1>` so callers can keep a stable anchor (e.g. e2e ids). */
  headingId?: string;
  /** Step-specific content rendered below the hero block. */
  children: ReactNode;
}

/**
 * Centered first-run hero shell shared by the bare onboarding routes
 * (`/onboarding`, `/onboarding/start`). It is the single source of truth for the
 * onboarding rhythm — FlamingoMark size, heading scale, and vertical spacing —
 * so the two consecutive wizard steps stay visually identical instead of each
 * re-spelling the layout. Presentational only (no I/O, no client hooks), so it
 * renders safely in either a server or client subtree.
 *
 * The shell centers its content (`text-center`); step content that must read
 * left-aligned (form fields, banners) opts back in with `text-left` on its own
 * container.
 */
export function OnboardingShell({ heading, subline, headingId, children }: OnboardingShellProps) {
  return (
    <div className="mx-auto max-w-2xl px-6 py-12 text-center sm:py-16">
      <FlamingoMark aria-hidden="true" className="mx-auto size-12" />
      <h1 id={headingId} className="mt-5 text-2xl font-semibold leading-[1.35] sm:text-3xl">
        {heading}
      </h1>
      {subline ? (
        <p className="mt-2 text-base leading-[1.7] tracking-[0.01em] text-muted-foreground sm:text-[17px]">
          {subline}
        </p>
      ) : null}
      {children}
    </div>
  );
}

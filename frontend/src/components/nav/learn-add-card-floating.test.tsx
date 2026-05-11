// @vitest-environment jsdom
import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

// Mock next/link so it renders a plain <a> in jsdom.
vi.mock("next/link", () => ({
  default: ({
    children,
    ...rest
  }: React.AnchorHTMLAttributes<HTMLAnchorElement> & { children?: React.ReactNode }) => (
    <a {...rest}>{children}</a>
  ),
}));

import { LearnAddCardFloating } from "./learn-add-card-floating";

describe("<LearnAddCardFloating>", () => {
  it("S1: href includes cardgroupId in both query param and return path", () => {
    render(<LearnAddCardFloating cardgroupId="abc-123" cardgroupName="Test Group" />);
    const link = screen.getByRole("link");
    expect(link).toHaveAttribute("href", "/cards/new?cardgroup=abc-123&return=/learn/abc-123");
  });

  it("S2: aria-label embeds cardgroupName", () => {
    render(<LearnAddCardFloating cardgroupId="abc-123" cardgroupName="My Vocabulary" />);
    // aria-label is on the Button (which renders as the outer element wrapping the Link via asChild)
    // With asChild, Radix Slot merges props onto the child <a>; the aria-label lands on the <a>.
    const link = screen.getByRole("link");
    expect(link).toHaveAttribute("aria-label", "Add a new card to My Vocabulary");
  });

  it("S3: renders as a visible ghost-style icon button positioned at the Learn header edge", () => {
    render(<LearnAddCardFloating cardgroupId="abc-123" cardgroupName="Test Group" />);
    const link = screen.getByRole("link");
    // The Button renders with asChild, so the className lands on the <a> element.
    expect(link.className).not.toContain("hidden");
    expect(link).toHaveClass("fixed", "top-1", "right-2", "rounded-md", "text-foreground");
    expect(link).toHaveClass("md:top-4", "md:right-6");
  });

  it("S4: URL-encodes cardgroupId containing special characters — href uses percent-encoding, aria-label uses raw name", () => {
    // "abc&evil" must become "abc%26evil" in both the query param and the return path.
    render(<LearnAddCardFloating cardgroupId="abc&evil" cardgroupName="Tricky & Group" />);
    const link = screen.getByRole("link");
    expect(link).toHaveAttribute(
      "href",
      "/cards/new?cardgroup=abc%26evil&return=/learn/abc%26evil",
    );
    // The aria-label uses the raw cardgroupName, not the id — no encoding needed here.
    expect(link).toHaveAttribute("aria-label", "Add a new card to Tricky & Group");
  });
});

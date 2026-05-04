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
    expect(link).toHaveAttribute(
      "href",
      "/cards/new?cardgroup=abc-123&return=/learn/abc-123",
    );
  });

  it("S2: aria-label embeds cardgroupName", () => {
    render(<LearnAddCardFloating cardgroupId="abc-123" cardgroupName="My Vocabulary" />);
    // aria-label is on the Button (which renders as the outer element wrapping the Link via asChild)
    // With asChild, Radix Slot merges props onto the child <a>; the aria-label lands on the <a>.
    const link = screen.getByRole("link");
    expect(link).toHaveAttribute("aria-label", "Add a new card to My Vocabulary");
  });

  it("S3: has 'hidden' and 'md:flex' classes — PC-only because mobile uses GlobalFAB", () => {
    render(<LearnAddCardFloating cardgroupId="abc-123" cardgroupName="Test Group" />);
    const link = screen.getByRole("link");
    // The Button renders with asChild, so the className lands on the <a> element.
    expect(link.className).toContain("hidden");
    expect(link.className).toContain("md:flex");
  });
});

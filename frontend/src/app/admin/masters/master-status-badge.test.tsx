// @vitest-environment jsdom

import { screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { renderWithIntl } from "@/test/render-with-intl";
import { MasterStatusBadge } from "./master-status-badge";

/** The status dot is the badge's only `aria-hidden` element. */
function statusDot(badge: HTMLElement): Element | null {
  return badge.querySelector('[aria-hidden="true"]');
}

describe("MasterStatusBadge", () => {
  it("published: renders the green (--success) status dot and the Published label", () => {
    renderWithIntl(<MasterStatusBadge published data-testid="status" />);
    const badge = screen.getByTestId("status");
    expect(badge).toHaveTextContent("Published");
    expect(statusDot(badge)?.className).toContain("bg-success");
  });

  it("draft: renders the muted status dot and the Draft label", () => {
    renderWithIntl(<MasterStatusBadge published={false} data-testid="status" />);
    const badge = screen.getByTestId("status");
    expect(badge).toHaveTextContent("Draft");
    expect(statusDot(badge)?.className).toContain("bg-muted-foreground");
  });

  it("forwards layout className onto the badge", () => {
    renderWithIntl(
      <MasterStatusBadge published data-testid="status" className="shrink-0 sm:order-1" />,
    );
    expect(screen.getByTestId("status").className).toContain("shrink-0");
  });

  it("carries role=status so the publish state is announced", () => {
    renderWithIntl(<MasterStatusBadge published={false} data-testid="status" />);
    expect(screen.getByTestId("status")).toHaveAttribute("role", "status");
  });
});

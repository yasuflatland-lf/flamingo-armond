// @vitest-environment happy-dom

import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { StatTile } from "./stat-tile";

describe("StatTile", () => {
  it("renders the label, the value string verbatim, and the caption when provided", () => {
    render(<StatTile label="Retention" value="87.5%" caption="last 30 days" />);
    expect(screen.getByText("Retention")).toBeDefined();
    expect(screen.getByText("87.5%")).toBeDefined();
    expect(screen.getByText("last 30 days")).toBeDefined();
  });

  it("renders an svg when an icon component is passed", () => {
    const Star = (props: { className?: string }) => <svg data-testid="ico" {...props} />;
    const { container } = render(<StatTile label="Streak" value="12" icon={Star} />);
    expect(container.querySelector("svg")).not.toBeNull();
    expect(screen.getByTestId("ico")).toBeDefined();
  });

  it("renders no caption element when caption is omitted", () => {
    render(<StatTile label="Due" value="42" />);
    expect(screen.queryByText("last 30 days")).toBeNull();
    // Only the label and value paragraphs exist; no third muted caption line.
    const muted = document.querySelectorAll("p.text-muted-foreground");
    expect(muted.length).toBe(0);
  });
});

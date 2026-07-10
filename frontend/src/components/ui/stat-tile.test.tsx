// @vitest-environment happy-dom

import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { StatTile } from "./stat-tile";

function Star(props: { className?: string }) {
  return <svg data-testid="ico" {...props} />;
}

describe("StatTile", () => {
  it("renders the label, the value string verbatim, and the caption when provided", () => {
    render(<StatTile label="Retention" value="87.5%" caption="last 30 days" />);
    expect(screen.getByText("Retention")).toBeDefined();
    expect(screen.getByText("87.5%")).toBeDefined();
    expect(screen.getByText("last 30 days")).toBeDefined();
  });

  it("renders an svg when an icon component is passed", () => {
    const { container } = render(<StatTile label="Streak" value="12" icon={Star} />);
    expect(container.querySelector("svg")).not.toBeNull();
    expect(screen.getByTestId("ico")).toBeDefined();
  });

  it("renders no caption element when caption is omitted", () => {
    const { container } = render(<StatTile label="Due" value="42" />);
    expect(screen.queryByText("last 30 days")).toBeNull();
    // Behaviour, not markup shape: only the value paragraph exists; the caption
    // <p> is not emitted (label is a <span>, so paragraph count === 1).
    expect(container.querySelectorAll("p").length).toBe(1);
  });

  it("marks the value with tabular-nums so columns of tiles align", () => {
    render(<StatTile label="Reviews" value="1,430" />);
    expect(screen.getByText("1,430").className).toContain("tabular-nums");
  });

  it("forwards the muted icon sizing classes to the icon", () => {
    render(<StatTile label="Streak" value="12" icon={Star} />);
    const cls = screen.getByTestId("ico").getAttribute("class") ?? "";
    expect(cls).toContain("h-4");
    expect(cls).toContain("w-4");
    expect(cls).toContain("text-muted-foreground");
  });
});

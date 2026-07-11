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

  it("renders the action node in the header when provided", () => {
    render(<StatTile label="Streak" value="12" action={<button type="button">act</button>} />);
    expect(screen.getByRole("button", { name: "act" })).toBeInTheDocument();
  });

  it("renders no action element when action is omitted", () => {
    render(<StatTile label="Due" value="42" />);
    expect(screen.queryByRole("button")).toBeNull();
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
});

// @vitest-environment happy-dom

import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { EmptyState } from "./empty-state";

describe("EmptyState", () => {
  it("renders the body copy", () => {
    render(<EmptyState body="Nothing here yet" />);
    expect(screen.getByText("Nothing here yet")).toBeInTheDocument();
  });

  it("renders the heading as a heading element when provided", () => {
    render(<EmptyState heading="All caught up" body="Come back later" />);
    expect(screen.getByRole("heading", { name: "All caught up" })).toBeInTheDocument();
  });

  it("renders no heading element when heading is omitted", () => {
    render(<EmptyState body="Body only" />);
    expect(screen.queryByRole("heading")).toBeNull();
  });

  it("renders the icon and action slots when provided", () => {
    render(
      <EmptyState
        icon={<svg aria-label="spark" role="img" />}
        heading="Welcome"
        body="Get started"
        actions={<button type="button">Browse</button>}
      />,
    );
    expect(screen.getByLabelText("spark")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Browse" })).toBeInTheDocument();
  });

  it("renders no action wrapper when actions is omitted", () => {
    render(<EmptyState heading="Done" body="Nothing to do" />);
    expect(screen.queryByRole("button")).toBeNull();
  });

  it("exposes the container via testId", () => {
    render(<EmptyState body="Empty" testId="my-empty" />);
    expect(screen.getByTestId("my-empty")).toBeInTheDocument();
  });

  it("carries the default dashed-border look and merges className overrides", () => {
    render(<EmptyState body="Empty" testId="styled" className="p-6 rounded-md" />);
    const el = screen.getByTestId("styled");
    expect(el.className).toContain("border-dashed");
    // tailwind-merge keeps the override and drops the conflicting default.
    expect(el.className).toContain("p-6");
    expect(el.className).not.toContain("p-8");
    expect(el.className).toContain("rounded-md");
    expect(el.className).not.toContain("rounded-lg");
  });

  it("merges heading and body className overrides over the defaults", () => {
    render(
      <EmptyState
        heading="Calm"
        body="Nothing tricky"
        headingClassName="text-sm"
        bodyClassName="text-xs max-w-xs"
      />,
    );
    const heading = screen.getByRole("heading", { name: "Calm" });
    expect(heading.className).toContain("text-sm");
    expect(heading.className).not.toContain("text-xl");
    const body = screen.getByText("Nothing tricky");
    expect(body.className).toContain("text-xs");
    expect(body.className).not.toContain("text-sm");
    expect(body.className).toContain("max-w-xs");
  });
});

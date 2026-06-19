// @vitest-environment jsdom
import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { DetailPageHeader } from "./detail-page-header";

describe("DetailPageHeader", () => {
  it("renders an inline back link with an sr-only label and the title", () => {
    render(
      <DetailPageHeader
        backHref="/admin/masters"
        backLabel="Back to list"
        title="Advanced Cards"
      />,
    );
    const back = screen.getByRole("link", { name: "Back to list" });
    expect(back).toHaveAttribute("href", "/admin/masters");
    expect(screen.getByRole("heading", { level: 1, name: "Advanced Cards" })).toBeInTheDocument();
  });

  it("renders meta, actions, and children slots when provided", () => {
    render(
      <DetailPageHeader
        backHref="/x"
        backLabel="back"
        title="T"
        meta={<span data-testid="meta-slot">meta</span>}
        actions={
          <button type="button" data-testid="actions-slot">
            a
          </button>
        }
      >
        <p data-testid="children-slot">hint</p>
      </DetailPageHeader>,
    );
    expect(screen.getByTestId("meta-slot")).toBeInTheDocument();
    expect(screen.getByTestId("actions-slot")).toBeInTheDocument();
    expect(screen.getByTestId("children-slot")).toBeInTheDocument();
  });

  it("omits meta and actions wrappers when not provided", () => {
    const { container } = render(<DetailPageHeader backHref="/x" backLabel="back" title="T" />);
    // Only the back link + title row exists; no stray slot containers.
    expect(screen.queryByTestId("meta-slot")).toBeNull();
    expect(screen.queryByTestId("actions-slot")).toBeNull();
    expect(container.querySelector("h1")?.textContent).toBe("T");
  });
});

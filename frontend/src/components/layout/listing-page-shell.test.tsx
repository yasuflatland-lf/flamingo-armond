// @vitest-environment happy-dom
import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { ListingPageShell } from "./listing-page-shell";

describe("<ListingPageShell>", () => {
  it("renders the title as a top-level heading and the children body", () => {
    render(
      <ListingPageShell title="Users">
        <div data-testid="body">body</div>
      </ListingPageShell>,
    );

    expect(screen.getByRole("heading", { level: 1, name: "Users" })).toBeInTheDocument();
    expect(screen.getByTestId("body")).toBeInTheDocument();
  });

  it("renders the description below the title when provided", () => {
    render(
      <ListingPageShell title="Users" description="Manage users and their roles.">
        <div />
      </ListingPageShell>,
    );

    expect(screen.getByText("Manage users and their roles.")).toBeInTheDocument();
  });

  it("does not render a description block when description is omitted", () => {
    const { container } = render(
      <ListingPageShell title="Users">
        <div />
      </ListingPageShell>,
    );

    const descriptions = container.querySelectorAll(".text-muted-foreground");
    expect(descriptions.length).toBe(0);
  });

  it("renders primaryActions on the trailing edge of the header row on desktop", () => {
    render(
      <ListingPageShell title="Users" primaryActions={<button type="button">Invite User</button>}>
        <div />
      </ListingPageShell>,
    );

    const button = screen.getByRole("button", { name: "Invite User" });
    // The actions cluster is pushed to the trailing edge of the row on desktop.
    expect(button.parentElement).toHaveClass("md:justify-end");
  });

  it("renders the toolbar slot between the header row and the body", () => {
    render(
      <ListingPageShell title="Users" toolbar={<input type="search" aria-label="Search users" />}>
        <div data-testid="body">body</div>
      </ListingPageShell>,
    );

    const toolbarWrapper = screen.getByLabelText("Search users").closest('[data-slot="toolbar"]');
    expect(toolbarWrapper).not.toBeNull();
  });

  it("does not render the toolbar wrapper when toolbar is omitted", () => {
    const { container } = render(
      <ListingPageShell title="Users">
        <div />
      </ListingPageShell>,
    );

    expect(container.querySelector('[data-slot="toolbar"]')).toBeNull();
  });

  it("forwards a className to the outer <main>", () => {
    render(
      <ListingPageShell title="Users" className="custom-shell">
        <div data-testid="body" />
      </ListingPageShell>,
    );

    const main = screen.getByTestId("body").closest("main");
    expect(main?.className).toContain("custom-shell");
    // The base liquid-layout classes are preserved.
    expect(main?.className).toContain("p-8");
  });

  it("renders the count as a muted pill sharing the title cluster, inline beside the title on desktop", () => {
    render(
      <ListingPageShell title="Users" count={42}>
        <div />
      </ListingPageShell>,
    );

    const heading = screen.getByRole("heading", { level: 1, name: "Users" });
    const count = screen.getByText("42");
    // Muted, pill-shaped, and a static label (never an interactive control).
    expect(count).toHaveClass("text-muted-foreground", "rounded-full");
    expect(count.tagName).toBe("SPAN");
    // Count and title share one cluster so the count sits next to the heading.
    expect(count.parentElement).toBe(heading.parentElement);
    // Desktop lays them out on one baseline-aligned row (mobile stacks them).
    expect(heading.parentElement).toHaveClass("md:flex-row", "md:items-baseline");
  });

  it("renders countLabel inside the pill when provided", () => {
    render(
      <ListingPageShell title="Users" count={42} countLabel="42 total">
        <div />
      </ListingPageShell>,
    );

    const pill = screen.getByText("42 total");
    expect(pill).toHaveClass("rounded-full", "text-muted-foreground");
    expect(pill.tagName).toBe("SPAN");
    // The bare number is replaced by the label, not shown alongside it.
    expect(screen.queryByText("42")).toBeNull();
  });

  it("does not render the count pill when count is omitted", () => {
    render(
      <ListingPageShell title="Users">
        <div />
      </ListingPageShell>,
    );

    const heading = screen.getByRole("heading", { level: 1, name: "Users" });
    expect(heading.parentElement?.querySelector(".rounded-full")).toBeNull();
  });

  it("renders the title with the shared page-title type token", () => {
    render(
      <ListingPageShell title="Users">
        <div />
      </ListingPageShell>,
    );

    expect(screen.getByRole("heading", { level: 1, name: "Users" })).toHaveClass("text-page-title");
  });

  it("centers the header on mobile and left-aligns it with a justify-between row on desktop", () => {
    render(
      <ListingPageShell title="Users">
        <div />
      </ListingPageShell>,
    );

    const heading = screen.getByRole("heading", { level: 1, name: "Users" });
    const header = heading.closest('[data-slot="page-header"]');
    expect(header).not.toBeNull();
    // Mobile: centered column.
    expect(header).toHaveClass("flex-col", "items-center", "text-center");
    // Desktop: left-aligned row with the trailing edge reserved for actions.
    expect(header).toHaveClass("md:flex-row", "md:justify-between", "md:text-left");
  });
});

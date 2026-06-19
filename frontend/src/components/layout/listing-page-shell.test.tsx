// @vitest-environment jsdom
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

  it("renders primaryActions in the trailing edge of the header row", () => {
    render(
      <ListingPageShell title="Users" primaryActions={<button type="button">Invite User</button>}>
        <div />
      </ListingPageShell>,
    );

    expect(screen.getByRole("button", { name: "Invite User" })).toBeInTheDocument();
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

  it("renders the count as a parenthesized muted subtitle when count is provided", () => {
    render(
      <ListingPageShell title="Users" count={42}>
        <div />
      </ListingPageShell>,
    );

    expect(screen.getByText("(42)")).toBeInTheDocument();
  });

  it("renders the title with the shared page-title type token", () => {
    render(
      <ListingPageShell title="Users">
        <div />
      </ListingPageShell>,
    );

    expect(screen.getByRole("heading", { level: 1, name: "Users" })).toHaveClass("text-page-title");
  });

  it("centers the header block so the title, count, and actions stack centered", () => {
    render(
      <ListingPageShell title="Users">
        <div />
      </ListingPageShell>,
    );

    const heading = screen.getByRole("heading", { level: 1, name: "Users" });
    expect(heading.parentElement).toHaveClass("items-center", "text-center");
  });
});

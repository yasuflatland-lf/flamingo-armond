// @vitest-environment jsdom
import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { AdminPageShell } from "./admin-page-shell";

describe("<AdminPageShell>", () => {
  it("renders the title as a top-level heading and the children body", () => {
    render(
      <AdminPageShell title="Users">
        <div data-testid="body">body</div>
      </AdminPageShell>,
    );

    expect(screen.getByRole("heading", { level: 1, name: "Users" })).toBeInTheDocument();
    expect(screen.getByTestId("body")).toBeInTheDocument();
  });

  it("renders the description below the title when provided", () => {
    render(
      <AdminPageShell title="Users" description="Manage users and their roles.">
        <div />
      </AdminPageShell>,
    );

    expect(screen.getByText("Manage users and their roles.")).toBeInTheDocument();
  });

  it("does not render a description paragraph when description is omitted", () => {
    const { container } = render(
      <AdminPageShell title="Users">
        <div />
      </AdminPageShell>,
    );

    // The header row holds the title + (optional) description in the same div.
    // When description is omitted, no <p> sibling should appear.
    const paragraphs = container.querySelectorAll("p");
    expect(paragraphs.length).toBe(0);
  });

  it("renders primaryActions in the trailing edge of the header row", () => {
    render(
      <AdminPageShell title="Users" primaryActions={<button type="button">Invite User</button>}>
        <div />
      </AdminPageShell>,
    );

    expect(screen.getByRole("button", { name: "Invite User" })).toBeInTheDocument();
  });

  it("renders the toolbar slot between the header row and the body", () => {
    render(
      <AdminPageShell title="Users" toolbar={<input type="search" aria-label="Search users" />}>
        <div data-testid="body">body</div>
      </AdminPageShell>,
    );

    const toolbarWrapper = screen.getByLabelText("Search users").closest('[data-slot="toolbar"]');
    expect(toolbarWrapper).not.toBeNull();
  });

  it("does not render the toolbar wrapper when toolbar is omitted", () => {
    const { container } = render(
      <AdminPageShell title="Users">
        <div />
      </AdminPageShell>,
    );

    expect(container.querySelector('[data-slot="toolbar"]')).toBeNull();
  });

  it("forwards a className to the outer <main>", () => {
    render(
      <AdminPageShell title="Users" className="custom-shell">
        <div data-testid="body" />
      </AdminPageShell>,
    );

    const main = screen.getByTestId("body").closest("main");
    expect(main?.className).toContain("custom-shell");
    // The base liquid-layout classes are preserved.
    expect(main?.className).toContain("p-8");
  });
});

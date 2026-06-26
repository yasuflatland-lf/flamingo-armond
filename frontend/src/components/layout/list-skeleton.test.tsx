// @vitest-environment happy-dom
import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { ListSkeleton } from "./list-skeleton";

describe("<ListSkeleton>", () => {
  it("renders a busy list with the provided aria-label and testId", () => {
    render(
      <ListSkeleton
        ariaLabel="Loading users"
        testId="admin-users-skeleton"
        renderRow={() => <div data-testid="row-body" />}
      />,
    );

    const list = screen.getByTestId("admin-users-skeleton");
    expect(list.tagName).toBe("UL");
    expect(list).toHaveAttribute("aria-busy", "true");
    expect(list).toHaveAccessibleName("Loading users");
  });

  it("renders five placeholder rows by default", () => {
    render(
      <ListSkeleton
        ariaLabel="Loading users"
        testId="admin-users-skeleton"
        renderRow={() => <div data-testid="row-body" />}
      />,
    );

    expect(screen.getAllByTestId("row-body")).toHaveLength(5);
    expect(screen.getByTestId("admin-users-skeleton").querySelectorAll("li")).toHaveLength(5);
  });

  it("renders the requested number of rows when rowCount is provided", () => {
    render(
      <ListSkeleton
        rowCount={3}
        ariaLabel="Loading roles"
        testId="admin-roles-skeleton"
        renderRow={() => <div data-testid="row-body" />}
      />,
    );

    expect(screen.getAllByTestId("row-body")).toHaveLength(3);
  });

  it("renders the default header (two title Skeletons + one search Skeleton) when no header is passed", () => {
    const { container } = render(
      <ListSkeleton
        ariaLabel="Loading users"
        testId="admin-users-skeleton"
        // The header occupies the only Skeletons outside the rows (renderRow has none).
        renderRow={() => <div />}
      />,
    );

    // Default header: two title Skeletons + one search Skeleton, all outside the <ul>.
    const headerSkeletons = container.querySelectorAll("main > div .animate-pulse");
    expect(headerSkeletons).toHaveLength(3);
  });

  it("renders a custom header in place of the default, emitting no default Skeletons", () => {
    const { container } = render(
      <ListSkeleton
        ariaLabel="Loading roles"
        testId="admin-roles-skeleton"
        header={<div data-testid="custom-header" />}
        renderRow={() => <div />}
      />,
    );

    expect(screen.getByTestId("custom-header")).toBeInTheDocument();
    // No default title/search Skeletons leak in alongside the custom header.
    expect(container.querySelectorAll(".animate-pulse")).toHaveLength(0);
  });

  it("renders the list inside a <main> landmark carrying the base p-8 padding", () => {
    render(
      <ListSkeleton
        ariaLabel="Loading users"
        testId="admin-users-skeleton"
        renderRow={() => <div />}
      />,
    );

    const main = screen.getByTestId("admin-users-skeleton").closest("main");
    expect(main).not.toBeNull();
    expect(main).toHaveClass("p-8");
  });

  it("merges className onto the outer <main> and keeps the base padding", () => {
    render(
      <ListSkeleton
        className="flex flex-1 flex-col"
        ariaLabel="Loading roles"
        testId="admin-roles-skeleton"
        renderRow={() => <div />}
      />,
    );

    const main = screen.getByTestId("admin-roles-skeleton").closest("main");
    expect(main).toHaveClass("p-8", "flex", "flex-1", "flex-col");
  });

  it("merges rowClassName onto each placeholder <li> alongside the base border classes", () => {
    render(
      <ListSkeleton
        rowClassName="px-4 py-3"
        ariaLabel="Loading roles"
        testId="admin-roles-skeleton"
        renderRow={() => <div />}
      />,
    );

    const items = screen.getByTestId("admin-roles-skeleton").querySelectorAll("li");
    for (const item of items) {
      expect(item).toHaveClass("rounded-md", "border", "border-border", "px-4", "py-3");
    }
  });
});

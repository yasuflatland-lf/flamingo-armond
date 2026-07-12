// @vitest-environment happy-dom
import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { ListSkeleton, SkeletonRows } from "./list-skeleton";

describe("<SkeletonRows>", () => {
  it("renders a busy list with the provided aria-label and testId", () => {
    render(
      <SkeletonRows
        rowCount={4}
        ariaLabel="Loading cardgroups"
        testId="cardgroups-skeleton"
        renderRow={() => <div data-testid="row-body" />}
      />,
    );

    const list = screen.getByTestId("cardgroups-skeleton");
    expect(list.tagName).toBe("UL");
    expect(list).toHaveAttribute("aria-busy", "true");
    expect(list).toHaveAccessibleName("Loading cardgroups");
    expect(screen.getAllByTestId("row-body")).toHaveLength(4);
  });

  it("applies the default space-y-3 list spacing when no className is passed", () => {
    render(
      <SkeletonRows
        rowCount={1}
        ariaLabel="Loading cardgroups"
        testId="cardgroups-skeleton"
        renderRow={() => <div />}
      />,
    );

    expect(screen.getByTestId("cardgroups-skeleton")).toHaveClass("space-y-3");
  });

  it("merges className onto the outer <ul> over the default spacing", () => {
    render(
      <SkeletonRows
        rowCount={1}
        ariaLabel="Loading catalog"
        testId="catalog-skeleton"
        className="space-y-2"
        renderRow={() => <div />}
      />,
    );

    expect(screen.getByTestId("catalog-skeleton")).toHaveClass("space-y-2");
  });

  it("applies rowClassName to each placeholder <li> verbatim", () => {
    render(
      <SkeletonRows
        rowCount={3}
        ariaLabel="Loading cardgroups"
        testId="cardgroups-skeleton"
        rowClassName="rounded-lg border border-border p-4"
        renderRow={() => <div />}
      />,
    );

    const items = screen.getByTestId("cardgroups-skeleton").querySelectorAll("li");
    expect(items).toHaveLength(3);
    for (const item of items) {
      expect(item).toHaveClass("rounded-lg", "border", "border-border", "p-4");
    }
  });
});

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

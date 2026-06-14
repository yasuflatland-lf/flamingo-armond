// @vitest-environment jsdom
import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { ErrorBanner } from "./error-banner";

describe("ErrorBanner", () => {
  it("renders its children", () => {
    render(<ErrorBanner>Something went wrong</ErrorBanner>);
    expect(screen.getByText("Something went wrong")).toBeInTheDocument();
  });

  it("defaults role to alert and carries the base destructive classes", () => {
    render(<ErrorBanner>boom</ErrorBanner>);
    const banner = screen.getByRole("alert");
    expect(banner).toHaveClass(
      "rounded-md",
      "bg-destructive/10",
      "p-3",
      "text-sm",
      "text-destructive",
    );
  });

  it("merges a passed className with the base classes", () => {
    render(<ErrorBanner className="mt-3 flex">boom</ErrorBanner>);
    const banner = screen.getByRole("alert");
    expect(banner).toHaveClass("mt-3", "flex", "rounded-md", "bg-destructive/10");
  });

  it("forwards arbitrary DOM props such as data-testid", () => {
    render(<ErrorBanner data-testid="my-banner">boom</ErrorBanner>);
    expect(screen.getByTestId("my-banner")).toBeInTheDocument();
  });

  it("allows the caller to override the default role", () => {
    render(<ErrorBanner role="status">boom</ErrorBanner>);
    expect(screen.getByRole("status")).toBeInTheDocument();
    expect(screen.queryByRole("alert")).toBeNull();
  });
});

// @vitest-environment jsdom
import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { BrandSplash } from "./brand-splash";

describe("<BrandSplash>", () => {
  it("pins the coral backdrop to the literal manifest hex for a seamless OS hand-off", () => {
    const { container } = render(<BrandSplash label="Loading" />);
    const root = container.firstElementChild as HTMLElement;
    expect(root).toHaveStyle({ backgroundColor: "#FF6F79" });
  });

  it("paints the donut mark with the pinned near-black hex via inline style", () => {
    // The mark color must be an inline style, not a Tailwind `fill-*` class, so
    // it survives the first paint before the stylesheet loads (the cause of the
    // mark rendering as default black on a real PWA launch).
    const { container } = render(<BrandSplash label="Loading" />);
    expect(container.querySelector("svg")).toHaveStyle({ fill: "#1f1f1f" });
  });

  it("exposes the accessible label as a status region", () => {
    render(<BrandSplash label="Loading" />);
    expect(screen.getByRole("status", { name: "Loading" })).toBeInTheDocument();
  });

  it("omits the status role when no label is given so caller semantics own the region", () => {
    render(
      <BrandSplash>
        <p>content</p>
      </BrandSplash>,
    );
    expect(screen.queryByRole("status")).not.toBeInTheDocument();
    expect(screen.getByText("content")).toBeInTheDocument();
  });

  it("spins the mark by default and holds it static when spin is false", () => {
    const { container, rerender } = render(<BrandSplash label="Loading" />);
    expect(container.querySelector("svg")).toHaveClass("motion-safe:animate-spin");

    rerender(<BrandSplash label="Loading" spin={false} />);
    expect(container.querySelector("svg")).not.toHaveClass("motion-safe:animate-spin");
  });

  it("marks the donut decorative with aria-hidden so it is not announced", () => {
    const { container } = render(<BrandSplash label="Loading" />);
    // The status region already names the splash "Loading"; the mark must stay
    // out of the accessibility tree to avoid double-announcing.
    expect(container.querySelector("svg")).toHaveAttribute("aria-hidden", "true");
  });
});

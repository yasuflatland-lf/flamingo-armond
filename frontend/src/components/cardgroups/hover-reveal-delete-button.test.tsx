// @vitest-environment jsdom
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { HoverRevealDeleteButton } from "./hover-reveal-delete-button";

describe("<HoverRevealDeleteButton>", () => {
  it("renders a button with the given accessible name", () => {
    render(<HoverRevealDeleteButton ariaLabel="Delete widget" onDelete={vi.fn()} />);
    expect(screen.getByRole("button", { name: "Delete widget" })).toBeInTheDocument();
  });

  it("forwards data-testid to the rendered button", () => {
    render(
      <HoverRevealDeleteButton
        ariaLabel="Delete widget"
        onDelete={vi.fn()}
        data-testid="delete-widget-1"
      />,
    );
    expect(screen.getByTestId("delete-widget-1")).toBeInTheDocument();
  });

  it("fires onDelete when clicked", async () => {
    const user = userEvent.setup();
    const onDelete = vi.fn();
    render(<HoverRevealDeleteButton ariaLabel="Delete widget" onDelete={onDelete} />);

    await user.click(screen.getByRole("button", { name: "Delete widget" }));
    expect(onDelete).toHaveBeenCalledTimes(1);
  });

  it("disables the button (and blocks the click) when disabled is true", async () => {
    const user = userEvent.setup();
    const onDelete = vi.fn();
    render(<HoverRevealDeleteButton ariaLabel="Delete widget" onDelete={onDelete} disabled />);

    const btn = screen.getByRole("button", { name: "Delete widget" });
    expect(btn).toBeDisabled();
    await user.click(btn);
    expect(onDelete).not.toHaveBeenCalled();
  });

  it("carries the base reveal classes so reduced-motion users always see it", () => {
    // motion-reduce:opacity-100 is the load-bearing partner to SwipeableRow's
    // reduced-motion early-return: when the swipe layer is absent this is the
    // only delete affordance, so it must reveal under prefers-reduced-motion.
    render(<HoverRevealDeleteButton ariaLabel="Delete widget" onDelete={vi.fn()} />);
    const cls = screen.getByRole("button", { name: "Delete widget" }).className;
    expect(cls).toContain("opacity-0");
    expect(cls).toContain("sm:group-hover:opacity-100");
    expect(cls).toContain("motion-reduce:opacity-100");
    expect(cls).toContain("transition-opacity");
  });

  it("appends the per-row className override on top of the base reveal classes", () => {
    // card-row passes pointer-events / focus-within tokens its overlay needs;
    // the base reveal classes must survive the merge alongside the override.
    render(
      <HoverRevealDeleteButton
        ariaLabel="Delete widget"
        onDelete={vi.fn()}
        className="pointer-events-none sm:group-focus-within:opacity-100"
      />,
    );
    const cls = screen.getByRole("button", { name: "Delete widget" }).className;
    expect(cls).toContain("pointer-events-none");
    expect(cls).toContain("sm:group-focus-within:opacity-100");
    // Base reveal tokens are not clobbered by the override.
    expect(cls).toContain("motion-reduce:opacity-100");
  });

  it("renders the outline icon-button variant", () => {
    render(<HoverRevealDeleteButton ariaLabel="Delete widget" onDelete={vi.fn()} />);
    const cls = screen.getByRole("button", { name: "Delete widget" }).className;
    // outline variant border + icon size (h-10 w-10) from buttonVariants.
    expect(cls).toContain("border");
    expect(cls).toContain("w-10");
  });
});

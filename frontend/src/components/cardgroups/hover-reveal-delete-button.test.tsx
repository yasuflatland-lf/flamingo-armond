// @vitest-environment happy-dom
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
    // Every opacity arm must carry a matching pointer-events arm ("visible iff
    // tappable"): opacity-0 alone leaves the hidden button clickable, so a
    // consumer relying on the base alone would ship an invisible mobile tap
    // target without the guard here.
    render(<HoverRevealDeleteButton ariaLabel="Delete widget" onDelete={vi.fn()} />);
    const cls = screen.getByRole("button", { name: "Delete widget" }).className;
    expect(cls).toContain("opacity-0");
    expect(cls).toContain("sm:group-hover:opacity-100");
    expect(cls).toContain("motion-reduce:opacity-100");
    expect(cls).toContain("transition-opacity");
    expect(cls).toContain("pointer-events-none");
    expect(cls).toContain("sm:group-hover:pointer-events-auto");
    expect(cls).toContain("sm:group-focus-within:pointer-events-auto");
    expect(cls).toContain("sm:group-focus-within:opacity-100");
    expect(cls).toContain("motion-reduce:pointer-events-auto");
  });

  it("appends the row-specific layout className on top of the base reveal classes", () => {
    // The reveal + pointer-events guard is base-owned, so the override token
    // here is a layout token deliberately absent from the base — a guard token
    // would pass vacuously and prove nothing about additivity.
    render(
      <HoverRevealDeleteButton ariaLabel="Delete widget" onDelete={vi.fn()} className="mt-1" />,
    );
    const cls = screen.getByRole("button", { name: "Delete widget" }).className;
    expect(cls).toContain("mt-1");
    // Base reveal + guard tokens are not clobbered by the override.
    expect(cls).toContain("motion-reduce:opacity-100");
    expect(cls).toContain("pointer-events-none");
  });

  it("renders the outline icon-button variant", () => {
    render(<HoverRevealDeleteButton ariaLabel="Delete widget" onDelete={vi.fn()} />);
    const cls = screen.getByRole("button", { name: "Delete widget" }).className;
    // outline variant border + icon size (h-10 w-10) from buttonVariants.
    expect(cls).toContain("border");
    expect(cls).toContain("w-10");
  });
});

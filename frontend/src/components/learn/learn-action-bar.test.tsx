// @vitest-environment jsdom
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { LearnActionBar } from "./learn-action-bar";

describe("<LearnActionBar>", () => {
  it("renders the three rating buttons with labels and shortcuts", () => {
    render(<LearnActionBar onRate={vi.fn()} />);

    expect(screen.getByRole("button", { name: "Rate as Again" })).toHaveAttribute(
      "aria-keyshortcuts",
      "ArrowLeft",
    );
    expect(screen.getByRole("button", { name: "Rate as Hard" })).toHaveAttribute(
      "aria-keyshortcuts",
      "ArrowDown",
    );
    expect(screen.getByRole("button", { name: "Rate as Easy" })).toHaveAttribute(
      "aria-keyshortcuts",
      "ArrowRight",
    );
    expect(screen.getByText("Again")).toBeInTheDocument();
    expect(screen.getByText("Hard")).toBeInTheDocument();
    expect(screen.getByText("Easy")).toBeInTheDocument();
  });

  it.each([
    ["Rate as Again", "left"],
    ["Rate as Hard", "down"],
    ["Rate as Easy", "right"],
  ] as const)("calls onRate with %s direction", async (name, direction) => {
    const user = userEvent.setup();
    const onRate = vi.fn();
    render(<LearnActionBar onRate={onRate} />);

    await user.click(screen.getByRole("button", { name }));

    expect(onRate).toHaveBeenCalledWith(direction);
  });

  it("disables all rating buttons", () => {
    render(<LearnActionBar onRate={vi.fn()} disabled />);

    expect(screen.getByRole("button", { name: "Rate as Again" })).toBeDisabled();
    expect(screen.getByRole("button", { name: "Rate as Hard" })).toBeDisabled();
    expect(screen.getByRole("button", { name: "Rate as Easy" })).toBeDisabled();
  });

  it("applies direction-specific outline colors", () => {
    render(<LearnActionBar onRate={vi.fn()} />);

    expect(screen.getByRole("button", { name: "Rate as Again" })).toHaveClass("border-red-600");
    expect(screen.getByRole("button", { name: "Rate as Hard" })).toHaveClass("border-sky-600");
    expect(screen.getByRole("button", { name: "Rate as Easy" })).toHaveClass("border-emerald-600");
  });

  it("keeps tab order as Again, Hard, Easy", async () => {
    const user = userEvent.setup();
    render(<LearnActionBar onRate={vi.fn()} />);

    await user.tab();
    expect(screen.getByRole("button", { name: "Rate as Again" })).toHaveFocus();
    await user.tab();
    expect(screen.getByRole("button", { name: "Rate as Hard" })).toHaveFocus();
    await user.tab();
    expect(screen.getByRole("button", { name: "Rate as Easy" })).toHaveFocus();
  });

  it("S-A1: outer container uses sticky positioning, not fixed", () => {
    const { container } = render(<LearnActionBar onRate={vi.fn()} />);
    const outer = container.firstElementChild as HTMLElement | null;
    expect(outer).not.toBeNull();
    expect(outer?.className).toContain("sticky");
    expect(outer?.className).not.toContain("fixed");
    expect(outer?.className).not.toContain("inset-x-0");
  });

  it("S-A2: outer container retains bottom-0 and centered horizontal flex", () => {
    const { container } = render(<LearnActionBar onRate={vi.fn()} />);
    const outer = container.firstElementChild as HTMLElement | null;
    expect(outer).not.toBeNull();
    expect(outer?.className).toContain("bottom-0");
    expect(outer?.className).toContain("justify-center");
  });
});

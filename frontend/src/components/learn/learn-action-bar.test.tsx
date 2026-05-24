// @vitest-environment jsdom
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { LearnActionBar } from "./learn-action-bar";

describe("<LearnActionBar>", () => {
  it("renders the three rating buttons with accessible labels and shortcuts but no visible text labels", () => {
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
    // The visible "Again"/"Hard"/"Easy" text labels were removed; the rating is
    // conveyed only via the button aria-label and color. The buttons remain.
    expect(screen.queryByText("Again")).not.toBeInTheDocument();
    expect(screen.queryByText("Hard")).not.toBeInTheDocument();
    expect(screen.queryByText("Easy")).not.toBeInTheDocument();
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

  it("disables all rating buttons and ignores clicks while disabled", async () => {
    const user = userEvent.setup();
    const onRate = vi.fn();
    render(<LearnActionBar onRate={onRate} disabled />);

    const again = screen.getByRole("button", { name: "Rate as Again" });
    const hard = screen.getByRole("button", { name: "Rate as Hard" });
    const easy = screen.getByRole("button", { name: "Rate as Easy" });

    expect(again).toBeDisabled();
    expect(hard).toBeDisabled();
    expect(easy).toBeDisabled();

    await user.click(again);
    await user.click(hard);
    await user.click(easy);

    expect(onRate).not.toHaveBeenCalled();
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
});

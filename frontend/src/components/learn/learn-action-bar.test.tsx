// @vitest-environment jsdom
import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { renderWithIntl } from "@/test/render-with-intl";
import { LearnActionBar } from "./learn-action-bar";

describe("<LearnActionBar>", () => {
  it("renders the three rating buttons with accessible labels and shortcuts but no visible text labels", () => {
    renderWithIntl(<LearnActionBar revealed={true} onRate={vi.fn()} />);

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
    renderWithIntl(<LearnActionBar revealed={true} onRate={onRate} />);

    await user.click(screen.getByRole("button", { name }));

    expect(onRate).toHaveBeenCalledWith(direction);
  });

  it("disables all rating buttons and ignores clicks while disabled", async () => {
    const user = userEvent.setup();
    const onRate = vi.fn();
    renderWithIntl(<LearnActionBar revealed={true} onRate={onRate} disabled />);

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
    renderWithIntl(<LearnActionBar revealed={true} onRate={vi.fn()} />);

    await user.tab();
    expect(screen.getByRole("button", { name: "Rate as Again" })).toHaveFocus();
    await user.tab();
    expect(screen.getByRole("button", { name: "Rate as Hard" })).toHaveFocus();
    await user.tab();
    expect(screen.getByRole("button", { name: "Rate as Easy" })).toHaveFocus();
  });

  it.each([
    ["Rate as Again", "ArrowLeft"],
    ["Rate as Hard", "ArrowDown"],
    ["Rate as Easy", "ArrowRight"],
  ] as const)("disables %s when revealed is false and enables it when revealed is true", async (name, shortcut) => {
    // Select locale-independently via aria-keyshortcuts so the assertion does
    // not depend on translated button copy. While the active card is
    // unrevealed (front_only phase) the rating buttons must be inert — the
    // learner reveals by tapping the card, not via this bar.
    const onRate = vi.fn();
    const { rerender } = renderWithIntl(<LearnActionBar revealed={false} onRate={onRate} />);

    const disabledButton = screen.getByRole("button", { name });
    expect(disabledButton).toHaveAttribute("aria-keyshortcuts", shortcut);
    expect(disabledButton).toBeDisabled();
    expect(disabledButton).toHaveAttribute("aria-disabled", "true");

    const user = userEvent.setup();
    await user.click(disabledButton);
    expect(onRate).not.toHaveBeenCalled();

    // Revealing the card enables the rating buttons.
    rerender(<LearnActionBar revealed={true} onRate={onRate} />);

    const enabledButton = screen.getByRole("button", { name });
    expect(enabledButton).toBeEnabled();
  });
});

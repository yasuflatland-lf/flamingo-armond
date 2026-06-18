// @vitest-environment jsdom
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { SplitButtonMenu } from "./split-button-menu";

describe("SplitButtonMenu", () => {
  it("renders an icon-only trigger labelled by triggerLabel", () => {
    render(
      <SplitButtonMenu
        triggerLabel="More options"
        data-testid="more"
        items={[{ key: "a", label: "Action A", onSelect: vi.fn() }]}
      />,
    );
    const trigger = screen.getByTestId("more");
    expect(trigger).toHaveAttribute("aria-label", "More options");
    // Closed by default: items are not in the document.
    expect(screen.queryByRole("menuitem", { name: "Action A" })).not.toBeInTheDocument();
  });

  it("reveals items on open and calls onSelect when one is chosen", async () => {
    const user = userEvent.setup();
    const onA = vi.fn();
    const onB = vi.fn();
    render(
      <SplitButtonMenu
        triggerLabel="More options"
        data-testid="more"
        items={[
          { key: "a", label: "Action A", onSelect: onA },
          { key: "b", label: "Action B", onSelect: onB },
        ]}
      />,
    );
    await user.click(screen.getByTestId("more"));
    await user.click(await screen.findByRole("menuitem", { name: "Action B" }));
    expect(onB).toHaveBeenCalledTimes(1);
    expect(onA).not.toHaveBeenCalled();
  });

  it("marks destructive items with the destructive intent and disables disabled items", async () => {
    const user = userEvent.setup();
    const onDelete = vi.fn();
    render(
      <SplitButtonMenu
        triggerLabel="More options"
        data-testid="more"
        items={[
          { key: "del", label: "Delete", onSelect: onDelete, destructive: true },
          { key: "off", label: "Disabled", onSelect: vi.fn(), disabled: true },
        ]}
      />,
    );
    await user.click(screen.getByTestId("more"));
    const del = await screen.findByRole("menuitem", { name: "Delete" });
    expect(del.className).toContain("text-destructive");
    const disabled = screen.getByRole("menuitem", { name: "Disabled" });
    expect(disabled).toHaveAttribute("aria-disabled", "true");
  });
});

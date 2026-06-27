// @vitest-environment happy-dom
import { screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { renderWithIntl } from "@/test/render-with-intl";
import { BulkActionBar } from "./bulk-action-bar";

describe("BulkActionBar hierarchy", () => {
  function renderBar() {
    renderWithIntl(<BulkActionBar count={2} busy={false} onConfirm={vi.fn()} onClear={vi.fn()} />);
  }

  // Delete is the bar's emphasized primary: the only bordered button, carrying a
  // visible red border for affordance/findability while staying a low-emphasis
  // ghost (transparent, red text — never the filled crimson commit).
  it("renders Delete selected as the only bordered, red-text action", () => {
    renderBar();
    const del = screen.getByTestId("cards-bulk-delete-button");
    expect(del).toHaveClass("border", "border-destructive/45", "text-destructive");
    // Still a trigger, not the filled commit.
    expect(del).not.toHaveClass("bg-destructive");
  });

  // Cancel is the quiet escape: demoted from outline to ghost, so it carries no
  // border and does not compete with Delete for emphasis.
  it("renders Cancel as a borderless ghost escape", () => {
    renderBar();
    const cancel = screen.getByRole("button", { name: /^cancel$/i });
    expect(cancel).not.toHaveClass("border");
    expect(cancel).not.toHaveClass("border-input");
  });
});

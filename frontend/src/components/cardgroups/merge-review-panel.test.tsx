// @vitest-environment jsdom
import { fireEvent, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { renderWithIntl } from "@/test/render-with-intl";
import { MergeReviewPanel } from "./merge-review-panel";

describe("MergeReviewPanel", () => {
  function setup(overrides = {}) {
    const onConfirm = vi.fn();
    const onBack = vi.fn();
    renderWithIntl(
      <MergeReviewPanel
        catalogName="Business English"
        destName="My English Deck"
        addedCount={312}
        updatedCount={508}
        merging={false}
        onConfirm={onConfirm}
        onBack={onBack}
        {...overrides}
      />,
    );
    return { onConfirm, onBack };
  }

  it("shows the diff counts, destination, and a destructive merge button", () => {
    setup();
    expect(screen.getByTestId("merge-review-added")).toHaveTextContent("312");
    expect(screen.getByTestId("merge-review-updated")).toHaveTextContent("508");
    expect(screen.getByText("My English Deck")).toBeInTheDocument();
    // Button label uses the total (added + updated) = 820.
    expect(screen.getByTestId("merge-review-confirm")).toHaveTextContent("820");
  });

  it("fires onConfirm / onBack", () => {
    const { onConfirm, onBack } = setup();
    fireEvent.click(screen.getByTestId("merge-review-confirm"));
    fireEvent.click(screen.getByTestId("merge-review-back"));
    expect(onConfirm).toHaveBeenCalledOnce();
    expect(onBack).toHaveBeenCalledOnce();
  });

  it("disables the confirm button while merging", () => {
    setup({ merging: true });
    expect(screen.getByTestId("merge-review-confirm")).toBeDisabled();
  });
});

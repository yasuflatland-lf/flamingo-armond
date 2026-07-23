// @vitest-environment happy-dom
import { screen } from "@testing-library/react";
import { createRef } from "react";
import { describe, expect, it, vi } from "vitest";
import type { SwipeableRowHandle } from "@/components/cardgroups/swipeable-row";
import { renderWithIntl } from "@/test/render-with-intl";

// Mock SwipeableRow as a plain div to avoid gesture library dependencies.
// The `disabled` prop is forwarded as a data attribute so tests can assert
// that selection mode propagates correctly.
// swipeable-row.test.tsx is the canonical test for gesture behaviour.
vi.mock("@/components/cardgroups/swipeable-row", async () => {
  const { forwardRef } = await import("react");
  return {
    SwipeableRow: forwardRef(function SwipeableRowMock(
      {
        children,
        disabled,
      }: {
        children: React.ReactNode;
        disabled?: boolean;
        onDelete: () => void;
        ariaLabel: string | null;
      },
      _ref: React.Ref<{ close(): void }>,
    ) {
      return (
        <div data-testid="swipeable-row-mock" data-disabled={disabled ? "true" : "false"}>
          {children}
        </div>
      );
    }),
  };
});

import { CardRow } from "./card-row";

const CARD = { id: "c-1", front: "Hello", back: "Hola" };

function renderCardRow(overrides: Partial<React.ComponentProps<typeof CardRow>> = {}) {
  const rowRef = createRef<SwipeableRowHandle | null>();
  renderWithIntl(
    <CardRow
      card={CARD}
      rowRef={rowRef}
      selected={false}
      disabled={false}
      onSelectToggle={vi.fn()}
      onEdit={vi.fn()}
      onDelete={vi.fn()}
      {...overrides}
    />,
  );
}

describe("<CardRow>", () => {
  it("Trash icon button className contains the hover-reveal and motion-reduce classes", () => {
    renderCardRow();

    const deleteButton = screen.getByTestId(`card-delete-${CARD.id}`);
    const cls = deleteButton.className;

    expect(cls).toContain("opacity-0");
    expect(cls).toContain("sm:group-hover:opacity-100");
    expect(cls).toContain("motion-reduce:opacity-100");
  });

  it("gates pointer-events on the same variants as opacity (hidden button is non-clickable)", () => {
    // opacity:0 alone leaves the button clickable, so on a narrow viewport (sm:
    // hover variant inactive) the invisible Delete button would steal a row tap
    // and delete the card. The guard tokens are base-provided by
    // HoverRevealDeleteButton; this consumer-level pin proves they survive the
    // cn merge so "visible ⟺ clickable" holds at every breakpoint.
    renderCardRow();

    const cls = screen.getByTestId(`card-delete-${CARD.id}`).className;
    expect(cls).toContain("pointer-events-none");
    expect(cls).toContain("sm:group-hover:pointer-events-auto");
    expect(cls).toContain("sm:group-focus-within:pointer-events-auto");
    expect(cls).toContain("motion-reduce:pointer-events-auto");
  });

  it("truncates the front and back text to a single line", () => {
    // Long card text must collapse to one line per field; without truncate the
    // back description wraps to many lines and inflates the row height.
    renderCardRow();

    expect(screen.getByText(CARD.front).className).toContain("truncate");
    expect(screen.getByText(CARD.back).className).toContain("truncate");
  });

  it("stretches the edit target across the whole row on mobile, but not on desktop", () => {
    // The hidden Delete button leaves a dead gap on the right on mobile; the
    // after:inset-0 overlay makes a tap anywhere on the row open the editor.
    // sm:after:content-none removes the overlay so desktop keeps its current
    // text-column-only edit target.
    renderCardRow();

    const cls = screen.getByTestId(`card-edit-target-${CARD.id}`).className;
    expect(cls).toContain("after:absolute");
    expect(cls).toContain("after:inset-0");
    expect(cls).toContain("after:content-['']");
    expect(cls).toContain("sm:after:content-none");
  });

  it("keeps the checkbox and Delete controls above the mobile overlay (z-10)", () => {
    // Both wrappers paint at relative z-10 above the stretched edit overlay. The
    // checkbox stays an interactive control on mobile; the Delete wrapper's z-10 is
    // only there so its desktop hover-reveal paints above the overlay (its mobile
    // tap pass-through is asserted separately below).
    renderCardRow();

    const checkboxWrapper = screen.getByTestId(`card-select-${CARD.id}`).parentElement;
    expect(checkboxWrapper?.className).toContain("z-10");

    const deleteWrapper = screen.getByTestId(`card-delete-${CARD.id}`).parentElement;
    expect(deleteWrapper?.className).toContain("z-10");
  });

  it("lets a mobile row tap fall through the Delete wrapper to the edit overlay", () => {
    // Bug fix: the Delete wrapper is z-10 and stops propagation, so on mobile its box
    // (the right-hand region of the row, where the hidden Delete button is laid out)
    // intercepted the tap and the editor never opened — only the left text column did.
    // pointer-events-none on mobile makes the wrapper transparent to taps so they reach
    // the after:inset-0 edit overlay; sm:pointer-events-auto restores the desktop
    // hover-to-delete behaviour. jsdom cannot model pointer-events hit-testing, so this
    // is pinned statically on the className.
    renderCardRow();

    const deleteWrapper = screen.getByTestId(`card-delete-${CARD.id}`).parentElement;
    expect(deleteWrapper?.className).toContain("pointer-events-none");
    expect(deleteWrapper?.className).toContain("sm:pointer-events-auto");

    // The checkbox stays an active control on mobile, so its wrapper must NOT be made
    // pointer-events-none — taps there toggle selection rather than open the editor.
    const checkboxWrapper = screen.getByTestId(`card-select-${CARD.id}`).parentElement;
    expect(checkboxWrapper?.className).not.toContain("pointer-events-none");
  });

  it("wraps the row content in SwipeableRow", () => {
    renderCardRow();

    expect(screen.getByTestId("swipeable-row-mock")).toBeInTheDocument();
  });

  it("passes disabled=true to SwipeableRow when selection mode is active", () => {
    // Selection mode is signalled to CardRow by the parent via disabled=true
    // (set when selectedIds.size > 0 in CardsClient).
    renderCardRow({ disabled: true });

    expect(screen.getByTestId("swipeable-row-mock")).toHaveAttribute("data-disabled", "true");
  });
});

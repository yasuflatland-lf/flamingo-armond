// @vitest-environment jsdom
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
    // and delete the card. pointer-events must track the exact opacity variants
    // so "visible ⟺ clickable" holds at every breakpoint.
    renderCardRow();

    const cls = screen.getByTestId(`card-delete-${CARD.id}`).className;
    expect(cls).toContain("pointer-events-none");
    expect(cls).toContain("sm:group-hover:pointer-events-auto");
    expect(cls).toContain("sm:group-focus-within:pointer-events-auto");
    expect(cls).toContain("motion-reduce:pointer-events-auto");
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

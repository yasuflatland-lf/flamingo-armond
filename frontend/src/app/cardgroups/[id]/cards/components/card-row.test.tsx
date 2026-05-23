// @vitest-environment jsdom
import { render, screen } from "@testing-library/react";
import { createRef } from "react";
import { describe, expect, it, vi } from "vitest";
import type { SwipeableRowHandle } from "@/components/cardgroups/swipeable-row";

// Mock SwipeableRow as a plain div to avoid gesture library dependencies.
// swipeable-row.test.tsx is the canonical test for gesture behaviour.
vi.mock("@/components/cardgroups/swipeable-row", async () => {
  const { forwardRef } = await import("react");
  return {
    SwipeableRow: forwardRef(function SwipeableRowMock(
      {
        children,
      }: {
        children: React.ReactNode;
        disabled?: boolean;
        onDelete: () => void;
        ariaLabel: string | null;
      },
      _ref: React.Ref<{ close(): void }>,
    ) {
      return <div data-testid="swipeable-row-mock">{children}</div>;
    }),
  };
});

import { CardRow } from "./card-row";

const CARD = { id: "c-1", front: "Hello", back: "Hola" };

function renderCardRow(overrides: Partial<React.ComponentProps<typeof CardRow>> = {}) {
  const rowRef = createRef<SwipeableRowHandle | null>();
  render(
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
});

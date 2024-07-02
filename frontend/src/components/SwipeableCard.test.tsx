import React from "react";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { describe, it, expect, vi } from "vitest";
import SwipeableCard from "./SwipeableCard";

const setup = (props = {}) => {
  const onSwiped = vi.fn();
  const utils = render(
    <SwipeableCard content="Test Content" onSwiped={onSwiped} {...props} />,
  );
  const card = screen.getByText("Test Content");
  return { ...utils, card, onSwiped };
};

const swipe = (element, direction) => {
  const touchStart = { clientX: 0, clientY: 0 };
  const touchEnd = { clientX: 0, clientY: 0 };

  if (direction === "left") {
    touchStart.clientX = 100;
    touchEnd.clientX = 0;
  } else if (direction === "right") {
    touchStart.clientX = 0;
    touchEnd.clientX = 100;
  } else if (direction === "up") {
    touchStart.clientY = 100;
    touchEnd.clientY = 0;
  } else if (direction === "down") {
    touchStart.clientY = 0;
    touchEnd.clientY = 100;
  }

  fireEvent.touchStart(element, { touches: [touchStart] });
  fireEvent.touchMove(element, { touches: [touchEnd] });
  fireEvent.touchEnd(element, { changedTouches: [touchEnd] });
};

describe("SwipeableCard", () => {
  it("renders content correctly", () => {
    setup();
    expect(screen.getByText("Test Content")).toBeInTheDocument();
  });

  const swipeDirections = [
    { direction: "left", expectedClass: "text-pink-700 opacity-30" },
    { direction: "right", expectedClass: "text-green-700 opacity-30" },
    { direction: "up", expectedClass: "" },
    { direction: "down", expectedClass: "text-green-700 opacity-30" },
  ];

  swipeDirections.forEach(({ direction, expectedClass }) => {
    if (direction === "up") {
      it(`flips the card on swipe ${direction}`, async () => {
        const { container } = setup();
        const card = container.querySelector(".swipeable-card");
        swipe(card, direction);

        // Wait for the flip to complete
        await waitFor(() => {
          expect(screen.getByText("Test Content (Back)")).toBeInTheDocument();
        });

        // Verify that both front and back contents are displayed correctly
        expect(screen.getByText("Test Content")).toBeInTheDocument();
      });
    } else {
      it(`calls onSwiped with correct direction on swipe ${direction}`, async () => {
        const { card, onSwiped } = setup();
        swipe(card, direction);
        await waitFor(() => expect(onSwiped).toHaveBeenCalledWith(direction));
      });

      it(`shows the correct watermark during swipe ${direction}`, async () => {
        const { card } = setup();
        swipe(card, direction);
        if (expectedClass) {
          await waitFor(() =>
            expect(screen.getByTestId("wartermark-id")).toHaveClass(
              expectedClass,
            ),
          );
        } else {
          await waitFor(() =>
            expect(screen.getByTestId("wartermark-id")).toBeEmptyDOMElement(),
          );
        }
      });
    }
  });

  it("keeps the watermark centered during card flip", async () => {
    const { getByTestId, container } = setup();
    const watermark = getByTestId("wartermark-id");
    const card = container.querySelector(".swipeable-card");

    // Get watermark position before the flip
    const initialRect = watermark.getBoundingClientRect();
    swipe(card, "up");

    // Wait for the flip to complete
    await waitFor(() => {
      const flippedRect = watermark.getBoundingClientRect();
      expect(initialRect.top).toBeCloseTo(flippedRect.top, 1);
      expect(initialRect.left).toBeCloseTo(flippedRect.left, 1);
    });

    // Ensure the watermark is still visible after the flip
    expect(watermark).toBeVisible();
  });
});

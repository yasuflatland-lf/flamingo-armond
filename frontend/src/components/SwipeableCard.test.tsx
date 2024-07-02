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
  });
});

// @vitest-environment jsdom
import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { SwipeCardStack } from "./swipe-card-stack";

describe("SwipeCardStack — Session-complete count line", () => {
  const baseProps = {
    cards: [] as Parameters<typeof SwipeCardStack>[0]["cards"],
    onCardSwiped: vi.fn(),
    onSwipeProgress: vi.fn(),
    swipeDirection: null as null,
    swipeProgress: 0,
  };

  it("renders Session-complete heading and no count line when completedCount is undefined", () => {
    render(<SwipeCardStack {...baseProps} />);
    expect(screen.getByRole("heading", { name: "Session complete" })).toBeInTheDocument();
    expect(screen.queryByText(/You reviewed/)).not.toBeInTheDocument();
  });

  it("renders Session-complete heading and no count line when completedCount is 0", () => {
    render(<SwipeCardStack {...baseProps} completedCount={0} />);
    expect(screen.getByRole("heading", { name: "Session complete" })).toBeInTheDocument();
    expect(screen.queryByText(/You reviewed/)).not.toBeInTheDocument();
  });

  it("renders singular count line when completedCount is 1", () => {
    render(<SwipeCardStack {...baseProps} completedCount={1} />);
    expect(screen.getByRole("heading", { name: "Session complete" })).toBeInTheDocument();
    expect(screen.getByText("You reviewed 1 card in this batch.")).toBeInTheDocument();
  });

  it("renders plural count line when completedCount is 2", () => {
    render(<SwipeCardStack {...baseProps} completedCount={2} />);
    expect(screen.getByRole("heading", { name: "Session complete" })).toBeInTheDocument();
    expect(screen.getByText("You reviewed 2 cards in this batch.")).toBeInTheDocument();
  });
});

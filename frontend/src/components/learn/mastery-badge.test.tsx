// @vitest-environment jsdom
import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { MasteryBadge } from "./mastery-badge";

describe("<MasteryBadge>", () => {
  it("renders Learned label and aria-label for state 2", () => {
    render(<MasteryBadge state={2} />);
    expect(screen.getByText("Learned")).toBeInTheDocument();
    expect(screen.getByLabelText("Mastery stage: Learned")).toBeInTheDocument();
  });

  it("renders New label and aria-label for state 0", () => {
    render(<MasteryBadge state={0} />);
    expect(screen.getByText("New")).toBeInTheDocument();
    expect(screen.getByLabelText("Mastery stage: New")).toBeInTheDocument();
  });

  it("renders Learning label and aria-label for state 1", () => {
    render(<MasteryBadge state={1} />);
    expect(screen.getByText("Learning")).toBeInTheDocument();
    expect(screen.getByLabelText("Mastery stage: Learning")).toBeInTheDocument();
  });

  it("renders Learning label and aria-label for state 3 (Relearning)", () => {
    render(<MasteryBadge state={3} />);
    expect(screen.getByText("Learning")).toBeInTheDocument();
    expect(screen.getByLabelText("Mastery stage: Learning")).toBeInTheDocument();
  });
});

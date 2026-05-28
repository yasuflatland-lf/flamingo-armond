// @vitest-environment jsdom
import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { MasteryBadge } from "./mastery-badge";

describe("<MasteryBadge>", () => {
  it("renders Learned for state 2", () => {
    render(<MasteryBadge state={2} />);
    expect(screen.getByText("Learned")).toBeInTheDocument();
  });

  it("renders New for state 0", () => {
    render(<MasteryBadge state={0} />);
    expect(screen.getByText("New")).toBeInTheDocument();
  });

  it("renders Learning with aria-label for state 1", () => {
    render(<MasteryBadge state={1} />);
    expect(screen.getByLabelText("Mastery stage: Learning")).toBeInTheDocument();
  });
});

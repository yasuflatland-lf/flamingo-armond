// @vitest-environment jsdom
import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { AllCaughtUp } from "./all-caught-up";

describe("<AllCaughtUp>", () => {
  it("renders the caught-up message and cardgroups link", () => {
    render(<AllCaughtUp />);

    expect(
      screen.getByRole("heading", { name: "Today's learning is complete" }),
    ).toBeInTheDocument();
    expect(screen.getByText(/Come back when the next review is due/i)).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Back to cardgroups" })).toHaveAttribute(
      "href",
      "/cardgroups",
    );
  });
});

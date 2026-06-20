// @vitest-environment jsdom
import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { ReadOnlyCardRow } from "./read-only-card-row";

const CARD = { id: "mc-1", front: "Front text", back: "Back text" };

describe("<ReadOnlyCardRow>", () => {
  it("renders the front and back text", () => {
    render(<ReadOnlyCardRow card={CARD} />);
    expect(screen.getByText("Front text")).toBeInTheDocument();
    expect(screen.getByText("Back text")).toBeInTheDocument();
  });

  it("renders no edit chrome — no selection checkbox, no delete button, no row button", () => {
    render(<ReadOnlyCardRow card={CARD} />);
    // The editable CardRow exposes these; the read-only row must not.
    expect(screen.queryByTestId("card-select-mc-1")).toBeNull();
    expect(screen.queryByTestId("card-delete-mc-1")).toBeNull();
    expect(screen.queryByRole("checkbox")).toBeNull();
    expect(screen.queryByRole("button")).toBeNull();
  });

  it("exposes a locale-independent row testId", () => {
    render(<ReadOnlyCardRow card={CARD} />);
    expect(screen.getByTestId("read-only-card-mc-1")).toBeInTheDocument();
  });
});

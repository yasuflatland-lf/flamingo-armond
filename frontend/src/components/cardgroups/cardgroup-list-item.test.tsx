// @vitest-environment jsdom
import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { CardgroupListItem } from "./cardgroup-list-item";

describe("<CardgroupListItem>", () => {
  const fixedDate = "2024-06-15T10:00:00.000Z";

  it("renders the cardgroup name", () => {
    render(<CardgroupListItem id="cg-1" name="My Flashcards" updatedAt={fixedDate} />);
    expect(screen.getByText("My Flashcards")).toBeInTheDocument();
  });

  it("renders a link to /cardgroups/[id]", () => {
    render(<CardgroupListItem id="cg-1" name="My Flashcards" updatedAt={fixedDate} />);
    const link = screen.getByRole("link");
    expect(link).toHaveAttribute("href", "/cardgroups/cg-1");
  });

  it("renders formatted date text", () => {
    render(<CardgroupListItem id="cg-1" name="My Flashcards" updatedAt={fixedDate} />);
    // Intl.DateTimeFormat en-US medium: "Jun 15, 2024"
    expect(screen.getByText(/Updated/)).toBeInTheDocument();
    expect(screen.getByText(/Jun 15, 2024/)).toBeInTheDocument();
  });
});

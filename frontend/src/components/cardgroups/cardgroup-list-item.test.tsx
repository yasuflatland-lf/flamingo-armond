// @vitest-environment jsdom
import { MockedProvider } from "@apollo/client/testing/react";
import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { CardgroupListItem } from "./cardgroup-list-item";

function renderItem(props: { id: string; name: string; updatedAt: string }) {
  render(
    <MockedProvider mocks={[]}>
      <ul>
        <CardgroupListItem {...props} />
      </ul>
    </MockedProvider>,
  );
}

describe("<CardgroupListItem>", () => {
  const fixedDate = "2024-06-15T10:00:00.000Z";

  it("renders the cardgroup name", () => {
    renderItem({ id: "cg-1", name: "My Flashcards", updatedAt: fixedDate });
    expect(screen.getByText("My Flashcards")).toBeInTheDocument();
  });

  it("renders a link to /cardgroups/[id]", () => {
    renderItem({ id: "cg-1", name: "My Flashcards", updatedAt: fixedDate });
    const link = screen.getByRole("link");
    expect(link).toHaveAttribute("href", "/cardgroups/cg-1");
  });

  it("renders formatted date text", () => {
    renderItem({ id: "cg-1", name: "My Flashcards", updatedAt: fixedDate });
    // Intl.DateTimeFormat en-US medium: "Jun 15, 2024"
    expect(screen.getByText(/Updated/)).toBeInTheDocument();
    expect(screen.getByText(/Jun 15, 2024/)).toBeInTheDocument();
  });

  it("renders a Delete button as a sibling of the link (not nested inside it)", () => {
    renderItem({ id: "cg-1", name: "My Flashcards", updatedAt: fixedDate });
    const link = screen.getByRole("link");
    const deleteBtn = screen.getByRole("button", { name: /delete cardgroup my flashcards/i });
    expect(deleteBtn).toBeInTheDocument();
    expect(link.contains(deleteBtn)).toBe(false);
  });
});

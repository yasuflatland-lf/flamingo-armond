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

  it("renders a link to /cardgroups/[id]/edit (canonical management screen)", () => {
    renderItem({ id: "cg-1", name: "My Flashcards", updatedAt: fixedDate });
    // Accessible name starts with the card name; edit link starts with "Edit cardgroup".
    const link = screen.getByRole("link", { name: /^My Flashcards/i });
    expect(link).toHaveAttribute("href", "/cardgroups/cg-1/edit");
  });

  it("renders formatted date text", () => {
    renderItem({ id: "cg-1", name: "My Flashcards", updatedAt: fixedDate });
    // Intl.DateTimeFormat en-US medium: "Jun 15, 2024"
    expect(screen.getByText(/Updated/)).toBeInTheDocument();
    expect(screen.getByText(/Jun 15, 2024/)).toBeInTheDocument();
  });

  it("renders a Delete button as a sibling of the name link (not nested inside it)", () => {
    renderItem({ id: "cg-1", name: "My Flashcards", updatedAt: fixedDate });
    const nameLink = screen.getByRole("link", { name: /^My Flashcards/i });
    const deleteBtn = screen.getByRole("button", { name: /delete cardgroup my flashcards/i });
    expect(deleteBtn).toBeInTheDocument();
    expect(nameLink.contains(deleteBtn)).toBe(false);
  });
});

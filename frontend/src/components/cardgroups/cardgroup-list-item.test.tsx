// @vitest-environment jsdom
import { MockedProvider } from "@apollo/client/testing/react";
import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { CardgroupListItem } from "./cardgroup-list-item";

// SwipeableRow uses useReducedMotion which reads matchMedia.
// Default stub: reduced-motion = false so the swipe layer renders and
// SwipeableRow wraps children in the animated div tree.
function stubMatchMedia(reducedMotion: boolean) {
  Object.defineProperty(window, "matchMedia", {
    writable: true,
    configurable: true,
    value: vi.fn().mockImplementation((query: string) => ({
      matches: query.includes("prefers-reduced-motion") ? reducedMotion : false,
      media: query,
      onchange: null,
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
      addListener: vi.fn(),
      removeListener: vi.fn(),
      dispatchEvent: vi.fn(),
    })),
  });
}

stubMatchMedia(false);

function renderItem(props: {
  id: string;
  name: string;
  updatedAt: string;
  onDelete?: (id: string, name: string) => void;
}) {
  const { onDelete = vi.fn(), ...rest } = props;
  render(
    <MockedProvider mocks={[]}>
      <ul>
        <CardgroupListItem {...rest} onDelete={onDelete} />
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

  // -------------------------------------------------------------------------
  // Spec § Testing: "Verify SwipeableRow wrapping"
  // -------------------------------------------------------------------------

  it("wraps the list item in a SwipeableRow (data-testid swipeable-row present)", () => {
    // SwipeableRow renders a container div with data-testid="swipeable-row-container"
    // when reduced-motion is false (the default stub above).
    renderItem({ id: "cg-1", name: "My Flashcards", updatedAt: fixedDate });
    expect(screen.getByTestId("swipeable-row-container")).toBeInTheDocument();
  });

  // -------------------------------------------------------------------------
  // Spec § Testing: "Trash button className includes `motion-reduce:opacity-100`"
  // -------------------------------------------------------------------------

  it("Trash button className includes motion-reduce:opacity-100", () => {
    renderItem({ id: "cg-1", name: "My Flashcards", updatedAt: fixedDate });
    const deleteBtn = screen.getByRole("button", { name: /delete cardgroup my flashcards/i });
    // The className must carry motion-reduce:opacity-100 so reduced-motion users
    // always see the affordance, matching the spec Architecture section.
    expect(deleteBtn.className).toContain("motion-reduce:opacity-100");
  });
});

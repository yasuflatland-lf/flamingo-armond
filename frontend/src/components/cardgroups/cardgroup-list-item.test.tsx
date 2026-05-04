// @vitest-environment jsdom
import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

vi.mock("next/link", () => ({
  default: ({ href, children, ...rest }: React.AnchorHTMLAttributes<HTMLAnchorElement> & { href: string }) => (
    <a href={href} {...rest}>{children}</a>
  ),
}));

import { CardgroupListItem } from "./cardgroup-list-item";

describe("<CardgroupListItem>", () => {
  const fixedDate = "2024-06-15T10:00:00.000Z";

  it("renders the cardgroup name", () => {
    render(<CardgroupListItem id="cg-1" name="My Flashcards" updatedAt={fixedDate} />);
    expect(screen.getByText("My Flashcards")).toBeInTheDocument();
  });

  it("renders the primary link to /cardgroups/<id>/cards (skipping the redirect hop)", () => {
    render(<CardgroupListItem id="cg-1" name="My Flashcards" updatedAt={fixedDate} />);
    const link = screen.getByRole("link", { name: /my flashcards/i });
    expect(link).toHaveAttribute("href", "/cardgroups/cg-1/cards");
  });

  it("renders a per-row Actions trigger labelled with the cardgroup name", () => {
    render(<CardgroupListItem id="cg-1" name="My Flashcards" updatedAt={fixedDate} />);
    expect(
      screen.getByRole("button", { name: "Actions for My Flashcards" }),
    ).toBeInTheDocument();
  });

  it("renders formatted date text", () => {
    render(<CardgroupListItem id="cg-1" name="My Flashcards" updatedAt={fixedDate} />);
    expect(screen.getByText(/Jun 15, 2024/)).toBeInTheDocument();
  });
});

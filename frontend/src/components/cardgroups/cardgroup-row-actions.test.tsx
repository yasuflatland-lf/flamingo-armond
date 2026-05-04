// @vitest-environment jsdom
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

vi.mock("next/link", () => ({
  default: ({
    href,
    children,
    ...rest
  }: React.AnchorHTMLAttributes<HTMLAnchorElement> & { href: string }) => (
    <a href={href} {...rest}>
      {children}
    </a>
  ),
}));

import { CardgroupRowActions } from "./cardgroup-row-actions";

describe("<CardgroupRowActions>", () => {
  it("renders a trigger labelled with the cardgroup name (per-row disambiguation)", () => {
    render(<CardgroupRowActions id="cg-1" name="Spanish 101" />);
    expect(screen.getByRole("button", { name: "Actions for Spanish 101" })).toBeInTheDocument();
  });

  it("opens the menu on click and exposes Start learning + Rename items", async () => {
    const user = userEvent.setup();
    render(<CardgroupRowActions id="cg-1" name="Spanish 101" />);

    await user.click(screen.getByRole("button", { name: "Actions for Spanish 101" }));

    const startLearning = await screen.findByRole("menuitem", { name: /start learning/i });
    expect(startLearning).toBeInTheDocument();
    const startLink = startLearning.querySelector("a") ?? startLearning;
    expect(startLink).toHaveAttribute("href", "/learn/cg-1");

    const rename = screen.getByRole("menuitem", { name: /rename/i });
    const renameLink = rename.querySelector("a") ?? rename;
    expect(renameLink).toHaveAttribute("href", "/cardgroups/cg-1/edit");
  });

  it("URL-encodes the id in both menu item hrefs", async () => {
    const user = userEvent.setup();
    render(<CardgroupRowActions id="a/b" name="Edge case" />);

    await user.click(screen.getByRole("button", { name: "Actions for Edge case" }));

    // Radix `asChild` forwards menuitem semantics onto the rendered anchor
    // itself (Slot pattern), so the `<a>` IS the menuitem — no nested `<a>`
    // descendant. Use the same `?? menuItem` fallback as test 2 above.
    const startMenuItem = await screen.findByRole("menuitem", { name: /start learning/i });
    const startLink = startMenuItem.querySelector("a") ?? startMenuItem;
    expect(startLink).toHaveAttribute("href", "/learn/a%2Fb");

    const renameMenuItem = screen.getByRole("menuitem", { name: /rename/i });
    const renameLink = renameMenuItem.querySelector("a") ?? renameMenuItem;
    expect(renameLink).toHaveAttribute("href", "/cardgroups/a%2Fb/edit");
  });
});

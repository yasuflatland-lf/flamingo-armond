// @vitest-environment jsdom
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("next/navigation", () => ({
  usePathname: vi.fn(),
  useRouter: vi.fn(),
}));

import { usePathname, useRouter } from "next/navigation";
import { GlobalFAB } from "./global-fab";

function makeRouter() {
  return { push: vi.fn(), replace: vi.fn() };
}

beforeEach(() => {
  vi.mocked(useRouter).mockReturnValue(makeRouter() as never);
});

afterEach(() => {
  vi.restoreAllMocks();
});

describe("<GlobalFAB>", () => {
  describe("hidden paths — returns null", () => {
    it.each([
      ["/login"],
      ["/learn/abc123"],
      ["/learn/abc123/"],
      ["/admin/users"],
      ["/admin"],
      ["/cards/new"],
      ["/cardgroups/new"],
    ])("renders nothing on %s", (path) => {
      vi.mocked(usePathname).mockReturnValue(path);
      const { container } = render(<GlobalFAB />);
      expect(container.firstChild).toBeNull();
    });
  });

  describe("visible paths — renders the FAB button", () => {
    it.each([
      ["/cardgroups"],
      ["/cardgroups/abc123"],
      ["/profile"],
      ["/"],
    ])("renders button on %s", (path) => {
      vi.mocked(usePathname).mockReturnValue(path);
      render(<GlobalFAB />);
      expect(screen.getByRole("button", { name: "Add new card" })).toBeInTheDocument();
    });
  });

  it("has aria-label 'Add new card'", () => {
    vi.mocked(usePathname).mockReturnValue("/cardgroups");
    render(<GlobalFAB />);
    expect(screen.getByRole("button", { name: "Add new card" })).toBeInTheDocument();
  });

  it("click navigates to /cards/new", async () => {
    const user = userEvent.setup();
    const router = makeRouter();
    vi.mocked(useRouter).mockReturnValue(router as never);
    vi.mocked(usePathname).mockReturnValue("/cardgroups");

    render(<GlobalFAB />);
    await user.click(screen.getByRole("button", { name: "Add new card" }));

    expect(router.push).toHaveBeenCalledWith("/cards/new");
  });

  it("wraps the button in an md:hidden container so the FAB is hidden at >= md breakpoint", () => {
    vi.mocked(usePathname).mockReturnValue("/cardgroups");
    render(<GlobalFAB />);
    const button = screen.getByRole("button", { name: /add new card/i });
    expect(button.parentElement).toHaveClass("md:hidden");
  });
});

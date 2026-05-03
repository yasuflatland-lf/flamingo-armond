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
      ["/profile"],
      ["/profile/"],
      ["/cardgroups/abc123/edit"],
      ["/cardgroups/abc123/edit/"],
    ])("renders nothing on %s", (path) => {
      vi.mocked(usePathname).mockReturnValue(path);
      const { container } = render(<GlobalFAB />);
      expect(container.firstChild).toBeNull();
    });
  });

  describe("visible paths — renders 'Add new cardgroup' label", () => {
    it.each([["/cardgroups"]])(
      "renders 'Add new cardgroup' button on %s",
      (path) => {
        vi.mocked(usePathname).mockReturnValue(path);
        render(<GlobalFAB />);
        expect(
          screen.getByRole("button", { name: "Add new cardgroup" }),
        ).toBeInTheDocument();
      },
    );
  });

  describe("visible paths — renders 'Add new card' label", () => {
    it.each([
      ["/cardgroups/abc123"],
      ["/cardgroups/abc123/cards"],
      ["/"],
    ])("renders 'Add new card' button on %s", (path) => {
      vi.mocked(usePathname).mockReturnValue(path);
      render(<GlobalFAB />);
      expect(
        screen.getByRole("button", { name: "Add new card" }),
      ).toBeInTheDocument();
    });
  });

  it("click on /cardgroups navigates to /cardgroups/new", async () => {
    const user = userEvent.setup();
    const router = makeRouter();
    vi.mocked(useRouter).mockReturnValue(router as never);
    vi.mocked(usePathname).mockReturnValue("/cardgroups");

    render(<GlobalFAB />);
    await user.click(screen.getByRole("button", { name: "Add new cardgroup" }));

    expect(router.push).toHaveBeenCalledWith("/cardgroups/new");
  });

  it("click on /cardgroups/abc123 navigates to /cards/new?cardgroup=abc123", async () => {
    const user = userEvent.setup();
    const router = makeRouter();
    vi.mocked(useRouter).mockReturnValue(router as never);
    vi.mocked(usePathname).mockReturnValue("/cardgroups/abc123");

    render(<GlobalFAB />);
    await user.click(screen.getByRole("button", { name: "Add new card" }));

    expect(router.push).toHaveBeenCalledWith("/cards/new?cardgroup=abc123");
  });

  it("click on /cardgroups/abc123/cards navigates to /cards/new?cardgroup=abc123", async () => {
    const user = userEvent.setup();
    const router = makeRouter();
    vi.mocked(useRouter).mockReturnValue(router as never);
    vi.mocked(usePathname).mockReturnValue("/cardgroups/abc123/cards");

    render(<GlobalFAB />);
    await user.click(screen.getByRole("button", { name: "Add new card" }));

    expect(router.push).toHaveBeenCalledWith("/cards/new?cardgroup=abc123");
  });

  it("click on / navigates to /cards/new", async () => {
    const user = userEvent.setup();
    const router = makeRouter();
    vi.mocked(useRouter).mockReturnValue(router as never);
    vi.mocked(usePathname).mockReturnValue("/");

    render(<GlobalFAB />);
    await user.click(screen.getByRole("button", { name: "Add new card" }));

    expect(router.push).toHaveBeenCalledWith("/cards/new");
  });

  it("wraps the button in an md:hidden container so the FAB is hidden at >= md breakpoint", () => {
    vi.mocked(usePathname).mockReturnValue("/cardgroups/abc123");
    render(<GlobalFAB />);
    const button = screen.getByRole("button", { name: /add new card/i });
    expect(button.parentElement).toHaveClass("md:hidden");
  });
});

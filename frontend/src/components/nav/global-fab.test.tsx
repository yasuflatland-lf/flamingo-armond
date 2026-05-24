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
      ["/admin/users"],
      ["/admin"],
      ["/cards/new"],
      ["/cardgroups/new"],
      ["/cardgroups/new/"],
      ["/profile"],
      ["/profile/"],
      ["/learn"],
      ["/learn/abc-123"],
      ["/learn/abc-123/"],
      // "/" is a server-redirect-only hub (HomePage always redirects); the FAB
      // there is only a transition flash during /learn -> "/" -> /learn.
      ["/"],
    ])("renders nothing on %s", (path) => {
      vi.mocked(usePathname).mockReturnValue(path);
      const { container } = render(<GlobalFAB />);
      expect(container.firstChild).toBeNull();
    });
  });

  describe("visible paths — renders 'Add new cardgroup' label", () => {
    it.each([["/cardgroups"]])("renders 'Add new cardgroup' button on %s", (path) => {
      vi.mocked(usePathname).mockReturnValue(path);
      render(<GlobalFAB />);
      expect(screen.getByRole("button", { name: "Add new cardgroup" })).toBeInTheDocument();
    });
  });

  describe("visible paths — renders 'Add new card' label", () => {
    it.each([
      ["/cardgroups/abc123/edit"],
      ["/cardgroups/abc123/edit/"],
    ])("renders 'Add new card' button on %s", (path) => {
      vi.mocked(usePathname).mockReturnValue(path);
      render(<GlobalFAB />);
      expect(screen.getByRole("button", { name: "Add new card" })).toBeInTheDocument();
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

  it("click on /cardgroups/abc123/edit dispatches an in-context add-card event and falls back when unhandled", async () => {
    const user = userEvent.setup();
    const router = makeRouter();
    const listener = vi.fn();
    window.addEventListener("flamingo:add-card", listener);
    vi.mocked(useRouter).mockReturnValue(router as never);
    vi.mocked(usePathname).mockReturnValue("/cardgroups/abc123/edit");

    render(<GlobalFAB />);
    await user.click(screen.getByRole("button", { name: "Add new card" }));

    expect(listener).toHaveBeenCalledTimes(1);
    const event = listener.mock.calls[0]?.[0] as CustomEvent<{ cardgroupId: string }>;
    expect(event.cancelable).toBe(true);
    expect(event.detail).toEqual({ cardgroupId: "abc123" });
    expect(router.push).toHaveBeenCalledWith("/cards/new?cardgroup=abc123");

    window.removeEventListener("flamingo:add-card", listener);
  });

  it("click on /cardgroups/abc123/edit does not navigate when the in-context event is handled", async () => {
    const user = userEvent.setup();
    const router = makeRouter();
    const listener = vi.fn((event: Event) => event.preventDefault());
    window.addEventListener("flamingo:add-card", listener);
    vi.mocked(useRouter).mockReturnValue(router as never);
    vi.mocked(usePathname).mockReturnValue("/cardgroups/abc123/edit");

    render(<GlobalFAB />);
    await user.click(screen.getByRole("button", { name: "Add new card" }));

    expect(listener).toHaveBeenCalledTimes(1);
    expect(router.push).not.toHaveBeenCalled();

    window.removeEventListener("flamingo:add-card", listener);
  });

  it("click on a cardgroup detail path navigates to /cards/new", async () => {
    const user = userEvent.setup();
    const router = makeRouter();
    vi.mocked(useRouter).mockReturnValue(router as never);
    vi.mocked(usePathname).mockReturnValue("/cardgroups/abc123");

    render(<GlobalFAB />);
    await user.click(screen.getByRole("button", { name: "Add new card" }));

    expect(router.push).toHaveBeenCalledWith("/cards/new");
  });

  it("wraps the button in an md:hidden container so the FAB is hidden at >= md breakpoint", () => {
    vi.mocked(usePathname).mockReturnValue("/cardgroups/abc123/edit");
    render(<GlobalFAB />);
    const button = screen.getByRole("button", { name: /add new card/i });
    expect(button.parentElement).toHaveClass("md:hidden");
  });
});

// @vitest-environment jsdom
import { act, fireEvent, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

// Mock next/link so it renders a plain <a> in jsdom.
vi.mock("next/link", () => ({
  default: ({
    children,
    ...rest
  }: React.AnchorHTMLAttributes<HTMLAnchorElement> & { children?: React.ReactNode }) => (
    <a {...rest}>{children}</a>
  ),
}));

// usePathname is mocked per-test via mockUsePathname.mockReturnValue.
const mockUsePathname = vi.fn();
vi.mock("next/navigation", () => ({
  useRouter: vi.fn(() => ({ replace: vi.fn(), refresh: vi.fn(), push: vi.fn() })),
  usePathname: () => mockUsePathname(),
}));

// Mock LogoutButton — it reaches into Supabase/router which are not needed
// here. Mirrors the pattern used in `logo-drawer.test.tsx`.
vi.mock("@/app/_components/logout-button", () => ({
  LogoutButton: () => (
    <button type="button" data-testid="logout-button">
      Sign out
    </button>
  ),
}));

import { SidebarProvider } from "@/components/ui/sidebar";
import { GlobalRail } from "./global-rail";

// jsdom does not implement matchMedia; useIsMobile (consumed by SidebarProvider)
// calls window.matchMedia at mount. Provide a minimal stub that reports
// non-mobile so the desktop rail layout is exercised.
beforeEach(() => {
  Object.defineProperty(window, "matchMedia", {
    writable: true,
    configurable: true,
    value: vi.fn().mockImplementation((query: string) => ({
      matches: false,
      media: query,
      onchange: null,
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
      addListener: vi.fn(),
      removeListener: vi.fn(),
      dispatchEvent: vi.fn(),
    })),
  });
  mockUsePathname.mockReturnValue("/");
});

afterEach(() => {
  vi.restoreAllMocks();
});

function renderRail(props: React.ComponentProps<typeof GlobalRail>) {
  return render(
    <SidebarProvider>
      <GlobalRail {...props} />
    </SidebarProvider>,
  );
}

/**
 * Returns the SidebarFooter element ([data-sidebar='footer']) so that footer-
 * scoped queries can disambiguate the footer Profile link from any other Profile
 * link that might appear elsewhere in the tree (today there is none, but
 * scoping keeps the assertion robust against future additions).
 */
function getFooter(container: HTMLElement): HTMLElement {
  const footer = container.querySelector<HTMLElement>("[data-sidebar='footer']");
  if (!footer) throw new Error("[data-sidebar='footer'] element not found");
  return footer;
}

describe("<GlobalRail>", () => {
  describe("S1 — items render when signed in", () => {
    it("renders Cardgroups, the three admin items (Users / Roles / Dictionary), and Settings with correct hrefs when isAdmin=true", () => {
      mockUsePathname.mockReturnValue("/");
      renderRail({ user: { email: "u@example.com" }, isAdmin: true });

      expect(screen.getByRole("link", { name: /cardgroups/i })).toHaveAttribute(
        "href",
        "/cardgroups",
      );
      expect(screen.getByRole("link", { name: /users/i })).toHaveAttribute("href", "/admin/users");
      expect(screen.getByRole("link", { name: /roles/i })).toHaveAttribute("href", "/admin/roles");
      expect(screen.getByRole("link", { name: /dictionary/i })).toHaveAttribute(
        "href",
        "/admin/dictionary",
      );
      expect(screen.getByRole("link", { name: /settings/i })).toHaveAttribute("href", "/settings");
    });
  });

  describe("S2 — Admin gate", () => {
    it("does not render any of the three admin links when isAdmin=false", () => {
      mockUsePathname.mockReturnValue("/");
      renderRail({ user: { email: "u@example.com" }, isAdmin: false });

      // The three admin links must be absent.
      expect(screen.queryByRole("link", { name: /users/i })).toBeNull();
      expect(screen.queryByRole("link", { name: /roles/i })).toBeNull();
      expect(screen.queryByRole("link", { name: /dictionary/i })).toBeNull();

      // The non-admin items remain present.
      expect(screen.getByRole("link", { name: /cardgroups/i })).toBeInTheDocument();
      expect(screen.getByRole("link", { name: /settings/i })).toBeInTheDocument();
      // Profile (footer) is also independent of the admin gate.
      expect(screen.getByRole("link", { name: /profile/i })).toBeInTheDocument();
    });
  });

  describe("S3 — active state from pathname", () => {
    it("marks Cardgroups as the current page when pathname is /cardgroups/abc/cards", () => {
      mockUsePathname.mockReturnValue("/cardgroups/abc/cards");
      renderRail({ user: { email: "u@example.com" }, isAdmin: true });

      const cardgroupsLink = screen.getByRole("link", { name: /cardgroups/i });
      expect(cardgroupsLink).toHaveAttribute("aria-current", "page");

      // Sibling items must not be marked as current.
      expect(screen.getByRole("link", { name: /users/i })).not.toHaveAttribute("aria-current");
      expect(screen.getByRole("link", { name: /roles/i })).not.toHaveAttribute("aria-current");
      expect(screen.getByRole("link", { name: /dictionary/i })).not.toHaveAttribute("aria-current");
      expect(screen.getByRole("link", { name: /settings/i })).not.toHaveAttribute("aria-current");
      expect(screen.getByRole("link", { name: /profile/i })).not.toHaveAttribute("aria-current");
    });

    it("marks Cardgroups as the current page when pathname is /learn/abc (study session is part of the cardgroups flow)", () => {
      mockUsePathname.mockReturnValue("/learn/abc");
      renderRail({ user: { email: "u@example.com" }, isAdmin: true });

      expect(screen.getByRole("link", { name: /cardgroups/i })).toHaveAttribute(
        "aria-current",
        "page",
      );
    });

    it("marks Users as the current page when pathname is /admin/users", () => {
      mockUsePathname.mockReturnValue("/admin/users");
      renderRail({ user: { email: "u@example.com" }, isAdmin: true });

      expect(screen.getByRole("link", { name: /users/i })).toHaveAttribute("aria-current", "page");

      // Sibling admin items must NOT be marked as current.
      expect(screen.getByRole("link", { name: /roles/i })).not.toHaveAttribute("aria-current");
      expect(screen.getByRole("link", { name: /dictionary/i })).not.toHaveAttribute("aria-current");
      // Other non-admin items are also not current.
      expect(screen.getByRole("link", { name: /cardgroups/i })).not.toHaveAttribute("aria-current");
      expect(screen.getByRole("link", { name: /settings/i })).not.toHaveAttribute("aria-current");
      expect(screen.getByRole("link", { name: /profile/i })).not.toHaveAttribute("aria-current");
    });

    it("marks Users as the current page when pathname is /admin (F-1 fallback for the bare admin root)", () => {
      mockUsePathname.mockReturnValue("/admin");
      renderRail({ user: { email: "u@example.com" }, isAdmin: true });

      expect(screen.getByRole("link", { name: /users/i })).toHaveAttribute("aria-current", "page");
      expect(screen.getByRole("link", { name: /roles/i })).not.toHaveAttribute("aria-current");
      expect(screen.getByRole("link", { name: /dictionary/i })).not.toHaveAttribute("aria-current");
    });

    it("marks Users as the current page when pathname is /admin/strange-unknown (F-1 fallback for unknown sub-routes)", () => {
      mockUsePathname.mockReturnValue("/admin/strange-unknown");
      renderRail({ user: { email: "u@example.com" }, isAdmin: true });

      expect(screen.getByRole("link", { name: /users/i })).toHaveAttribute("aria-current", "page");
      expect(screen.getByRole("link", { name: /roles/i })).not.toHaveAttribute("aria-current");
      expect(screen.getByRole("link", { name: /dictionary/i })).not.toHaveAttribute("aria-current");
    });

    it("marks Roles as the current page when pathname is /admin/roles (and Users is NOT current)", () => {
      mockUsePathname.mockReturnValue("/admin/roles");
      renderRail({ user: { email: "u@example.com" }, isAdmin: true });

      expect(screen.getByRole("link", { name: /roles/i })).toHaveAttribute("aria-current", "page");
      expect(screen.getByRole("link", { name: /users/i })).not.toHaveAttribute("aria-current");
      expect(screen.getByRole("link", { name: /dictionary/i })).not.toHaveAttribute("aria-current");
    });

    it("marks Settings as the current page when pathname is /settings", () => {
      mockUsePathname.mockReturnValue("/settings");
      renderRail({ user: { email: "u@example.com" }, isAdmin: true });

      expect(screen.getByRole("link", { name: /settings/i })).toHaveAttribute(
        "aria-current",
        "page",
      );

      // Other items must NOT be marked as current.
      expect(screen.getByRole("link", { name: /cardgroups/i })).not.toHaveAttribute("aria-current");
      expect(screen.getByRole("link", { name: /users/i })).not.toHaveAttribute("aria-current");
      expect(screen.getByRole("link", { name: /profile/i })).not.toHaveAttribute("aria-current");
    });

    it("marks the footer Profile link as the current page when pathname is /profile", () => {
      mockUsePathname.mockReturnValue("/profile");
      const { container } = renderRail({ user: { email: "u@example.com" }, isAdmin: true });

      const profileLink = within(getFooter(container)).getByRole("link", { name: /profile/i });
      expect(profileLink).toHaveAttribute("aria-current", "page");

      // Center items must NOT be marked as current.
      expect(screen.getByRole("link", { name: /cardgroups/i })).not.toHaveAttribute("aria-current");
      expect(screen.getByRole("link", { name: /settings/i })).not.toHaveAttribute("aria-current");
    });

    it("marks the footer Profile link as the current page when pathname is /profile/change-email (sub-route prefix match)", () => {
      mockUsePathname.mockReturnValue("/profile/change-email");
      const { container } = renderRail({ user: { email: "u@example.com" }, isAdmin: true });

      const profileLink = within(getFooter(container)).getByRole("link", { name: /profile/i });
      expect(profileLink).toHaveAttribute("aria-current", "page");
    });
  });

  describe("S4 — logo button toggles aria-expanded", () => {
    it("clicking the logo toggles aria-expanded between true and false", async () => {
      const user = userEvent.setup();
      mockUsePathname.mockReturnValue("/");
      renderRail({ user: { email: "u@example.com" }, isAdmin: true });

      const logo = screen.getByRole("button", { name: /toggle navigation rail/i });
      // SidebarProvider defaults to open=true, so initial state is "expanded".
      expect(logo).toHaveAttribute("aria-expanded", "true");

      await user.click(logo);
      expect(logo).toHaveAttribute("aria-expanded", "false");

      await user.click(logo);
      expect(logo).toHaveAttribute("aria-expanded", "true");
    });
  });

  describe("S5 — anonymous user", () => {
    it("renders a Sign in link in the footer and no authenticated nav items when user is null", () => {
      mockUsePathname.mockReturnValue("/cardgroups");
      renderRail({ user: null, isAdmin: false });

      // Positive: anonymous users see a Sign in CTA in the rail footer.
      expect(screen.getByRole("link", { name: /sign in/i })).toBeInTheDocument();

      // Negative: none of the authenticated rail navigation items appear.
      expect(screen.queryByRole("link", { name: /cardgroups/i })).toBeNull();
      expect(screen.queryByRole("link", { name: /profile/i })).toBeNull();
      expect(screen.queryByRole("link", { name: /users/i })).toBeNull();
      expect(screen.queryByRole("link", { name: /roles/i })).toBeNull();
      expect(screen.queryByRole("link", { name: /dictionary/i })).toBeNull();
      expect(screen.queryByRole("link", { name: /settings/i })).toBeNull();

      // Negative: anonymous users get neither the LogoutButton nor an email line.
      expect(screen.queryByTestId("logout-button")).toBeNull();

      // The logo button still renders so anonymous viewers can read the brand.
      expect(screen.getByRole("button", { name: /toggle navigation rail/i })).toBeInTheDocument();
    });

    it("anonymous on /login: rail footer does not render the Sign in link or its empty wrapper", () => {
      mockUsePathname.mockReturnValue("/login");
      renderRail({ user: null, isAdmin: false });

      // The Sign in link must be absent — the parent guard suppresses the entire
      // footer block on /login, so no empty wrapper container is mounted either.
      expect(screen.queryByRole("link", { name: /sign in/i })).toBeNull();
    });
  });

  describe("rail footer", () => {
    it("renders a Profile link pointing to /profile", () => {
      mockUsePathname.mockReturnValue("/");
      const { container } = renderRail({
        user: { email: "alice@example.com" },
        isAdmin: false,
      });

      const profileLink = within(getFooter(container)).getByRole("link", { name: /profile/i });
      expect(profileLink).toHaveAttribute("href", "/profile");
    });

    it("renders the user's email in the footer when user.email is a string", () => {
      mockUsePathname.mockReturnValue("/");
      const { container } = renderRail({
        user: { email: "alice@example.com" },
        isAdmin: false,
      });

      expect(within(getFooter(container)).getByText("alice@example.com")).toBeInTheDocument();
    });

    it("does NOT render any email <p> when user.email is null", () => {
      mockUsePathname.mockReturnValue("/");
      const { container } = renderRail({
        user: { email: null },
        isAdmin: false,
      });

      // The footer is still mounted (Profile + Logout still render), but no
      // email text appears anywhere inside it.
      const footer = getFooter(container);
      expect(within(footer).queryByText(/@/)).not.toBeInTheDocument();
    });

    it("renders the Logout button (mocked) in the footer", () => {
      mockUsePathname.mockReturnValue("/");
      const { container } = renderRail({
        user: { email: "alice@example.com" },
        isAdmin: false,
      });

      expect(within(getFooter(container)).getByTestId("logout-button")).toBeInTheDocument();
    });

    it("the email <p> carries the group-data-[collapsible=icon]:hidden class so it collapses with the rail (per A-3)", () => {
      mockUsePathname.mockReturnValue("/");
      const { container } = renderRail({
        user: { email: "alice@example.com" },
        isAdmin: false,
      });

      const emailEl = within(getFooter(container)).getByText("alice@example.com");
      // Use className.includes — the element carries multiple Tailwind classes
      // and we only need to assert the presence of the collapsed-hide one.
      expect(emailEl.className.includes("group-data-[collapsible=icon]:hidden")).toBe(true);
    });

    it("the Logout button's wrapping div carries the group-data-[collapsible=icon]:hidden class so the labelled button hides in the icon-only state", () => {
      mockUsePathname.mockReturnValue("/");
      const { container } = renderRail({
        user: { email: "alice@example.com" },
        isAdmin: false,
      });

      const logoutBtn = within(getFooter(container)).getByTestId("logout-button");
      const wrapper = logoutBtn.parentElement;
      if (!wrapper) throw new Error("LogoutButton has no parent element");
      expect(wrapper.className.includes("group-data-[collapsible=icon]:hidden")).toBe(true);
    });
  });

  describe("S6 — hover flyout (useRef-based timer guard)", () => {
    it("pointerEnter on a collapsed rail expands it (aria-expanded becomes true)", async () => {
      const user = userEvent.setup();
      mockUsePathname.mockReturnValue("/");
      const { container } = renderRail({
        user: { email: "u@example.com" },
        isAdmin: false,
      });

      const logoBtn = screen.getByRole("button", { name: /toggle navigation rail/i });

      // First collapse the rail so we can test expand on hover.
      await user.click(logoBtn);
      expect(logoBtn).toHaveAttribute("aria-expanded", "false");

      // Find the Sidebar element that has the onPointerEnter handler.
      const sidebar = container.querySelector("[data-sidebar='sidebar']");
      if (!sidebar) throw new Error("[data-sidebar='sidebar'] element not found");

      // Fire pointer enter — React synthetic event fires through fireEvent.
      act(() => {
        fireEvent.pointerEnter(sidebar);
      });

      expect(logoBtn).toHaveAttribute("aria-expanded", "true");
    });

    it("no late state update after unmount when pointerLeave timer is pending", () => {
      vi.useFakeTimers();
      try {
        mockUsePathname.mockReturnValue("/");
        const { unmount, container } = renderRail({
          user: { email: "u@example.com" },
          isAdmin: false,
        });

        const sidebar = container.querySelector("[data-sidebar='sidebar']");
        if (!sidebar) throw new Error("[data-sidebar='sidebar'] element not found");

        // Trigger the hover-leave timer without letting it fire.
        act(() => {
          fireEvent.pointerLeave(sidebar);
        });

        // Unmount before the 150ms timer fires — the useEffect cleanup clears it.
        act(() => {
          unmount();
        });

        // Advance time past the timer: should not throw "Can't perform a React
        // state update on an unmounted component" warnings.
        act(() => {
          vi.advanceTimersByTime(200);
        });
        // If we reach here without error, the cleanup ref guard is working.
      } finally {
        vi.useRealTimers();
      }
    });
  });
});

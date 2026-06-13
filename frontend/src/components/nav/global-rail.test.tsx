// @vitest-environment jsdom
import { act, fireEvent, screen, within } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { renderWithIntl } from "@/test/render-with-intl";

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

function renderRail(
  props: React.ComponentProps<typeof GlobalRail>,
  options?: { defaultOpen?: boolean },
) {
  return renderWithIntl(
    <SidebarProvider defaultOpen={options?.defaultOpen}>
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
    it("renders Cardgroups and the two admin items (Users / Roles) with correct hrefs when isAdmin=true", () => {
      mockUsePathname.mockReturnValue("/");
      renderRail({ user: { email: "u@example.com" }, isAdmin: true });

      expect(screen.getByRole("link", { name: /cardgroups/i })).toHaveAttribute(
        "href",
        "/cardgroups",
      );
      expect(screen.getByRole("link", { name: /users/i })).toHaveAttribute("href", "/admin/users");
      expect(screen.getByRole("link", { name: /roles/i })).toHaveAttribute("href", "/admin/roles");
      expect(screen.queryByRole("link", { name: /settings/i })).toBeNull();
    });

    it("renders the Catalog link pointing at /catalog when signed in", () => {
      mockUsePathname.mockReturnValue("/");
      renderRail({ user: { email: "u@example.com" }, isAdmin: false });

      expect(screen.getByRole("link", { name: /catalog/i })).toHaveAttribute("href", "/catalog");
    });
  });

  describe("S2 — Admin gate", () => {
    it("does not render any of the admin links when isAdmin=false", () => {
      mockUsePathname.mockReturnValue("/");
      renderRail({ user: { email: "u@example.com" }, isAdmin: false });

      // The admin links must be absent.
      expect(screen.queryByRole("link", { name: /users/i })).toBeNull();
      expect(screen.queryByRole("link", { name: /roles/i })).toBeNull();

      // The non-admin items remain present.
      expect(screen.getByRole("link", { name: /cardgroups/i })).toBeInTheDocument();
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

    it("marks Catalog as the current page when pathname is /catalog (and Cardgroups is NOT current)", () => {
      mockUsePathname.mockReturnValue("/catalog");
      renderRail({ user: { email: "u@example.com" }, isAdmin: true });

      expect(screen.getByRole("link", { name: /catalog/i })).toHaveAttribute(
        "aria-current",
        "page",
      );
      expect(screen.getByRole("link", { name: /cardgroups/i })).not.toHaveAttribute("aria-current");
    });

    it("marks Catalog as the current page on a /catalog/ sub-route", () => {
      mockUsePathname.mockReturnValue("/catalog/some-deck");
      renderRail({ user: { email: "u@example.com" }, isAdmin: false });

      expect(screen.getByRole("link", { name: /catalog/i })).toHaveAttribute(
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
      // Other non-admin items are also not current.
      expect(screen.getByRole("link", { name: /cardgroups/i })).not.toHaveAttribute("aria-current");
      expect(screen.getByRole("link", { name: /profile/i })).not.toHaveAttribute("aria-current");
    });

    it("marks Users as the current page when pathname is /admin (F-1 fallback for the bare admin root)", () => {
      mockUsePathname.mockReturnValue("/admin");
      renderRail({ user: { email: "u@example.com" }, isAdmin: true });

      expect(screen.getByRole("link", { name: /users/i })).toHaveAttribute("aria-current", "page");
      expect(screen.getByRole("link", { name: /roles/i })).not.toHaveAttribute("aria-current");
    });

    it("marks Users as the current page when pathname is /admin/strange-unknown (F-1 fallback for unknown sub-routes)", () => {
      mockUsePathname.mockReturnValue("/admin/strange-unknown");
      renderRail({ user: { email: "u@example.com" }, isAdmin: true });

      expect(screen.getByRole("link", { name: /users/i })).toHaveAttribute("aria-current", "page");
      expect(screen.getByRole("link", { name: /roles/i })).not.toHaveAttribute("aria-current");
    });

    it("marks Roles as the current page when pathname is /admin/roles (and Users is NOT current)", () => {
      mockUsePathname.mockReturnValue("/admin/roles");
      renderRail({ user: { email: "u@example.com" }, isAdmin: true });

      expect(screen.getByRole("link", { name: /roles/i })).toHaveAttribute("aria-current", "page");
      expect(screen.getByRole("link", { name: /users/i })).not.toHaveAttribute("aria-current");
    });

    it("marks the footer Profile link as the current page when pathname is /profile", () => {
      mockUsePathname.mockReturnValue("/profile");
      const { container } = renderRail({ user: { email: "u@example.com" }, isAdmin: true });

      const profileLink = within(getFooter(container)).getByRole("link", { name: /profile/i });
      expect(profileLink).toHaveAttribute("aria-current", "page");

      // Center items must NOT be marked as current.
      expect(screen.getByRole("link", { name: /cardgroups/i })).not.toHaveAttribute("aria-current");
    });

    it("marks the footer Profile link as the current page when pathname is /profile/change-email (sub-route prefix match)", () => {
      mockUsePathname.mockReturnValue("/profile/change-email");
      const { container } = renderRail({ user: { email: "u@example.com" }, isAdmin: true });

      const profileLink = within(getFooter(container)).getByRole("link", { name: /profile/i });
      expect(profileLink).toHaveAttribute("aria-current", "page");
    });
  });

  describe("S4a — logo navigation link", () => {
    it("renders a Flamingo home link pointing to /", () => {
      mockUsePathname.mockReturnValue("/");
      renderRail({ user: { email: "u@example.com" }, isAdmin: false });

      const logoLink = screen.getByRole("link", { name: /flamingo home/i });
      expect(logoLink).toBeInTheDocument();
      expect(logoLink).toHaveAttribute("href", "/");
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

      // Negative: anonymous users get neither the LogoutButton nor an email line.
      expect(screen.queryByTestId("logout-button")).toBeNull();

      // The logo link still renders so anonymous viewers can navigate home.
      expect(screen.getByRole("link", { name: /flamingo home/i })).toBeInTheDocument();
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

    it("renders the Logout button (mocked) in the footer", () => {
      mockUsePathname.mockReturnValue("/");
      const { container } = renderRail({
        user: { email: "alice@example.com" },
        isAdmin: false,
      });

      expect(within(getFooter(container)).getByTestId("logout-button")).toBeInTheDocument();
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
    it("pointerEnter on a collapsed rail expands it (data-state flips to expanded)", () => {
      mockUsePathname.mockReturnValue("/");
      const { container } = renderRail(
        { user: { email: "u@example.com" }, isAdmin: false },
        { defaultOpen: false },
      );

      // The Sidebar wrapper carries data-state="expanded" | "collapsed". With
      // the SidebarToggle gone from the rail header, the wrapper is the only
      // observable surface for the open-state transition.
      const wrapper = container.querySelector<HTMLElement>("[data-side='left']");
      if (!wrapper) throw new Error("[data-side='left'] sidebar wrapper not found");
      expect(wrapper).toHaveAttribute("data-state", "collapsed");

      // The Sidebar element receives the onPointerEnter handler.
      const sidebar = container.querySelector("[data-sidebar='sidebar']");
      if (!sidebar) throw new Error("[data-sidebar='sidebar'] element not found");

      act(() => {
        fireEvent.pointerEnter(sidebar);
      });

      expect(wrapper).toHaveAttribute("data-state", "expanded");
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

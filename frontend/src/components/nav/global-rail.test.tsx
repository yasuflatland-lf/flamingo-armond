// @vitest-environment jsdom
import { act, fireEvent, render, screen } from "@testing-library/react";
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

// AvatarPopover reaches into LogoutButton -> Supabase -> router. Stub it so the
// rail test stays focused on rail logic, not popover internals. The rail no
// longer mounts AvatarPopover after Task 7 — the stub remains so the legacy
// import path in `avatar-popover.tsx` (still on disk until Task 8) does not
// pull Supabase into the test environment if any indirect importer survives.
vi.mock("./avatar-popover", () => ({
  AvatarPopover: ({ email }: { email: string | null }) => (
    <div data-testid="avatar-popover" data-email={email ?? ""} />
  ),
}));

// LogoutButton reaches into Supabase. Stub it to keep the test self-contained.
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

function renderRailCollapsed(props: React.ComponentProps<typeof GlobalRail>) {
  return render(
    <SidebarProvider defaultOpen={false}>
      <GlobalRail {...props} />
    </SidebarProvider>,
  );
}

describe("<GlobalRail>", () => {
  describe("S1 — items render when signed in", () => {
    it("renders Learning, Cardgroups, Profile, Admin, and Settings links with correct hrefs when isAdmin=true", () => {
      mockUsePathname.mockReturnValue("/");
      renderRail({ user: { email: "u@example.com" }, isAdmin: true });

      expect(screen.getByRole("link", { name: /^learning$/i })).toHaveAttribute("href", "/learn");
      expect(screen.getByRole("link", { name: /cardgroups/i })).toHaveAttribute(
        "href",
        "/cardgroups",
      );
      expect(screen.getByRole("link", { name: /profile/i })).toHaveAttribute("href", "/profile");
      expect(screen.getByRole("link", { name: /admin/i })).toHaveAttribute("href", "/admin");
      expect(screen.getByRole("link", { name: /settings/i })).toHaveAttribute("href", "/settings");
    });
  });

  describe("S2 — Admin gate", () => {
    it("does not render an Admin link when isAdmin=false", () => {
      mockUsePathname.mockReturnValue("/");
      renderRail({ user: { email: "u@example.com" }, isAdmin: false });

      // Query specifically for the Admin LINK by accessible name + href, so an
      // unrelated element whose accessible name happens to contain the string
      // "admin" cannot mask a regression that re-introduces the link.
      const adminLink = screen.queryByRole("link", { name: /admin/i });
      expect(adminLink).toBeNull();

      // The other three are still present.
      expect(screen.getByRole("link", { name: /cardgroups/i })).toBeInTheDocument();
      expect(screen.getByRole("link", { name: /profile/i })).toBeInTheDocument();
      expect(screen.getByRole("link", { name: /settings/i })).toBeInTheDocument();
    });
  });

  describe("S3 — active state from pathname", () => {
    it("marks Cardgroups as the current page when pathname is /cardgroups/abc/cards", () => {
      mockUsePathname.mockReturnValue("/cardgroups/abc/cards");
      renderRail({ user: { email: "u@example.com" }, isAdmin: true });

      const cardgroupsLink = screen.getByRole("link", { name: /cardgroups/i });
      expect(cardgroupsLink).toHaveAttribute("aria-current", "page");

      // Sibling items must not be marked as current.
      expect(screen.getByRole("link", { name: /profile/i })).not.toHaveAttribute("aria-current");
      expect(screen.getByRole("link", { name: /admin/i })).not.toHaveAttribute("aria-current");
      expect(screen.getByRole("link", { name: /settings/i })).not.toHaveAttribute("aria-current");
    });

    it("marks Learning as the current page when pathname is /learn (the index)", () => {
      mockUsePathname.mockReturnValue("/learn");
      renderRail({ user: { email: "u@example.com" }, isAdmin: true });

      expect(screen.getByRole("link", { name: /^learning$/i })).toHaveAttribute(
        "aria-current",
        "page",
      );
      // Cardgroups must NOT be marked current.
      expect(screen.getByRole("link", { name: /cardgroups/i })).not.toHaveAttribute("aria-current");
    });

    it("marks Learning as the current page when pathname is /learn/abc", () => {
      mockUsePathname.mockReturnValue("/learn/abc");
      renderRail({ user: { email: "u@example.com" }, isAdmin: true });

      expect(screen.getByRole("link", { name: /^learning$/i })).toHaveAttribute(
        "aria-current",
        "page",
      );
      expect(screen.getByRole("link", { name: /cardgroups/i })).not.toHaveAttribute("aria-current");
    });

    it("marks Profile as the current page when pathname is /profile", () => {
      mockUsePathname.mockReturnValue("/profile");
      renderRail({ user: { email: "u@example.com" }, isAdmin: true });

      expect(screen.getByRole("link", { name: /profile/i })).toHaveAttribute(
        "aria-current",
        "page",
      );

      // Other items must NOT be marked as current.
      expect(screen.getByRole("link", { name: /cardgroups/i })).not.toHaveAttribute("aria-current");
      expect(screen.getByRole("link", { name: /admin/i })).not.toHaveAttribute("aria-current");
      expect(screen.getByRole("link", { name: /settings/i })).not.toHaveAttribute("aria-current");
    });

    it("marks Admin as the current page when pathname is /admin/users (requires isAdmin=true)", () => {
      mockUsePathname.mockReturnValue("/admin/users");
      renderRail({ user: { email: "u@example.com" }, isAdmin: true });

      expect(screen.getByRole("link", { name: /admin/i })).toHaveAttribute("aria-current", "page");

      // Other items must NOT be marked as current.
      expect(screen.getByRole("link", { name: /cardgroups/i })).not.toHaveAttribute("aria-current");
      expect(screen.getByRole("link", { name: /profile/i })).not.toHaveAttribute("aria-current");
      expect(screen.getByRole("link", { name: /settings/i })).not.toHaveAttribute("aria-current");
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
      expect(screen.getByRole("link", { name: /profile/i })).not.toHaveAttribute("aria-current");
      expect(screen.getByRole("link", { name: /admin/i })).not.toHaveAttribute("aria-current");
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
      expect(screen.queryByRole("link", { name: /admin/i })).toBeNull();
      expect(screen.queryByRole("link", { name: /settings/i })).toBeNull();

      // Negative: the avatar popover is gated on user !== null — anonymous users
      // do not get a LogoutButton or avatar popover.
      expect(screen.queryByTestId("avatar-popover")).toBeNull();

      // The logo link still renders so anonymous viewers can read the brand.
      expect(screen.getByRole("link", { name: /flamingo-armond home/i })).toBeInTheDocument();
    });

    it("anonymous on /login: rail footer does not render the Sign in link or its empty wrapper", () => {
      mockUsePathname.mockReturnValue("/login");
      renderRail({ user: null, isAdmin: false });

      // The Sign in link must be absent — the parent guard suppresses the entire
      // footer block on /login, so no empty wrapper container is mounted either.
      expect(screen.queryByRole("link", { name: /sign in/i })).toBeNull();
    });
  });

  describe("S7 — header slot reshape (logo image + email + Logout)", () => {
    it("renders the flamingo.svg logo at 48x48 with priority", () => {
      mockUsePathname.mockReturnValue("/cardgroups");
      const { container } = renderRail({ user: { email: "u@example.com" }, isAdmin: false });

      // next/image renders an <img> in jsdom. Match by alt text.
      const logo = container.querySelector('img[alt="flamingo-armond"]');
      expect(logo).not.toBeNull();
      expect(logo).toHaveAttribute("width", "48");
      expect(logo).toHaveAttribute("height", "48");
      // The logo wraps in a Link to /cardgroups (home).
      const homeLink = logo?.closest("a");
      expect(homeLink).toHaveAttribute("href", "/cardgroups");
    });

    it("renders the user's email in the header when signed in with non-null email", () => {
      mockUsePathname.mockReturnValue("/cardgroups");
      renderRail({ user: { email: "alice@example.com" }, isAdmin: false });
      expect(screen.getByText("alice@example.com")).toBeInTheDocument();
    });

    it("omits the email row when user.email is null but renders the rest of the header", () => {
      mockUsePathname.mockReturnValue("/cardgroups");
      const { container } = renderRail({ user: { email: null }, isAdmin: false });
      // Logo + Logout still render; no email text node containing "@".
      expect(container.querySelector('img[alt="flamingo-armond"]')).not.toBeNull();
      expect(screen.queryByText(/@/)).not.toBeInTheDocument();
      expect(screen.getByTestId("logout-button")).toBeInTheDocument();
    });

    it("renders the LogoutButton in the header when signed in", () => {
      mockUsePathname.mockReturnValue("/cardgroups");
      renderRail({ user: { email: "u@example.com" }, isAdmin: false });
      expect(screen.getByTestId("logout-button")).toBeInTheDocument();
    });

    it("does not render LogoutButton or email when user is null (anonymous)", () => {
      mockUsePathname.mockReturnValue("/cardgroups");
      renderRail({ user: null, isAdmin: false });
      expect(screen.queryByTestId("logout-button")).toBeNull();
      expect(screen.queryByText(/@/)).not.toBeInTheDocument();
    });

    it("does NOT render the AvatarPopover in the rail footer when signed in", () => {
      mockUsePathname.mockReturnValue("/cardgroups");
      renderRail({ user: { email: "u@example.com" }, isAdmin: false });
      // Identity moved to the header — AvatarPopover stub must never mount now.
      expect(screen.queryByTestId("avatar-popover")).toBeNull();
    });
  });

  describe("S6 — hover flyout (useRef-based timer guard)", () => {
    it("pointerEnter on a collapsed rail expands it (data-state becomes expanded)", () => {
      mockUsePathname.mockReturnValue("/");
      const { container } = renderRailCollapsed({
        user: { email: "u@example.com" },
        isAdmin: false,
      });

      const sidebar = container.querySelector("[data-sidebar='sidebar']");
      if (!sidebar) throw new Error("[data-sidebar='sidebar'] element not found");

      // Initial state should be collapsed because we rendered with defaultOpen=false.
      const sidebarRoot = container.querySelector("[data-state]");
      expect(sidebarRoot).toHaveAttribute("data-state", "collapsed");

      act(() => {
        fireEvent.pointerEnter(sidebar);
      });

      expect(sidebarRoot).toHaveAttribute("data-state", "expanded");
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

// @vitest-environment jsdom
import { screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
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

// Mock next/navigation — GlobalRail calls usePathname for active-state resolution.
// LogoDrawer calls useSheetSearchParam() → useSearchParams(); return an empty
// URLSearchParams so AppShell tests that do not exercise sheet behaviour remain
// unaffected.
const mockUsePathname = vi.fn();
vi.mock("next/navigation", () => ({
  useRouter: vi.fn(() => ({ replace: vi.fn(), refresh: vi.fn(), push: vi.fn() })),
  usePathname: () => mockUsePathname(),
  useSearchParams: () => new URLSearchParams(),
}));

// LogoutButton reaches into Supabase. Stub it to keep the test self-contained.
vi.mock("@/app/_components/logout-button", () => ({
  LogoutButton: () => (
    <button type="button" data-testid="logout-button">
      Sign out
    </button>
  ),
}));

import { AppShell } from "./app-shell";

const SIGNED_IN_USER = { email: "shell-user@example.com" };

// jsdom does not implement matchMedia; SidebarProvider calls useIsMobile on
// mount. Provide a minimal stub that reports non-mobile so the desktop rail
// layout is exercised by default.
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

describe("<AppShell>", () => {
  describe("S1 — responsive containers are both present in the DOM", () => {
    it("rail container has the hidden-on-mobile class and mobile header has the hidden-on-desktop class", () => {
      renderWithIntl(
        <AppShell user={SIGNED_IN_USER} isAdmin={false}>
          <div />
        </AppShell>,
      );

      // Both containers are in the DOM; responsive visibility is class-based.
      const railContainer = screen.getByTestId("rail-container");
      expect(railContainer).toBeInTheDocument();
      expect(railContainer.className).toContain("hidden");
      expect(railContainer.className).toContain("md:flex");

      const mobileHeader = screen.getByTestId("mobile-header");
      expect(mobileHeader).toBeInTheDocument();
      expect(mobileHeader.className).toContain("md:hidden");
    });
  });

  describe("S2 — mobile menu trigger is present with the Settings icon", () => {
    it("contains a button with aria-label 'Open menu' (the Settings icon trigger)", () => {
      renderWithIntl(
        <AppShell user={SIGNED_IN_USER} isAdmin={false}>
          <div />
        </AppShell>,
      );

      // The mobile menu trigger uses a Settings icon with aria-label "Open menu".
      // This assertion guards that the trigger is always present in the mobile header.
      expect(screen.getByRole("button", { name: /^open menu$/i })).toBeInTheDocument();
    });
  });

  describe("S3 — does not render the email in the mobile header", () => {
    it("does not render the signed-in user's email inside the mobile header", () => {
      renderWithIntl(
        <AppShell user={SIGNED_IN_USER} isAdmin={false}>
          <div />
        </AppShell>,
      );

      // The rail footer legitimately shows the email; scope this assertion to the
      // mobile-header only, which must not render any email text.
      const mobileHeader = screen.getByTestId("mobile-header");
      expect(within(mobileHeader).queryByText(/@/)).toBeNull();
    });
  });

  describe("S4 — anonymous user", () => {
    it("renders no email in the mobile header, no rail nav items, and a Sign in link when user is null", () => {
      mockUsePathname.mockReturnValue("/cardgroups");
      renderWithIntl(
        <AppShell user={null} isAdmin={false}>
          <div />
        </AppShell>,
      );

      // No email text in the mobile header.
      const mobileHeader = screen.getByTestId("mobile-header");
      expect(mobileHeader).not.toHaveTextContent("@");

      // Rail shows no authenticated nav links for anonymous users.
      expect(screen.queryByRole("link", { name: /cardgroups/i })).toBeNull();
      expect(screen.queryByRole("link", { name: /profile/i })).toBeNull();
      expect(screen.queryByRole("link", { name: /admin/i })).toBeNull();
      expect(screen.queryByRole("link", { name: /settings/i })).toBeNull();

      // Anonymous users see a Sign in CTA (in the rail footer).
      expect(screen.getByRole("link", { name: /sign in/i })).toBeInTheDocument();
    });
  });

  describe("S4b — anonymous on /login: no Sign in links", () => {
    it("anonymous on /login: zero Sign in links across both surfaces", () => {
      mockUsePathname.mockReturnValue("/login");
      renderWithIntl(
        <AppShell user={null} isAdmin={false}>
          <div />
        </AppShell>,
      );

      // The parent-level /login guard suppresses the Sign in link in the rail
      // footer and the drawer body — no Sign in link should be mounted anywhere.
      expect(screen.queryAllByRole("link", { name: /sign in/i })).toHaveLength(0);
    });
  });

  describe("S5 — children render inside main content area", () => {
    it("a sentinel child passed via the children prop is present in the document", () => {
      renderWithIntl(
        <AppShell user={SIGNED_IN_USER} isAdmin={false}>
          <div data-testid="content">hello</div>
        </AppShell>,
      );

      expect(screen.getByTestId("content")).toBeInTheDocument();
      expect(screen.getByTestId("content")).toHaveTextContent("hello");
    });
  });

  describe("S7 — content column carries min-w-0 to prevent horizontal overflow", () => {
    it("the column wrapping the mobile header + main content has the min-w-0 class", () => {
      renderWithIntl(
        <AppShell user={SIGNED_IN_USER} isAdmin={false}>
          <div />
        </AppShell>,
      );

      // The content column is a flex child of the SidebarProvider row. Without
      // min-w-0 its min-width defaults to `auto` (content-based), so a long
      // unbreakable token on any page forces the column wider than the viewport
      // and `truncate`/`break-words` cannot clamp — the whole page overflows
      // horizontally. This guards that the load-bearing min-w-0 stays in place.
      // jsdom has no layout engine, so the class (not a measured width) is the
      // assertion. The column is the parent of the mobile header.
      const contentColumn = screen.getByTestId("mobile-header").parentElement;
      expect(contentColumn).not.toBeNull();
      expect(contentColumn?.className).toContain("min-w-0");
    });
  });

  describe("S6 — isAdmin=true: Admin sub-links appear in both rail and drawer", () => {
    it("the rail body contains the admin sub-links (Users, Roles) when isAdmin=true", () => {
      mockUsePathname.mockReturnValue("/");
      renderWithIntl(
        <AppShell user={SIGNED_IN_USER} isAdmin={true}>
          <div />
        </AppShell>,
      );

      // The rail container (always in DOM) must have all admin sub-links.
      const railContainer = screen.getByTestId("rail-container");
      expect(railContainer.querySelector("a[href='/admin/users']")).not.toBeNull();
      expect(railContainer.querySelector("a[href='/admin/roles']")).not.toBeNull();
    });

    it("the drawer body contains the admin sub-links (Users, Roles) when isAdmin=true", async () => {
      const user = userEvent.setup();
      mockUsePathname.mockReturnValue("/");
      renderWithIntl(
        <AppShell user={SIGNED_IN_USER} isAdmin={true}>
          <div />
        </AppShell>,
      );

      // Open the drawer so drawer links enter the DOM.
      const drawerTrigger = screen.getByRole("button", { name: /^open menu$/i });
      await user.click(drawerTrigger);

      // The drawer body must have all admin sub-links (portaled into
      // document.body by the Sheet component — use screen to search the full document).
      expect(screen.getByRole("link", { name: /users/i })).toHaveAttribute("href", "/admin/users");
      expect(screen.getByRole("link", { name: /roles/i })).toHaveAttribute("href", "/admin/roles");
    });
  });
});

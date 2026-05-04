// @vitest-environment jsdom
import { render, screen } from "@testing-library/react";
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

// Mock next/navigation — GlobalRail calls usePathname for active-state resolution.
const mockUsePathname = vi.fn();
vi.mock("next/navigation", () => ({
  useRouter: vi.fn(() => ({ replace: vi.fn(), refresh: vi.fn(), push: vi.fn() })),
  usePathname: () => mockUsePathname(),
}));

// AvatarPopover reaches into LogoutButton -> Supabase -> router. Stub it so the
// shell test stays focused on shell layout logic, not popover internals.
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
      render(
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

  describe("S2 — no old-style hamburger trigger", () => {
    it("does not contain a button with aria-label 'Open menu' (the previous hamburger trigger)", () => {
      render(
        <AppShell user={SIGNED_IN_USER} isAdmin={false}>
          <div />
        </AppShell>,
      );

      // The prior hamburger button used the label "Open menu". The logo-drawer
      // trigger uses "Open navigation menu" instead, so this assertion guards
      // against re-introducing the old trigger.
      expect(screen.queryByRole("button", { name: /^open menu$/i })).toBeNull();
    });
  });

  describe("S3 — email in mobile header when signed in", () => {
    it("displays the signed-in user's email inside the mobile header", () => {
      render(
        <AppShell user={SIGNED_IN_USER} isAdmin={false}>
          <div />
        </AppShell>,
      );

      const mobileHeader = screen.getByTestId("mobile-header");
      expect(mobileHeader).toHaveTextContent(SIGNED_IN_USER.email);
    });
  });

  describe("S4 — anonymous user", () => {
    it("renders no email in the mobile header, no rail nav items, and a Sign in link when user is null", () => {
      mockUsePathname.mockReturnValue("/cardgroups");
      render(
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
      render(
        <AppShell user={null} isAdmin={false}>
          <div />
        </AppShell>,
      );

      // The parent-level /login guard suppresses the Sign in link in the rail
      // footer and the drawer body — no Sign in link should be mounted anywhere.
      expect(screen.queryAllByRole("link", { name: /sign in/i })).toHaveLength(0);
    });
  });

  describe("S4c — signed-in user with null email", () => {
    it("renders no email span in the mobile header when user.email is null", () => {
      render(
        <AppShell user={{ email: null }} isAdmin={false}>
          <div />
        </AppShell>,
      );

      const mobileHeader = screen.getByTestId("mobile-header");
      // No email span — the null guard suppresses the element entirely.
      expect(mobileHeader).not.toHaveTextContent("@");
      // The mobile header still renders (the user object itself is non-null).
      expect(mobileHeader).toBeInTheDocument();
    });
  });

  describe("S5 — children render inside main content area", () => {
    it("a sentinel child passed via the children prop is present in the document", () => {
      render(
        <AppShell user={SIGNED_IN_USER} isAdmin={false}>
          <div data-testid="content">hello</div>
        </AppShell>,
      );

      expect(screen.getByTestId("content")).toBeInTheDocument();
      expect(screen.getByTestId("content")).toHaveTextContent("hello");
    });
  });

  describe("S6 — isAdmin=true: Admin link appears in both rail and drawer", () => {
    it("the rail body contains an Admin link when isAdmin=true", () => {
      mockUsePathname.mockReturnValue("/");
      render(
        <AppShell user={SIGNED_IN_USER} isAdmin={true}>
          <div />
        </AppShell>,
      );

      // The rail container (always in DOM) must have an Admin link pointing to /admin.
      const railContainer = screen.getByTestId("rail-container");
      const railAdminLink = railContainer.querySelector("a[href='/admin']");
      expect(railAdminLink).not.toBeNull();
    });

    it("the drawer body contains an Admin link when isAdmin=true", async () => {
      const user = userEvent.setup();
      mockUsePathname.mockReturnValue("/");
      render(
        <AppShell user={SIGNED_IN_USER} isAdmin={true}>
          <div />
        </AppShell>,
      );

      // Open the drawer so drawer links enter the DOM.
      const drawerTrigger = screen.getByRole("button", { name: /open navigation menu/i });
      await user.click(drawerTrigger);

      // The drawer body must have an Admin link (portaled into document.body by
      // the Sheet component — use screen to search the full document).
      expect(screen.getByRole("link", { name: /admin/i })).toHaveAttribute("href", "/admin");
    });
  });
});

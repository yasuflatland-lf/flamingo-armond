// @vitest-environment jsdom
import { render, screen, within } from "@testing-library/react";
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

// AvatarPopover reaches into LogoutButton -> Supabase -> router. Stub it so the
// rail test stays focused on rail logic, not popover internals.
vi.mock("./avatar-popover", () => ({
  AvatarPopover: ({ email }: { email: string }) => (
    <div data-testid="avatar-popover" data-email={email} />
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

describe("<GlobalRail>", () => {
  describe("S1 — items render when signed in", () => {
    it("renders Cardgroups, Profile, Admin, and Settings links with correct hrefs when isAdmin=true", () => {
      mockUsePathname.mockReturnValue("/");
      renderRail({ user: { email: "u@example.com" }, isAdmin: true });

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

    it("marks Cardgroups as the current page when pathname is /learn/abc (study session is part of the cardgroups flow)", () => {
      mockUsePathname.mockReturnValue("/learn/abc");
      renderRail({ user: { email: "u@example.com" }, isAdmin: true });

      expect(screen.getByRole("link", { name: /cardgroups/i })).toHaveAttribute(
        "aria-current",
        "page",
      );
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
    it("renders no rail items and no avatar popover when user is null", () => {
      mockUsePathname.mockReturnValue("/");
      renderRail({ user: null, isAdmin: false });

      // Negative: none of the rail navigation items appear.
      expect(screen.queryByRole("link", { name: /cardgroups/i })).toBeNull();
      expect(screen.queryByRole("link", { name: /profile/i })).toBeNull();
      expect(screen.queryByRole("link", { name: /admin/i })).toBeNull();
      expect(screen.queryByRole("link", { name: /settings/i })).toBeNull();

      // Negative: the avatar popover is gated on user !== null, matching the
      // existing anonymous behaviour in global-header.tsx (no LogoutButton when
      // anonymous).
      expect(screen.queryByTestId("avatar-popover")).toBeNull();

      // The logo button still renders so anonymous viewers can read the brand.
      expect(screen.getByRole("button", { name: /toggle navigation rail/i })).toBeInTheDocument();
    });
  });

  describe("avatar popover wiring", () => {
    it("forwards the user's email to the AvatarPopover when signed in", () => {
      mockUsePathname.mockReturnValue("/");
      const { container } = renderRail({
        user: { email: "alice@example.com" },
        isAdmin: false,
      });

      const popover = within(container).getByTestId("avatar-popover");
      expect(popover).toHaveAttribute("data-email", "alice@example.com");
    });
  });
});

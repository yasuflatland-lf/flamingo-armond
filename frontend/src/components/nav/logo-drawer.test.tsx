// @vitest-environment jsdom
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

// usePathname is mocked per-test so anonymous Sign-in-link tests can set a
// non-/login pathname (the link self-suppresses on /login).
const mockUsePathname = vi.fn();
// Stable router.push spy so the '+' button's fallback navigation is assertable.
// useSheetSearchParam (called unconditionally for the role-create branch) reads
// useSearchParams and writes via router.push; the role test asserts on the URL.
const mockPush = vi.fn();
const mockSearchParams = new URLSearchParams("");
vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: mockPush, replace: vi.fn() }),
  usePathname: () => mockUsePathname(),
  useSearchParams: () => mockSearchParams,
}));

// Mock LogoutButton — it reaches into Supabase/router which are not needed here.
vi.mock("@/app/_components/logout-button", () => ({
  LogoutButton: () => (
    <button type="button" data-testid="logout-button">
      Sign out
    </button>
  ),
}));

import { LogoDrawer } from "./logo-drawer";

const SIGNED_IN_USER = { email: "user@example.com" };

beforeEach(() => {
  // Default to a non-/login path so HeaderSignInLink renders for anonymous tests.
  mockUsePathname.mockReturnValue("/cardgroups");
  mockPush.mockClear();
});

afterEach(() => {
  vi.restoreAllMocks();
});

describe("<LogoDrawer>", () => {
  it("logo trigger opens the drawer and shows the Cardgroups link", async () => {
    const user = userEvent.setup();
    render(<LogoDrawer user={SIGNED_IN_USER} isAdmin={false} />);

    await user.click(screen.getByRole("button", { name: "Open menu" }));

    expect(screen.getByRole("link", { name: /cardgroups/i })).toBeInTheDocument();
  });

  it("logo is a home link with aria-label='Flamingo home' and href='/'", () => {
    render(<LogoDrawer user={SIGNED_IN_USER} isAdmin={false} />);

    const logoLink = screen.getByRole("link", { name: /flamingo home/i });
    expect(logoLink).toBeInTheDocument();
    expect(logoLink).toHaveAttribute("href", "/");
  });

  it("the drawer trigger button is labeled 'Open menu' (was 'Open navigation menu')", () => {
    render(<LogoDrawer user={SIGNED_IN_USER} isAdmin={false} />);
    expect(screen.getByRole("button", { name: "Open menu" })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /open navigation menu/i })).toBeNull();
  });

  it("Settings link is not rendered in the drawer body", async () => {
    const user = userEvent.setup();
    render(<LogoDrawer user={SIGNED_IN_USER} isAdmin={false} />);

    await user.click(screen.getByRole("button", { name: "Open menu" }));

    expect(screen.queryByRole("link", { name: /settings/i })).toBeNull();
  });

  it("isAdmin=false does not render any admin nav links", async () => {
    const user = userEvent.setup();
    render(<LogoDrawer user={SIGNED_IN_USER} isAdmin={false} />);

    await user.click(screen.getByRole("button", { name: "Open menu" }));

    expect(screen.queryByRole("link", { name: /^users$/i })).not.toBeInTheDocument();
    expect(screen.queryByRole("link", { name: /^roles$/i })).not.toBeInTheDocument();
    expect(screen.queryByRole("link", { name: /^dictionary$/i })).not.toBeInTheDocument();
  });

  it("isAdmin=true renders all three admin nav links with correct hrefs", async () => {
    const user = userEvent.setup();
    render(<LogoDrawer user={SIGNED_IN_USER} isAdmin={true} />);

    await user.click(screen.getByRole("button", { name: "Open menu" }));

    const usersLink = screen.getByRole("link", { name: /^users$/i });
    expect(usersLink).toBeInTheDocument();
    expect(usersLink).toHaveAttribute("href", "/admin/users");

    const rolesLink = screen.getByRole("link", { name: /^roles$/i });
    expect(rolesLink).toBeInTheDocument();
    expect(rolesLink).toHaveAttribute("href", "/admin/roles");

    const dictionaryLink = screen.getByRole("link", { name: /^dictionary$/i });
    expect(dictionaryLink).toBeInTheDocument();
    expect(dictionaryLink).toHaveAttribute("href", "/admin/dictionary");
  });

  it("Profile link in the bottom block has href=/profile and is present for signed-in users", async () => {
    const user = userEvent.setup();
    render(<LogoDrawer user={SIGNED_IN_USER} isAdmin={false} />);

    await user.click(screen.getByRole("button", { name: "Open menu" }));

    const bottomBlock = screen.getByTestId("bottom-block");
    const profileLink = within(bottomBlock).getByRole("link", { name: /profile/i });
    expect(profileLink).toBeInTheDocument();
    expect(profileLink).toHaveAttribute("href", "/profile");
  });

  it("anonymous user (user === null): drawer body shows a Sign in link but no nav items and no email", async () => {
    const user = userEvent.setup();
    render(<LogoDrawer user={null} isAdmin={false} />);

    await user.click(screen.getByRole("button", { name: "Open menu" }));

    expect(screen.getByRole("link", { name: /sign in/i })).toBeInTheDocument();
    expect(screen.queryByRole("link", { name: /cardgroups/i })).not.toBeInTheDocument();
    expect(screen.queryByRole("link", { name: /profile/i })).not.toBeInTheDocument();
    expect(screen.queryByText(/@/)).not.toBeInTheDocument();
  });

  it("anonymous user (user === null): no LogoutButton in the drawer body", async () => {
    const user = userEvent.setup();
    render(<LogoDrawer user={null} isAdmin={false} />);

    await user.click(screen.getByRole("button", { name: "Open menu" }));

    expect(screen.queryByTestId("logout-button")).not.toBeInTheDocument();
  });

  it("anonymous user on /login: Sign in link is suppressed (self-suppression behavior)", async () => {
    const user = userEvent.setup();
    mockUsePathname.mockReturnValue("/login");
    render(<LogoDrawer user={null} isAdmin={false} />);

    await user.click(screen.getByRole("button", { name: "Open menu" }));

    expect(screen.queryByRole("link", { name: /sign in/i })).not.toBeInTheDocument();
  });

  it("signed-in user: email address is not shown in the drawer body", async () => {
    const user = userEvent.setup();
    render(<LogoDrawer user={SIGNED_IN_USER} isAdmin={false} />);

    await user.click(screen.getByRole("button", { name: "Open menu" }));

    expect(screen.queryByText("user@example.com")).not.toBeInTheDocument();
  });

  it("signed-in user: LogoutButton is visible in the drawer body", async () => {
    const user = userEvent.setup();
    render(<LogoDrawer user={SIGNED_IN_USER} isAdmin={false} />);

    await user.click(screen.getByRole("button", { name: "Open menu" }));

    expect(screen.getByTestId("logout-button")).toBeInTheDocument();
  });

  it("signed-in user with null email: drawer shows nav items but no email text", async () => {
    const user = userEvent.setup();
    render(<LogoDrawer user={{ email: null }} isAdmin={false} />);

    await user.click(screen.getByRole("button", { name: "Open menu" }));

    expect(screen.getByRole("link", { name: /cardgroups/i })).toBeInTheDocument();
    expect(screen.queryByText(/@/)).not.toBeInTheDocument();
    expect(screen.getByTestId("logout-button")).toBeInTheDocument();
  });

  it("S-C1: on /cardgroups the '+' (Add new cardgroup) dispatches a cancelable add-cardgroup event and falls back to /cardgroups/new when unhandled", async () => {
    const user = userEvent.setup();
    mockUsePathname.mockReturnValue("/cardgroups");
    const listener = vi.fn();
    window.addEventListener("flamingo:add-cardgroup", listener);
    render(<LogoDrawer user={SIGNED_IN_USER} isAdmin={false} />);

    await user.click(screen.getByRole("button", { name: /add new cardgroup/i }));

    expect(listener).toHaveBeenCalledTimes(1);
    const event = listener.mock.calls[0]?.[0] as CustomEvent;
    expect(event.cancelable).toBe(true);
    expect(event.detail).toBeNull();
    expect(mockPush).toHaveBeenCalledWith("/cardgroups/new");

    window.removeEventListener("flamingo:add-cardgroup", listener);
  });

  it("S-C1b: on /cardgroups the '+' does not navigate when the add-cardgroup event is handled", async () => {
    const user = userEvent.setup();
    mockUsePathname.mockReturnValue("/cardgroups");
    const listener = vi.fn((event: Event) => event.preventDefault());
    window.addEventListener("flamingo:add-cardgroup", listener);
    render(<LogoDrawer user={SIGNED_IN_USER} isAdmin={false} />);

    await user.click(screen.getByRole("button", { name: /add new cardgroup/i }));

    expect(listener).toHaveBeenCalledTimes(1);
    expect(mockPush).not.toHaveBeenCalled();

    window.removeEventListener("flamingo:add-cardgroup", listener);
  });

  it("S-E1: on /cardgroups/:id/edit the '+' (Add new card) dispatches add-card with the decoded id and falls back to an encoded /cards/new href", async () => {
    const user = userEvent.setup();
    mockUsePathname.mockReturnValue("/cardgroups/abc-123/edit");
    const listener = vi.fn();
    window.addEventListener("flamingo:add-card", listener);
    render(<LogoDrawer user={SIGNED_IN_USER} isAdmin={false} />);

    await user.click(screen.getByRole("button", { name: /add new card/i }));

    expect(listener).toHaveBeenCalledTimes(1);
    const event = listener.mock.calls[0]?.[0] as CustomEvent<{ cardgroupId: string }>;
    expect(event.cancelable).toBe(true);
    expect(event.detail).toEqual({ cardgroupId: "abc-123" });
    expect(mockPush).toHaveBeenCalledWith("/cards/new?cardgroup=abc-123");

    window.removeEventListener("flamingo:add-card", listener);
  });

  it("S-L1: on /learn/:id the '+' (Add new card) dispatches a cancelable add-card event and falls back to /cards/new with a return param when unhandled", async () => {
    const user = userEvent.setup();
    mockUsePathname.mockReturnValue("/learn/abc-123");
    const listener = vi.fn();
    window.addEventListener("flamingo:add-card", listener);
    render(<LogoDrawer user={SIGNED_IN_USER} isAdmin={false} />);

    await user.click(screen.getByRole("button", { name: /add new card/i }));

    expect(listener).toHaveBeenCalledTimes(1);
    const event = listener.mock.calls[0]?.[0] as CustomEvent<{ cardgroupId: string }>;
    expect(event.cancelable).toBe(true);
    expect(event.detail).toEqual({ cardgroupId: "abc-123" });
    expect(mockPush).toHaveBeenCalledWith("/cards/new?cardgroup=abc-123&return=/learn/abc-123");

    window.removeEventListener("flamingo:add-card", listener);
  });

  it("S-L1b: on /learn/:id the '+' does not navigate when the add-card event is handled", async () => {
    const user = userEvent.setup();
    mockUsePathname.mockReturnValue("/learn/abc-123");
    const listener = vi.fn((event: Event) => event.preventDefault());
    window.addEventListener("flamingo:add-card", listener);
    render(<LogoDrawer user={SIGNED_IN_USER} isAdmin={false} />);

    await user.click(screen.getByRole("button", { name: /add new card/i }));

    expect(listener).toHaveBeenCalledTimes(1);
    expect(mockPush).not.toHaveBeenCalled();

    window.removeEventListener("flamingo:add-card", listener);
  });

  it("S-R1: on /admin/roles the '+' (Add new role) opens the create sheet by writing ?new=true to the URL", async () => {
    const user = userEvent.setup();
    mockUsePathname.mockReturnValue("/admin/roles");
    render(<LogoDrawer user={SIGNED_IN_USER} isAdmin={true} />);

    await user.click(screen.getByRole("button", { name: /add new role/i }));

    expect(mockPush).toHaveBeenCalledTimes(1);
    const target = mockPush.mock.calls[0]?.[0] as string;
    expect(target).toContain("new=true");
  });

  it("S-G1: on an unknown route (/profile) the '+' button is not rendered", () => {
    mockUsePathname.mockReturnValue("/profile");
    render(<LogoDrawer user={SIGNED_IN_USER} isAdmin={false} />);
    expect(screen.queryByRole("button", { name: /add new/i })).toBeNull();
  });

  it("S-G2: anonymous user (user === null) sees no '+' button on any create route", () => {
    mockUsePathname.mockReturnValue("/cardgroups");
    render(<LogoDrawer user={null} isAdmin={false} />);
    expect(screen.queryByRole("button", { name: /add new/i })).toBeNull();
  });

  it("S-G3: the '+' button receives keyboard focus before the menu trigger (order: logo · + · menu)", async () => {
    const user = userEvent.setup();
    mockUsePathname.mockReturnValue("/cardgroups");
    render(<LogoDrawer user={SIGNED_IN_USER} isAdmin={false} />);

    const addButton = screen.getByRole("button", { name: /add new cardgroup/i });
    const menuButton = screen.getByRole("button", { name: "Open menu" });

    addButton.focus();
    expect(addButton).toHaveFocus();
    await user.tab();
    expect(menuButton).toHaveFocus();
  });

  it("S-G4: the menu trigger renders alongside the '+' on create routes", () => {
    mockUsePathname.mockReturnValue("/cardgroups");
    render(<LogoDrawer user={SIGNED_IN_USER} isAdmin={false} />);
    expect(screen.getByRole("button", { name: /add new cardgroup/i })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Open menu" })).toBeInTheDocument();
  });
});

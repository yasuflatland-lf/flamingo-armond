// @vitest-environment jsdom
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

// usePathname is mocked per-test so anonymous Sign-in-link tests can set a
// non-/login pathname (the link self-suppresses on /login).
const mockUsePathname = vi.fn();
vi.mock("next/navigation", () => ({
  useRouter: vi.fn(() => ({ push: vi.fn(), replace: vi.fn() })),
  usePathname: () => mockUsePathname(),
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
    mockUsePathname.mockReturnValue("/cardgroups");
    render(<LogoDrawer user={null} isAdmin={false} />);

    await user.click(screen.getByRole("button", { name: "Open menu" }));

    // Anonymous users see a Sign in CTA.
    expect(screen.getByRole("link", { name: /sign in/i })).toBeInTheDocument();

    // No authenticated nav items.
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

    // HeaderSignInLink self-suppresses on /login.
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

    // Nav links are still present — the user is signed in.
    expect(screen.getByRole("link", { name: /cardgroups/i })).toBeInTheDocument();
    // No email text in the bottom block when email is null.
    expect(screen.queryByText(/@/)).not.toBeInTheDocument();
    // LogoutButton is still present.
    expect(screen.getByTestId("logout-button")).toBeInTheDocument();
  });

  it("S-L1: on /learn/:id the '+' link exists with the right href", () => {
    mockUsePathname.mockReturnValue("/learn/abc-123");
    render(<LogoDrawer user={SIGNED_IN_USER} isAdmin={false} />);
    const addLink = screen.getByRole("link", { name: /add a new card to this cardgroup/i });
    expect(addLink).toHaveAttribute("href", "/cards/new?cardgroup=abc-123&return=/learn/abc-123");
  });

  it("S-L2: on a non-learn route the '+' link is not rendered", () => {
    mockUsePathname.mockReturnValue("/cardgroups");
    render(<LogoDrawer user={SIGNED_IN_USER} isAdmin={false} />);
    expect(screen.queryByRole("link", { name: /add a new card to this cardgroup/i })).toBeNull();
  });

  it("S-L3: on /learn/:id the '+' link receives keyboard focus before the menu trigger", async () => {
    const user = userEvent.setup();
    mockUsePathname.mockReturnValue("/learn/abc-123");
    render(<LogoDrawer user={SIGNED_IN_USER} isAdmin={false} />);

    const addLink = screen.getByRole("link", { name: /add a new card to this cardgroup/i });
    const menuButton = screen.getByRole("button", { name: "Open menu" });

    addLink.focus();
    expect(addLink).toHaveFocus();
    await user.tab();
    expect(menuButton).toHaveFocus();
  });

  it("S-L4: cardgroupId with special characters round-trips to single-encoded href (matches LearnAddCardFloating)", () => {
    // usePathname returns the percent-encoded pathname as delivered by the browser.
    // The component decodes the segment, then the JSX re-encodes once via
    // encodeURIComponent, producing a href identical to what LearnAddCardFloating
    // generates from a decoded `cardgroupId` prop.
    mockUsePathname.mockReturnValue("/learn/abc%26evil");
    render(<LogoDrawer user={SIGNED_IN_USER} isAdmin={false} />);
    const addLink = screen.getByRole("link", { name: /add a new card to this cardgroup/i });
    expect(addLink).toHaveAttribute(
      "href",
      "/cards/new?cardgroup=abc%26evil&return=/learn/abc%26evil",
    );
  });

  it("S-L5: anonymous user on /learn/:id does not see the '+' link", () => {
    mockUsePathname.mockReturnValue("/learn/abc-123");
    render(<LogoDrawer user={null} isAdmin={false} />);
    expect(screen.queryByRole("link", { name: /add a new card to this cardgroup/i })).toBeNull();
  });

  it("S-L6: regex rejects /learn/:id sub-routes — '+' link is absent on /learn/abc-123/edit", () => {
    mockUsePathname.mockReturnValue("/learn/abc-123/edit");
    render(<LogoDrawer user={SIGNED_IN_USER} isAdmin={false} />);
    expect(screen.queryByRole("link", { name: /add a new card to this cardgroup/i })).toBeNull();
  });

  it("S-L7: malformed percent-escape in /learn/:id does not crash — '+' link is absent", () => {
    // safeDecodePathSegment returns null for malformed %XX sequences; the conditional
    // JSX skips the '+' link rather than throwing URIError and crashing the layout shell.
    mockUsePathname.mockReturnValue("/learn/abc%XX");
    render(<LogoDrawer user={SIGNED_IN_USER} isAdmin={false} />);
    expect(screen.queryByRole("link", { name: /add a new card to this cardgroup/i })).toBeNull();
  });
});

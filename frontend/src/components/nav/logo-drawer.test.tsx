// @vitest-environment jsdom
import { render, screen } from "@testing-library/react";
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

    await user.click(screen.getByRole("button", { name: "Open navigation menu" }));

    expect(screen.getByRole("link", { name: /cardgroups/i })).toBeInTheDocument();
  });

  it("renders the flamingo.svg trigger at 48x48 (text label is gone)", () => {
    render(<LogoDrawer user={SIGNED_IN_USER} isAdmin={false} />);

    const trigger = screen.getByRole("button", { name: "Open navigation menu" });
    const logo = trigger.querySelector('img[alt="flamingo-armond"]');
    expect(logo).not.toBeNull();
    expect(logo).toHaveAttribute("width", "48");
    expect(logo).toHaveAttribute("height", "48");
    // The literal text "flamingo-armond" must not appear inside the trigger.
    expect(trigger).not.toHaveTextContent(/flamingo-armond/);
  });

  it("Learning link is present at the top of the drawer body and points at /learn", async () => {
    const user = userEvent.setup();
    render(<LogoDrawer user={SIGNED_IN_USER} isAdmin={false} />);

    await user.click(screen.getByRole("button", { name: "Open navigation menu" }));

    const learning = screen.getByRole("link", { name: /^learning$/i });
    expect(learning).toBeInTheDocument();
    expect(learning).toHaveAttribute("href", "/learn");
  });

  it("no hamburger-style 'Open menu' trigger exists — the new logo-driven drawer trigger replaces the prior hamburger trigger", () => {
    render(<LogoDrawer user={SIGNED_IN_USER} isAdmin={false} />);

    // The prior hamburger trigger used aria-label "Open menu". The logo-driven
    // drawer must not carry that label — it uses "Open navigation menu" instead.
    expect(screen.queryByRole("button", { name: /open menu/i })).toBeNull();
  });

  it("Settings link is present in the drawer body", async () => {
    const user = userEvent.setup();
    render(<LogoDrawer user={SIGNED_IN_USER} isAdmin={false} />);

    await user.click(screen.getByRole("button", { name: "Open navigation menu" }));

    const settingsLink = screen.getByRole("link", { name: /settings/i });
    expect(settingsLink).toBeInTheDocument();
    expect(settingsLink).toHaveAttribute("href", "/settings");
  });

  it("isAdmin=false does not render the Admin link", async () => {
    const user = userEvent.setup();
    render(<LogoDrawer user={SIGNED_IN_USER} isAdmin={false} />);

    await user.click(screen.getByRole("button", { name: "Open navigation menu" }));

    expect(screen.queryByRole("link", { name: /^admin$/i })).not.toBeInTheDocument();
  });

  it("isAdmin=true renders the Admin link pointing to /admin", async () => {
    const user = userEvent.setup();
    render(<LogoDrawer user={SIGNED_IN_USER} isAdmin={true} />);

    await user.click(screen.getByRole("button", { name: "Open navigation menu" }));

    const adminLink = screen.getByRole("link", { name: /admin/i });
    expect(adminLink).toBeInTheDocument();
    expect(adminLink).toHaveAttribute("href", "/admin");
  });

  it("anonymous user (user === null): drawer body shows a Sign in link but no nav items and no email", async () => {
    const user = userEvent.setup();
    mockUsePathname.mockReturnValue("/cardgroups");
    render(<LogoDrawer user={null} isAdmin={false} />);

    await user.click(screen.getByRole("button", { name: "Open navigation menu" }));

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

    await user.click(screen.getByRole("button", { name: "Open navigation menu" }));

    expect(screen.queryByTestId("logout-button")).not.toBeInTheDocument();
  });

  it("anonymous user on /login: Sign in link is suppressed (self-suppression behavior)", async () => {
    const user = userEvent.setup();
    mockUsePathname.mockReturnValue("/login");
    render(<LogoDrawer user={null} isAdmin={false} />);

    await user.click(screen.getByRole("button", { name: "Open navigation menu" }));

    // HeaderSignInLink self-suppresses on /login.
    expect(screen.queryByRole("link", { name: /sign in/i })).not.toBeInTheDocument();
  });

  it("signed-in user: email address is visible in the drawer body", async () => {
    const user = userEvent.setup();
    render(<LogoDrawer user={SIGNED_IN_USER} isAdmin={false} />);

    await user.click(screen.getByRole("button", { name: "Open navigation menu" }));

    expect(screen.getByText("user@example.com")).toBeInTheDocument();
  });

  it("signed-in user: LogoutButton is visible in the drawer body", async () => {
    const user = userEvent.setup();
    render(<LogoDrawer user={SIGNED_IN_USER} isAdmin={false} />);

    await user.click(screen.getByRole("button", { name: "Open navigation menu" }));

    expect(screen.getByTestId("logout-button")).toBeInTheDocument();
  });

  it("signed-in user with null email: drawer shows nav items but no email text", async () => {
    const user = userEvent.setup();
    render(<LogoDrawer user={{ email: null }} isAdmin={false} />);

    await user.click(screen.getByRole("button", { name: "Open navigation menu" }));

    // Nav links are still present — the user is signed in.
    expect(screen.getByRole("link", { name: /cardgroups/i })).toBeInTheDocument();
    // No email text in the bottom block when email is null.
    expect(screen.queryByText(/@/)).not.toBeInTheDocument();
    // LogoutButton is still present.
    expect(screen.getByTestId("logout-button")).toBeInTheDocument();
  });
});

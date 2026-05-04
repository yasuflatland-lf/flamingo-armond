// @vitest-environment jsdom
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";

// Mock next/navigation so Link and other hooks resolve in jsdom.
vi.mock("next/navigation", () => ({
  useRouter: vi.fn(() => ({ push: vi.fn(), replace: vi.fn() })),
  usePathname: vi.fn(() => "/"),
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

  it("anonymous user (user === null): drawer body shows no nav items and no email", async () => {
    const user = userEvent.setup();
    render(<LogoDrawer user={null} isAdmin={false} />);

    await user.click(screen.getByRole("button", { name: "Open navigation menu" }));

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
});

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
  LogoutButton: () => <button data-testid="logout-button">Sign out</button>,
}));

import { HamburgerDrawer } from "./hamburger-drawer";

afterEach(() => {
  vi.restoreAllMocks();
});

describe("<HamburgerDrawer>", () => {
  it("renders the trigger button with aria-label 'Open menu'", () => {
    render(<HamburgerDrawer isAdmin={false} />);
    expect(screen.getByRole("button", { name: "Open menu" })).toBeInTheDocument();
  });

  it("drawer content is not visible before trigger is clicked", () => {
    render(<HamburgerDrawer isAdmin={false} />);
    // The Sheet portal is in the DOM but closed; nav links should not be visible.
    expect(screen.queryByRole("link", { name: /cardgroups/i })).not.toBeInTheDocument();
  });

  it("clicking trigger opens the drawer and shows Cardgroups link", async () => {
    const user = userEvent.setup();
    render(<HamburgerDrawer isAdmin={false} />);

    await user.click(screen.getByRole("button", { name: "Open menu" }));

    expect(screen.getByRole("link", { name: /cardgroups/i })).toBeInTheDocument();
  });

  it("clicking trigger opens the drawer and shows Profile link", async () => {
    const user = userEvent.setup();
    render(<HamburgerDrawer isAdmin={false} />);

    await user.click(screen.getByRole("button", { name: "Open menu" }));

    expect(screen.getByRole("link", { name: /profile/i })).toBeInTheDocument();
  });

  it("clicking trigger opens the drawer and shows LogoutButton", async () => {
    const user = userEvent.setup();
    render(<HamburgerDrawer isAdmin={false} />);

    await user.click(screen.getByRole("button", { name: "Open menu" }));

    expect(screen.getByTestId("logout-button")).toBeInTheDocument();
  });

  it("Cardgroups link points to /cardgroups", async () => {
    const user = userEvent.setup();
    render(<HamburgerDrawer isAdmin={false} />);

    await user.click(screen.getByRole("button", { name: "Open menu" }));

    expect(screen.getByRole("link", { name: /cardgroups/i })).toHaveAttribute("href", "/cardgroups");
  });

  it("Profile link points to /profile", async () => {
    const user = userEvent.setup();
    render(<HamburgerDrawer isAdmin={false} />);

    await user.click(screen.getByRole("button", { name: "Open menu" }));

    expect(screen.getByRole("link", { name: /profile/i })).toHaveAttribute("href", "/profile");
  });

  it("isAdmin=true renders Admin link pointing to /admin", async () => {
    const user = userEvent.setup();
    render(<HamburgerDrawer isAdmin={true} />);

    await user.click(screen.getByRole("button", { name: "Open menu" }));

    const adminLink = screen.getByRole("link", { name: /admin/i });
    expect(adminLink).toBeInTheDocument();
    expect(adminLink).toHaveAttribute("href", "/admin");
  });

  it("isAdmin=false does not render Admin link", async () => {
    const user = userEvent.setup();
    render(<HamburgerDrawer isAdmin={false} />);

    await user.click(screen.getByRole("button", { name: "Open menu" }));

    expect(screen.queryByRole("link", { name: /^admin$/i })).not.toBeInTheDocument();
  });

  it("displays userEmail when provided", async () => {
    const user = userEvent.setup();
    render(<HamburgerDrawer isAdmin={false} userEmail="user@example.com" />);

    await user.click(screen.getByRole("button", { name: "Open menu" }));

    expect(screen.getByText("user@example.com")).toBeInTheDocument();
  });

  it("does not display email section when userEmail is not provided", async () => {
    const user = userEvent.setup();
    render(<HamburgerDrawer isAdmin={false} />);

    await user.click(screen.getByRole("button", { name: "Open menu" }));

    expect(screen.queryByText(/@/)).not.toBeInTheDocument();
  });

  it("does not display email section when userEmail is null", async () => {
    const user = userEvent.setup();
    render(<HamburgerDrawer isAdmin={false} userEmail={null} />);

    await user.click(screen.getByRole("button", { name: "Open menu" }));

    expect(screen.queryByText(/@/)).not.toBeInTheDocument();
  });
});

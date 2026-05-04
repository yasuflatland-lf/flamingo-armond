// @vitest-environment jsdom
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";

// Mock next/navigation — LogoutButton uses useRouter internally.
vi.mock("next/navigation", () => ({
  useRouter: vi.fn(() => ({ replace: vi.fn(), refresh: vi.fn() })),
  usePathname: vi.fn(() => "/"),
}));

// Mock the Supabase browser client so signOut is observable in tests.
const mockSignOut = vi.fn();
vi.mock("@/lib/supabase/client", () => ({
  createSupabaseBrowserClient: () => ({
    auth: { signOut: () => mockSignOut() },
  }),
}));

import { AvatarPopover } from "./avatar-popover";

afterEach(() => {
  vi.restoreAllMocks();
  mockSignOut.mockReset();
});

describe("<AvatarPopover>", () => {
  describe("S1 — closed by default", () => {
    it("does not show email or Logout button before trigger is clicked", () => {
      render(<AvatarPopover email="user@example.com" />);

      // Email content is inside the popover content which is not visible initially.
      expect(screen.queryByText("user@example.com")).not.toBeInTheDocument();
      // Logout button is also inside the popover and should not be visible.
      expect(screen.queryByRole("button", { name: /logout/i })).not.toBeInTheDocument();
    });
  });

  describe("S2 — open on click", () => {
    it("shows email and Logout button after clicking the trigger", async () => {
      const user = userEvent.setup();
      render(<AvatarPopover email="user@example.com" />);

      await user.click(screen.getByRole("button", { name: "Open account menu" }));

      expect(screen.getByText("user@example.com")).toBeInTheDocument();
      expect(screen.getByRole("button", { name: /logout/i })).toBeInTheDocument();
    });
  });

  describe("S3 — PII negative: Profile/Admin absence", () => {
    it("does not contain Profile, Admin, or display_name text after opening", async () => {
      // PII rule per .claude/rules/frontend-typescript-conventions.md —
      // popover shows only email (no Profile link, no Admin link, no display_name).
      const user = userEvent.setup();
      render(<AvatarPopover email="user@example.com" />);

      await user.click(screen.getByRole("button", { name: "Open account menu" }));

      expect(screen.queryByText(/profile/i)).toBeNull();
      expect(screen.queryByText(/admin/i)).toBeNull();
      expect(screen.queryByText(/display_name/i)).toBeNull();
    });
  });

  describe("S4 — Logout dispatch", () => {
    it("calls signOut once when Logout button is clicked", async () => {
      mockSignOut.mockResolvedValue({ error: null });
      const user = userEvent.setup();
      render(<AvatarPopover email="user@example.com" />);

      // Open the popover first.
      await user.click(screen.getByRole("button", { name: "Open account menu" }));

      // Click the Logout button inside the popover.
      await user.click(screen.getByRole("button", { name: /logout/i }));

      expect(mockSignOut).toHaveBeenCalledTimes(1);
    });
  });

  describe("avatar trigger", () => {
    it("shows the uppercased first letter of the email as the avatar initial", () => {
      render(<AvatarPopover email="alice@example.com" />);
      // The trigger shows "A" (first letter of "alice" uppercased).
      expect(screen.getByText("A")).toBeInTheDocument();
    });

    it("renders the trigger button with correct aria-label", () => {
      render(<AvatarPopover email="user@example.com" />);
      expect(screen.getByRole("button", { name: "Open account menu" })).toBeInTheDocument();
    });
  });
});

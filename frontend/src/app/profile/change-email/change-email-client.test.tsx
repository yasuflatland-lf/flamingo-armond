// @vitest-environment jsdom

import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const mockUpdateUser = vi.fn();
vi.mock("@/lib/supabase/client", () => ({
  createSupabaseBrowserClient: () => ({ auth: { updateUser: mockUpdateUser } }),
}));

import { ChangeEmailClient } from "./change-email-client";

beforeEach(() => {
  mockUpdateUser.mockReset();
});

afterEach(() => {
  vi.restoreAllMocks();
});

describe("<ChangeEmailClient>", () => {
  it("S1 renders the current email and a new email input", () => {
    render(<ChangeEmailClient currentEmail="alice@example.com" />);

    expect(screen.getByText("alice@example.com")).toBeInTheDocument();
    expect(screen.getByLabelText(/new email/i)).toBeInTheDocument();
  });

  it("S2 successful update switches to the success view", async () => {
    const user = userEvent.setup();
    mockUpdateUser.mockResolvedValue({ data: { user: {} }, error: null });

    render(<ChangeEmailClient currentEmail="alice@example.com" />);

    const input = screen.getByLabelText(/new email/i);
    await user.type(input, "bob@example.com");
    await user.click(screen.getByRole("button", { name: /send confirmation link/i }));

    await waitFor(() => {
      expect(screen.getByText(/confirmation link sent to/i)).toBeInTheDocument();
    });
    expect(screen.getByText("bob@example.com")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /back to profile/i })).toHaveAttribute(
      "href",
      "/profile",
    );
  });

  it("S3 maps a Supabase rate-limit error to user-facing copy", async () => {
    const user = userEvent.setup();
    // Suppress the deliberate console.warn from the client's failure branch
    // so the test output stays clean. The warn does not contain PII (no email).
    const consoleWarnSpy = vi.spyOn(console, "warn").mockImplementation(() => {});
    mockUpdateUser.mockResolvedValue({
      data: null,
      error: { name: "AuthApiError", message: "Email rate limit exceeded" },
    });

    render(<ChangeEmailClient currentEmail="alice@example.com" />);

    const input = screen.getByLabelText(/new email/i);
    await user.type(input, "bob@example.com");
    await user.click(screen.getByRole("button", { name: /send confirmation link/i }));

    await waitFor(() => {
      expect(
        screen.getByText("Too many requests. Please wait a moment and try again."),
      ).toBeInTheDocument();
    });
    // The warn must not include the new email address (PII absence).
    for (const call of consoleWarnSpy.mock.calls) {
      for (const arg of call) {
        expect(String(arg)).not.toContain("bob@example.com");
      }
    }
  });

  it("S4 Cancel button links to /profile", () => {
    render(<ChangeEmailClient currentEmail="alice@example.com" />);

    const cancelLink = screen.getByRole("link", { name: /cancel/i });
    expect(cancelLink).toHaveAttribute("href", "/profile");
  });
});

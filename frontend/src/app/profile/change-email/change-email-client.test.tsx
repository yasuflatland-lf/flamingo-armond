// @vitest-environment jsdom

import { fireEvent, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { renderWithIntl } from "@/test/render-with-intl";

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
    renderWithIntl(<ChangeEmailClient currentEmail="alice@example.com" />);

    expect(screen.getByText("alice@example.com")).toBeInTheDocument();
    expect(screen.getByLabelText(/new email/i)).toBeInTheDocument();
  });

  it("S2 successful update switches to the success view", async () => {
    const user = userEvent.setup();
    mockUpdateUser.mockResolvedValue({ data: { user: {} }, error: null });

    renderWithIntl(<ChangeEmailClient currentEmail="alice@example.com" />);

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

    renderWithIntl(<ChangeEmailClient currentEmail="alice@example.com" />);

    const input = screen.getByLabelText(/new email/i);
    await user.type(input, "bob@example.com");
    await user.click(screen.getByRole("button", { name: /send confirmation link/i }));

    await waitFor(() => {
      expect(
        screen.getByText("Too many requests. Please wait a moment and try again."),
      ).toBeInTheDocument();
    });
    // Structural exact-call assertion: only the error name is logged, never the email (PII).
    expect(consoleWarnSpy).toHaveBeenCalledWith(
      "[change-email] updateUser failed:",
      "AuthApiError",
    );
  });

  it("S5 maps an already-registered error to user-facing copy", async () => {
    const user = userEvent.setup();
    const consoleWarnSpy = vi.spyOn(console, "warn").mockImplementation(() => {});
    mockUpdateUser.mockResolvedValue({
      data: null,
      error: { name: "AuthApiError", message: "User already registered" },
    });

    renderWithIntl(<ChangeEmailClient currentEmail="alice@example.com" />);

    const input = screen.getByLabelText(/new email/i);
    await user.type(input, "bob@example.com");
    await user.click(screen.getByRole("button", { name: /send confirmation link/i }));

    await waitFor(() => {
      expect(screen.getByText("That email address is already in use.")).toBeInTheDocument();
    });
    expect(consoleWarnSpy).toHaveBeenCalledWith(
      "[change-email] updateUser failed:",
      "AuthApiError",
    );
  });

  it("S6 shows the generic copy and logs the raw message on unmapped errors", async () => {
    const user = userEvent.setup();
    const consoleWarnSpy = vi.spyOn(console, "warn").mockImplementation(() => {});
    mockUpdateUser.mockResolvedValue({
      data: null,
      error: { name: "AuthApiError", message: "some unmapped error xyz" },
    });

    renderWithIntl(<ChangeEmailClient currentEmail="alice@example.com" />);

    const input = screen.getByLabelText(/new email/i);
    await user.type(input, "bob@example.com");
    await user.click(screen.getByRole("button", { name: /send confirmation link/i }));

    await waitFor(() => {
      expect(
        screen.getByText("Could not send confirmation link. Please try again."),
      ).toBeInTheDocument();
    });
    // Structural exact 3-arg assertion: name + raw message logged on unmapped path only.
    expect(consoleWarnSpy).toHaveBeenCalledWith(
      "[change-email] updateUser failed (unmapped):",
      "AuthApiError",
      "some unmapped error xyz",
    );
  });

  it("S7 shows a network-error banner and logs the error name (not message) on transport-level rejection", async () => {
    const user = userEvent.setup();
    const consoleWarnSpy = vi.spyOn(console, "warn").mockImplementation(() => {});
    const transportError = Object.assign(new Error("network down"), { name: "FetchError" });
    mockUpdateUser.mockRejectedValue(transportError);

    renderWithIntl(<ChangeEmailClient currentEmail="alice@example.com" />);

    const input = screen.getByLabelText(/new email/i);
    await user.type(input, "bob@example.com");
    await user.click(screen.getByRole("button", { name: /send confirmation link/i }));

    await waitFor(() => {
      expect(
        screen.getByText("Network error. Please check your connection and try again."),
      ).toBeInTheDocument();
    });
    // Structural exact-call assertion: only err.name ("FetchError") is logged,
    // never err.message ("network down" — potential PII carrier).
    expect(consoleWarnSpy).toHaveBeenCalledWith("[change-email] updateUser threw:", "FetchError");
  });

  it("S8 disables the submit button and shows 'Sending...' while the request is in-flight", async () => {
    const user = userEvent.setup();
    // Never-resolving promise keeps loading=true for the duration of the assertion.
    mockUpdateUser.mockReturnValue(new Promise<never>(() => {}));

    renderWithIntl(<ChangeEmailClient currentEmail="alice@example.com" />);

    const input = screen.getByLabelText(/new email/i);
    await user.type(input, "bob@example.com");

    const button = screen.getByRole("button", { name: /send confirmation link/i });
    await user.click(button);

    await waitFor(() => {
      expect(screen.getByRole("button", { name: /sending/i })).toBeDisabled();
    });
    expect(screen.getByRole("button", { name: /sending/i })).toHaveTextContent(/sending/i);
  });

  it("S4 Cancel button links to /profile", () => {
    renderWithIntl(<ChangeEmailClient currentEmail="alice@example.com" />);

    const cancelLink = screen.getByRole("link", { name: /cancel/i });
    expect(cancelLink).toHaveAttribute("href", "/profile");
  });

  it("S9 trims a whitespace-padded email before calling updateUser", async () => {
    mockUpdateUser.mockResolvedValue({ data: { user: {} }, error: null });

    renderWithIntl(<ChangeEmailClient currentEmail="alice@example.com" />);

    // jsdom sanitizes type="email" inputs and strips whitespace from .value, so
    // we cannot inject a padded value via normal DOM assignment. Override the
    // value property to bypass the sanitizer and prove the component trims.
    const input = screen.getByLabelText(/new email/i);
    let stored = "";
    Object.defineProperty(input, "value", {
      get: () => stored,
      set: (v: string) => {
        stored = v;
      },
      configurable: true,
    });

    // Inject the padded value and fire the React synthetic onChange.
    stored = "  bob@example.com  ";
    fireEvent.change(input, { target: input });
    // Use fireEvent.submit to bypass native constraint validation (which would
    // reject a padded email and block the submit handler).
    const form = input.closest("form");
    if (!form) throw new Error("expected the new-email input to be inside a form");
    fireEvent.submit(form);

    await waitFor(() => {
      expect(mockUpdateUser).toHaveBeenCalledWith({ email: "bob@example.com" });
    });
    // Success screen shows the trimmed address.
    expect(screen.getByText("bob@example.com")).toBeInTheDocument();
  });
});

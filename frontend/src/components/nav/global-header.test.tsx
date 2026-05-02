// @vitest-environment jsdom
import { render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  mockSupabaseServerClient,
  resetMockSupabase,
  setMockSupabaseUser,
  setMockSupabaseUserError,
} from "../../../__tests__/utils/mock-supabase";

// Minimal stubs for Next.js server components in jsdom.
vi.mock("next/link", () => ({
  default: ({
    children,
    ...rest
  }: React.AnchorHTMLAttributes<HTMLAnchorElement> & { children?: React.ReactNode }) => (
    <a {...rest}>{children}</a>
  ),
}));

vi.mock("next/navigation", () => ({
  redirect: vi.fn((url: string) => {
    throw new Error(`REDIRECT:${url}`);
  }),
}));

vi.mock("@/lib/supabase/server", () => ({
  createSupabaseServerClient: () => Promise.resolve(mockSupabaseServerClient()),
}));

vi.mock("@/lib/apollo/server", () => ({
  gqlFetch: vi.fn(),
}));

// HamburgerDrawer is "use client" — mock it to avoid portal issues in jsdom
// and to keep the RSC test focused on header logic, not drawer rendering.
vi.mock("./hamburger-drawer", () => ({
  HamburgerDrawer: ({ isAdmin }: { isAdmin: boolean }) => (
    <div data-testid="hamburger-drawer" data-is-admin={String(isAdmin)} />
  ),
}));

// AdminPill default export
vi.mock("./admin-pill", () => ({
  default: () => (
    <a href="/admin" aria-label="Admin area">
      Admin
    </a>
  ),
}));

// LogoutButton
vi.mock("@/app/_components/logout-button", () => ({
  LogoutButton: () => (
    <button type="button" data-testid="logout-button">
      Logout
    </button>
  ),
}));

import { gqlFetch } from "@/lib/apollo/server";
import { GlobalHeader } from "./global-header";

let consoleErrorSpy: ReturnType<typeof vi.spyOn>;
let consoleWarnSpy: ReturnType<typeof vi.spyOn>;

beforeEach(() => {
  vi.clearAllMocks();
  resetMockSupabase();
  consoleErrorSpy = vi.spyOn(console, "error").mockImplementation(() => {});
  consoleWarnSpy = vi.spyOn(console, "warn").mockImplementation(() => {});
});

afterEach(() => {
  vi.restoreAllMocks();
});

// Helper to build the UNAUTHENTICATED error message exactly as gqlFetch throws it.
function makeUnauthenticatedError(): Error {
  const errors = [{ extensions: { code: "UNAUTHENTICATED" } }];
  return new Error(`GraphQL errors: ${JSON.stringify(errors)}`);
}

describe("<GlobalHeader> (RSC)", () => {
  it("anonymous request (AuthSessionMissingError) renders no admin pill and does not call gqlFetch", async () => {
    const noSession = new Error("Auth session missing!");
    noSession.name = "AuthSessionMissingError";
    setMockSupabaseUserError(noSession);

    const jsx = await GlobalHeader();
    render(jsx);

    expect(screen.queryByLabelText("Admin area")).not.toBeInTheDocument();
    expect(gqlFetch).not.toHaveBeenCalled();
  });

  it("anonymous request (AuthSessionMissingError) does not throw", async () => {
    const noSession = new Error("Auth session missing!");
    noSession.name = "AuthSessionMissingError";
    setMockSupabaseUserError(noSession);

    await expect(GlobalHeader()).resolves.not.toThrow();
  });

  it("logged-in user + admin role → AdminPill is rendered", async () => {
    setMockSupabaseUser({ id: "u-1", email: "admin@test.com" });
    vi.mocked(gqlFetch).mockResolvedValueOnce({
      me: { roles: [{ name: "admin" }] },
    } as never);

    const jsx = await GlobalHeader();
    render(jsx);

    expect(screen.getByLabelText("Admin area")).toBeInTheDocument();
  });

  it("logged-in user + non-admin role → AdminPill is absent", async () => {
    setMockSupabaseUser({ id: "u-2", email: "user@test.com" });
    vi.mocked(gqlFetch).mockResolvedValueOnce({
      me: { roles: [{ name: "member" }] },
    } as never);

    const jsx = await GlobalHeader();
    render(jsx);

    expect(screen.queryByLabelText("Admin area")).not.toBeInTheDocument();
  });

  it("logged-in user + empty roles → AdminPill is absent", async () => {
    setMockSupabaseUser({ id: "u-3", email: "user@test.com" });
    vi.mocked(gqlFetch).mockResolvedValueOnce({
      me: { roles: [] },
    } as never);

    const jsx = await GlobalHeader();
    render(jsx);

    expect(screen.queryByLabelText("Admin area")).not.toBeInTheDocument();
  });

  it("logged-in user + UNAUTHENTICATED from gqlFetch → degraded shell, no admin pill, no error thrown", async () => {
    setMockSupabaseUser({ id: "u-4", email: "user@test.com" });
    vi.mocked(gqlFetch).mockRejectedValueOnce(makeUnauthenticatedError());

    const jsx = await GlobalHeader();
    render(jsx);

    expect(screen.queryByLabelText("Admin area")).not.toBeInTheDocument();
    // UNAUTHENTICATED is silenced — no console.error, no console.warn for this case
    expect(consoleErrorSpy).not.toHaveBeenCalled();
  });

  it("logged-in user + unexpected gqlFetch error → no admin pill and console.warn is called", async () => {
    setMockSupabaseUser({ id: "u-5", email: "user@test.com" });
    vi.mocked(gqlFetch).mockRejectedValueOnce(new Error("GraphQL HTTP 500"));

    const jsx = await GlobalHeader();
    render(jsx);

    expect(screen.queryByLabelText("Admin area")).not.toBeInTheDocument();
    expect(consoleWarnSpy).toHaveBeenCalledWith(
      expect.stringContaining("[global-header]"),
      expect.objectContaining({ error_message: expect.any(String) }),
    );
  });

  it("non-AuthSessionMissingError from getUser → degraded shell, console.error called, no admin pill", async () => {
    const transportError = new Error("network failure");
    transportError.name = "FetchError";
    setMockSupabaseUserError(transportError);

    const jsx = await GlobalHeader();
    render(jsx);

    expect(screen.queryByLabelText("Admin area")).not.toBeInTheDocument();
    expect(gqlFetch).not.toHaveBeenCalled();
    expect(consoleErrorSpy).toHaveBeenCalledWith(
      expect.stringContaining("[global-header]"),
      transportError.name,
      transportError.message,
    );
  });
});

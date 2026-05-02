// @vitest-environment jsdom
import { render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  mockSupabaseServerClient,
  resetMockSupabase,
  setMockSupabaseUser,
  setMockSupabaseUserError,
} from "../../../__tests__/utils/mock-supabase";

// Hoist vi.mock calls — Vitest processes these before any imports.

vi.mock("@/lib/supabase/server", () => ({
  createSupabaseServerClient: () => Promise.resolve(mockSupabaseServerClient()),
}));

vi.mock("@/lib/apollo/server", () => ({
  gqlFetch: vi.fn(),
}));

vi.mock("./logout-button", () => ({
  LogoutButton: () => <button type="button">Logout</button>,
}));

// Static mock for next/link — renders a plain <a> so href assertions work.
vi.mock("next/link", () => ({
  default: ({
    href,
    children,
    className,
  }: {
    href: string;
    children: React.ReactNode;
    className?: string;
  }) => (
    <a href={href} className={className}>
      {children}
    </a>
  ),
}));

// Static mock for lucide-react ShieldCheck icon.
vi.mock("lucide-react", () => ({
  ShieldCheck: (props: React.SVGProps<SVGSVGElement>) => (
    <svg data-testid="shield-check" {...props} />
  ),
}));

import { gqlFetch } from "@/lib/apollo/server";
import { Header } from "./header";

function makeRolesResponse(roleNames: string[]) {
  return {
    me: {
      roles: roleNames.map((name) => ({ name })),
    },
  };
}

beforeEach(() => {
  resetMockSupabase();
  vi.clearAllMocks();
});

afterEach(() => {
  vi.restoreAllMocks();
});

describe("Header", () => {
  it("anonymous: renders Sign in link and does NOT call gqlFetch", async () => {
    setMockSupabaseUser(null);

    render(await Header());

    expect(screen.getByRole("link", { name: /sign in/i })).toBeInTheDocument();
    expect(screen.queryByRole("link", { name: /cardgroups/i })).not.toBeInTheDocument();
    expect(screen.queryByRole("link", { name: /admin/i })).not.toBeInTheDocument();
    expect(vi.mocked(gqlFetch)).not.toHaveBeenCalled();
  });

  it("logged-in non-admin: renders Cardgroups + email + Logout but no Admin link", async () => {
    setMockSupabaseUser({ id: "u-1", email: "user@test.com" });
    vi.mocked(gqlFetch).mockResolvedValue(makeRolesResponse(["user"]) as never);

    render(await Header());

    expect(screen.getByRole("link", { name: /cardgroups/i })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /user@test\.com/i })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /logout/i })).toBeInTheDocument();
    expect(screen.queryByRole("link", { name: /admin/i })).not.toBeInTheDocument();
  });

  it("logged-in admin: renders Admin link with href=/admin", async () => {
    setMockSupabaseUser({ id: "u-2", email: "admin@test.com" });
    vi.mocked(gqlFetch).mockResolvedValue(makeRolesResponse(["admin", "user"]) as never);

    render(await Header());

    expect(screen.getByRole("link", { name: /cardgroups/i })).toBeInTheDocument();
    // Use exact text "Admin" to avoid matching the email address "admin@test.com".
    const adminLink = screen.getByRole("link", { name: "Admin" });
    expect(adminLink).toBeInTheDocument();
    expect(adminLink).toHaveAttribute("href", "/admin");
  });

  it("me throws UNAUTHENTICATED: Admin link hidden; console.warn NOT called", async () => {
    setMockSupabaseUser({ id: "u-3", email: "race@test.com" });
    vi.mocked(gqlFetch).mockRejectedValue(
      new Error(
        "GraphQL errors: " +
          JSON.stringify([
            { message: "Not authenticated", extensions: { code: "UNAUTHENTICATED" } },
          ]),
      ),
    );
    const warnSpy = vi.spyOn(console, "warn").mockImplementation(() => {});

    render(await Header());

    expect(screen.queryByRole("link", { name: /admin/i })).not.toBeInTheDocument();
    expect(warnSpy).not.toHaveBeenCalled();
  });

  it("me throws unexpected error: Admin link hidden; console.warn called with message and user id", async () => {
    setMockSupabaseUser({ id: "u-4", email: "err@test.com" });
    vi.mocked(gqlFetch).mockRejectedValue(new Error("503 service unavailable"));
    const warnSpy = vi.spyOn(console, "warn").mockImplementation(() => {});

    render(await Header());

    expect(screen.queryByRole("link", { name: /admin/i })).not.toBeInTheDocument();
    expect(warnSpy).toHaveBeenCalledOnce();
    expect(warnSpy).toHaveBeenCalledWith(
      "[header] me query unexpectedly failed",
      expect.objectContaining({
        error_message: "503 service unavailable",
        user_id: "u-4",
      }),
    );
  });

  it("me resolves with null user: Admin link hidden", async () => {
    setMockSupabaseUser({ id: "u-5", email: "ghost@test.com" });
    vi.mocked(gqlFetch).mockResolvedValue({ me: null } as never);

    render(await Header());

    expect(screen.queryByRole("link", { name: /admin/i })).not.toBeInTheDocument();
    expect(screen.getByRole("link", { name: /cardgroups/i })).toBeInTheDocument();
  });

  it("getUser non-session error: renders only logo header; console.error called", async () => {
    const networkError = new Error("boom");
    networkError.name = "NetworkError";
    setMockSupabaseUserError(networkError);
    const errorSpy = vi.spyOn(console, "error").mockImplementation(() => {});

    render(await Header());

    // Only the logo link should be present — no nav, no sign-in link.
    const logoLink = screen.getByRole("link", { name: /flamingo-armond/i });
    expect(logoLink).toBeInTheDocument();
    expect(screen.queryByRole("nav")).not.toBeInTheDocument();
    expect(screen.queryByRole("link", { name: /sign in/i })).not.toBeInTheDocument();
    expect(errorSpy).toHaveBeenCalledWith("[header] getUser() failed:", "boom");
  });
});

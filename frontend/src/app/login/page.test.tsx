// @vitest-environment jsdom
import { render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

// `redirect` throws so the server component aborts the same way Next.js's server runtime does.
vi.mock("next/navigation", () => ({
  redirect: vi.fn((url: string) => {
    throw new Error(`REDIRECT:${url}`);
  }),
}));

vi.mock("@/lib/supabase/server", () => ({
  createSupabaseServerClient: vi.fn(),
}));

// Mock LoginButton to avoid pulling in the Supabase browser client
vi.mock("./login-button", () => ({
  LoginButton: () => <button type="button">Sign in</button>,
}));

import { createSupabaseServerClient } from "@/lib/supabase/server";
import LoginPage from "./page";

function makeSupabaseMock(user: { id: string; email?: string } | null, error: Error | null = null) {
  return {
    auth: {
      getUser: vi.fn().mockResolvedValue({
        data: { user },
        error,
      }),
    },
  };
}

describe("LoginPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.spyOn(console, "error").mockImplementation(() => {});
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("anonymous user: renders LoginButton and no error banner", async () => {
    vi.mocked(createSupabaseServerClient).mockResolvedValue(makeSupabaseMock(null) as never);

    const jsx = await LoginPage({ searchParams: Promise.resolve({}) });
    render(jsx);

    expect(screen.getByRole("button", { name: /sign in/i })).toBeInTheDocument();
    expect(screen.queryByText(/sign-in failed/i)).not.toBeInTheDocument();
  });

  it("logged-in user: calls redirect to /cardgroups", async () => {
    vi.mocked(createSupabaseServerClient).mockResolvedValue(
      makeSupabaseMock({ id: "u-1", email: "user@example.com" }) as never,
    );

    await expect(LoginPage({ searchParams: Promise.resolve({}) })).rejects.toThrow(
      "REDIRECT:/cardgroups",
    );
  });

  it("searchParams.error set: displays sign-in failed banner", async () => {
    vi.mocked(createSupabaseServerClient).mockResolvedValue(makeSupabaseMock(null) as never);

    const jsx = await LoginPage({ searchParams: Promise.resolve({ error: "access_denied" }) });
    render(jsx);

    expect(screen.getByText(/sign-in failed: access_denied/i)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /sign in/i })).toBeInTheDocument();
  });

  it("non-AuthSessionMissingError is rethrown", async () => {
    const boom = new Error("network failure");
    boom.name = "FetchError";
    vi.mocked(createSupabaseServerClient).mockResolvedValue(makeSupabaseMock(null, boom) as never);

    await expect(LoginPage({ searchParams: Promise.resolve({}) })).rejects.toThrow(
      "network failure",
    );
    expect(console.error).toHaveBeenCalledWith(
      "[login] getUser() failed:",
      "FetchError",
      "network failure",
    );
  });

  it("AuthSessionMissingError is silenced and renders LoginButton", async () => {
    const noSession = new Error("Auth session missing!");
    noSession.name = "AuthSessionMissingError";
    vi.mocked(createSupabaseServerClient).mockResolvedValue(
      makeSupabaseMock(null, noSession) as never,
    );

    const jsx = await LoginPage({ searchParams: Promise.resolve({}) });
    render(jsx);

    expect(screen.getByRole("button", { name: /sign in/i })).toBeInTheDocument();
  });

  it("split-screen: outer wrapper has h-svh and lg:grid-cols-2", async () => {
    vi.mocked(createSupabaseServerClient).mockResolvedValue(makeSupabaseMock(null) as never);

    const jsx = await LoginPage({ searchParams: Promise.resolve({}) });
    const { container } = render(jsx);

    const grid = container.querySelector("[data-testid='login-grid']");
    expect(grid).toBeInTheDocument();
    expect(grid?.className).toMatch(/h-svh/);
    expect(grid?.className).toMatch(/lg:grid-cols-2/);
  });

  it("split-screen: brand panel has max-lg:hidden", async () => {
    vi.mocked(createSupabaseServerClient).mockResolvedValue(makeSupabaseMock(null) as never);

    const jsx = await LoginPage({ searchParams: Promise.resolve({}) });
    const { container } = render(jsx);

    const brandPanel = container.querySelector("[data-testid='brand-panel']");
    expect(brandPanel).toBeInTheDocument();
    expect(brandPanel?.className).toMatch(/max-lg:hidden/);
  });

  it("split-screen: brand panel shows flamingo logo and app name", async () => {
    vi.mocked(createSupabaseServerClient).mockResolvedValue(makeSupabaseMock(null) as never);

    const jsx = await LoginPage({ searchParams: Promise.resolve({}) });
    const { container } = render(jsx);

    const brandPanel = container.querySelector("[data-testid='brand-panel']");
    expect(brandPanel).toBeInTheDocument();
    expect(brandPanel?.querySelector("[aria-label='Flamingo']")).toBeInTheDocument();
    expect(brandPanel?.textContent).toContain("flamingo-armond");
  });

  it("split-screen: brand panel must not contain any email address (PII)", async () => {
    vi.mocked(createSupabaseServerClient).mockResolvedValue(makeSupabaseMock(null) as never);

    const jsx = await LoginPage({ searchParams: Promise.resolve({}) });
    const { container } = render(jsx);

    const brandPanel = container.querySelector("[data-testid='brand-panel']");
    expect(brandPanel?.textContent).not.toMatch(/@/);
  });

  it("split-screen: form column renders OAuth button, Terms link, and Privacy link", async () => {
    vi.mocked(createSupabaseServerClient).mockResolvedValue(makeSupabaseMock(null) as never);

    const jsx = await LoginPage({ searchParams: Promise.resolve({}) });
    render(jsx);

    expect(screen.getByRole("button", { name: /sign in/i })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /terms/i })).toHaveAttribute("href", "/terms");
    expect(screen.getByRole("link", { name: /privacy/i })).toHaveAttribute("href", "/privacy");
  });
});

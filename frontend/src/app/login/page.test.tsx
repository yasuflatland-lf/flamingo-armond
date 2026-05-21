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

  describe("split-screen layout (anonymous user)", () => {
    let container: HTMLElement;

    beforeEach(async () => {
      vi.mocked(createSupabaseServerClient).mockResolvedValue(makeSupabaseMock(null) as never);
      const jsx = await LoginPage({ searchParams: Promise.resolve({}) });
      ({ container } = render(jsx));
    });

    it("outer wrapper has h-svh and lg:grid-cols-2", () => {
      const grid = container.querySelector("[data-testid='login-grid']");
      expect(grid).toBeInTheDocument();
      expect(grid?.className).toMatch(/h-svh/);
      expect(grid?.className).toMatch(/lg:grid-cols-2/);
    });

    it("brand panel has max-lg:hidden", () => {
      const brandPanel = container.querySelector("[data-testid='brand-panel']");
      expect(brandPanel).toBeInTheDocument();
      expect(brandPanel?.className).toMatch(/max-lg:hidden/);
    });

    it("brand panel shows flamingo logo and app name", () => {
      const brandPanel = container.querySelector("[data-testid='brand-panel']");
      expect(brandPanel).toBeInTheDocument();
      expect(brandPanel?.querySelector("[aria-label='Flamingo']")).toBeInTheDocument();
      expect(brandPanel?.textContent).toContain("flamingo-armond");
    });

    it("brand panel does not contain an email address (PII)", () => {
      const brandPanel = container.querySelector("[data-testid='brand-panel']");
      expect(brandPanel?.textContent).not.toMatch(/@/);
    });

    it("form column renders OAuth button, Terms link, and Privacy link", () => {
      expect(screen.getByRole("button", { name: /sign in/i })).toBeInTheDocument();
      expect(screen.getByRole("link", { name: /terms/i })).toHaveAttribute("href", "/terms");
      expect(screen.getByRole("link", { name: /privacy/i })).toHaveAttribute("href", "/privacy");
    });

    it("Sign in heading renders as h1", () => {
      expect(screen.getByRole("heading", { level: 1, name: /sign in/i })).toBeInTheDocument();
    });

    it("main landmark is present for screen reader navigation", () => {
      expect(screen.getByRole("main")).toBeInTheDocument();
    });

    it("mobile brand header exists with lg:hidden class (visible on sub-lg only)", () => {
      const mobileBrand = container.querySelector("[data-testid='form-brand-header']");
      expect(mobileBrand).toBeInTheDocument();
      expect(mobileBrand?.className).toMatch(/lg:hidden/);
      expect(mobileBrand?.textContent).toContain("flamingo-armond");
    });

    it("sub-copy explaining OAuth-first-time semantics is rendered under h1", () => {
      expect(screen.getByText(/sign in with your google account to continue/i)).toBeInTheDocument();
      expect(screen.getByText(/an account is created on first sign-in/i)).toBeInTheDocument();
    });

    it("brand panel comes after the form column in DOM order (right-side placement)", () => {
      const grid = container.querySelector("[data-testid='login-grid']");
      const brandPanel = container.querySelector("[data-testid='brand-panel']");
      expect(grid).toBeInTheDocument();
      expect(brandPanel).toBeInTheDocument();
      const children = Array.from(grid?.children ?? []);
      const brandIndex = children.indexOf(brandPanel as Element);
      expect(brandIndex).toBeGreaterThan(0);
    });
  });

  it("split-screen: error banner and brand panel both render when error param is set", async () => {
    vi.mocked(createSupabaseServerClient).mockResolvedValue(makeSupabaseMock(null) as never);

    const jsx = await LoginPage({ searchParams: Promise.resolve({ error: "access_denied" }) });
    const { container } = render(jsx);

    expect(container.querySelector("[data-testid='login-grid']")).toBeInTheDocument();
    expect(container.querySelector("[data-testid='brand-panel']")).toBeInTheDocument();
    expect(screen.getByText(/sign-in failed: access_denied/i)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /sign in/i })).toBeInTheDocument();
  });

  it("error banner has role='alert' so screen readers announce sign-in failure", async () => {
    vi.mocked(createSupabaseServerClient).mockResolvedValue(makeSupabaseMock(null) as never);

    const jsx = await LoginPage({ searchParams: Promise.resolve({ error: "access_denied" }) });
    render(jsx);

    expect(screen.getByRole("alert")).toHaveTextContent(/sign-in failed: access_denied/i);
  });
});

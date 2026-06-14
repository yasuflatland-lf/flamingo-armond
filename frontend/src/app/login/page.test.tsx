// @vitest-environment jsdom
import { render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

// `redirect` throws so the server component aborts the same way Next.js's server runtime does.
vi.mock("next/navigation", () => ({
  redirect: vi.fn((url: string) => {
    throw new Error(`REDIRECT:${url}`);
  }),
}));

vi.mock("next/headers", () => ({
  headers: vi.fn(async () => new Headers({ "x-auth-status": "anonymous" })),
}));

// Mock LoginButton to avoid pulling in the Supabase browser client
vi.mock("./login-button", () => ({
  LoginButton: () => <button type="button">Sign in</button>,
}));

// InAppBrowserNotice is a client component (useTranslations) that renders
// nothing in a standalone browser; the broad page test has no NextIntlClient
// provider, so mock it to null to mirror the standalone-browser DOM.
vi.mock("./in-app-browser-notice", () => ({
  InAppBrowserNotice: () => null,
}));

// next-intl/server — the page resolves the Login namespace via getTranslations.
// Back the mock with createTranslator + the real en catalog so t()/t.rich()
// produce the original English copy (and rendered Terms/Privacy links), keeping
// the existing English string assertions valid.
vi.mock("next-intl/server", () => ({
  getTranslations: vi.fn(async (namespace: "Login") =>
    createTranslator({ locale: "en", messages: enMessages, namespace }),
  ),
  // Default to "en" so the existing Latin-tracking assertions hold; the ja test
  // overrides this once to exercise the locale-aware typography branch.
  getLocale: vi.fn(async () => "en"),
}));

import { headers } from "next/headers";
import { createTranslator } from "next-intl";
import { getLocale } from "next-intl/server";
import enMessages from "../../../messages/en.json";
import LoginPage from "./page";

describe("LoginPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("authenticated user: redirects to /cardgroups", async () => {
    vi.mocked(headers).mockResolvedValueOnce(new Headers({ "x-auth-status": "authenticated" }));

    await expect(LoginPage({ searchParams: Promise.resolve({}) })).rejects.toThrow(
      "REDIRECT:/cardgroups",
    );
  });

  it("anonymous user: renders the Sign in heading", async () => {
    const jsx = await LoginPage({ searchParams: Promise.resolve({}) });
    render(jsx);

    expect(screen.getByRole("heading", { level: 1, name: /sign in/i })).toBeInTheDocument();
  });

  it("stale session: renders the Sign in heading (no redirect)", async () => {
    vi.mocked(headers).mockResolvedValueOnce(new Headers({ "x-auth-status": "stale" }));

    const jsx = await LoginPage({ searchParams: Promise.resolve({}) });
    render(jsx);

    expect(screen.getByRole("heading", { level: 1, name: /sign in/i })).toBeInTheDocument();
  });

  it("error status: renders the Sign in heading (no redirect)", async () => {
    vi.mocked(headers).mockResolvedValueOnce(new Headers({ "x-auth-status": "error" }));

    const jsx = await LoginPage({ searchParams: Promise.resolve({}) });
    render(jsx);

    expect(screen.getByRole("heading", { level: 1, name: /sign in/i })).toBeInTheDocument();
  });

  it("anonymous user: renders LoginButton and no error banner", async () => {
    const jsx = await LoginPage({ searchParams: Promise.resolve({}) });
    render(jsx);

    expect(screen.getByRole("button", { name: /sign in/i })).toBeInTheDocument();
    expect(screen.queryByText(/sign-in failed/i)).not.toBeInTheDocument();
  });

  it("searchParams.error set: displays sign-in failed banner", async () => {
    const jsx = await LoginPage({ searchParams: Promise.resolve({ error: "access_denied" }) });
    render(jsx);

    expect(screen.getByText(/sign-in failed: access_denied/i)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /sign in/i })).toBeInTheDocument();
  });

  describe("split-screen layout (anonymous user)", () => {
    let container: HTMLElement;

    beforeEach(async () => {
      const jsx = await LoginPage({ searchParams: Promise.resolve({}) });
      ({ container } = render(jsx));
    });

    it("outer wrapper has h-svh and the even-split grid template", () => {
      const grid = container.querySelector("[data-testid='login-grid']");
      expect(grid).toBeInTheDocument();
      expect(grid?.className).toMatch(/h-svh/);
      // Even split: two equal columns put the panel boundary at the centre.
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
      expect(brandPanel?.textContent).toContain("Flamingo Armond");
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
      expect(mobileBrand?.textContent).toContain("Flamingo Armond");
    });

    it("sub-copy explaining OAuth-first-time semantics is rendered under h1", () => {
      expect(screen.getByText(/sign in with your google account to continue/i)).toBeInTheDocument();
      expect(screen.getByText(/an account is created on first sign-in/i)).toBeInTheDocument();
    });

    it("brand panel comes before the form column in DOM order (left-side placement)", () => {
      const grid = container.querySelector("[data-testid='login-grid']");
      const brandPanel = container.querySelector("[data-testid='brand-panel']");
      expect(grid).toBeInTheDocument();
      expect(brandPanel).toBeInTheDocument();
      const children = Array.from(grid?.children ?? []);
      const formIndex = children.findIndex(
        (el) => el.querySelector("[data-testid='form-brand-header']") !== null,
      );
      const brandIndex = children.indexOf(brandPanel as Element);
      expect(brandIndex).not.toBe(-1);
      // Golden-ratio split places the brand panel in the wider left column.
      expect(brandIndex).toBe(0);
      expect(formIndex).toBe(1);
    });

    it("form-brand-header and brand-panel have mutually exclusive viewport visibility", () => {
      const formBrandHeader = container.querySelector("[data-testid='form-brand-header']");
      const brandPanel = container.querySelector("[data-testid='brand-panel']");
      expect(formBrandHeader?.className).toMatch(/lg:hidden/);
      expect(formBrandHeader?.className).not.toMatch(/max-lg:hidden/);
      expect(brandPanel?.className).toMatch(/max-lg:hidden/);
      expect(brandPanel?.className).not.toMatch(/(^|\s)lg:hidden(\s|$)/);
    });
  });

  it("split-screen: error banner and brand panel both render when error param is set", async () => {
    const jsx = await LoginPage({ searchParams: Promise.resolve({ error: "access_denied" }) });
    const { container } = render(jsx);

    expect(container.querySelector("[data-testid='login-grid']")).toBeInTheDocument();
    expect(container.querySelector("[data-testid='brand-panel']")).toBeInTheDocument();
    expect(screen.getByText(/sign-in failed: access_denied/i)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /sign in/i })).toBeInTheDocument();
  });

  it("error banner has role='alert' so screen readers announce sign-in failure", async () => {
    const jsx = await LoginPage({ searchParams: Promise.resolve({ error: "access_denied" }) });
    render(jsx);

    expect(screen.getByRole("alert")).toHaveTextContent(/sign-in failed: access_denied/i);
  });

  it("no error param: role='alert' element is not present", async () => {
    const jsx = await LoginPage({ searchParams: Promise.resolve({}) });
    render(jsx);

    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  });

  // Apple's Japanese typography uses locale-specific letter-spacing: Latin display
  // text takes negative tracking, Japanese does not (it cramps kana). These pin
  // that the heading switches treatment with the active locale.
  describe("locale-aware typography (apple.com/jp)", () => {
    it("en (default): heading carries tight Latin negative tracking", async () => {
      const jsx = await LoginPage({ searchParams: Promise.resolve({}) });
      const { container } = render(jsx);

      expect(container.querySelector("h1")?.className).toMatch(/tracking-\[-0\.018em\]/);
    });

    it("ja: heading drops Latin negative tracking so kana is not cramped", async () => {
      vi.mocked(getLocale).mockResolvedValueOnce("ja");
      const jsx = await LoginPage({ searchParams: Promise.resolve({}) });
      const { container } = render(jsx);

      const h1 = container.querySelector("h1");
      expect(h1?.className).not.toMatch(/tracking-\[-/);
      expect(h1?.className).toMatch(/tracking-\[0\.01em\]/);
    });
  });
});

import { afterEach, beforeEach, describe, expect, test, vi } from "vitest";

// next/navigation mock — redirect throws so the RSC aborts the same way
// Next.js's server runtime does.
const REDIRECT_PREFIX = "REDIRECT:";

vi.mock("next/navigation", () => ({
  redirect: vi.fn((path: string) => {
    throw new Error(`${REDIRECT_PREFIX}${path}`);
  }),
}));

vi.mock("next/headers", () => ({
  headers: vi.fn(async () => new Headers({ "x-auth-status": "authenticated" })),
}));

vi.mock("@/lib/apollo/server", () => ({
  gqlFetch: vi.fn(),
}));

// Stub OnboardingForm — it is a "use client" component that requires an Apollo
// Provider. The RSC page test only needs to verify the render branch; the
// client component has its own dedicated test file.
vi.mock("./onboarding-form", () => ({
  OnboardingForm: () => <div data-testid="onboarding-form">OnboardingForm</div>,
}));

import { headers } from "next/headers";
import { redirect } from "next/navigation";
import OnboardingPage from "@/app/onboarding/page";
import { gqlFetch } from "@/lib/apollo/server";

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function makeOnboardingData(opts: { displayName?: string | null } = {}) {
  return {
    me: {
      id: "u-1",
      displayName: opts.displayName ?? null,
    },
  };
}

// ---------------------------------------------------------------------------
// Setup / teardown
// ---------------------------------------------------------------------------

beforeEach(() => {
  vi.clearAllMocks();
  // Default: authenticated
  vi.mocked(headers).mockResolvedValue(new Headers({ "x-auth-status": "authenticated" }));
});

afterEach(() => {
  vi.restoreAllMocks();
});

// ---------------------------------------------------------------------------
// Auth guard branches
// ---------------------------------------------------------------------------

describe("OnboardingPage — auth branches", () => {
  test("anonymous status → redirect /login, gqlFetch not called", async () => {
    vi.mocked(headers).mockResolvedValue(new Headers({ "x-auth-status": "anonymous" }));

    await expect(OnboardingPage()).rejects.toThrow(`${REDIRECT_PREFIX}/login`);

    expect(redirect).toHaveBeenCalledWith("/login");
    expect(gqlFetch).not.toHaveBeenCalled();
  });

  test("stale status → redirect /login, gqlFetch not called", async () => {
    vi.mocked(headers).mockResolvedValue(new Headers({ "x-auth-status": "stale" }));

    await expect(OnboardingPage()).rejects.toThrow(`${REDIRECT_PREFIX}/login`);

    expect(redirect).toHaveBeenCalledWith("/login");
    expect(gqlFetch).not.toHaveBeenCalled();
  });

  test("authenticated status → gqlFetch is called", async () => {
    vi.mocked(headers).mockResolvedValue(new Headers({ "x-auth-status": "authenticated" }));
    vi.mocked(gqlFetch).mockResolvedValueOnce(makeOnboardingData({ displayName: null }) as never);

    await OnboardingPage();

    expect(gqlFetch).toHaveBeenCalled();
  });
});

// ---------------------------------------------------------------------------
// gqlFetch error branches
// ---------------------------------------------------------------------------

describe("OnboardingPage — gqlFetch error branches", () => {
  let consoleErrorSpy: ReturnType<typeof vi.spyOn>;

  beforeEach(() => {
    consoleErrorSpy = vi.spyOn(console, "error").mockImplementation(() => {});
  });

  test("UNAUTHENTICATED from gqlFetch → redirect /login", async () => {
    vi.mocked(gqlFetch).mockRejectedValueOnce(
      new Error(`GraphQL errors: ${JSON.stringify([{ extensions: { code: "UNAUTHENTICATED" } }])}`),
    );

    await expect(OnboardingPage()).rejects.toThrow(`${REDIRECT_PREFIX}/login`);

    expect(redirect).toHaveBeenCalledWith("/login");
    expect(consoleErrorSpy).not.toHaveBeenCalled();
  });

  test("non-UNAUTHENTICATED gqlFetch error → console.error + rethrow", async () => {
    const otherErr = new Error("GraphQL HTTP 500");
    vi.mocked(gqlFetch).mockRejectedValueOnce(otherErr);

    await expect(OnboardingPage()).rejects.toBe(otherErr);

    expect(redirect).not.toHaveBeenCalled();
    // PII redaction: only the error name is logged, never the message.
    expect(consoleErrorSpy).toHaveBeenCalledWith(expect.stringContaining("[onboarding]"), {
      name: otherErr.name,
    });
  });
});

// ---------------------------------------------------------------------------
// Happy path / onboarding gate branches
// ---------------------------------------------------------------------------

describe("OnboardingPage — onboarding gate", () => {
  test("already-onboarded user (displayName: 'Alice') → redirect /", async () => {
    vi.mocked(gqlFetch).mockResolvedValueOnce(
      makeOnboardingData({ displayName: "Alice" }) as never,
    );

    await expect(OnboardingPage()).rejects.toThrow(`${REDIRECT_PREFIX}/`);

    expect(redirect).toHaveBeenCalledWith("/");
  });

  test("user needs onboarding (displayName: null) → renders OnboardingForm", async () => {
    vi.mocked(gqlFetch).mockResolvedValueOnce(makeOnboardingData({ displayName: null }) as never);

    const result = await OnboardingPage();

    expect(redirect).not.toHaveBeenCalled();
    // The page must render a <main> landmark (this page bypasses AppShell).
    const mainEl = findElementByType(result, "main");
    expect(mainEl).not.toBeNull();
    // The OnboardingForm stub should appear inside the page output.
    const formEl = findElementByDisplayName(result, "OnboardingForm");
    expect(formEl).not.toBeNull();
  });

  test("user needs onboarding (displayName: '') → renders OnboardingForm", async () => {
    vi.mocked(gqlFetch).mockResolvedValueOnce(makeOnboardingData({ displayName: "" }) as never);

    const result = await OnboardingPage();

    expect(redirect).not.toHaveBeenCalled();
    const formEl = findElementByDisplayName(result, "OnboardingForm");
    expect(formEl).not.toBeNull();
  });
});

// ---------------------------------------------------------------------------
// JSX tree helpers
// ---------------------------------------------------------------------------

/** Recursively walk a React element tree and return the first node that satisfies `predicate`. */
function findElement(
  node: unknown,
  predicate: (el: Record<string, unknown>) => boolean,
): unknown | null {
  if (node == null || typeof node !== "object") return null;
  const el = node as Record<string, unknown>;
  if (predicate(el)) return el;
  if ("props" in el && el.props != null) {
    const children = (el.props as Record<string, unknown>).children;
    if (Array.isArray(children)) {
      for (const child of children) {
        const found = findElement(child, predicate);
        if (found) return found;
      }
    } else if (children != null) {
      return findElement(children, predicate);
    }
  }
  return null;
}

function findElementByType(node: unknown, tag: string): unknown | null {
  return findElement(node, (el) => "type" in el && el.type === tag);
}

function findElementByDisplayName(node: unknown, componentName: string): unknown | null {
  return findElement(
    node,
    (el) =>
      "type" in el &&
      "props" in el &&
      typeof el.type === "function" &&
      (el.type as { name?: string }).name === componentName,
  );
}

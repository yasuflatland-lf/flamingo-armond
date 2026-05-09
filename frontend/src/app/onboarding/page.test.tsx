import { afterEach, beforeEach, describe, expect, test, vi } from "vitest";
import {
  mockSupabaseServerClient,
  resetMockSupabase,
  setMockSupabaseUser,
  setMockSupabaseUserError,
} from "../../../__tests__/utils/mock-supabase";

// next/navigation mock — redirect throws so the RSC aborts the same way
// Next.js's server runtime does.
const REDIRECT_PREFIX = "REDIRECT:";

vi.mock("next/navigation", () => ({
  redirect: vi.fn((path: string) => {
    throw new Error(`${REDIRECT_PREFIX}${path}`);
  }),
}));

vi.mock("@/lib/supabase/server", () => ({
  createSupabaseServerClient: () => Promise.resolve(mockSupabaseServerClient()),
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

let consoleErrorSpy: ReturnType<typeof vi.spyOn>;

beforeEach(() => {
  vi.clearAllMocks();
  resetMockSupabase();
  consoleErrorSpy = vi.spyOn(console, "error").mockImplementation(() => {});
});

afterEach(() => {
  vi.restoreAllMocks();
});

// ---------------------------------------------------------------------------
// Auth guard branches
// ---------------------------------------------------------------------------

describe("OnboardingPage — auth branches", () => {
  test("unauthenticated user (user=null, no error) → redirect /login, gqlFetch not called", async () => {
    setMockSupabaseUser(null);

    await expect(OnboardingPage()).rejects.toThrow(`${REDIRECT_PREFIX}/login`);

    expect(redirect).toHaveBeenCalledWith("/login");
    expect(gqlFetch).not.toHaveBeenCalled();
  });

  test("AuthSessionMissingError is silenced → redirect /login, no console.error", async () => {
    const noSession = new Error("Auth session missing!");
    noSession.name = "AuthSessionMissingError";
    setMockSupabaseUserError(noSession);

    await expect(OnboardingPage()).rejects.toThrow(`${REDIRECT_PREFIX}/login`);

    expect(redirect).toHaveBeenCalledWith("/login");
    expect(consoleErrorSpy).not.toHaveBeenCalled();
    expect(gqlFetch).not.toHaveBeenCalled();
  });

  test("non-AuthSessionMissingError → console.error + rethrow", async () => {
    const transportError = new Error("network failure");
    transportError.name = "FetchError";
    setMockSupabaseUserError(transportError);

    await expect(OnboardingPage()).rejects.toBe(transportError);

    expect(redirect).not.toHaveBeenCalled();
    expect(consoleErrorSpy).toHaveBeenCalledWith(
      expect.stringContaining("[onboarding]"),
      transportError.name,
      transportError.message,
    );
  });
});

// ---------------------------------------------------------------------------
// gqlFetch error branches
// ---------------------------------------------------------------------------

describe("OnboardingPage — gqlFetch error branches", () => {
  beforeEach(() => {
    setMockSupabaseUser({ id: "u-1", email: "user@test.com" });
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
    expect(consoleErrorSpy).toHaveBeenCalledWith(
      expect.stringContaining("[onboarding]"),
      otherErr.name,
      otherErr.message,
    );
  });
});

// ---------------------------------------------------------------------------
// Happy path / onboarding gate branches
// ---------------------------------------------------------------------------

describe("OnboardingPage — onboarding gate", () => {
  beforeEach(() => {
    setMockSupabaseUser({ id: "u-1", email: "user@test.com" });
  });

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

/**
 * Recursively search a React element tree for an element whose `type` is the
 * given HTML tag string (e.g. "main") and return it.
 */
function findElementByType(node: unknown, tag: string): unknown | null {
  if (node == null || typeof node !== "object") return null;
  const el = node as Record<string, unknown>;
  if ("type" in el && el.type === tag) return el;
  if ("props" in el && el.props != null) {
    const children = (el.props as Record<string, unknown>).children;
    if (Array.isArray(children)) {
      for (const child of children) {
        const found = findElementByType(child, tag);
        if (found) return found;
      }
    } else if (children != null) {
      return findElementByType(children, tag);
    }
  }
  return null;
}

/**
 * Recursively search a React element tree for a node whose `type` display name
 * matches `componentName` and return it.
 */
function findElementByDisplayName(node: unknown, componentName: string): unknown | null {
  if (node == null || typeof node !== "object") return null;
  const el = node as Record<string, unknown>;
  if (
    "type" in el &&
    "props" in el &&
    typeof el.type === "function" &&
    (el.type as { name?: string; displayName?: string }).name === componentName
  ) {
    return el;
  }
  if ("props" in el && el.props != null) {
    const children = (el.props as Record<string, unknown>).children;
    if (Array.isArray(children)) {
      for (const child of children) {
        const found = findElementByDisplayName(child, componentName);
        if (found) return found;
      }
    } else if (children != null) {
      return findElementByDisplayName(children, componentName);
    }
  }
  return null;
}

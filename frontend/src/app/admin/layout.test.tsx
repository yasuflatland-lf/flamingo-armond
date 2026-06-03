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

import { headers } from "next/headers";
import { redirect } from "next/navigation";
import { gqlFetch } from "@/lib/apollo/server";
import AdminLayout from "./layout";

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function setAuthStatus(status: string) {
  vi.mocked(headers).mockResolvedValue(
    new Headers({ "x-auth-status": status }) as Awaited<ReturnType<typeof headers>>,
  );
}

/** Build a minimal AdminLayoutMe response with the given role names. */
function makeAdminLayoutData(roleNames: string[]) {
  return {
    me: {
      id: "u-1",
      roles: roleNames.map((name, i) => ({ id: `r-${i}`, name })),
    },
  };
}

// ---------------------------------------------------------------------------
// Setup / teardown
// ---------------------------------------------------------------------------

beforeEach(() => {
  vi.clearAllMocks();
  // Default: authenticated.
  vi.mocked(headers).mockResolvedValue(
    new Headers({ "x-auth-status": "authenticated" }) as Awaited<ReturnType<typeof headers>>,
  );
});

afterEach(() => {
  vi.restoreAllMocks();
});

// ---------------------------------------------------------------------------
// Step 1: middleware-forwarded auth-status gate
// ---------------------------------------------------------------------------

describe("AdminLayout — Step 1: x-auth-status gate", () => {
  test("anonymous (x-auth-status: anonymous) → redirect /, gqlFetch not called", async () => {
    setAuthStatus("anonymous");

    await expect(AdminLayout({ children: null })).rejects.toThrow(`${REDIRECT_PREFIX}/`);

    expect(redirect).toHaveBeenCalledWith("/");
    expect(gqlFetch).not.toHaveBeenCalled();
  });

  test("stale session (x-auth-status: stale) → redirect /, gqlFetch not called", async () => {
    setAuthStatus("stale");

    await expect(AdminLayout({ children: null })).rejects.toThrow(`${REDIRECT_PREFIX}/`);

    expect(redirect).toHaveBeenCalledWith("/");
    expect(gqlFetch).not.toHaveBeenCalled();
  });

  test("error status (x-auth-status: error) → redirect /, gqlFetch not called", async () => {
    setAuthStatus("error");

    await expect(AdminLayout({ children: null })).rejects.toThrow(`${REDIRECT_PREFIX}/`);

    expect(redirect).toHaveBeenCalledWith("/");
    expect(gqlFetch).not.toHaveBeenCalled();
  });

  test("authenticated → proceeds to Step 2 (gqlFetch called)", async () => {
    vi.mocked(gqlFetch).mockResolvedValueOnce(makeAdminLayoutData(["admin"]) as never);

    await AdminLayout({ children: null });

    expect(redirect).not.toHaveBeenCalled();
    expect(gqlFetch).toHaveBeenCalled();
  });
});

// ---------------------------------------------------------------------------
// Step 2: GraphQL admin-role gate (kept intact)
// ---------------------------------------------------------------------------

describe("AdminLayout — Step 2: GraphQL role check", () => {
  test("UNAUTHENTICATED from gqlFetch → redirect /", async () => {
    vi.mocked(gqlFetch).mockRejectedValueOnce(
      new Error(
        'GraphQL errors: [{"message":"Unauthenticated","extensions":{"code":"UNAUTHENTICATED"}}]',
      ),
    );

    await expect(AdminLayout({ children: null })).rejects.toThrow(`${REDIRECT_PREFIX}/`);

    expect(redirect).toHaveBeenCalledWith("/");
  });

  test("FORBIDDEN from gqlFetch → redirect /", async () => {
    vi.mocked(gqlFetch).mockRejectedValueOnce(
      new Error('GraphQL errors: [{"message":"Forbidden","extensions":{"code":"FORBIDDEN"}}]'),
    );

    await expect(AdminLayout({ children: null })).rejects.toThrow(`${REDIRECT_PREFIX}/`);

    expect(redirect).toHaveBeenCalledWith("/");
  });

  test("non-auth gqlFetch error → rethrows (propagates to error boundary)", async () => {
    const networkErr = new Error("Network unreachable");
    vi.mocked(gqlFetch).mockRejectedValueOnce(networkErr);

    await expect(AdminLayout({ children: null })).rejects.toBe(networkErr);

    expect(redirect).not.toHaveBeenCalled();
  });

  test("authenticated user without admin role → redirect /", async () => {
    vi.mocked(gqlFetch).mockResolvedValueOnce(makeAdminLayoutData(["viewer"]) as never);

    await expect(AdminLayout({ children: null })).rejects.toThrow(`${REDIRECT_PREFIX}/`);

    expect(redirect).toHaveBeenCalledWith("/");
  });

  test("me is null (no roles) → redirect /", async () => {
    vi.mocked(gqlFetch).mockResolvedValueOnce({ me: null } as never);

    await expect(AdminLayout({ children: null })).rejects.toThrow(`${REDIRECT_PREFIX}/`);

    expect(redirect).toHaveBeenCalledWith("/");
  });

  test("user with admin role → renders children", async () => {
    vi.mocked(gqlFetch).mockResolvedValueOnce(makeAdminLayoutData(["admin"]) as never);

    const result = await AdminLayout({ children: <span data-testid="child" /> });

    expect(redirect).not.toHaveBeenCalled();
    // The layout wraps children in a React fragment; confirm it returns a node.
    expect(result).not.toBeNull();
  });

  test("user with multiple roles including admin → renders children", async () => {
    vi.mocked(gqlFetch).mockResolvedValueOnce(
      makeAdminLayoutData(["viewer", "admin", "editor"]) as never,
    );

    const result = await AdminLayout({ children: <span data-testid="child" /> });

    expect(redirect).not.toHaveBeenCalled();
    expect(result).not.toBeNull();
  });
});

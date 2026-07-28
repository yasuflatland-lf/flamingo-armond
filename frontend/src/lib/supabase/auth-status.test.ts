import { beforeEach, describe, expect, it, vi } from "vitest";

// redirect() throws in the Next.js server runtime; the mock reproduces that so
// requireAuthenticated's control flow is exercised the same way in tests.
const REDIRECT_PREFIX = "REDIRECT:";

vi.mock("next/navigation", () => ({
  redirect: vi.fn((path: string) => {
    throw new Error(`${REDIRECT_PREFIX}${path}`);
  }),
}));

vi.mock("next/headers", () => ({
  headers: vi.fn(async () => new Headers()),
}));

import { headers } from "next/headers";
import { redirect } from "next/navigation";
import {
  AUTH_STATUS_HEADER,
  IDENTITY_HEADERS,
  readAuthContext,
  requireAuthenticated,
  USER_EMAIL_HEADER,
  USER_IS_ADMIN_HEADER,
} from "./auth-status";

function headersWith(entries: Record<string, string>): Headers {
  const h = new Headers();
  for (const [k, v] of Object.entries(entries)) h.set(k, v);
  return h;
}

describe("readAuthContext", () => {
  it("reads an authenticated admin context", () => {
    const ctx = readAuthContext(
      headersWith({
        [AUTH_STATUS_HEADER]: "authenticated",
        [USER_EMAIL_HEADER]: "a@b.c",
        [USER_IS_ADMIN_HEADER]: "true",
      }),
    );
    expect(ctx).toEqual({ status: "authenticated", email: "a@b.c", isAdmin: true });
  });

  it("maps an empty email header to null and missing admin to false", () => {
    const ctx = readAuthContext(
      headersWith({ [AUTH_STATUS_HEADER]: "anonymous", [USER_EMAIL_HEADER]: "" }),
    );
    expect(ctx).toEqual({ status: "anonymous", email: null, isAdmin: false });
  });

  it("falls back to anonymous for a missing or invalid status header", () => {
    expect(readAuthContext(new Headers()).status).toBe("anonymous");
    expect(readAuthContext(headersWith({ [AUTH_STATUS_HEADER]: "garbage" })).status).toBe(
      "anonymous",
    );
  });

  it("passes through the stale and error statuses unchanged", () => {
    expect(readAuthContext(headersWith({ [AUTH_STATUS_HEADER]: "stale" })).status).toBe("stale");
    expect(readAuthContext(headersWith({ [AUTH_STATUS_HEADER]: "error" })).status).toBe("error");
  });

  it("exposes the three identity header names for stripping", () => {
    expect(IDENTITY_HEADERS).toEqual([AUTH_STATUS_HEADER, USER_EMAIL_HEADER, USER_IS_ADMIN_HEADER]);
  });
});

describe("requireAuthenticated", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("returns the full auth context without redirecting when authenticated", async () => {
    vi.mocked(headers).mockResolvedValue(
      headersWith({
        [AUTH_STATUS_HEADER]: "authenticated",
        [USER_EMAIL_HEADER]: "a@b.c",
        [USER_IS_ADMIN_HEADER]: "true",
      }) as unknown as Awaited<ReturnType<typeof headers>>,
    );

    await expect(requireAuthenticated("/login")).resolves.toEqual({
      status: "authenticated",
      email: "a@b.c",
      isAdmin: true,
    });
    expect(redirect).not.toHaveBeenCalled();
  });

  it.each(["anonymous", "stale", "error"] as const)(
    "redirects to the supplied target for the %s status",
    async (status) => {
      vi.mocked(headers).mockResolvedValue(
        headersWith({ [AUTH_STATUS_HEADER]: status }) as unknown as Awaited<
          ReturnType<typeof headers>
        >,
      );

      await expect(requireAuthenticated("/login")).rejects.toThrow(`${REDIRECT_PREFIX}/login`);
      expect(redirect).toHaveBeenCalledWith("/login");
    },
  );

  it("redirects to a missing status header target, honouring the anonymous fallback", async () => {
    vi.mocked(headers).mockResolvedValue(
      new Headers() as unknown as Awaited<ReturnType<typeof headers>>,
    );

    await expect(requireAuthenticated("/login")).rejects.toThrow(`${REDIRECT_PREFIX}/login`);
  });

  it("uses the caller-supplied target rather than a hard-coded /login", async () => {
    vi.mocked(headers).mockResolvedValue(
      headersWith({ [AUTH_STATUS_HEADER]: "anonymous" }) as unknown as Awaited<
        ReturnType<typeof headers>
      >,
    );

    await expect(requireAuthenticated("/")).rejects.toThrow(`${REDIRECT_PREFIX}/`);
    expect(redirect).toHaveBeenCalledWith("/");
  });
});

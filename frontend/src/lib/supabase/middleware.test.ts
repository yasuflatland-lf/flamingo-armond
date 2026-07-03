import { NextRequest, NextResponse } from "next/server";
import { afterEach, describe, expect, it, vi } from "vitest";
import type { HtmlCspOptions } from "@/lib/security/csp";
import { updateSession } from "./middleware";

const mockGetUser = vi.hoisted(() =>
  vi.fn().mockResolvedValue({ data: { user: null }, error: null }),
);

const mockGetClaims = vi.hoisted(() => vi.fn().mockResolvedValue({ data: null, error: null }));

vi.mock("@supabase/ssr", () => ({
  createServerClient: vi.fn().mockReturnValue({
    auth: { getUser: mockGetUser, getClaims: mockGetClaims },
  }),
}));

const mockBuildHtmlCsp = vi.hoisted(() => vi.fn<(opts: HtmlCspOptions) => string>());

vi.mock("@/lib/security/csp", async (importOriginal) => {
  const mod = await importOriginal<typeof import("@/lib/security/csp")>();
  mockBuildHtmlCsp.mockImplementation(mod.buildHtmlCsp);
  return { ...mod, buildHtmlCsp: mockBuildHtmlCsp };
});

function makeRequest(url = "http://localhost/", cookies: Record<string, string> = {}) {
  const req = new NextRequest(new URL(url));
  for (const [name, value] of Object.entries(cookies)) {
    req.cookies.set(name, value);
  }
  return req;
}

function forwardedRequestHeader(response: NextResponse, name: string) {
  return response.headers.get(`x-middleware-request-${name}`);
}

function forwardedRequestHeaderNames(response: NextResponse) {
  return response.headers.get("x-middleware-override-headers")?.split(",") ?? [];
}

function nonceFromPolicy(policy: string) {
  const match = policy.match(/'nonce-([^']+)'/);
  expect(match).not.toBeNull();
  return match?.[1] ?? "";
}

describe("updateSession", () => {
  afterEach(() => {
    vi.restoreAllMocks();
    vi.clearAllMocks();
  });

  it("uses getClaims() as the single auth source and never calls getUser()", async () => {
    await updateSession(makeRequest());
    expect(mockGetClaims).toHaveBeenCalledTimes(1);
    expect(mockGetUser).not.toHaveBeenCalled();
  });

  it("returns a NextResponse instance in the pass-through (no session) case", async () => {
    const response = await updateSession(makeRequest());
    expect(response).toBeInstanceOf(NextResponse);
  });

  it("carries request cookies into response cookies when setAll is invoked by the SDK", async () => {
    const { createServerClient } = await import("@supabase/ssr");
    vi.mocked(createServerClient).mockImplementationOnce(
      // biome-ignore lint/suspicious/noExplicitAny: test-only cast to drive setAll
      (_url, _key, opts: any) => {
        // Simulate SDK calling setAll to persist refreshed auth cookies
        opts.cookies.setAll([{ name: "sb-auth-token", value: "refreshed", options: {} }]);
        // biome-ignore lint/suspicious/noExplicitAny: test-only stub return
        return { auth: { getUser: mockGetUser, getClaims: mockGetClaims } } as any;
      },
    );

    const response = await updateSession(
      makeRequest("http://localhost/", { "sb-auth-token": "old" }),
    );
    expect(response.cookies.get("sb-auth-token")?.value).toBe("refreshed");
  });

  it("adds an enforcing CSP response header while forwarding a nonce-bearing CSP for Next rendering", async () => {
    const response = await updateSession(makeRequest("http://localhost/dashboard"));

    const cspPolicy = response.headers.get("Content-Security-Policy");
    expect(cspPolicy).toContain("script-src 'self' 'nonce-");
    expect(response.headers.get("Content-Security-Policy-Report-Only")).toBeNull();

    const nonce = nonceFromPolicy(cspPolicy ?? "");
    expect(nonce).toMatch(/^[A-Za-z0-9_-]+$/);
    expect(nonce).not.toMatch(/[<>&]/);

    expect(forwardedRequestHeader(response, "content-security-policy")).toBe(cspPolicy);
    expect(forwardedRequestHeader(response, "x-nonce")).toBe(nonce);
    expect(forwardedRequestHeaderNames(response)).toEqual(
      expect.arrayContaining(["content-security-policy", "x-nonce"]),
    );
  });

  it("generates a different nonce for separate requests", async () => {
    const firstBytes = Uint8Array.from({ length: 18 }, (_, index) => index);
    const secondBytes = Uint8Array.from({ length: 18 }, (_, index) => 17 - index);
    const getRandomValuesSpy = vi.spyOn(globalThis.crypto, "getRandomValues");
    getRandomValuesSpy.mockImplementationOnce((bytes) => {
      if (!(bytes instanceof Uint8Array)) {
        throw new TypeError("expected Uint8Array");
      }

      bytes.set(firstBytes);
      return bytes;
    });
    getRandomValuesSpy.mockImplementationOnce((bytes) => {
      if (!(bytes instanceof Uint8Array)) {
        throw new TypeError("expected Uint8Array");
      }

      bytes.set(secondBytes);
      return bytes;
    });

    const firstResponse = await updateSession(makeRequest("http://localhost/dashboard"));
    const secondResponse = await updateSession(makeRequest("http://localhost/dashboard"));

    const firstNonce = nonceFromPolicy(firstResponse.headers.get("Content-Security-Policy") ?? "");
    const secondNonce = nonceFromPolicy(
      secondResponse.headers.get("Content-Security-Policy") ?? "",
    );

    expect(firstNonce).not.toBe(secondNonce);
  });

  it("preserves nonce propagation when Supabase refreshes cookies", async () => {
    const { createServerClient } = await import("@supabase/ssr");
    vi.mocked(createServerClient).mockImplementationOnce(
      // biome-ignore lint/suspicious/noExplicitAny: test-only cast to drive setAll
      (_url, _key, opts: any) => {
        opts.cookies.setAll([{ name: "sb-auth-token", value: "refreshed", options: {} }]);
        // biome-ignore lint/suspicious/noExplicitAny: test-only stub return
        return { auth: { getUser: mockGetUser, getClaims: mockGetClaims } } as any;
      },
    );

    const response = await updateSession(
      makeRequest("http://localhost/dashboard", { "sb-auth-token": "old" }),
    );

    const cspPolicy = response.headers.get("Content-Security-Policy");
    const nonce = nonceFromPolicy(cspPolicy ?? "");

    expect(response.cookies.get("sb-auth-token")?.value).toBe("refreshed");
    expect(forwardedRequestHeader(response, "content-security-policy")).toBe(cspPolicy);
    expect(forwardedRequestHeader(response, "x-nonce")).toBe(nonce);
    expect(mockGetClaims).toHaveBeenCalledTimes(1);
    expect(mockGetUser).not.toHaveBeenCalled();
  });

  it("returns a valid response for any request path", async () => {
    const response = await updateSession(makeRequest("http://localhost/dashboard"));
    expect(response).toBeInstanceOf(NextResponse);
    expect(mockGetClaims).toHaveBeenCalledTimes(1);
    expect(mockGetUser).not.toHaveBeenCalled();
  });

  it("handles requests with pre-existing cookies without mutating them unexpectedly", async () => {
    const req = makeRequest("http://localhost/", { "existing-cookie": "stays" });
    const response = await updateSession(req);
    expect(response).toBeInstanceOf(NextResponse);
    // No new cookies were set by the mock SDK, so response cookies should be empty
    expect(response.cookies.get("existing-cookie")).toBeUndefined();
  });

  it("logs console.error when getClaims() returns a non-ignorable error", async () => {
    const consoleErrorSpy = vi.spyOn(console, "error").mockImplementation(() => {});
    mockGetClaims.mockResolvedValueOnce({
      data: null,
      error: { name: "AuthError", message: "network failure" },
    });

    await updateSession(makeRequest());

    expect(consoleErrorSpy).toHaveBeenCalledWith(
      "[supabase/middleware] getClaims() failed:",
      "AuthError",
    );
    consoleErrorSpy.mockRestore();
  });

  it("does not log an error when getClaims() returns AuthSessionMissingError", async () => {
    const consoleErrorSpy = vi.spyOn(console, "error").mockImplementation(() => {});
    mockGetClaims.mockResolvedValueOnce({
      data: null,
      error: { name: "AuthSessionMissingError", message: "missing session" },
    });

    await updateSession(makeRequest());

    expect(consoleErrorSpy).not.toHaveBeenCalled();
    consoleErrorSpy.mockRestore();
  });

  it("omits CSP headers and x-nonce and logs console.error when buildHtmlCsp throws", async () => {
    mockBuildHtmlCsp.mockImplementationOnce(() => {
      throw new Error("invalid supabaseUrl");
    });
    const consoleErrorSpy = vi.spyOn(console, "error").mockImplementation(() => {});

    const response = await updateSession(makeRequest("http://localhost/dashboard"));

    expect(response).toBeInstanceOf(NextResponse);
    expect(response.headers.get("Content-Security-Policy")).toBeNull();
    expect(response.headers.get("Content-Security-Policy-Report-Only")).toBeNull();
    expect(forwardedRequestHeader(response, "content-security-policy")).toBeNull();
    expect(forwardedRequestHeader(response, "x-nonce")).toBeNull();
    expect(consoleErrorSpy).toHaveBeenCalledTimes(1);
    expect(consoleErrorSpy).toHaveBeenCalledWith(
      expect.stringContaining("[supabase/middleware] buildHtmlCsp failed"),
      expect.any(Error),
    );
    expect(mockGetClaims).toHaveBeenCalledTimes(1);
    expect(mockGetUser).not.toHaveBeenCalled();

    consoleErrorSpy.mockRestore();
  });

  it("forwards anonymous identity headers when there is no session", async () => {
    const response = await updateSession(makeRequest());
    expect(forwardedRequestHeader(response, "x-auth-status")).toBe("anonymous");
    expect(forwardedRequestHeader(response, "x-user-email")).toBe("");
    expect(forwardedRequestHeader(response, "x-user-is-admin")).toBe("false");
  });

  it("forwards authenticated identity headers, isAdmin true for an admin user", async () => {
    // Both the email and the admin role live in the verified JWT claims — the
    // Custom Access Token Hook injects the role into the JWT only.
    mockGetClaims.mockResolvedValueOnce({
      data: { claims: { email: "admin@example.com", app_metadata: { role: "admin" } } },
      error: null,
    });
    const response = await updateSession(makeRequest());
    expect(forwardedRequestHeader(response, "x-auth-status")).toBe("authenticated");
    expect(forwardedRequestHeader(response, "x-user-email")).toBe("admin@example.com");
    expect(forwardedRequestHeader(response, "x-user-is-admin")).toBe("true");
    expect(mockGetUser).not.toHaveBeenCalled();
  });

  it("forwards isAdmin false for a non-admin authenticated user", async () => {
    mockGetClaims.mockResolvedValueOnce({
      data: { claims: { email: "u@example.com", app_metadata: {} } },
      error: null,
    });
    const response = await updateSession(makeRequest());
    expect(forwardedRequestHeader(response, "x-auth-status")).toBe("authenticated");
    expect(forwardedRequestHeader(response, "x-user-email")).toBe("u@example.com");
    expect(forwardedRequestHeader(response, "x-user-is-admin")).toBe("false");
  });

  it("classifies a stale-session error at the token-refresh boundary", async () => {
    // Deleted-user detection now surfaces as a getClaims() refresh failure.
    const consoleErrorSpy = vi.spyOn(console, "error").mockImplementation(() => {});
    mockGetClaims.mockResolvedValueOnce({
      data: null,
      error: { name: "AuthApiError", message: "User from sub claim in JWT does not exist" },
    });
    const response = await updateSession(makeRequest());
    expect(forwardedRequestHeader(response, "x-auth-status")).toBe("stale");
    expect(consoleErrorSpy).not.toHaveBeenCalled();
    consoleErrorSpy.mockRestore();
  });

  it("classifies a non-ignorable error", async () => {
    const consoleErrorSpy = vi.spyOn(console, "error").mockImplementation(() => {});
    mockGetClaims.mockResolvedValueOnce({
      data: null,
      error: { name: "AuthError", message: "network failure" },
    });
    const response = await updateSession(makeRequest());
    expect(forwardedRequestHeader(response, "x-auth-status")).toBe("error");
    consoleErrorSpy.mockRestore();
  });

  it("defaults isAdmin false and email empty when getClaims returns a verification error", async () => {
    const consoleErrorSpy = vi.spyOn(console, "error").mockImplementation(() => {});
    mockGetClaims.mockResolvedValueOnce({
      data: null,
      error: { name: "AuthApiError", message: "JWKS fetch failed" },
    });
    const response = await updateSession(makeRequest());
    expect(forwardedRequestHeader(response, "x-auth-status")).toBe("error");
    expect(forwardedRequestHeader(response, "x-user-is-admin")).toBe("false");
    expect(forwardedRequestHeader(response, "x-user-email")).toBe("");
    expect(consoleErrorSpy).toHaveBeenCalled();
    consoleErrorSpy.mockRestore();
  });

  it("does not 500 when getClaims throws; degrades to error status, isAdmin false", async () => {
    const consoleWarnSpy = vi.spyOn(console, "warn").mockImplementation(() => {});
    mockGetClaims.mockRejectedValueOnce(new Error("JWT has expired"));
    const response = await updateSession(makeRequest());
    expect(response).toBeInstanceOf(NextResponse);
    expect(forwardedRequestHeader(response, "x-auth-status")).toBe("error");
    expect(forwardedRequestHeader(response, "x-user-is-admin")).toBe("false");
    expect(consoleWarnSpy).toHaveBeenCalled();
    consoleWarnSpy.mockRestore();
  });

  it("strips inbound (spoofed) identity headers", async () => {
    const req = makeRequest();
    req.headers.set("x-user-is-admin", "true");
    req.headers.set("x-user-email", "evil@attacker.test");
    req.headers.set("x-auth-status", "authenticated");
    const response = await updateSession(req);
    expect(forwardedRequestHeader(response, "x-auth-status")).toBe("anonymous");
    expect(forwardedRequestHeader(response, "x-user-email")).toBe("");
    expect(forwardedRequestHeader(response, "x-user-is-admin")).toBe("false");
  });

  it("preserves refreshed auth cookies alongside identity headers", async () => {
    const { createServerClient } = await import("@supabase/ssr");
    vi.mocked(createServerClient).mockImplementationOnce(
      // biome-ignore lint/suspicious/noExplicitAny: test-only cast to drive setAll
      (_url, _key, opts: any) => {
        opts.cookies.setAll([{ name: "sb-auth-token", value: "refreshed", options: {} }]);
        // biome-ignore lint/suspicious/noExplicitAny: test-only stub return
        return { auth: { getUser: mockGetUser, getClaims: mockGetClaims } } as any;
      },
    );
    const response = await updateSession(
      makeRequest("http://localhost/", { "sb-auth-token": "old" }),
    );
    expect(response.cookies.get("sb-auth-token")?.value).toBe("refreshed");
    expect(forwardedRequestHeader(response, "x-auth-status")).toBe("anonymous");
  });
});

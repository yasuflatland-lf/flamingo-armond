import { NextRequest, NextResponse } from "next/server";
import { afterEach, describe, expect, it, vi } from "vitest";
import { updateSession } from "./middleware";

const mockGetUser = vi.hoisted(() =>
  vi.fn().mockResolvedValue({ data: { user: null }, error: null }),
);

vi.mock("@supabase/ssr", () => ({
  createServerClient: vi.fn().mockReturnValue({
    auth: { getUser: mockGetUser },
  }),
}));

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

  it("calls auth.getUser() exactly once", async () => {
    await updateSession(makeRequest());
    expect(mockGetUser).toHaveBeenCalledTimes(1);
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
        return { auth: { getUser: mockGetUser } } as any;
      },
    );

    const response = await updateSession(
      makeRequest("http://localhost/", { "sb-auth-token": "old" }),
    );
    expect(response.cookies.get("sb-auth-token")?.value).toBe("refreshed");
  });

  it("adds a report-only CSP response header while forwarding a nonce-bearing CSP for Next rendering", async () => {
    const response = await updateSession(makeRequest("http://localhost/dashboard"));

    const reportOnlyPolicy = response.headers.get("Content-Security-Policy-Report-Only");
    expect(reportOnlyPolicy).toContain("script-src 'self' 'nonce-");
    expect(response.headers.get("Content-Security-Policy")).toBeNull();

    const nonce = nonceFromPolicy(reportOnlyPolicy ?? "");
    expect(nonce).toMatch(/^[A-Za-z0-9_-]+$/);
    expect(nonce).not.toMatch(/[<>&]/);

    expect(forwardedRequestHeader(response, "content-security-policy")).toBe(reportOnlyPolicy);
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
      bytes.set(firstBytes);
      return bytes;
    });
    getRandomValuesSpy.mockImplementationOnce((bytes) => {
      bytes.set(secondBytes);
      return bytes;
    });

    const firstResponse = await updateSession(makeRequest("http://localhost/dashboard"));
    const secondResponse = await updateSession(makeRequest("http://localhost/dashboard"));

    const firstNonce = nonceFromPolicy(
      firstResponse.headers.get("Content-Security-Policy-Report-Only") ?? "",
    );
    const secondNonce = nonceFromPolicy(
      secondResponse.headers.get("Content-Security-Policy-Report-Only") ?? "",
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
        return { auth: { getUser: mockGetUser } } as any;
      },
    );

    const response = await updateSession(
      makeRequest("http://localhost/dashboard", { "sb-auth-token": "old" }),
    );

    const reportOnlyPolicy = response.headers.get("Content-Security-Policy-Report-Only");
    const nonce = nonceFromPolicy(reportOnlyPolicy ?? "");

    expect(response.cookies.get("sb-auth-token")?.value).toBe("refreshed");
    expect(forwardedRequestHeader(response, "content-security-policy")).toBe(reportOnlyPolicy);
    expect(forwardedRequestHeader(response, "x-nonce")).toBe(nonce);
    expect(mockGetUser).toHaveBeenCalledTimes(1);
  });

  it("forwards the request pathname for server components", async () => {
    const response = await updateSession(makeRequest("http://localhost/dashboard?tab=due"));

    expect(forwardedRequestHeader(response, "x-pathname")).toBe("/dashboard");
    expect(forwardedRequestHeaderNames(response)).toContain("x-pathname");
  });

  it("returns a valid response for any request path", async () => {
    const response = await updateSession(makeRequest("http://localhost/dashboard"));
    expect(response).toBeInstanceOf(NextResponse);
    expect(mockGetUser).toHaveBeenCalledTimes(1);
  });

  it("handles requests with pre-existing cookies without mutating them unexpectedly", async () => {
    const req = makeRequest("http://localhost/", { "existing-cookie": "stays" });
    const response = await updateSession(req);
    expect(response).toBeInstanceOf(NextResponse);
    // No new cookies were set by the mock SDK, so response cookies should be empty
    expect(response.cookies.get("existing-cookie")).toBeUndefined();
  });
});

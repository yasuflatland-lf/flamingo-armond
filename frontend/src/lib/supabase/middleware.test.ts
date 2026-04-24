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

describe("updateSession", () => {
  afterEach(() => vi.clearAllMocks());

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

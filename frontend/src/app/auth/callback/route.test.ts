import { NextRequest } from "next/server";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { GET } from "./route";

const mockExchangeCodeForSession = vi.hoisted(() => vi.fn());

vi.mock("@/lib/supabase/server", () => ({
  createSupabaseServerClient: vi.fn().mockResolvedValue({
    auth: { exchangeCodeForSession: mockExchangeCodeForSession },
  }),
}));

function makeRequest(url: string) {
  return new NextRequest(new URL(url));
}

describe("GET /auth/callback", () => {
  let consoleErrorSpy: ReturnType<typeof vi.spyOn>;

  beforeEach(() => {
    consoleErrorSpy = vi.spyOn(console, "error").mockImplementation(() => {});
  });

  afterEach(() => {
    consoleErrorSpy.mockRestore();
    vi.clearAllMocks();
  });

  it("exchanges valid code and redirects to /", async () => {
    mockExchangeCodeForSession.mockResolvedValueOnce({ error: null });

    const response = await GET(makeRequest("http://localhost/auth/callback?code=abc123"));

    expect(mockExchangeCodeForSession).toHaveBeenCalledWith("abc123");
    expect([301, 302, 307, 308]).toContain(response.status);
    expect(response.headers.get("location")).toBe("http://localhost/");
  });

  it("redirects to /login?error=missing_code when code is absent", async () => {
    const response = await GET(makeRequest("http://localhost/auth/callback"));

    expect(mockExchangeCodeForSession).not.toHaveBeenCalled();
    expect([301, 302, 307, 308]).toContain(response.status);
    expect(response.headers.get("location")).toBe("http://localhost/login?error=missing_code");
  });

  it("redirects to /login?error=exchange_failed when exchangeCodeForSession errors", async () => {
    mockExchangeCodeForSession.mockResolvedValueOnce({
      error: new Error("exchange error"),
    });

    const response = await GET(makeRequest("http://localhost/auth/callback?code=bad"));

    expect([301, 302, 307, 308]).toContain(response.status);
    expect(response.headers.get("location")).toBe("http://localhost/login?error=exchange_failed");
  });

  it("honors ?next query param for deep-link redirect", async () => {
    mockExchangeCodeForSession.mockResolvedValueOnce({ error: null });

    const response = await GET(
      makeRequest("http://localhost/auth/callback?code=xyz&next=/profile"),
    );

    expect([301, 302, 307, 308]).toContain(response.status);
    expect(response.headers.get("location")).toBe("http://localhost/profile");
  });

  it("falls back to / when next is an absolute URL", async () => {
    mockExchangeCodeForSession.mockResolvedValueOnce({ error: null });

    const response = await GET(
      makeRequest("http://localhost/auth/callback?code=valid&next=https://evil.com"),
    );

    expect([301, 302, 307, 308]).toContain(response.status);
    const location = response.headers.get("location") ?? "";
    expect(location.startsWith("http://localhost")).toBe(true);
    expect(new URL(location).pathname).toBe("/");
  });

  it("falls back to / when next is protocol-relative", async () => {
    mockExchangeCodeForSession.mockResolvedValueOnce({ error: null });

    const response = await GET(
      makeRequest("http://localhost/auth/callback?code=valid&next=//evil.com"),
    );

    expect([301, 302, 307, 308]).toContain(response.status);
    const location = response.headers.get("location") ?? "";
    expect(location.startsWith("http://localhost")).toBe(true);
    expect(new URL(location).pathname).toBe("/");
  });

  it("falls back to / when next contains a backslash (URL-parser normalizes to /)", async () => {
    mockExchangeCodeForSession.mockResolvedValueOnce({ error: null });
    const response = await GET(
      makeRequest("http://localhost/auth/callback?code=valid&next=/%5Cevil.com"),
    );
    // location header should NOT have host "evil.com"
    const location = response.headers.get("location");
    expect(location).toBe("http://localhost/");
  });
});

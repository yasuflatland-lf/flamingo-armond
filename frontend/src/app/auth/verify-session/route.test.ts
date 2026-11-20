import { NextRequest } from "next/server";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { VerifySessionQuery } from "./queries";
import { GET } from "./route";

const mockSignOut = vi.hoisted(() => vi.fn());

vi.mock("@/lib/supabase/server", () => ({
  createSupabaseServerClient: vi.fn().mockResolvedValue({
    auth: { signOut: mockSignOut },
  }),
}));

vi.mock("@/lib/apollo/server", () => ({
  gqlFetch: vi.fn(),
}));

import { gqlFetch } from "@/lib/apollo/server";

function gqlError(code: string) {
  return new Error(`GraphQL errors: ${JSON.stringify([{ extensions: { code } }])}`);
}

function makeRequest() {
  return new NextRequest(new URL("http://localhost/auth/verify-session"));
}

describe("GET /auth/verify-session", () => {
  let consoleErrorSpy: ReturnType<typeof vi.spyOn>;
  let consoleWarnSpy: ReturnType<typeof vi.spyOn>;

  beforeEach(() => {
    consoleErrorSpy = vi.spyOn(console, "error").mockImplementation(() => {});
    consoleWarnSpy = vi.spyOn(console, "warn").mockImplementation(() => {});
  });

  afterEach(() => {
    consoleErrorSpy.mockRestore();
    consoleWarnSpy.mockRestore();
    vi.clearAllMocks();
  });

  it("live session: redirects to / without signing out", async () => {
    vi.mocked(gqlFetch).mockResolvedValueOnce({ me: { id: "u1" } } as never);

    const response = await GET(makeRequest());

    expect(response.status).toBe(307);
    expect(response.headers.get("location")).toBe("http://localhost/");
    expect(gqlFetch).toHaveBeenCalledTimes(1);
    expect(gqlFetch).toHaveBeenCalledWith(VerifySessionQuery, { revalidate: 0 });
    expect(mockSignOut).not.toHaveBeenCalled();
  });

  it("UNAUTHENTICATED: signs out locally and redirects to /login?reason=session_invalid", async () => {
    vi.mocked(gqlFetch).mockRejectedValueOnce(gqlError("UNAUTHENTICATED"));
    mockSignOut.mockResolvedValueOnce({ error: null });

    const response = await GET(makeRequest());

    expect(mockSignOut).toHaveBeenCalledTimes(1);
    expect(mockSignOut).toHaveBeenCalledWith({ scope: "local" });
    expect(response.status).toBe(307);
    expect(response.headers.get("location")).toBe("http://localhost/login?reason=session_invalid");
  });

  it("UNAUTHENTICATED + signOut error: still redirects to /login?reason=session_invalid", async () => {
    vi.mocked(gqlFetch).mockRejectedValueOnce(gqlError("UNAUTHENTICATED"));
    mockSignOut.mockResolvedValueOnce({
      error: Object.assign(new Error("x"), { name: "AuthApiError" }),
    });

    const response = await GET(makeRequest());

    expect(response.headers.get("location")).toBe("http://localhost/login?reason=session_invalid");
    expect(consoleWarnSpy).toHaveBeenCalledWith("[auth/verify-session] signOut failed:", {
      name: "AuthApiError",
    });
  });

  it("non-auth failure: redirects to / without signing out", async () => {
    vi.mocked(gqlFetch).mockRejectedValueOnce(gqlError("INTERNAL"));

    const response = await GET(makeRequest());

    expect(response.headers.get("location")).toBe("http://localhost/");
    expect(mockSignOut).not.toHaveBeenCalled();
    expect(consoleErrorSpy).toHaveBeenCalledTimes(1);
    expect(consoleErrorSpy).toHaveBeenCalledWith("[auth/verify-session] gqlFetch failed:", {
      name: "Error",
    });
    expect(consoleErrorSpy).not.toHaveBeenCalledWith(
      expect.anything(),
      expect.objectContaining({ message: expect.anything() }),
    );
  });

  it("transport HTTP 401 (bad signature, aud/iss mismatch) is not treated as a rejected session", async () => {
    vi.mocked(gqlFetch).mockRejectedValueOnce(
      new Error("GraphQL HTTP 401 Unauthorized: invalid token"),
    );

    const response = await GET(makeRequest());

    expect(response.headers.get("location")).toBe("http://localhost/");
    expect(mockSignOut).not.toHaveBeenCalled();
    expect(consoleErrorSpy).toHaveBeenCalledWith("[auth/verify-session] gqlFetch failed:", {
      name: "Error",
    });
    expect(consoleErrorSpy).not.toHaveBeenCalledWith(
      expect.anything(),
      expect.objectContaining({ message: expect.anything() }),
    );
  });

  it("FORBIDDEN is not treated as a rejected session", async () => {
    vi.mocked(gqlFetch).mockRejectedValueOnce(gqlError("FORBIDDEN"));

    const response = await GET(makeRequest());

    expect(response.headers.get("location")).toBe("http://localhost/");
    expect(mockSignOut).not.toHaveBeenCalled();
  });
});

// Testing strategy: Option A-variant — buildAuthHeaders is extracted to auth-link.ts
// and tested directly, avoiding the complexity of the Apollo Link Observable API.
import { afterEach, describe, expect, it, vi } from "vitest";
import { buildAuthHeaders } from "./auth-link";

const mockGetSession = vi.hoisted(() =>
  vi.fn().mockResolvedValue({ data: { session: null }, error: null }),
);

vi.mock("@/lib/supabase/client", () => ({
  createSupabaseBrowserClient: vi.fn().mockReturnValue({
    auth: { getSession: mockGetSession },
  }),
}));

describe("buildAuthHeaders (authLink)", () => {
  afterEach(() => vi.clearAllMocks());

  it("attaches Bearer token when getSession returns an access_token", async () => {
    mockGetSession.mockResolvedValueOnce({
      data: { session: { access_token: "tok_abc123" } },
    });

    const result = await buildAuthHeaders({ "content-type": "application/json" });

    expect(result.authorization).toBe("Bearer tok_abc123");
    expect(result["content-type"]).toBe("application/json");
  });

  it("omits authorization header when getSession returns no session", async () => {
    mockGetSession.mockResolvedValueOnce({ data: { session: null } });

    const result = await buildAuthHeaders({});

    expect(result.authorization).toBeUndefined();
  });

  it("fails open when getSession rejects — request proceeds with no authorization header", async () => {
    mockGetSession.mockRejectedValueOnce(new Error("network error"));

    const result = await buildAuthHeaders({ "x-custom": "keep" });

    expect(result.authorization).toBeUndefined();
    expect(result["x-custom"]).toBe("keep");
  });
});

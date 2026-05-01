import { afterEach, describe, expect, it, vi } from "vitest";
import { GET } from "./route";

const mockGqlFetch = vi.hoisted(() => vi.fn());

vi.mock("@/lib/apollo/server", () => ({
  gqlFetch: mockGqlFetch,
}));

describe("GET /api/healthz", () => {
  afterEach(() => {
    vi.clearAllMocks();
  });

  it("returns 200 with ok:true and backend value when gqlFetch resolves", async () => {
    mockGqlFetch.mockResolvedValueOnce({ health: "ok" });

    const response = await GET();
    const body = await response.json();

    expect(response.status).toBe(200);
    expect(body).toEqual({ ok: true, backend: "ok" });
  });

  it("returns 503 with ok:false and error message when gqlFetch rejects", async () => {
    mockGqlFetch.mockRejectedValueOnce(new Error("backend unavailable"));

    const response = await GET();
    const body = await response.json();

    expect(response.status).toBe(503);
    expect(body).toEqual({ ok: false, error: "backend unavailable" });
  });
});

// Testing strategy: Option A-variant — buildAuthHeaders is extracted to auth-link.ts
// and tested directly, avoiding the complexity of the Apollo Link Observable API.
import type { ApolloLink } from "@apollo/client";
import { afterEach, describe, expect, it, vi } from "vitest";
import { buildAuthHeaders } from "./auth-link";
import { makeClient } from "./client";

// Collect top-level segments of a chain built with ApolloLink.from / concat.
// from([a, b, c]) creates plain ApolloLink glue nodes whose constructor is exactly
// ApolloLink. Recursing only into those nodes (not into named sub-classes like
// HttpLink or PersistedQueryLink) yields the original three segments in order.
function collectSegments(link: ApolloLink): ApolloLink[] {
  if (link.constructor?.name === "ApolloLink" && link.left != null) {
    return [...collectSegments(link.left), ...collectSegments(link.right ?? link.left)];
  }
  return [link];
}

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

describe("makeClient link chain", () => {
  it("chains authLink -> apqLink -> httpLink in order (3 segments)", () => {
    const c = makeClient();
    expect(c.link).toBeDefined();
    // from([authLink, apqLink, httpLink]) wraps segments in plain ApolloLink
    // glue nodes. collectSegments stops at named sub-classes so we recover the
    // original user-supplied links plus any framework-injected wrappers.
    // @apollo/client-integration-nextjs prepends 2 streaming links, so the
    // full segment list is: [ReadFromReadableStreamLink, TeeToReadableStreamLink,
    // SetContextLink(auth), PersistedQueryLink(apq), HttpLink(http)].
    const links = collectSegments(c.link);
    const names = links.map((l) => l.constructor?.name ?? "");
    // Core three user-supplied links must all be present.
    expect(names).toContain("SetContextLink");
    expect(names).toContain("PersistedQueryLink");
    expect(names).toContain("HttpLink");
    // Order: auth must come before apq, and apq must come before http.
    const authIdx = names.indexOf("SetContextLink");
    const apqIdx = names.indexOf("PersistedQueryLink");
    const httpIdx = names.indexOf("HttpLink");
    expect(authIdx).toBeLessThan(apqIdx);
    expect(apqIdx).toBeLessThan(httpIdx);
  });
});

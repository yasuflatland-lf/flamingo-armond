import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { graphql } from "@/generated";
import { createSupabaseServerClient } from "@/lib/supabase/server";
import { gqlFetch } from "./server";

vi.mock("@/lib/supabase/server", () => ({
  createSupabaseServerClient: vi.fn(),
}));

const HealthQuery = graphql(`query Health { health }`);

function mockSession(session: { access_token: string } | null) {
  vi.mocked(createSupabaseServerClient).mockResolvedValue({
    auth: {
      getSession: vi.fn().mockResolvedValue({ data: { session } }),
    },
  } as any);
}

describe("gqlFetch", () => {
  beforeEach(() => {
    mockSession(null);
  });

  afterEach(() => vi.restoreAllMocks());

  it("POSTs to BACKEND_URL/query with printed query body", async () => {
    const fetchSpy = vi
      .spyOn(global, "fetch")
      .mockResolvedValue(new Response(JSON.stringify({ data: { health: "ok" } })));

    const data = await gqlFetch(HealthQuery);

    expect(data.health).toBe("ok");
    const [url, init] = fetchSpy.mock.calls[0] as [string, RequestInit];
    expect(url).toMatch(/\/query$/);
    expect(init.method).toBe("POST");
    expect(JSON.parse(init.body as string).query).toContain("health");
  });

  it("throws on non-200", async () => {
    vi.spyOn(global, "fetch").mockResolvedValue(new Response("boom", { status: 500 }));
    await expect(gqlFetch(HealthQuery)).rejects.toThrow(/HTTP 500.*boom/);
  });

  it("throws when errors array is present", async () => {
    vi.spyOn(global, "fetch").mockResolvedValue(
      new Response(JSON.stringify({ errors: [{ message: "nope" }] })),
    );
    await expect(gqlFetch(HealthQuery)).rejects.toThrow(/GraphQL errors/);
  });

  it("throws when data is missing", async () => {
    vi.spyOn(global, "fetch").mockResolvedValue(new Response(JSON.stringify({})));
    await expect(gqlFetch(HealthQuery)).rejects.toThrow(/missing data/);
  });

  it("passes next.revalidate through", async () => {
    const fetchSpy = vi
      .spyOn(global, "fetch")
      .mockResolvedValue(new Response(JSON.stringify({ data: { health: "ok" } })));
    await gqlFetch(HealthQuery, { revalidate: 60 });
    const init = fetchSpy.mock.calls[0]?.[1] as RequestInit & { next?: { revalidate?: number } };
    expect(init.next?.revalidate).toBe(60);
  });

  it("passes revalidate: false through distinctly from undefined", async () => {
    const fetchSpy = vi
      .spyOn(global, "fetch")
      .mockResolvedValue(new Response(JSON.stringify({ data: { health: "ok" } })));
    await gqlFetch(HealthQuery, { revalidate: false });
    const init = fetchSpy.mock.calls[0]?.[1] as RequestInit & {
      next?: { revalidate?: number | false };
    };
    expect(init.next?.revalidate).toBe(false);
  });

  it("defaults variables to empty object when caller omits them", async () => {
    const fetchSpy = vi
      .spyOn(global, "fetch")
      .mockResolvedValue(new Response(JSON.stringify({ data: { health: "ok" } })));
    await gqlFetch(HealthQuery);
    const body = JSON.parse((fetchSpy.mock.calls[0]?.[1] as RequestInit).body as string);
    expect(body.variables).toEqual({});
  });

  it("forwards Bearer token when session is present", async () => {
    mockSession({ access_token: "tok-abc" });
    const fetchSpy = vi
      .spyOn(global, "fetch")
      .mockResolvedValue(new Response(JSON.stringify({ data: { health: "ok" } })));

    await gqlFetch(HealthQuery);

    const init = fetchSpy.mock.calls[0]?.[1] as RequestInit & { headers?: Record<string, string> };
    expect((init.headers as Record<string, string>).authorization).toBe("Bearer tok-abc");
  });

  it("omits authorization header when session is absent", async () => {
    mockSession(null);
    const fetchSpy = vi
      .spyOn(global, "fetch")
      .mockResolvedValue(new Response(JSON.stringify({ data: { health: "ok" } })));

    await gqlFetch(HealthQuery);

    const init = fetchSpy.mock.calls[0]?.[1] as RequestInit & { headers?: Record<string, string> };
    expect((init.headers as Record<string, string>)).not.toHaveProperty("authorization");
  });

  it("propagates error when getSession throws", async () => {
    vi.mocked(createSupabaseServerClient).mockResolvedValue({
      auth: {
        getSession: vi.fn().mockRejectedValue(new Error("session error")),
      },
    } as any);

    await expect(gqlFetch(HealthQuery)).rejects.toThrow("session error");
  });
});

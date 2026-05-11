import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { graphql } from "@/generated";
import { createSupabaseServerClient } from "@/lib/supabase/server";
import { gqlFetch } from "./server";

vi.mock("@/lib/supabase/server", () => ({
  createSupabaseServerClient: vi.fn(),
}));

const HealthQuery = graphql(`query Health { health }`);

function mockSession(session: { access_token: string } | null) {
  vi.mocked(createSupabaseServerClient).mockResolvedValue(
    // biome-ignore lint/suspicious/noExplicitAny: partial mock of Supabase client type
    { auth: { getSession: vi.fn().mockResolvedValue({ data: { session }, error: null }) } } as any,
  );
}

function mockSessionError(err: Error) {
  vi.mocked(createSupabaseServerClient).mockResolvedValue({
    auth: { getSession: vi.fn().mockResolvedValue({ data: { session: null }, error: err }) },
    // biome-ignore lint/suspicious/noExplicitAny: partial mock of Supabase client type
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

  it("throws when errors array is present and data is absent", async () => {
    vi.spyOn(global, "fetch").mockResolvedValue(
      new Response(JSON.stringify({ errors: [{ message: "nope" }] })),
    );
    await expect(gqlFetch(HealthQuery)).rejects.toThrow(/GraphQL errors/);
  });

  it("returns data and warns when partial response has both errors and data", async () => {
    vi.spyOn(global, "fetch").mockResolvedValue(
      new Response(
        JSON.stringify({ data: { health: "partial" }, errors: [{ message: "partial" }] }),
      ),
    );
    const warnSpy = vi.spyOn(console, "warn").mockImplementation(() => {});

    const result = await gqlFetch(HealthQuery);

    expect(result.health).toBe("partial");
    expect(warnSpy).toHaveBeenCalledOnce();
    expect(warnSpy.mock.calls[0]?.[0]).toBe("[gqlFetch] partial response with errors:");
  });

  it("throws with GraphQL errors prefix when partial response contains UNAUTHENTICATED", async () => {
    const errors = [{ message: "not authenticated", extensions: { code: "UNAUTHENTICATED" } }];
    vi.spyOn(global, "fetch").mockResolvedValue(
      new Response(JSON.stringify({ data: { health: "partial" }, errors })),
    );
    vi.spyOn(console, "warn").mockImplementation(() => {});

    await expect(gqlFetch(HealthQuery)).rejects.toThrow(
      `GraphQL errors: ${JSON.stringify(errors)}`,
    );
  });

  it("throws with GraphQL errors prefix when partial response contains FORBIDDEN", async () => {
    const errors = [{ message: "access denied", extensions: { code: "FORBIDDEN" } }];
    vi.spyOn(global, "fetch").mockResolvedValue(
      new Response(JSON.stringify({ data: { health: "partial" }, errors })),
    );
    vi.spyOn(console, "warn").mockImplementation(() => {});

    await expect(gqlFetch(HealthQuery)).rejects.toThrow(
      `GraphQL errors: ${JSON.stringify(errors)}`,
    );
  });

  it("returns data and warns when partial response contains a non-auth business error code", async () => {
    const errors = [{ message: "rate limit exceeded", extensions: { code: "RATE_LIMITED" } }];
    vi.spyOn(global, "fetch").mockResolvedValue(
      new Response(JSON.stringify({ data: { health: "ok" }, errors })),
    );
    const warnSpy = vi.spyOn(console, "warn").mockImplementation(() => {});

    const result = await gqlFetch(HealthQuery);

    expect(result.health).toBe("ok");
    expect(warnSpy).toHaveBeenCalledOnce();
    expect(warnSpy.mock.calls[0]?.[0]).toBe("[gqlFetch] partial response with errors:");
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
    expect(init.headers as Record<string, string>).not.toHaveProperty("authorization");
  });

  it("propagates error when getSession throws", async () => {
    vi.mocked(createSupabaseServerClient).mockResolvedValue(
      // biome-ignore lint/suspicious/noExplicitAny: partial mock of Supabase client type
      { auth: { getSession: vi.fn().mockRejectedValue(new Error("session error")) } } as any,
    );

    await expect(gqlFetch(HealthQuery)).rejects.toThrow("session error");
  });

  it("throws when getSession returns an error field", async () => {
    const authErr = new Error("auth down");
    mockSessionError(authErr);

    await expect(gqlFetch(HealthQuery)).rejects.toThrow("auth down");
  });

  it("sends Accept: application/graphql-response+json header on every fetch", async () => {
    const fetchSpy = vi
      .spyOn(global, "fetch")
      .mockResolvedValue(new Response(JSON.stringify({ data: { health: "ok" } })));

    await gqlFetch(HealthQuery);

    const init = fetchSpy.mock.calls[0]?.[1] as RequestInit & { headers?: Record<string, string> };
    expect((init.headers as Record<string, string>).Accept).toContain(
      "application/graphql-response+json",
    );
  });

  it("injects a well-formed UUID v7 X-Request-ID header on every fetch", async () => {
    const fetchSpy = vi
      .spyOn(global, "fetch")
      .mockResolvedValue(new Response(JSON.stringify({ data: { health: "ok" } })));

    await gqlFetch(HealthQuery);

    const init = fetchSpy.mock.calls[0]?.[1] as RequestInit & { headers?: Record<string, string> };
    const requestId = (init.headers as Record<string, string>)["X-Request-ID"];
    expect(requestId).toMatch(
      /^[0-9a-f]{8}-[0-9a-f]{4}-7[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/,
    );
  });
});

import { afterEach, describe, expect, it, vi } from "vitest";
import { graphql } from "@/generated";
import { gqlFetch } from "./server";

const HealthQuery = graphql(`query Health { health }`);

describe("gqlFetch", () => {
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
    await expect(gqlFetch(HealthQuery)).rejects.toThrow(/HTTP 500/);
  });

  it("throws when errors array is present", async () => {
    vi.spyOn(global, "fetch").mockResolvedValue(
      new Response(JSON.stringify({ errors: [{ message: "nope" }] })),
    );
    await expect(gqlFetch(HealthQuery)).rejects.toThrow(/GraphQL errors/);
  });

  it("throws when data is missing", async () => {
    vi.spyOn(global, "fetch").mockResolvedValue(
      new Response(JSON.stringify({})),
    );
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

  it("serializes variables into body", async () => {
    const fetchSpy = vi
      .spyOn(global, "fetch")
      .mockResolvedValue(new Response(JSON.stringify({ data: { health: "ok" } })));
    await gqlFetch(HealthQuery, { variables: { foo: "bar" } });
    const body = JSON.parse((fetchSpy.mock.calls[0]?.[1] as RequestInit).body as string);
    expect(body.variables).toEqual({ foo: "bar" });
  });
});

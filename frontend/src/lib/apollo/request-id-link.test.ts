import { ApolloClient, ApolloLink, gql, InMemoryCache, Observable } from "@apollo/client";
import { describe, expect, it } from "vitest";
import { requestIdLink } from "./request-id-link";

const query = gql`
  query Health {
    health
  }
`;

const UUID_V7_REGEX = /^[0-9a-f]{8}-[0-9a-f]{4}-7[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/;

/**
 * Build a composed link and return an accessor for the context captured by the
 * terminal link. Apollo Client v4 requires an ApolloClient instance in the
 * execute context, so we spin up a minimal one using the composed link itself.
 */
function captureContext(): {
  readonly ctx: { headers?: Record<string, string> };
  client: ApolloClient;
} {
  let captured: { headers?: Record<string, string> } = {};

  const terminal = new ApolloLink((op) => {
    captured = op.getContext() as { headers?: Record<string, string> };
    return new Observable((subscriber) => {
      subscriber.next({ data: { health: "ok" } });
      subscriber.complete();
    });
  });

  const link = ApolloLink.from([requestIdLink, terminal]);

  const client = new ApolloClient({
    cache: new InMemoryCache(),
    link,
  });

  return {
    get ctx() {
      return captured;
    },
    client,
  };
}

function runQuery(client: ApolloClient, context: Record<string, unknown> = {}): Promise<void> {
  return new Promise((resolve, reject) => {
    client
      .query({ query, context, fetchPolicy: "no-cache" })
      .then(() => resolve())
      .catch(reject);
  });
}

describe("requestIdLink", () => {
  it("injects X-Request-ID when header is absent", async () => {
    const spy = captureContext();
    await runQuery(spy.client);

    const requestId = spy.ctx.headers?.["X-Request-ID"];
    expect(requestId).toBeDefined();
    expect(requestId).toMatch(UUID_V7_REGEX);
  });

  it("preserves X-Request-ID when already set (exact case)", async () => {
    const spy = captureContext();
    await runQuery(spy.client, { headers: { "X-Request-ID": "preset-abc" } });

    expect(spy.ctx.headers?.["X-Request-ID"]).toBe("preset-abc");
  });

  it("does not inject a new X-Request-ID when lowercase x-request-id is already set", async () => {
    const spy = captureContext();
    await runQuery(spy.client, { headers: { "x-request-id": "preset-xyz" } });

    // The case-insensitive guard must prevent generating a new ID.
    // The original lowercase key is preserved as-is by setContext since no
    // new header object is merged over it.
    const headers = spy.ctx.headers ?? {};
    const allValues = Object.values(headers);
    // "preset-xyz" must appear somewhere in the headers — the original value is kept.
    expect(allValues).toContain("preset-xyz");
    // No freshly generated UUID v7 must be present on top of the preserved value.
    const freshIds = allValues.filter((v) => typeof v === "string" && UUID_V7_REGEX.test(v));
    expect(freshIds).toHaveLength(0);
  });
});

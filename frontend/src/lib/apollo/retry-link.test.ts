import { ApolloClient, ApolloLink, InMemoryCache, Observable } from "@apollo/client";
import { RetryLink } from "@apollo/client/link/retry";
import { parse } from "graphql";
import { describe, expect, it } from "vitest";
import { makeRetryLink } from "./retry-link";

const mutation = parse(`
  mutation RetryLinkDeleteCardgroup {
    deleteCardgroup(id: "cardgroup-1")
  }
`);

const query = parse(`
  query RetryLinkHealth {
    health
  }
`);

const client = new ApolloClient({
  cache: new InMemoryCache(),
  link: ApolloLink.empty(),
});

function executeOperation(link: ApolloLink, document: typeof query) {
  return new Promise<ApolloLink.Result | undefined>((resolve, reject) => {
    let result: ApolloLink.Result | undefined;
    ApolloLink.execute(link, { query: document }, { client }).subscribe({
      next: (value) => {
        result = value;
      },
      error: reject,
      complete: () => resolve(result),
    });
  });
}

function makeFastRetryLink() {
  // The shipped 300 ms backoff is not used here because timing it would slow the suite.
  return new RetryLink({
    delay: { initial: 1, max: 1, jitter: false },
    attempts: { max: 3 },
  });
}

describe("makeRetryLink", () => {
  it("does not retry a mutation after a transport failure", async () => {
    let invocations = 0;
    const terminal = new ApolloLink(
      () =>
        new Observable((subscriber) => {
          invocations += 1;
          subscriber.error(new Error("network error"));
        }),
    );

    const link = ApolloLink.from([makeRetryLink(), terminal]);

    await expect(executeOperation(link, mutation)).rejects.toThrow("network error");
    expect(invocations).toBe(1);
  });

  it("retries a query without exceeding three total attempts", async () => {
    let invocations = 0;
    const terminal = new ApolloLink(
      () =>
        new Observable((subscriber) => {
          invocations += 1;
          subscriber.error(new Error("network error"));
        }),
    );

    const link = ApolloLink.from([makeFastRetryLink(), terminal]);

    await expect(executeOperation(link, query)).rejects.toThrow("network error");
    expect(invocations).toBeGreaterThan(1);
    expect(invocations).toBeLessThanOrEqual(3);
  });

  it("resolves a query that succeeds on the second attempt", async () => {
    let invocations = 0;
    const result = { data: { health: "ok" } };
    const terminal = new ApolloLink(
      () =>
        new Observable((subscriber) => {
          invocations += 1;
          if (invocations === 1) {
            subscriber.error(new Error("network error"));
            return;
          }
          subscriber.next(result);
          subscriber.complete();
        }),
    );

    const link = ApolloLink.from([makeFastRetryLink(), terminal]);

    await expect(executeOperation(link, query)).resolves.toEqual(result);
    expect(invocations).toBe(2);
  });

  it("retries a query with the shipped retry predicate", async () => {
    let invocations = 0;
    const result = { data: { health: "ok" } };
    const terminal = new ApolloLink(
      () =>
        new Observable((subscriber) => {
          invocations += 1;
          if (invocations === 1) {
            subscriber.error(new Error("network error"));
            return;
          }
          subscriber.next(result);
          subscriber.complete();
        }),
    );

    const link = ApolloLink.from([makeRetryLink(), terminal]);

    await expect(executeOperation(link, query)).resolves.toEqual(result);
    expect(invocations).toBe(2);
  });
});

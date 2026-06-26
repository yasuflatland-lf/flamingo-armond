// @vitest-environment happy-dom
import { InMemoryCache } from "@apollo/client";
import type { MockedResponse } from "@apollo/client/testing";
import { MockedProvider } from "@apollo/client/testing/react";
import { act, renderHook } from "@testing-library/react";
import { GraphQLError } from "graphql";
import { createElement, type ReactNode } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { CARDGROUPS_DEFAULT_VARS } from "@/app/cardgroups/queries";
import {
  MyCardgroupsConnectionDocument,
  SeedDefaultStarterCardgroupsDocument,
} from "@/generated/graphql";
import type { SeedDefaultStartersOutcome } from "./use-seed-default-starters";
import { useSeedDefaultStarters } from "./use-seed-default-starters";

function cardgroupNode(id: string, name = "Deck A") {
  return {
    __typename: "Cardgroup" as const,
    id,
    name,
    updatedAt: "2024-01-01T00:00:00Z",
  };
}

function render(mocks: MockedResponse[], cache = new InMemoryCache()) {
  return renderHook(() => useSeedDefaultStarters(), {
    wrapper: ({ children }: { children: ReactNode }) =>
      createElement(MockedProvider, { mocks, cache }, children),
  });
}

function readConnection(cache: InMemoryCache) {
  return cache.readQuery({
    query: MyCardgroupsConnectionDocument,
    variables: CARDGROUPS_DEFAULT_VARS,
  });
}

afterEach(() => {
  vi.restoreAllMocks();
});

describe("useSeedDefaultStarters", () => {
  it("does not carry optimisticResponse on the seed mutation", () => {
    // Static-source guard: this mutation can fail with a typed/auth GraphQL error,
    // so it must never use optimisticResponse (Apollo does not roll those back
    // reliably for typed errors). See .claude/rules/pagination.md.
    expect(useSeedDefaultStarters.toString()).not.toContain("optimisticResponse");
  });

  it("returns { status: 'success', count } and prepends each cardgroup edge on success", async () => {
    const cg1 = cardgroupNode("cg-1", "English Basics");
    const cg2 = cardgroupNode("cg-2", "JLPT N4");
    const cache = new InMemoryCache();
    const mocks: MockedResponse[] = [
      {
        request: { query: SeedDefaultStarterCardgroupsDocument, variables: {} },
        result: {
          data: {
            seedDefaultStarterCardgroups: {
              __typename: "SeedDefaultStartersPayload",
              cardgroups: [cg1, cg2],
            },
          },
        },
      },
    ];
    const { result } = render(mocks, cache);

    let outcome: SeedDefaultStartersOutcome | undefined;
    await act(async () => {
      outcome = await result.current.seedDefaultStarters();
    });

    expect(outcome).toEqual({ status: "success", count: 2 });

    // Both cardgroup edges should be prepended to the myCardgroupsConnection cache.
    const connection = readConnection(cache);
    expect(connection).not.toBeNull();
    const edges = (connection as { myCardgroupsConnection: { edges: { node: { id: string } }[] } })
      ?.myCardgroupsConnection?.edges;
    expect(edges).toHaveLength(2);
    // Most-recently prepended edge is first — cg2 was processed after cg1 in the loop.
    expect(edges?.[0]?.node?.id).toBe("cg-2");
    expect(edges?.[1]?.node?.id).toBe("cg-1");
  });

  it("returns { status: 'success', count: 0 } when no default starters exist", async () => {
    const cache = new InMemoryCache();
    const mocks: MockedResponse[] = [
      {
        request: { query: SeedDefaultStarterCardgroupsDocument, variables: {} },
        result: {
          data: {
            seedDefaultStarterCardgroups: {
              __typename: "SeedDefaultStartersPayload",
              cardgroups: [],
            },
          },
        },
      },
    ];
    const { result } = render(mocks, cache);

    let outcome: SeedDefaultStartersOutcome | undefined;
    await act(async () => {
      outcome = await result.current.seedDefaultStarters();
    });

    expect(outcome).toEqual({ status: "success", count: 0 });
    // Cold cache — no write because the cardgroups array is empty.
    expect(readConnection(cache)).toBeNull();
  });

  it("returns { status: 'auth', kind: 'unauthenticated' } on UNAUTHENTICATED error", async () => {
    const mocks: MockedResponse[] = [
      {
        request: { query: SeedDefaultStarterCardgroupsDocument, variables: {} },
        result: {
          errors: [
            new GraphQLError("Unauthenticated", {
              extensions: { code: "UNAUTHENTICATED" },
            }),
          ],
        },
      },
    ];
    const { result } = render(mocks);

    let outcome: SeedDefaultStartersOutcome | undefined;
    await act(async () => {
      outcome = await result.current.seedDefaultStarters();
    });

    expect(outcome).toEqual({ status: "auth", kind: "unauthenticated" });
  });

  it("returns { status: 'auth', kind: 'forbidden' } on FORBIDDEN error", async () => {
    const mocks: MockedResponse[] = [
      {
        request: { query: SeedDefaultStarterCardgroupsDocument, variables: {} },
        result: {
          errors: [
            new GraphQLError("Forbidden", {
              extensions: { code: "FORBIDDEN" },
            }),
          ],
        },
      },
    ];
    const { result } = render(mocks);

    let outcome: SeedDefaultStartersOutcome | undefined;
    await act(async () => {
      outcome = await result.current.seedDefaultStarters();
    });

    expect(outcome).toEqual({ status: "auth", kind: "forbidden" });
  });

  it("returns { status: 'rejected' } on a transport error", async () => {
    const consoleWarnSpy = vi.spyOn(console, "warn").mockImplementation(() => {});
    const mocks: MockedResponse[] = [
      {
        request: { query: SeedDefaultStarterCardgroupsDocument, variables: {} },
        error: new Error("Network error"),
      },
    ];
    const { result } = render(mocks);

    let outcome: SeedDefaultStartersOutcome | undefined;
    await act(async () => {
      outcome = await result.current.seedDefaultStarters();
    });

    expect(outcome).toEqual({ status: "rejected" });
    expect(consoleWarnSpy).toHaveBeenCalledWith(
      expect.stringContaining("[useSeedDefaultStarters]"),
      expect.anything(),
    );
  });
});

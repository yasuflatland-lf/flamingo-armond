/**
 * Shared helpers for testing Relay-style Connection pagination with
 * `MockedProvider`. Two facts drive these helpers:
 *
 * 1. `MockedProvider` matches mocks by `(query, variables)` tuple and consumes
 *    each mock entry **once**. Therefore each `fetchMore` invocation needs its
 *    OWN mock entry whose variables include the cursor (`after` or `before`).
 *    Reusing a single mock entry for two page transitions silently returns
 *    `undefined` on the second call and the test passes on a stale cache.
 *
 * 2. When MockedProvider cannot match a request, it logs
 *    `"No more mocked responses for the query: <Op>"` via `console.warn`.
 *    `installApolloMockLeakSpy` captures those warnings so the caller can
 *    assert no leaks occurred — applying the "Spy on console.warn for
 *    MockedProvider leaks" rule from `docs/pagination/capture-mockedprovider-warn-leaks.md`. Failure
 *    requires an explicit call to `assertNoLeaks()`; the spy alone does not
 *    fail the test.
 *
 * Canonical usage:
 *
 * ```ts
 * const leak = installApolloMockLeakSpy();
 * afterEach(() => {
 *   leak.assertNoLeaks();
 *   leak.teardown();
 * });
 *
 * const mocks = buildPaginatedMocks({
 *   query: CardsByCardgroupConnectionDocument,
 *   initialVariables: { cardgroupId, first: 20 },
 *   pages: [
 *     { result: { data: page1Data }, nextVariables: { cardgroupId, first: 20, after: "c-20" } },
 *     { result: { data: page2Data } },
 *   ],
 * });
 * ```
 *
 * The first entry of `pages` is the response to the initial query; each
 * subsequent entry is one `fetchMore` page transition. Each entry's
 * `nextVariables` (when present) becomes the `request.variables` of the NEXT
 * mock entry — i.e. the variables Apollo issues on the next `fetchMore`.
 * `buildPaginatedMocks` throws synchronously when a non-terminal page is
 * missing `nextVariables`.
 */

import type { DocumentNode, OperationVariables, TypedDocumentNode } from "@apollo/client";
import type { MockedResponse } from "@apollo/client/testing";
import { vi } from "vitest";

/**
 * One page in a paginated fixture.
 *
 * - `result` — the response MockedProvider should return for this page. Either
 *   `{ data }` or `{ errors }` (or both). May be a function, mirroring
 *   `MockedResponse["result"]`.
 * - `nextVariables` — the variables MockedProvider should expect on the NEXT
 *   fetchMore call. Omit on the terminal page (no further fetchMore).
 * - `delay` — optional artificial latency, useful for in-flight-guard tests.
 */
export type PaginatedPage<TData, TVariables extends OperationVariables> = {
  result: MockedResponse<TData, TVariables>["result"];
  nextVariables?: TVariables;
  delay?: number;
};

export type BuildPaginatedMocksOptions<TData, TVariables extends OperationVariables> = {
  query: DocumentNode | TypedDocumentNode<TData, TVariables>;
  initialVariables: TVariables;
  pages: PaginatedPage<TData, TVariables>[];
};

/**
 * Build a `MockedResponse[]` for a paginated query. The first page uses
 * `initialVariables`; each subsequent page uses the previous page's
 * `nextVariables`. A page without `nextVariables` is treated as terminal —
 * no further mocks are emitted.
 *
 * Throws synchronously if a non-terminal page is missing `nextVariables`,
 * which is almost always a fixture-construction bug (a fetchMore without a
 * cursor variables object would never match).
 */
export function buildPaginatedMocks<TData, TVariables extends OperationVariables>(
  opts: BuildPaginatedMocksOptions<TData, TVariables>,
): MockedResponse<TData, TVariables>[] {
  const { query, initialVariables, pages } = opts;
  if (pages.length === 0) {
    return [];
  }

  const mocks: MockedResponse<TData, TVariables>[] = [];
  let currentVariables: TVariables = initialVariables;

  for (const [i, page] of pages.entries()) {
    const isTerminal = i === pages.length - 1;

    const entry: MockedResponse<TData, TVariables> = {
      request: { query, variables: currentVariables },
      result: page.result,
    };
    if (page.delay !== undefined) {
      entry.delay = page.delay;
    }
    mocks.push(entry);

    if (!isTerminal) {
      if (!page.nextVariables) {
        throw new Error(
          `buildPaginatedMocks: page[${i}] is non-terminal but has no nextVariables; ` +
            "every fetchMore needs the cursor variables of the next request.",
        );
      }
      currentVariables = page.nextVariables;
    }
  }

  return mocks;
}

/**
 * The exact substring MockedProvider logs for an unmatched request.
 * Exposed for tests that want to assert their own targeted match logic.
 */
export const APOLLO_MOCK_LEAK_NEEDLE = "No more mocked responses for the query";

export type ApolloMockLeakSpyOptions = {
  /**
   * Restrict the leak detector to a single operation (or a list) — useful when
   * a test deliberately exercises an unmatched fallback for some other query.
   * Match is substring-based on each `console.warn` argument.
   */
  operationNames?: string[];
  /**
   * If `true`, the spy still swallows the warning so it does not pollute test
   * output. Default `true`. Set to `false` to also surface the warning to the
   * real console for local debugging.
   */
  silent?: boolean;
};

export type ApolloMockLeakSpyResult = {
  /** Tear down the spy (e.g. in `afterEach`). Idempotent. */
  teardown: () => void;
  /**
   * Assertion helper. Throws (failing the test) if any unmatched-mock warning
   * matching `operationNames` was emitted since install. Tests that prefer
   * `expect(...).toEqual([])` can introspect `getLeakedCalls()` instead.
   */
  assertNoLeaks: () => void;
  /**
   * Returns the `console.warn` calls that look like MockedProvider leak
   * warnings (filtered by `operationNames` when provided).
   */
  getLeakedCalls: () => unknown[][];
};

/**
 * Install a `console.warn` spy that records calls matching the leak needle
 * `"No more mocked responses for the query"` — emitted by MockedProvider when
 * a request leaked past the in-flight guard or a fixture forgot to mock a
 * fetchMore page.
 *
 * Returns `{ teardown, assertNoLeaks, getLeakedCalls }`. The spy alone does
 * NOT fail the test; callers must invoke `assertNoLeaks()` (which throws if
 * any matching warnings were captured) — typically in `afterEach` before
 * `teardown()`. Restoring the original `console.warn` is the responsibility
 * of the caller via `teardown`.
 */
export function installApolloMockLeakSpy(
  options: ApolloMockLeakSpyOptions = {},
): ApolloMockLeakSpyResult {
  const { operationNames, silent = true } = options;
  const originalWarn = console.warn;
  const spy = vi.spyOn(console, "warn").mockImplementation((...args: unknown[]) => {
    if (!silent) {
      originalWarn.apply(console, args as Parameters<typeof console.warn>);
    }
  });

  const matchesOperation = (args: unknown[]): boolean => {
    if (!operationNames || operationNames.length === 0) return true;
    return args.some((a) => typeof a === "string" && operationNames.some((op) => a.includes(op)));
  };

  const isLeak = (args: unknown[]): boolean =>
    args.some((a) => typeof a === "string" && a.includes(APOLLO_MOCK_LEAK_NEEDLE)) &&
    matchesOperation(args);

  const getLeakedCalls = (): unknown[][] =>
    spy.mock.calls.filter((args) => isLeak(args as unknown[])) as unknown[][];

  const assertNoLeaks = (): void => {
    const leaks = getLeakedCalls();
    if (leaks.length > 0) {
      const formatted = leaks
        .map((args) => args.map((a) => (typeof a === "string" ? a : JSON.stringify(a))).join(" "))
        .join("\n");
      throw new Error(
        `Apollo MockedProvider emitted ${leaks.length} unmatched-mock warning(s):\n${formatted}`,
      );
    }
  };

  const teardown = (): void => {
    spy.mockRestore();
  };

  return { teardown, assertNoLeaks, getLeakedCalls };
}

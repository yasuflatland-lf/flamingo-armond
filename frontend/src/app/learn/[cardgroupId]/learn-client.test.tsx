// @vitest-environment jsdom
import { gql, InMemoryCache } from "@apollo/client";
import { MockedProvider } from "@apollo/client/testing/react";
import { act, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { GraphQLError } from "graphql";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { HandleSwipeDocument, SetLastViewedCardgroupDocument } from "@/generated/graphql";
import {
  type ApolloMockLeakSpyResult,
  installApolloMockLeakSpy,
} from "../../../../__tests__/utils/mock-apollo-paginated";
import { LearnClient } from "./learn-client";

// ---------------------------------------------------------------------------
// SwipeCardStack mock — captures the `onCardSwiped` reference on every render
// so the identity-stability test can assert the callback is not re-created
// after a swipe triggers a queue state update.
//
// `vi.mock` is hoisted before imports by Vite, so the factory runs first.
// The captured array is populated at render time and is cleared in `beforeEach`
// so leakage between tests is impossible.
// ---------------------------------------------------------------------------
type SwipeCardStackOnCardSwiped = Parameters<
  typeof import("@/components/learn/swipe-card-stack")["SwipeCardStack"]
>[0]["onCardSwiped"];

const capturedOnCardSwiped: SwipeCardStackOnCardSwiped[] = [];

vi.mock("@/components/learn/swipe-card-stack", () => ({
  SwipeCardStack: (props: {
    cards: { id: string; front: string; back: string }[];
    onCardSwiped: SwipeCardStackOnCardSwiped;
    completedCount?: number;
  }) => {
    capturedOnCardSwiped.push(props.onCardSwiped);
    // Render minimal UI so existing tests that assert on card text or swipe
    // buttons continue to work. The `next/dynamic` AnimatedCard does not render
    // in jsdom (ssr:false), so the full SwipeCardStack cannot be used as-is.
    const activeCard = props.cards[0];
    if (!activeCard) {
      return (
        <div>
          <p>Session complete</p>
          {props.completedCount != null && props.completedCount > 0 && (
            <p>
              You reviewed {props.completedCount} {props.completedCount === 1 ? "card" : "cards"} in
              this batch.
            </p>
          )}
        </div>
      );
    }
    return (
      <div>
        <p>{activeCard.front}</p>
        <button type="button" onClick={() => props.onCardSwiped(activeCard as never, "left")}>
          Again
        </button>
        <button type="button" onClick={() => props.onCardSwiped(activeCard as never, "down")}>
          Hard
        </button>
        <button type="button" onClick={() => props.onCardSwiped(activeCard as never, "right")}>
          Easy
        </button>
      </div>
    );
  },
}));

// ---------------------------------------------------------------------------
// File-wide MockedProvider leak spy.
//
// Installed as the OUTERMOST `console.warn` spy (top-level `beforeEach` runs
// before any describe-level `beforeEach`), and torn down LAST in the matching
// `afterEach`. Per docs/pagination/capture-mockedprovider-warn-leaks.md § "Spy stacking: install order is outer-first,
// teardown is LIFO", any per-describe `console.warn` spy installed below
// stacks on top and MUST NOT call `mockImplementation(() => {})` — that would
// swallow the leak warning before the leak spy records it.
// ---------------------------------------------------------------------------
let leakSpy: ApolloMockLeakSpyResult;

beforeEach(() => {
  leakSpy = installApolloMockLeakSpy({
    operationNames: ["HandleSwipe", "SetLastViewedCardgroup"],
  });
  // Reset the captured onCardSwiped array so tests do not bleed into each other.
  capturedOnCardSwiped.length = 0;
});

afterEach(() => {
  leakSpy.assertNoLeaks();
  leakSpy.teardown();
});

const CG_ID = "cg-1";
const CG_NAME = "Spanish Basics";

const CARD_1 = {
  __typename: "Card" as const,
  id: "c-1",
  front: "Hello",
  back: "Hola",
  due: "2026-04-30T00:00:00Z",
  state: 0,
  cardgroupId: CG_ID,
};

const CARD_2 = {
  __typename: "Card" as const,
  id: "c-2",
  front: "Bye",
  back: "Adios",
  due: "2026-04-30T00:00:00Z",
  state: 0,
  cardgroupId: CG_ID,
};

const SERVER_CARD = {
  __typename: "Card" as const,
  id: "c-3",
  front: "Server next",
  back: "Siguiente",
  due: "2026-04-30T00:00:00Z",
  state: 1,
  cardgroupId: CG_ID,
};

const DEFAULT_METRICS = {
  __typename: "PerformanceMetrics" as const,
  successRate: 0.5,
  avgDifficulty: 0.5,
  retentionRate: 0.5,
  studyStreak: 0,
  lapseRate: 0,
  reviewCount: 1,
};

function renderLearnClient(mocks: unknown[], initialCards = [CARD_1]) {
  // Pass `lastViewedCardgroupId === CG_ID` so the persist-last-viewed effect
  // short-circuits before issuing a mutation; that mutation is exercised in
  // its own test below and would otherwise need a mock entry in every case.
  render(
    <MockedProvider mocks={mocks as never}>
      <LearnClient
        cardgroupId={CG_ID}
        cardgroupName={CG_NAME}
        initialCards={initialCards}
        lastViewedCardgroupId={CG_ID}
      />
    </MockedProvider>,
  );
}

function makeSwipeMock(mode: 1 | 2 | 4, nextCards: (typeof CARD_1)[] = []) {
  let called = false;
  return {
    mock: {
      request: {
        query: HandleSwipeDocument,
        variables: { input: { cardId: CARD_1.id, cardgroupId: CG_ID, mode } },
      },
      result: () => {
        called = true;
        return {
          data: {
            handleSwipe: {
              __typename: "SwipeResponse" as const,
              nextCards,
              performanceMode: 0,
              metrics: DEFAULT_METRICS,
            },
          },
        };
      },
    },
    wasCalled: () => called,
  };
}

describe("<LearnClient>", () => {
  it.each([
    ["Again", 1],
    ["Hard", 2],
    ["Easy", 4],
  ] as const)("maps %s to mode %d", async (label, mode) => {
    const user = userEvent.setup();
    const swipe = makeSwipeMock(mode);
    renderLearnClient([swipe.mock]);

    await user.click(screen.getByRole("button", { name: label }));

    await waitFor(() => {
      expect(swipe.wasCalled()).toBe(true);
    });
  });

  it("updates the queue optimistically then reconciles with server nextCards", async () => {
    const user = userEvent.setup();
    const swipe = makeSwipeMock(4, [SERVER_CARD]);
    renderLearnClient([swipe.mock], [CARD_1, CARD_2]);

    await user.click(screen.getByRole("button", { name: "Easy" }));

    expect(screen.queryByText("Hello")).not.toBeInTheDocument();

    await waitFor(() => {
      expect(screen.getByText("Server next")).toBeInTheDocument();
    });
    expect(screen.queryByText("Bye")).not.toBeInTheDocument();
  });

  it("renders the Session-complete count line after the queue empties", async () => {
    const user = userEvent.setup();
    const swipe = makeSwipeMock(4, []);
    renderLearnClient([swipe.mock], [CARD_1]);

    await user.click(screen.getByRole("button", { name: "Easy" }));

    await waitFor(() => {
      expect(screen.getByText("Session complete")).toBeInTheDocument();
    });
    expect(screen.getByText("You reviewed 1 card in this batch.")).toBeInTheDocument();
  });

  it("rolls back the card and shows an error when handleSwipe fails", async () => {
    const user = userEvent.setup();
    const consoleErrorSpy = vi.spyOn(console, "error").mockImplementation(() => {});
    const mock = {
      request: {
        query: HandleSwipeDocument,
        variables: { input: { cardId: CARD_1.id, cardgroupId: CG_ID, mode: 1 } },
      },
      result: {
        errors: [new GraphQLError("bad swipe", { extensions: { code: "BAD_USER_INPUT" } })],
      },
    };
    renderLearnClient([mock], [CARD_1]);

    await user.click(screen.getByRole("button", { name: "Again" }));

    await waitFor(() => {
      expect(screen.getByText("Hello")).toBeInTheDocument();
      expect(screen.getByRole("alert")).toHaveTextContent("Could not save that swipe");
    });

    // PII redaction contract — frontend-rsc-error-handling.md
    // § "Redact `err.message` from structured `console` payloads".
    const errorCall = consoleErrorSpy.mock.calls.find(
      (call) => call[0] === "[LearnClient] handleSwipe rejected",
    );
    expect(errorCall).toBeDefined();
    const payload = errorCall?.[1];
    expect(payload).toMatchObject({
      cardId: expect.any(String),
      cardgroupId: CG_ID,
      name: expect.any(String),
    });
    expect(payload).not.toHaveProperty("message");

    consoleErrorSpy.mockRestore();
  });

  describe("handleSwipe resolved-without-data branch", () => {
    // Forwarding spy: do NOT call `mockImplementation(() => {})` here. Per
    // docs/pagination/capture-mockedprovider-warn-leaks.md § "Spy stacking": this spy is the OUTER spy (installed after
    // the file-wide leak spy) and must forward every `console.warn` call through
    // to the underlying leak spy so MockedProvider leaks are still recorded.
    let consoleWarnSpy: ReturnType<typeof vi.spyOn>;

    beforeEach(() => {
      consoleWarnSpy = vi.spyOn(console, "warn");
    });

    afterEach(() => {
      consoleWarnSpy.mockRestore();
    });

    it("emits a console.warn with cardId and cardgroupId when handleSwipe resolves with null data", async () => {
      const user = userEvent.setup();
      const mock = {
        request: {
          query: HandleSwipeDocument,
          variables: { input: { cardId: CARD_1.id, cardgroupId: CG_ID, mode: 4 } },
        },
        result: {
          data: {
            handleSwipe: null,
          },
        },
      };
      renderLearnClient([mock], [CARD_1]);

      await user.click(screen.getByRole("button", { name: "Easy" }));

      await waitFor(() => {
        expect(consoleWarnSpy).toHaveBeenCalledWith(
          "[LearnClient] handleSwipe resolved without data",
          expect.objectContaining({
            cardId: expect.any(String),
            cardgroupId: CG_ID,
          }),
        );
      });
    });
  });

  it("renders an empty-card state with a manage cards link", () => {
    renderLearnClient([], []);

    expect(screen.getByText("No cards to learn")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Manage cards" })).toHaveAttribute(
      "href",
      `/cardgroups/${CG_ID}/cards`,
    );
  });

  it("renders the floating plus button in the empty-card state", () => {
    renderLearnClient([], []);

    expect(screen.getByRole("link", { name: `Add a new card to ${CG_NAME}` })).toBeInTheDocument();
  });

  it("renders the floating plus button with the correct aria-label", () => {
    renderLearnClient([]);

    expect(screen.getByRole("link", { name: `Add a new card to ${CG_NAME}` })).toBeInTheDocument();
  });

  it("floating plus button href points to the new-card form with cardgroup and return params", () => {
    renderLearnClient([]);

    expect(screen.getByRole("link", { name: `Add a new card to ${CG_NAME}` })).toHaveAttribute(
      "href",
      `/cards/new?cardgroup=${CG_ID}&return=/learn/${CG_ID}`,
    );
  });

  it("floating plus button aria-label embeds the cardgroup name", () => {
    renderLearnClient([]);

    const link = screen.getByRole("link", { name: `Add a new card to ${CG_NAME}` });
    expect(link).toHaveAccessibleName(`Add a new card to ${CG_NAME}`);
  });
});

// ---------------------------------------------------------------------------
// Persist-last-viewed path
// ---------------------------------------------------------------------------

const USER_ID = "u-1";

/** Builds a SetLastViewedCardgroup mock that tracks whether it was called. */
function makePersistMock(cardgroupId: string, onCalled?: () => void) {
  return {
    request: {
      query: SetLastViewedCardgroupDocument,
      variables: { cardgroupId },
    },
    result: () => {
      onCalled?.();
      return {
        data: {
          setLastViewedCardgroup: {
            __typename: "User" as const,
            id: USER_ID,
            lastViewedCardgroup: {
              __typename: "Cardgroup" as const,
              id: cardgroupId,
            },
          },
        },
      };
    },
  };
}

describe("<LearnClient> persist-last-viewed path", () => {
  // Forwarding spy: do NOT call `mockImplementation(() => {})` here. Per
  // docs/pagination/capture-mockedprovider-warn-leaks.md § "Spy stacking": this spy is the OUTER spy (installed after
  // the file-wide leak spy) and must forward every `console.warn` call through
  // to the underlying leak spy so MockedProvider leaks are still recorded.
  // Tests that expect a `[learn] setLastViewedCardgroup failed` warn assert
  // it explicitly via `toHaveBeenCalledWith` below. LIFO teardown is preserved
  // automatically: this describe-scoped `afterEach` runs before the file-wide
  // `afterEach`, so `consoleWarnSpy` restores first, then the leak spy.
  let consoleWarnSpy: ReturnType<typeof vi.spyOn>;

  beforeEach(() => {
    consoleWarnSpy = vi.spyOn(console, "warn");
  });

  afterEach(() => {
    consoleWarnSpy.mockRestore();
  });

  it("fires SetLastViewedCardgroup mutation when ids differ", async () => {
    const mutationCalled = vi.fn();
    render(
      <MockedProvider mocks={[makePersistMock(CG_ID, mutationCalled)]}>
        <LearnClient
          cardgroupId={CG_ID}
          cardgroupName={CG_NAME}
          initialCards={[CARD_1]}
          lastViewedCardgroupId="cg-other"
        />
      </MockedProvider>,
    );

    await waitFor(() => {
      expect(mutationCalled).toHaveBeenCalledOnce();
    });
  });

  it("writes lastViewedCardgroup into the Apollo cache after mutation resolves", async () => {
    const cache = new InMemoryCache();

    render(
      <MockedProvider mocks={[makePersistMock(CG_ID)]} cache={cache}>
        <LearnClient
          cardgroupId={CG_ID}
          cardgroupName={CG_NAME}
          initialCards={[CARD_1]}
          lastViewedCardgroupId="cg-other"
        />
      </MockedProvider>,
    );

    const LastViewedFragment = gql`
      fragment LastViewedCheck on User {
        lastViewedCardgroup {
          id
        }
      }
    `;

    await waitFor(() => {
      const cached = cache.readFragment<{ lastViewedCardgroup: { id: string } | null }>({
        id: `User:${USER_ID}`,
        fragment: LastViewedFragment,
      });
      expect(cached?.lastViewedCardgroup?.id).toBe(CG_ID);
    });
  });

  it.each([
    [
      "a GraphQLError",
      {
        request: { query: SetLastViewedCardgroupDocument, variables: { cardgroupId: CG_ID } },
        result: {
          errors: [new GraphQLError("forbidden", { extensions: { code: "BAD_USER_INPUT" } })],
        },
      },
    ],
    [
      "a network error",
      {
        request: { query: SetLastViewedCardgroupDocument, variables: { cardgroupId: CG_ID } },
        error: new Error("network failure"),
      },
    ],
  ] as const)("swallows %s from the persist mutation without throwing", async (_, mockEntry) => {
    render(
      <MockedProvider mocks={[mockEntry]}>
        <LearnClient
          cardgroupId={CG_ID}
          cardgroupName={CG_NAME}
          initialCards={[CARD_1]}
          lastViewedCardgroupId="cg-other"
        />
      </MockedProvider>,
    );

    await waitFor(() => {
      expect(consoleWarnSpy).toHaveBeenCalledWith(
        "[learn] setLastViewedCardgroup failed",
        expect.objectContaining({ cardgroupId: CG_ID }),
      );
    });
    // Component must still render the card stack — no crash.
    expect(screen.getByText("Hello")).toBeInTheDocument();

    // PII redaction contract — frontend-rsc-error-handling.md
    // § "Redact `err.message` from structured `console` payloads".
    const warnCall = consoleWarnSpy.mock.calls.find(
      (call: unknown[]) => call[0] === "[learn] setLastViewedCardgroup failed",
    );
    expect(warnCall).toBeDefined();
    const payload = warnCall?.[1];
    expect(payload).toMatchObject({
      cardgroupId: CG_ID,
      name: expect.any(String),
    });
    expect(payload).not.toHaveProperty("message");
  });

  it("does not carry optimisticResponse in the persist mutation", () => {
    // Static assertion: the source of the LearnClient function must not include
    // `optimisticResponse` inside the SetLastViewedCardgroup mutate call.
    // The comment block in learn-client.tsx explains why — typed errors from
    // @apollo/client v3.x are not reliably rolled back from optimistic writes
    // (see docs/pagination/drop-optimistic-response-typed-errors.md).
    //
    // Strategy: find the section of source between `SetLastViewedCardgroup` and
    // the next `.catch(` that follows it, and assert no `optimisticResponse`
    // key appears there. `handleSwipe` is the only call that legitimately
    // uses `optimisticResponse` and it appears earlier in the source.
    const source = LearnClient.toString();

    const persistStart = source.indexOf("SetLastViewedCardgroup");
    expect(persistStart).toBeGreaterThan(-1);

    // Find the .catch( that closes the persist mutation chain.
    const persistCatchIdx = source.indexOf(".catch(", persistStart);
    expect(persistCatchIdx).toBeGreaterThan(-1);

    const persistBlock = source.slice(persistStart, persistCatchIdx);
    expect(persistBlock).not.toContain("optimisticResponse");
  });
});

// ---------------------------------------------------------------------------
// onSwipe identity-stability test
//
// Asserts that the `onSwipe` callback passed to SwipeCardStack as `onCardSwiped`
// keeps the same reference across re-renders caused by queue state updates.
//
// Regression guard for commit f293612 which replaced `queue` in the
// `useCallback` dependency array with a `queueRef`. Reverting that commit
// (putting `queue` back as a dep) would cause `onSwipe` to be re-created on
// every swipe and this test would fail: `capturedOnCardSwiped[0]` and
// `capturedOnCardSwiped[1]` would be different function objects.
//
// The SwipeCardStack module mock at the top of this file captures the
// `onCardSwiped` reference on every render into `capturedOnCardSwiped`. The
// test triggers a swipe by calling the captured callback directly — no button
// interaction needed, which keeps the test immune to changes in SwipeCardStack's
// internal UI structure (the `next/dynamic` AnimatedCard rendering, etc.).
// ---------------------------------------------------------------------------

describe("<LearnClient> onSwipe identity stability", () => {
  it("passes the same onCardSwiped reference to SwipeCardStack after a swipe re-renders the parent", async () => {
    const swipeMock = {
      request: {
        query: HandleSwipeDocument,
        variables: { input: { cardId: CARD_1.id, cardgroupId: CG_ID, mode: 4 } },
      },
      result: {
        data: {
          handleSwipe: {
            __typename: "SwipeResponse" as const,
            nextCards: [CARD_2],
            performanceMode: 0,
            metrics: DEFAULT_METRICS,
          },
        },
      },
    };

    render(
      <MockedProvider mocks={[swipeMock]}>
        <LearnClient
          cardgroupId={CG_ID}
          cardgroupName={CG_NAME}
          initialCards={[CARD_1, CARD_2]}
          lastViewedCardgroupId={CG_ID}
        />
      </MockedProvider>,
    );

    // The first render must have pushed a callback reference.
    expect(capturedOnCardSwiped.length).toBeGreaterThanOrEqual(1);
    // capturedOnCardSwiped[0] is always defined here: the expect above would
    // have thrown if the array were empty. `as` cast satisfies noUncheckedIndexedAccess.
    const firstRef = capturedOnCardSwiped[0] as SwipeCardStackOnCardSwiped;

    // Trigger a swipe by calling the captured callback directly. This causes
    // `setQueue` inside onSwipe to fire, which re-renders LearnClient, which
    // passes `onCardSwiped` to the mock again — capturing a second reference.
    await act(async () => {
      await firstRef(CARD_1, "right");
    });

    // Wait until the re-render caused by the swipe state updates has produced
    // a second capture. Because the mock SwipeCardStack runs synchronously on
    // each render, `capturedOnCardSwiped` accumulates one entry per render.
    await waitFor(() => {
      expect(capturedOnCardSwiped.length).toBeGreaterThanOrEqual(2);
    });

    const secondRef = capturedOnCardSwiped[capturedOnCardSwiped.length - 1];

    // The core assertion: onSwipe must be the same function object across
    // re-renders. If `queue` were in the useCallback dep array (the pre-f293612
    // state), every queue state update would produce a new function and this
    // assertion would fail.
    expect(Object.is(firstRef, secondRef)).toBe(true);
  });
});

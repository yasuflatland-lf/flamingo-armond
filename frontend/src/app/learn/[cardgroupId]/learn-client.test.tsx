// @vitest-environment jsdom
import { gql, InMemoryCache } from "@apollo/client";
import { MockedProvider } from "@apollo/client/testing/react";
import { act, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { GraphQLError } from "graphql";
import { type RefObject, useImperativeHandle, useRef } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { LEARN_PAGE_LIMIT } from "@/app/learn/queries";
import type { SwipeCardData } from "@/components/learn/swipe-card";
import type { SwipeCardStackHandle } from "@/components/learn/swipe-card-stack";
import {
  HandleSwipeDocument,
  LearnNextDueCardsDocument,
  SetLastViewedCardgroupDocument,
} from "@/generated/graphql";
import {
  type ApolloMockLeakSpyResult,
  installApolloMockLeakSpy,
} from "../../../../__tests__/utils/mock-apollo-paginated";
import { LearnClient, PREFETCH_THRESHOLD } from "./learn-client";

// ---------------------------------------------------------------------------
// SwipeCardStack mock
//
// React 19 passes refs as plain props, so the mock accepts a `ref` prop and
// wires it to useImperativeHandle. triggerSwipe calls onCardSwiped with the
// first card in the `cards` array — enough for LearnClient integration tests.
//
// swipeDirection / swipeProgress are NOT in the mock props because LearnClient
// no longer owns overlay state (SwipeCardStack owns it — see swipe-card-stack.tsx).
//
// capturedOnCardSwiped accumulates the onCardSwiped reference on every render
// so the identity-stability test can assert it does not change across re-renders.
//
// capturedCardSnapshots accumulates the full `cards` array on every render so
// dedup tests can assert the exact queue contents without swipe-exhausting the
// stack. Reset in beforeEach alongside capturedOnCardSwiped.
// ---------------------------------------------------------------------------
type SwipeCardStackOnCardSwiped = Parameters<
  typeof import("@/components/learn/swipe-card-stack")["SwipeCardStack"]
>[0]["onCardSwiped"];

const capturedOnCardSwiped: SwipeCardStackOnCardSwiped[] = [];
const capturedCardSnapshots: SwipeCardData[][] = [];

vi.mock("@/components/learn/swipe-card-stack", () => ({
  SwipeCardStack: (props: {
    cards: SwipeCardData[];
    onCardSwiped: SwipeCardStackOnCardSwiped;
    completedCount?: number;
    ref?: RefObject<SwipeCardStackHandle | null>;
  }) => {
    capturedOnCardSwiped.push(props.onCardSwiped);
    capturedCardSnapshots.push([...props.cards]);

    const activeCardRef = useRef(props.cards[0] ?? null);
    activeCardRef.current = props.cards[0] ?? null;

    useImperativeHandle(props.ref, () => ({
      triggerSwipe: (direction: "left" | "right" | "down") => {
        const card = activeCardRef.current;
        if (card) props.onCardSwiped(card, direction);
      },
    }));

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
      </div>
    );
  },
}));

// ---------------------------------------------------------------------------
// LearnActionBar mock — exposes the onRate callback and disabled state for testing.
// ---------------------------------------------------------------------------
vi.mock("@/components/learn/learn-action-bar", () => ({
  LearnActionBar: (props: {
    onRate: (d: "left" | "down" | "right") => void;
    disabled: boolean;
  }) => (
    <div data-testid="learn-action-bar" data-disabled={String(props.disabled)}>
      <button type="button" onClick={() => props.onRate("left")} disabled={props.disabled}>
        Rate as Again
      </button>
      <button type="button" onClick={() => props.onRate("down")} disabled={props.disabled}>
        Rate as Hard
      </button>
      <button type="button" onClick={() => props.onRate("right")} disabled={props.disabled}>
        Rate as Easy
      </button>
    </div>
  ),
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
  vi.useRealTimers();
  leakSpy = installApolloMockLeakSpy({
    operationNames: ["HandleSwipe", "SetLastViewedCardgroup", "LearnNextDueCards"],
  });
  // Reset the captured arrays so tests do not bleed into each other.
  capturedOnCardSwiped.length = 0;
  capturedCardSnapshots.length = 0;
});

afterEach(() => {
  vi.useRealTimers();
  leakSpy.assertNoLeaks();
  leakSpy.teardown();
});

const CG_ID = "cg-1";

const userCardState = (due: string, state: number) => ({
  __typename: "UserCardState" as const,
  due,
  state,
});

const CARD_1 = {
  __typename: "Card" as const,
  id: "c-1",
  front: "Hello",
  back: "Hola",
  userCardState: userCardState("2026-04-30T00:00:00Z", 0),
  cardgroupId: CG_ID,
};

const CARD_2 = {
  __typename: "Card" as const,
  id: "c-2",
  front: "Bye",
  back: "Adios",
  userCardState: userCardState("2026-04-30T00:00:00Z", 0),
  cardgroupId: CG_ID,
};

const SERVER_CARD = {
  __typename: "Card" as const,
  id: "c-3",
  front: "Server next",
  back: "Siguiente",
  userCardState: userCardState("2026-04-30T00:00:00Z", 1),
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

/**
 * Default `LearnNextDueCards` mocks returning `[]`.
 *
 * The background prefetch effect in `LearnClient` fires `LearnNextDueCards`
 * whenever `queue.length` is between 1 and `PREFETCH_THRESHOLD` inclusive.
 * Tests outside the dedicated `queue prefetch` describe do not exercise the
 * prefetch behaviour intentionally, but they DO render queues short enough to
 * trigger the effect — without these no-op mocks, the file-wide leak spy
 * (which now includes `LearnNextDueCards` in `operationNames`) would record
 * an unmatched-mock warning and fail the test.
 *
 * The effect can re-fire after each `queue.length` transition (initial mount,
 * post-swipe optimistic shrink, server reconciliation), and `MockedProvider`
 * consumes each mock entry once. Returning a generous count of empty-result
 * entries covers every reasonable test without requiring per-test bookkeeping;
 * leftover entries that are never matched produce no warning.
 */
function makeDefaultPrefetchMocks(count = 4) {
  return Array.from({ length: count }, () => ({
    request: {
      query: LearnNextDueCardsDocument,
      variables: { cardgroupId: CG_ID, limit: LEARN_PAGE_LIMIT },
    },
    result: { data: { learnNextDueCards: [] } },
  }));
}

type RenderLearnClientOptions = {
  /**
   * When `true`, do NOT append default `LearnNextDueCards` no-op mocks. Used
   * by the dedicated `queue prefetch` describe block, whose tests supply their
   * own prefetch mocks and rely on unmatched-request leak detection to assert
   * the effect did or did not fire.
   */
  skipDefaultPrefetchMocks?: boolean;
};

function renderLearnClient(
  mocks: unknown[],
  initialCards = [CARD_1],
  options: RenderLearnClientOptions = {},
) {
  // Pass `lastViewedCardgroupId === CG_ID` so the persist-last-viewed effect
  // short-circuits before issuing a mutation; that mutation is exercised in
  // its own test below and would otherwise need a mock entry in every case.
  //
  // Append default no-op `LearnNextDueCards` mocks so the prefetch effect is
  // satisfied for any test whose initial queue length is 1..PREFETCH_THRESHOLD.
  // See `makeDefaultPrefetchMocks` JSDoc for rationale.
  const mergedMocks = options.skipDefaultPrefetchMocks
    ? mocks
    : [...mocks, ...makeDefaultPrefetchMocks()];
  render(
    <MockedProvider mocks={mergedMocks as never}>
      <LearnClient cardgroupId={CG_ID} initialCards={initialCards} lastViewedCardgroupId={CG_ID} />
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
              __typename: "HandleSwipeSuccess" as const,
              response: {
                __typename: "SwipeResponse" as const,
                nextCards,
                performanceMode: 0,
                metrics: DEFAULT_METRICS,
              },
            },
          },
        };
      },
    },
    wasCalled: () => called,
  };
}

describe("<LearnClient>", () => {
  // handleRate delegates to swipeStackRef.current?.triggerSwipe(direction).
  // The mock SwipeCardStack exposes triggerSwipe via useImperativeHandle and
  // immediately calls onCardSwiped, which fires the mutation. No timer delay
  // needed here — timing logic lives in SwipeCardStack, not LearnClient.
  it.each([
    ["Rate as Again", 1],
    ["Rate as Hard", 2],
    ["Rate as Easy", 4],
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

    await user.click(screen.getByRole("button", { name: "Rate as Easy" }));

    expect(screen.queryByText("Hello")).not.toBeInTheDocument();

    await waitFor(() => {
      expect(screen.getByText("Server next")).toBeInTheDocument();
    });
    expect(screen.queryByText("Bye")).not.toBeInTheDocument();
  });

  it("renders the caught-up state after the queue empties", async () => {
    const user = userEvent.setup();
    const swipe = makeSwipeMock(4, []);
    renderLearnClient([swipe.mock], [CARD_1]);

    await user.click(screen.getByRole("button", { name: "Rate as Easy" }));

    await waitFor(() => {
      expect(
        screen.getByRole("heading", { name: "Today's learning is complete" }),
      ).toBeInTheDocument();
    });
    expect(screen.getByRole("link", { name: "Back to cardgroups" })).toHaveAttribute(
      "href",
      "/cardgroups",
    );
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

    await user.click(screen.getByRole("button", { name: "Rate as Again" }));

    await waitFor(() => {
      expect(screen.getByText("Hello")).toBeInTheDocument();
      expect(screen.getByRole("alert")).toHaveTextContent("Could not save that swipe");
    });

    // PII redaction contract — docs/frontend/rsc-error-handling/redact-err-message-from-console-payloads.md.
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
    // docs/pagination/capture-mockedprovider-warn-leaks.md § "Spy stacking",
    // this spy is the OUTER spy (installed after the file-wide leak spy) and
    // must forward every `console.warn` call through to the underlying leak
    // spy so MockedProvider leaks are still recorded.
    let consoleWarnSpy: ReturnType<typeof vi.spyOn>;

    beforeEach(() => {
      consoleWarnSpy = vi.spyOn(console, "warn");
    });

    afterEach(() => {
      consoleWarnSpy.mockRestore();
    });

    it("InputValidationError union variant — emits console.warn, advances optimistic queue, shows no banner", async () => {
      // Fix #2 (sibling agent): handleSwipe receiving an InputValidationError union
      // variant must emit a structured console.warn for operator triage, but must
      // NOT surface a banner (the swipe is non-fatal) and must NOT roll back the
      // optimistic queue advance (the card is removed from the queue).
      //
      // Payload shape: `data: { handleSwipe: { __typename: "InputValidationError",
      // field, message } }` — union data, not the error channel.
      const user = userEvent.setup();
      const mock = {
        request: {
          query: HandleSwipeDocument,
          variables: { input: { cardId: CARD_1.id, cardgroupId: CG_ID, mode: 4 } },
        },
        result: {
          data: {
            handleSwipe: {
              __typename: "InputValidationError" as const,
              field: "cardId",
              message: "card not found",
            },
          },
        },
      };

      // Render with CARD_1 and CARD_2 so we can observe the optimistic remove
      // without immediately hitting the empty-queue caught-up screen.
      renderLearnClient([mock], [CARD_1, CARD_2]);

      // Swipe CARD_1 right (mode 4 = Easy).
      await user.click(screen.getByRole("button", { name: "Rate as Easy" }));

      // console.warn must be emitted with the structured payload for operator triage.
      // The message must include cardId and cardgroupId, but NOT the server's
      // `message` field — backend messages may echo user-authored content.
      // See docs/frontend/rsc-error-handling/redact-err-message-from-console-payloads.md.
      await waitFor(() => {
        expect(consoleWarnSpy).toHaveBeenCalledWith(
          "[LearnClient] handleSwipe InputValidationError",
          expect.objectContaining({
            cardId: CARD_1.id,
            cardgroupId: CG_ID,
            field: "cardId",
          }),
        );
      });

      // The warn payload must NOT include the server message (PII redaction).
      const warnCall = consoleWarnSpy.mock.calls.find(
        (call: unknown[]) => call[0] === "[LearnClient] handleSwipe InputValidationError",
      );
      expect(warnCall).toBeDefined();
      expect(warnCall?.[1]).not.toHaveProperty("message");

      // No error banner — the swipe is deliberately non-fatal at the UI level.
      expect(screen.queryByRole("alert")).not.toBeInTheDocument();

      // The optimistic queue advanced: CARD_1 is no longer the active card.
      // CARD_2 (the next card in the queue) is now shown.
      await waitFor(() => {
        expect(screen.getByText("Bye")).toBeInTheDocument();
      });
      expect(screen.queryByText("Hello")).not.toBeInTheDocument();
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

      await user.click(screen.getByRole("button", { name: "Rate as Easy" }));

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

  it("renders the caught-up state when the initial due queue is empty", () => {
    renderLearnClient([], []);

    expect(
      screen.getByRole("heading", { name: "Today's learning is complete" }),
    ).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Back to cardgroups" })).toHaveAttribute(
      "href",
      "/cardgroups",
    );
  });

  // The 180ms commit-delay and overlay-paint logic lives in SwipeCardStack, not LearnClient.
  // LearnClient's handleRate only calls swipeStackRef.current?.triggerSwipe() — no setTimeout,
  // no swipeDirection/swipeProgress state. Timing behavior is covered by swipe-card-stack.test.tsx.
  it("handleRate dispatches triggerSwipe exactly once per button click without async delay", async () => {
    const user = userEvent.setup();
    const swipe = makeSwipeMock(1);
    renderLearnClient([swipe.mock]);

    // Wrap in act so the synchronous triggerSwipe → onCardSwiped → setQueue
    // flush completes before we check interim state.
    await act(async () => {
      await user.click(screen.getByRole("button", { name: "Rate as Again" }));
    });

    await waitFor(() => {
      expect(swipe.wasCalled()).toBe(true);
    });
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
            __typename: "SetLastViewedCardgroupSuccess" as const,
            user: {
              __typename: "User" as const,
              id: USER_ID,
              lastViewedCardgroup: {
                __typename: "Cardgroup" as const,
                id: cardgroupId,
              },
            },
          },
        },
      };
    },
  };
}

describe("<LearnClient> persist-last-viewed path", () => {
  // Forwarding spy: do NOT call `mockImplementation(() => {})` here. Per
  // docs/pagination/capture-mockedprovider-warn-leaks.md § "Spy stacking",
  // this spy is the OUTER spy (installed after the file-wide leak spy) and
  // must forward every `console.warn` call through to the underlying leak spy
  // so MockedProvider leaks are still recorded.
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
      <MockedProvider
        mocks={[makePersistMock(CG_ID, mutationCalled), ...makeDefaultPrefetchMocks()]}
      >
        <LearnClient cardgroupId={CG_ID} initialCards={[CARD_1]} lastViewedCardgroupId="cg-other" />
      </MockedProvider>,
    );

    await waitFor(() => {
      expect(mutationCalled).toHaveBeenCalledOnce();
    });
  });

  it("writes lastViewedCardgroup into the Apollo cache after mutation resolves", async () => {
    const cache = new InMemoryCache();

    render(
      <MockedProvider mocks={[makePersistMock(CG_ID), ...makeDefaultPrefetchMocks()]} cache={cache}>
        <LearnClient cardgroupId={CG_ID} initialCards={[CARD_1]} lastViewedCardgroupId="cg-other" />
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

  it("InputValidationError variant — does not write to the cache (regression guard)", async () => {
    // When the server returns an InputValidationError variant, the update callback
    // must return early without calling cache.writeFragment. This test asserts that
    // the User fragment is NOT written to the cache — the __typename narrowing guard
    // in learn-client.tsx is load-bearing.
    const cache = new InMemoryCache();

    const validationMock = {
      request: {
        query: SetLastViewedCardgroupDocument,
        variables: { cardgroupId: CG_ID },
      },
      result: () => ({
        data: {
          setLastViewedCardgroup: {
            __typename: "InputValidationError" as const,
            field: "cardgroupId",
            message: "cardgroup not found or not owned",
          },
        },
      }),
    };

    render(
      <MockedProvider mocks={[validationMock, ...makeDefaultPrefetchMocks()]} cache={cache}>
        <LearnClient cardgroupId={CG_ID} initialCards={[CARD_1]} lastViewedCardgroupId="cg-other" />
      </MockedProvider>,
    );

    // Wait for the mutation to resolve (the non-success warn fires).
    await waitFor(() => {
      expect(consoleWarnSpy).toHaveBeenCalledWith(
        "[learn] setLastViewedCardgroup non-success variant",
        expect.objectContaining({ typename: "InputValidationError" }),
      );
    });

    // The User fragment must NOT have been written.
    const LastViewedFragment = gql`
      fragment LastViewedValidationCheck on User {
        lastViewedCardgroup {
          id
        }
      }
    `;
    const cached = cache.readFragment<{ lastViewedCardgroup: { id: string } | null }>({
      id: `User:${USER_ID}`,
      fragment: LastViewedFragment,
    });
    expect(cached).toBeNull();

    // Component still renders the card stack — no crash.
    expect(screen.getByText("Hello")).toBeInTheDocument();
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
      <MockedProvider mocks={[mockEntry, ...makeDefaultPrefetchMocks()]}>
        <LearnClient cardgroupId={CG_ID} initialCards={[CARD_1]} lastViewedCardgroupId="cg-other" />
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

    // PII redaction contract — docs/frontend/rsc-error-handling/redact-err-message-from-console-payloads.md.
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

describe("<LearnClient> LearnActionBar integration", () => {
  it("renders LearnActionBar when cards exist", () => {
    renderLearnClient([]);
    expect(screen.getByTestId("learn-action-bar")).toBeInTheDocument();
  });

  it("removes LearnActionBar when the session queue empties", async () => {
    const user = userEvent.setup();
    const swipe = makeSwipeMock(4, []);
    renderLearnClient([swipe.mock]);

    expect(screen.getByTestId("learn-action-bar")).toHaveAttribute("data-disabled", "false");

    await user.click(screen.getByRole("button", { name: "Rate as Easy" }));

    await waitFor(() => {
      expect(screen.queryByTestId("learn-action-bar")).not.toBeInTheDocument();
    });
  });

  it("does not render rating buttons once the caught-up state is reached", async () => {
    const user = userEvent.setup();
    const swipe = makeSwipeMock(4, []);
    renderLearnClient([swipe.mock], [CARD_1]);

    await user.click(screen.getByRole("button", { name: "Rate as Easy" }));

    await waitFor(() => {
      expect(
        screen.getByRole("heading", { name: "Today's learning is complete" }),
      ).toBeInTheDocument();
    });
    expect(screen.queryByRole("button", { name: "Rate as Easy" })).not.toBeInTheDocument();
  });
});

// ---------------------------------------------------------------------------
// onSwipe identity-stability test
//
// Asserts that the `onSwipe` callback passed to SwipeCardStack as `onCardSwiped`
// keeps the same reference across re-renders caused by queue state updates.
//
// Regression guard: onSwipe must keep a stable callback identity across
// re-renders caused by queue mutations. The implementation uses a `queueRef`
// instead of putting `queue` in the useCallback dep array, so the callback
// is created once. If a future change adds `queue` to the deps, every swipe
// would mint a fresh function and this assertion would fail.
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
            __typename: "HandleSwipeSuccess" as const,
            response: {
              __typename: "SwipeResponse" as const,
              nextCards: [CARD_2],
              performanceMode: 0,
              metrics: DEFAULT_METRICS,
            },
          },
        },
      },
    };

    render(
      <MockedProvider mocks={[swipeMock, ...makeDefaultPrefetchMocks()]}>
        <LearnClient
          cardgroupId={CG_ID}
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
    // re-renders. If `queue` were in the useCallback dep array (instead of the
    // queueRef pattern), every queue state update would produce a new function
    // and this assertion would fail.
    expect(Object.is(firstRef, secondRef)).toBe(true);
  });
});

// ---------------------------------------------------------------------------
// Background prefetch
//
// LearnClient runs a background prefetch of LearnNextDueCards when the queue
// length drops to PREFETCH_THRESHOLD (5) or below. The fetched cards are
// merged into the existing queue with id-based dedup. This describe block
// installs its OWN leak spy that includes "LearnNextDueCards" in
// `operationNames` so unmatched prefetch requests are surfaced.
// ---------------------------------------------------------------------------

/** Build a queue of `n` cards with deterministic ids (`q-1` … `q-n`). */
function makeQueue(n: number, idPrefix = "q") {
  return Array.from({ length: n }, (_, i) => ({
    __typename: "Card" as const,
    id: `${idPrefix}-${i + 1}`,
    front: `Front ${i + 1}`,
    back: `Back ${i + 1}`,
    userCardState: userCardState("2026-04-30T00:00:00Z", 0),
    cardgroupId: CG_ID,
  }));
}

type PrefetchCard = ReturnType<typeof makeQueue>[number];

/**
 * Builds a LearnNextDueCards mock that returns `cards` and tracks call count.
 * Optional `delay` (ms) holds the response open, allowing tests to fire the
 * effect multiple times before the first response resolves — used to verify
 * the in-flight guard prevents concurrent prefetch requests.
 */
function makePrefetchMock(cards: PrefetchCard[], opts?: { delay?: number }) {
  let calls = 0;
  const mock: {
    request: { query: typeof LearnNextDueCardsDocument; variables: object };
    result: () => { data: { learnNextDueCards: PrefetchCard[] } };
    delay?: number;
  } = {
    request: {
      query: LearnNextDueCardsDocument,
      variables: { cardgroupId: CG_ID, limit: LEARN_PAGE_LIMIT },
    },
    result: () => {
      calls += 1;
      return { data: { learnNextDueCards: cards } };
    },
  };
  if (opts?.delay !== undefined) {
    mock.delay = opts.delay;
  }
  return { mock, callCount: () => calls };
}

describe("<LearnClient> queue prefetch", () => {
  // The file-wide `leakSpy` (initialized at top of file) already covers
  // `LearnNextDueCards`, `HandleSwipe`, and `SetLastViewedCardgroup`. A
  // describe-scoped spy is unnecessary and would shadow the outer spy.

  it("fires prefetch query once when initial queue is at threshold", async () => {
    const initial = makeQueue(PREFETCH_THRESHOLD); // q-1 .. q-5
    const incoming = makeQueue(2, "p"); // p-1, p-2
    const prefetch = makePrefetchMock(incoming);

    renderLearnClient([prefetch.mock], initial, { skipDefaultPrefetchMocks: true });

    await waitFor(() => {
      expect(prefetch.callCount()).toBe(1);
    });
    // Merged queue should now contain the original five plus two prefetched
    // cards. The active card (top of queue) is unchanged.
    await waitFor(() => {
      expect(screen.getByText("Front 1")).toBeInTheDocument();
    });
  });

  it("does not double-fire while a prefetch is in flight", async () => {
    const initial = makeQueue(PREFETCH_THRESHOLD); // 5 cards — at threshold
    const incoming = makeQueue(0, "p");
    // Hold the response open long enough for a re-render to fire the effect
    // again before the in-flight ref clears in `finally`.
    const prefetch = makePrefetchMock(incoming, { delay: 50 });
    // The first card's swipe will fire HandleSwipe — provide a mock that
    // returns nextCards: [] so the post-swipe queue is just the leftover four.
    const swipe = {
      request: {
        query: HandleSwipeDocument,
        variables: { input: { cardId: "q-1", cardgroupId: CG_ID, mode: 4 } },
      },
      result: {
        data: {
          handleSwipe: {
            __typename: "HandleSwipeSuccess" as const,
            response: {
              __typename: "SwipeResponse" as const,
              nextCards: initial.slice(1),
              performanceMode: 0,
              metrics: DEFAULT_METRICS,
            },
          },
        },
      },
    };

    const user = userEvent.setup();
    renderLearnClient([prefetch.mock, swipe], initial, { skipDefaultPrefetchMocks: true });

    // Trigger a swipe while the prefetch is in flight. The optimistic queue
    // shrinks to 4, which crosses the threshold again and would re-fire the
    // effect if the in-flight guard were absent.
    await user.click(screen.getByRole("button", { name: "Rate as Easy" }));

    // Wait long enough for the delayed prefetch to resolve.
    await waitFor(
      () => {
        expect(prefetch.callCount()).toBe(1);
      },
      { timeout: 1000 },
    );
  });

  it("dedups prefetched cards against the existing queue by id", async () => {
    const initial = makeQueue(PREFETCH_THRESHOLD); // q-1 .. q-5
    // Prefetch returns one duplicate (q-3, id "q-3") and one new card (id "p-unique").
    // Only the new card should be appended — q-3 must not appear twice.
    const duplicate = initial[2] as PrefetchCard; // q-3, front "Front 3"
    const freshCard: PrefetchCard = {
      __typename: "Card" as const,
      id: "p-unique",
      front: "Prefetched New",
      back: "Prefetched Back",
      userCardState: userCardState("2026-04-30T00:00:00Z", 0),
      cardgroupId: CG_ID,
    };
    const prefetch = makePrefetchMock([duplicate, freshCard]);

    // After prefetch merges, the queue becomes q-1..q-5 + p-unique = 6 cards,
    // which is above threshold, so no second prefetch fires. No noop needed.

    renderLearnClient([prefetch.mock], initial, {
      skipDefaultPrefetchMocks: true,
    });

    // Wait for the first prefetch to resolve and the merged queue to render.
    // capturedCardSnapshots records the full `cards` array on every render, so
    // we can assert the exact queue contents without swipe-exhausting the stack.
    await waitFor(() => {
      expect(prefetch.callCount()).toBe(1);
    });

    // After merging, the queue must have exactly 6 entries: q-1..q-5 + p-unique.
    // We wait for a snapshot that reflects the post-merge state (length === 6).
    await waitFor(() => {
      const latest = capturedCardSnapshots.at(-1);
      expect(latest).toBeDefined();
      expect(latest?.length).toBe(6);
    });

    const mergedCards = capturedCardSnapshots.at(-1);
    if (!mergedCards) throw new Error("capturedCardSnapshots is empty after waitFor");
    const mergedIds = mergedCards.map((c) => c.id);

    // All original cards are present.
    expect(mergedIds).toContain("q-1");
    expect(mergedIds).toContain("q-2");
    expect(mergedIds).toContain("q-3");
    expect(mergedIds).toContain("q-4");
    expect(mergedIds).toContain("q-5");
    // The fresh prefetched card is appended exactly once.
    expect(mergedIds).toContain("p-unique");
    // The duplicate (q-3) was not added a second time — id appears exactly once.
    expect(mergedIds.filter((id) => id === "q-3").length).toBe(1);
    // p-unique appears exactly once — no accidental duplication.
    expect(mergedIds.filter((id) => id === "p-unique").length).toBe(1);
  });

  it("does not prefetch when queue is above threshold", async () => {
    const initial = makeQueue(PREFETCH_THRESHOLD + 5); // 10 cards — above
    // Provide NO prefetch mock; if the effect fires anyway, the leak spy will
    // record an unmatched-request warning and `assertNoLeaks` will fail.
    renderLearnClient([], initial, { skipDefaultPrefetchMocks: true });

    // Give effects a tick to settle.
    await waitFor(() => {
      expect(screen.getByText("Front 1")).toBeInTheDocument();
    });
  });

  it("does not prefetch when queue is empty", async () => {
    // Empty queue — the user has finished. Effect must short-circuit before
    // dispatching a prefetch. No mock supplied: the leak spy enforces this.
    renderLearnClient([], [], { skipDefaultPrefetchMocks: true });

    expect(
      screen.getByRole("heading", { name: "Today's learning is complete" }),
    ).toBeInTheDocument();
  });

  it("warns and re-allows prefetch after a failed attempt", async () => {
    // First mock: an error response. After it rejects, the in-flight ref must
    // clear inside `finally`, so a subsequent threshold crossing can dispatch
    // a fresh prefetch (covered by the second mock).
    const initial = makeQueue(PREFETCH_THRESHOLD);
    const failingMock = {
      request: {
        query: LearnNextDueCardsDocument,
        variables: { cardgroupId: CG_ID, limit: LEARN_PAGE_LIMIT },
      },
      error: new Error("network failure"),
    };
    const swipe = {
      request: {
        query: HandleSwipeDocument,
        variables: { input: { cardId: "q-1", cardgroupId: CG_ID, mode: 4 } },
      },
      result: {
        data: {
          handleSwipe: {
            __typename: "HandleSwipeSuccess" as const,
            response: {
              __typename: "SwipeResponse" as const,
              nextCards: initial.slice(1),
              performanceMode: 0,
              metrics: DEFAULT_METRICS,
            },
          },
        },
      },
    };
    // Second prefetch attempt after the swipe shrinks the queue to 4.
    const recovery = makePrefetchMock(makeQueue(1, "r"));

    // Forwarding spy — does NOT call mockImplementation so the outer leak spy
    // still sees calls.
    const consoleWarnSpy = vi.spyOn(console, "warn");

    try {
      const user = userEvent.setup();
      renderLearnClient([failingMock, swipe, recovery.mock], initial, {
        skipDefaultPrefetchMocks: true,
      });

      // Wait for the first prefetch to fail and emit the warn.
      await waitFor(() => {
        const warnCall = consoleWarnSpy.mock.calls.find(
          (call) => call[0] === "[learn] prefetch failed",
        );
        expect(warnCall).toBeDefined();
      });

      // Queue is unchanged — original card 1 is still active.
      expect(screen.getByText("Front 1")).toBeInTheDocument();

      // PII redaction contract — docs/frontend/rsc-error-handling/redact-err-message-from-console-payloads.md.
      const warnCall = consoleWarnSpy.mock.calls.find(
        (call) => call[0] === "[learn] prefetch failed",
      );
      expect(warnCall).toBeDefined();
      const payload = warnCall?.[1] as Record<string, unknown>;
      expect(payload).toMatchObject({
        cardgroupId: CG_ID,
        name: expect.any(String),
      });
      expect(payload).not.toHaveProperty("message");

      // Trigger a swipe to shrink queue to 4 (still ≤ threshold) — the in-flight
      // ref must have reset, so the recovery mock is consumed.
      await user.click(screen.getByRole("button", { name: "Rate as Easy" }));

      await waitFor(() => {
        expect(recovery.callCount()).toBe(1);
      });
    } finally {
      consoleWarnSpy.mockRestore();
    }
  });
});

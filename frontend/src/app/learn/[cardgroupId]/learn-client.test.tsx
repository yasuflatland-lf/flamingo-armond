// @vitest-environment happy-dom
import { gql, InMemoryCache } from "@apollo/client";
import { MockedProvider } from "@apollo/client/testing/react";
import { act, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { GraphQLError } from "graphql";
import { type RefObject, useImperativeHandle, useRef } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { LEARN_PAGE_LIMIT } from "@/app/learn/queries";
import { CefrBadge } from "@/components/learn/cefr-badge";
import { CardContent, type SwipeCardData } from "@/components/learn/swipe-card";
import type { SwipeCardStackHandle } from "@/components/learn/swipe-card-stack";
import type { LearnDisplayMode } from "@/components/learn/types";
import {
  HandleSwipeDocument,
  type HandleSwipeMutationVariables,
  LearnNextDueCardsDocument,
  SetLastViewedCardgroupDocument,
} from "@/generated/graphql";
import { renderWithIntl } from "@/test/render-with-intl";
import {
  type ApolloMockLeakSpyResult,
  installApolloMockLeakSpy,
} from "../../../../__tests__/utils/mock-apollo-paginated";
import enMessages from "../../../../messages/en.json";
import jaMessages from "../../../../messages/ja.json";
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
const capturedDisplayModes: Array<LearnDisplayMode | undefined> = [];

vi.mock("@/components/learn/swipe-card-stack", () => ({
  SwipeCardStack: (props: {
    cards: SwipeCardData[];
    displayMode: LearnDisplayMode;
    onCardSwiped: SwipeCardStackOnCardSwiped;
    completedCount?: number;
    ref?: RefObject<SwipeCardStackHandle | null>;
  }) => {
    capturedOnCardSwiped.push(props.onCardSwiped);
    capturedCardSnapshots.push([...props.cards]);
    capturedDisplayModes.push(props.displayMode);

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
    // Render the real presentational layer so badge-render integration tests can
    // assert the full LearnClient → SwipeCardStack → card pipeline without the
    // next/dynamic AnimatedCard chunk. The real AnimatedCard overlays the
    // CefrBadge OUTSIDE the flip rotator (so the reveal flip cannot duplicate it);
    // this mock mirrors that layout — CardContent for the content, CefrBadge
    // alongside it.
    return (
      <>
        <CardContent card={activeCard} revealed={props.displayMode === "ALWAYS_VISIBLE"} />
        <CefrBadge level={activeCard.cefrLevel} />
      </>
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
  }) => {
    // Mirror the real bar: rating buttons are inert only when the caller's
    // `disabled` flag is set (e.g. the queue has emptied); reveal does not gate.
    const ratingDisabled = props.disabled;
    return (
      <div data-testid="learn-action-bar" data-disabled={String(ratingDisabled)}>
        <button type="button" onClick={() => props.onRate("left")} disabled={ratingDisabled}>
          Rate as Again
        </button>
        <button type="button" onClick={() => props.onRate("down")} disabled={ratingDisabled}>
          Rate as Hard
        </button>
        <button type="button" onClick={() => props.onRate("right")} disabled={ratingDisabled}>
          Rate as Easy
        </button>
      </div>
    );
  },
}));

// ---------------------------------------------------------------------------
// PracticeClient mock — a stub marker so the learn-client phase switch can be
// asserted without rendering the real practice query pipeline (covered by
// practice-client.test.tsx). The marker is keyed by cardgroupId so the test can
// confirm the prop is threaded through.
// ---------------------------------------------------------------------------
vi.mock("./practice-client", () => ({
  PracticeClient: (props: { cardgroupId: string }) => (
    <div data-testid="practice-client">Practice mode for {props.cardgroupId}</div>
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
  capturedDisplayModes.length = 0;
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
  cefrLevel: null,
  userCardState: userCardState("2026-04-30T00:00:00Z", 0),
  cardgroupId: CG_ID,
};

const CARD_2 = {
  __typename: "Card" as const,
  id: "c-2",
  front: "Bye",
  back: "Adios",
  cefrLevel: null,
  userCardState: userCardState("2026-04-30T00:00:00Z", 0),
  cardgroupId: CG_ID,
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
    result: { data: { learnNextDueCards: [], me: null } },
  }));
}

/**
 * Default `SetLastViewedCardgroup` no-op success mock.
 *
 * The persist-last-viewed effect fires unconditionally on mount (the page no
 * longer fetches a last-viewed value to short-circuit it), so every render of
 * `LearnClient` issues one `SetLastViewedCardgroup` mutation. Without a matching
 * mock the file-wide leak spy (which includes `SetLastViewedCardgroup` in
 * `operationNames`) records an unmatched-request warning. Tests in the dedicated
 * persist-last-viewed describe supply their own mocks instead.
 */
function makeDefaultPersistMock() {
  return {
    request: { query: SetLastViewedCardgroupDocument, variables: { cardgroupId: CG_ID } },
    result: {
      data: {
        setLastViewedCardgroup: {
          __typename: "SetLastViewedCardgroupSuccess" as const,
          user: {
            __typename: "User" as const,
            id: "u-default",
            lastViewedCardgroup: { __typename: "Cardgroup" as const, id: CG_ID },
          },
        },
      },
    },
  };
}

type RenderLearnClientOptions = {
  /**
   * When `true`, do NOT append default `LearnNextDueCards` no-op mocks. Used
   * by the dedicated `queue prefetch` describe block, whose tests supply their
   * own prefetch mocks and rely on unmatched-request leak detection to assert
   * the effect did or did not fire.
   */
  skipDefaultPrefetchMocks?: boolean;
  displayMode?: LearnDisplayMode;
  /** Locale + catalog override for the intl harness. Defaults to en. */
  intl?: { locale: "en" | "ja"; messages: typeof enMessages };
};

function renderLearnClient(
  mocks: unknown[],
  initialCards = [CARD_1],
  options: RenderLearnClientOptions = {},
) {
  // The persist-last-viewed effect fires unconditionally on mount, so append a
  // no-op `SetLastViewedCardgroup` mock to satisfy it; that mutation's behaviour
  // is exercised in its own describe below. Append default no-op
  // `LearnNextDueCards` mocks so the prefetch effect is satisfied for any test
  // whose initial queue length is 1..PREFETCH_THRESHOLD. See the
  // `makeDefaultPersistMock` / `makeDefaultPrefetchMocks` JSDoc for rationale.
  const mergedMocks = options.skipDefaultPrefetchMocks
    ? [...mocks, makeDefaultPersistMock()]
    : [...mocks, makeDefaultPersistMock(), ...makeDefaultPrefetchMocks()];
  renderWithIntl(
    <MockedProvider mocks={mergedMocks as never}>
      <LearnClient
        cardgroupId={CG_ID}
        initialCards={initialCards}
        // Default to ALWAYS_VISIBLE so the card back is shown for the
        // swipe-mechanics / queue / prefetch tests, which are display-mode
        // agnostic. Tests that exercise FLIP_TO_REVEAL (back hidden until the
        // learner taps to reveal) pass `displayMode` explicitly.
        displayMode={options.displayMode ?? "ALWAYS_VISIBLE"}
      />
    </MockedProvider>,
    options.intl,
  );
}

function makeSwipeMock(rating: 1 | 2 | 4) {
  let called = false;
  return {
    mock: {
      request: {
        query: HandleSwipeDocument,
        variables: { input: { cardId: CARD_1.id, cardgroupId: CG_ID, rating } },
      },
      result: () => {
        called = true;
        return {
          data: {
            handleSwipe: {
              __typename: "HandleSwipeSuccess" as const,
              response: {
                __typename: "SwipeResponse" as const,
                performanceMode: "DIFFICULT",
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
  it("forwards ALWAYS_VISIBLE display mode to SwipeCardStack and shows the back immediately", () => {
    renderLearnClient([], [CARD_1], { displayMode: "ALWAYS_VISIBLE" });

    expect(capturedDisplayModes.at(-1)).toBe("ALWAYS_VISIBLE");
    expect(screen.getByText("Hola")).toBeInTheDocument();
  });

  it("forwards FLIP_TO_REVEAL display mode to SwipeCardStack and keeps the back hidden", () => {
    renderLearnClient([], [CARD_1], { displayMode: "FLIP_TO_REVEAL" });

    expect(capturedDisplayModes.at(-1)).toBe("FLIP_TO_REVEAL");
    expect(screen.queryByText("Hola")).not.toBeInTheDocument();
  });

  it("keeps down rating mapped to Hard rating 2", async () => {
    const user = userEvent.setup();
    const swipe = makeSwipeMock(2);
    renderLearnClient([swipe.mock]);

    await user.click(screen.getByRole("button", { name: "Rate as Hard" }));

    await waitFor(() => {
      expect(swipe.wasCalled()).toBe(true);
    });
  });

  // handleRate delegates to swipeStackRef.current?.triggerSwipe(direction).
  // The mock SwipeCardStack exposes triggerSwipe via useImperativeHandle and
  // immediately calls onCardSwiped, which fires the mutation. No timer delay
  // needed here — timing logic lives in SwipeCardStack, not LearnClient.
  it.each([
    ["Rate as Again", 1],
    ["Rate as Hard", 2],
    ["Rate as Easy", 4],
  ] as const)("maps %s to rating %d", async (label, rating) => {
    const user = userEvent.setup();
    const swipe = makeSwipeMock(rating);
    renderLearnClient([swipe.mock]);

    await user.click(screen.getByRole("button", { name: label }));

    await waitFor(() => {
      expect(swipe.wasCalled()).toBe(true);
    });
  });

  it("advances the queue via optimistic delete; server response does not replace the queue", async () => {
    const user = userEvent.setup();
    const swipe = makeSwipeMock(4);
    renderLearnClient([swipe.mock], [CARD_1, CARD_2]);

    await user.click(screen.getByRole("button", { name: "Rate as Easy" }));

    // The swiped card is removed.
    expect(screen.queryByText("Hello")).not.toBeInTheDocument();

    // The next card in the original queue is now active — no server-supplied
    // replacement card appears; the queue retains its original tail order.
    await waitFor(() => {
      expect(screen.getByText("Bye")).toBeInTheDocument();
    });
    // The mutation was called.
    await waitFor(() => {
      expect(swipe.wasCalled()).toBe(true);
    });

    // After the server reconciles, the visible queue is still the retained
    // tail — exactly [CARD_2]. No card was injected by the server response.
    const latest = capturedCardSnapshots.at(-1);
    expect(latest).toBeDefined();
    expect(latest?.map((c) => c.id)).toEqual([CARD_2.id]);
  });

  it("queue order is stable across handleSwipe — same-due cards do not reshuffle on swipe", async () => {
    // Regression guard for issue #229: when the user swipes the first card of a
    // multi-card queue, the remaining cards must keep their original relative
    // order. Prior to the fix, the HandleSwipe response carried a FSRS-rescored
    // replacement array that the client applied to the local queue, which
    // could re-order same-due cards on every swipe. The new contract drops
    // that field; the queue is advanced exclusively by the optimistic delete
    // and refilled exclusively by the background prefetch.
    const user = userEvent.setup();
    const cards = [
      CARD_1,
      { ...CARD_2, id: "c-2", front: "Card2" },
      { ...CARD_2, id: "c-3", front: "Card3" },
      { ...CARD_2, id: "c-4", front: "Card4" },
    ];
    const swipe = makeSwipeMock(4);
    renderLearnClient([swipe.mock], cards);

    await user.click(screen.getByRole("button", { name: "Rate as Easy" }));

    await waitFor(() => {
      expect(swipe.wasCalled()).toBe(true);
    });

    // After the swipe + server reconciliation, the visible queue must be
    // exactly [c-2, c-3, c-4] — same order as the input minus the swiped card.
    // No card from a later position has jumped to the front.
    await waitFor(() => {
      const latest = capturedCardSnapshots.at(-1);
      expect(latest).toBeDefined();
      expect(latest?.map((c) => c.id)).toEqual(["c-2", "c-3", "c-4"]);
    });
  });

  it("renders the caught-up state after the queue empties", async () => {
    const user = userEvent.setup();
    const swipe = makeSwipeMock(4);
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
        variables: { input: { cardId: CARD_1.id, cardgroupId: CG_ID, rating: 1 } },
      },
      result: {
        errors: [new GraphQLError("bad swipe", { extensions: { code: "BAD_USER_INPUT" } })],
      },
    };
    renderLearnClient([mock], [CARD_1]);

    await user.click(screen.getByRole("button", { name: "Rate as Again" }));

    await waitFor(() => {
      expect(screen.getByText("Hello")).toBeInTheDocument();
      // Sourced from the catalog rather than re-spelled, so a reverted
      // hardcoded literal in learn-client.tsx would fail here.
      expect(screen.getByRole("alert")).toHaveTextContent(enMessages.Learn.swipeSaveFailed);
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

  it("renders the swipe-failure banner in the active locale", async () => {
    const user = userEvent.setup();
    const consoleErrorSpy = vi.spyOn(console, "error").mockImplementation(() => {});
    const mock = {
      request: {
        query: HandleSwipeDocument,
        variables: { input: { cardId: CARD_1.id, cardgroupId: CG_ID, rating: 1 } },
      },
      result: {
        errors: [new GraphQLError("bad swipe", { extensions: { code: "BAD_USER_INPUT" } })],
      },
    };
    renderLearnClient([mock], [CARD_1], { intl: { locale: "ja", messages: jaMessages } });

    // The LearnActionBar mock renders fixed English labels, so the rating
    // control is addressed by that label regardless of the active locale.
    await user.click(screen.getByRole("button", { name: "Rate as Again" }));

    await waitFor(() => {
      expect(screen.getByRole("alert")).toHaveTextContent(jaMessages.Learn.swipeSaveFailed);
    });

    consoleErrorSpy.mockRestore();
  });

  describe("swipe retry", () => {
    let consoleErrorSpy: ReturnType<typeof vi.spyOn>;

    beforeEach(() => {
      consoleErrorSpy = vi.spyOn(console, "error").mockImplementation(() => {});
    });

    afterEach(() => {
      consoleErrorSpy.mockRestore();
    });

    it("retries with the originally chosen rating", async () => {
      const user = userEvent.setup();
      const capturedVariables: HandleSwipeMutationVariables[] = [];
      const captureVariables = (variables: HandleSwipeMutationVariables) => {
        capturedVariables.push(variables);
        return true;
      };
      const failedMock = {
        request: {
          query: HandleSwipeDocument,
          variables: captureVariables,
        },
        error: new Error("network failure"),
      };
      const successMock = {
        request: {
          query: HandleSwipeDocument,
          variables: captureVariables,
        },
        result: {
          data: {
            handleSwipe: {
              __typename: "HandleSwipeSuccess" as const,
              response: {
                __typename: "SwipeResponse" as const,
                performanceMode: "DIFFICULT",
              },
            },
          },
        },
      };
      renderLearnClient([failedMock, successMock], [CARD_1, CARD_2]);

      await user.click(screen.getByRole("button", { name: "Rate as Hard" }));
      await user.click(await screen.findByRole("button", { name: enMessages.Common.retry }));

      await waitFor(() => {
        expect(capturedVariables).toHaveLength(2);
      });
      expect(capturedVariables[0]?.input.rating).toBe(2);
      expect(capturedVariables[1]?.input.rating).toBe(capturedVariables[0]?.input.rating);
    });

    it("shows Retry only after a failure and removes it after a successful retry", async () => {
      const user = userEvent.setup();
      const request = {
        query: HandleSwipeDocument,
        variables: { input: { cardId: CARD_1.id, cardgroupId: CG_ID, rating: 1 } },
      };
      const failedMock = {
        request,
        error: new Error("network failure"),
      };
      const successMock = {
        request,
        result: {
          data: {
            handleSwipe: {
              __typename: "HandleSwipeSuccess" as const,
              response: {
                __typename: "SwipeResponse" as const,
                performanceMode: "DIFFICULT",
              },
            },
          },
        },
      };
      renderLearnClient([failedMock, successMock], [CARD_1, CARD_2]);

      expect(
        screen.queryByRole("button", { name: enMessages.Common.retry }),
      ).not.toBeInTheDocument();

      await user.click(screen.getByRole("button", { name: "Rate as Again" }));
      const retryButton = await screen.findByRole("button", { name: enMessages.Common.retry });
      expect(screen.getByRole("alert")).toHaveTextContent(enMessages.Learn.swipeSaveFailed);

      await user.click(retryButton);

      await waitFor(() => {
        expect(screen.queryByRole("alert")).not.toBeInTheDocument();
        expect(
          screen.queryByRole("button", { name: enMessages.Common.retry }),
        ).not.toBeInTheDocument();
        expect(screen.queryByText("Hello")).not.toBeInTheDocument();
        expect(screen.getByText("Bye")).toBeInTheDocument();
      });
    });

    it("clears a pending failure when a new card is swiped", async () => {
      const user = userEvent.setup();
      const failedMock = {
        request: {
          query: HandleSwipeDocument,
          variables: { input: { cardId: CARD_1.id, cardgroupId: CG_ID, rating: 1 } },
        },
        error: new Error("network failure"),
      };
      const successMock = {
        request: {
          query: HandleSwipeDocument,
          variables: { input: { cardId: CARD_2.id, cardgroupId: CG_ID, rating: 4 } },
        },
        result: {
          data: {
            handleSwipe: {
              __typename: "HandleSwipeSuccess" as const,
              response: {
                __typename: "SwipeResponse" as const,
                performanceMode: "DIFFICULT",
              },
            },
          },
        },
      };
      renderLearnClient([failedMock, successMock], [CARD_1, CARD_2]);

      await user.click(screen.getByRole("button", { name: "Rate as Again" }));
      await screen.findByRole("button", { name: enMessages.Common.retry });

      const onCardSwiped = capturedOnCardSwiped.at(-1);
      expect(onCardSwiped).toBeDefined();
      await act(async () => {
        await onCardSwiped?.(CARD_2, "right");
      });

      await waitFor(() => {
        expect(screen.queryByRole("alert")).not.toBeInTheDocument();
        expect(
          screen.queryByRole("button", { name: enMessages.Common.retry }),
        ).not.toBeInTheDocument();
        expect(screen.getByText("Hello")).toBeInTheDocument();
        expect(screen.queryByText("Bye")).not.toBeInTheDocument();
      });
    });
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
          variables: { input: { cardId: CARD_1.id, cardgroupId: CG_ID, rating: 4 } },
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

      // Swipe CARD_1 right (rating 4 = Easy).
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
      expect(
        screen.queryByRole("button", { name: enMessages.Common.retry }),
      ).not.toBeInTheDocument();

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
          variables: { input: { cardId: CARD_1.id, cardgroupId: CG_ID, rating: 4 } },
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
          "[LearnClient] handleSwipe unexpected payload",
          expect.objectContaining({
            typename: null,
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

  it("renders the CefrBadge for a card whose cefrLevel is non-null (end-to-end)", async () => {
    // End-to-end acceptance criterion: a card with a non-null cefrLevel must
    // show a CEFR badge on the learn page. The path exercised here is:
    //   LearnClient (state) → SwipeCardStack mock → card content + CefrBadge
    // The mock mirrors the real AnimatedCard layout (badge overlaid outside the
    // flip rotator) so the badge pipeline is exercised without the next/dynamic
    // AnimatedCard chunk.
    //
    // `renderLearnClient`'s `initialCards` parameter is inferred from the
    // default `[CARD_1]`, which narrows `cefrLevel` to `null`. Inline the render
    // to pass a LearnClient-compatible card with a non-null level without fighting
    // that inference — the same pattern used by the persist-last-viewed tests.
    renderWithIntl(
      <MockedProvider mocks={[makeDefaultPersistMock(), ...makeDefaultPrefetchMocks()]}>
        <LearnClient
          cardgroupId={CG_ID}
          initialCards={[
            {
              __typename: "Card",
              id: "c-1",
              front: "Hello",
              back: "Hola",
              cefrLevel: "B1",
              userCardState: userCardState("2026-04-30T00:00:00Z", 0),
              cardgroupId: CG_ID,
            },
          ]}
          displayMode="FLIP_TO_REVEAL"
        />
      </MockedProvider>,
    );

    expect(screen.getByLabelText("CEFR level B1")).toBeInTheDocument();
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

  it("fires SetLastViewedCardgroup mutation on mount", async () => {
    const mutationCalled = vi.fn();
    renderWithIntl(
      <MockedProvider
        mocks={[makePersistMock(CG_ID, mutationCalled), ...makeDefaultPrefetchMocks()]}
      >
        <LearnClient cardgroupId={CG_ID} initialCards={[CARD_1]} displayMode="FLIP_TO_REVEAL" />
      </MockedProvider>,
    );

    await waitFor(() => {
      expect(mutationCalled).toHaveBeenCalledOnce();
    });
  });

  it("writes lastViewedCardgroup into the Apollo cache after mutation resolves", async () => {
    const cache = new InMemoryCache();

    renderWithIntl(
      <MockedProvider mocks={[makePersistMock(CG_ID), ...makeDefaultPrefetchMocks()]} cache={cache}>
        <LearnClient cardgroupId={CG_ID} initialCards={[CARD_1]} displayMode="FLIP_TO_REVEAL" />
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

    renderWithIntl(
      <MockedProvider mocks={[validationMock, ...makeDefaultPrefetchMocks()]} cache={cache}>
        <LearnClient cardgroupId={CG_ID} initialCards={[CARD_1]} displayMode="FLIP_TO_REVEAL" />
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
    renderWithIntl(
      <MockedProvider mocks={[mockEntry, ...makeDefaultPrefetchMocks()]}>
        <LearnClient cardgroupId={CG_ID} initialCards={[CARD_1]} displayMode="FLIP_TO_REVEAL" />
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
    // (see .claude/rules/pagination.md § "Drop `optimisticResponse` for mutations
    // that can fail with typed GraphQL errors").
    //
    // Strategy: find the section of source between `SetLastViewedCardgroup` and
    // the next `.catch(` that follows it, and assert no `optimisticResponse`
    // key appears there.
    const source = LearnClient.toString();

    const persistStart = source.indexOf("SetLastViewedCardgroup");
    expect(persistStart).toBeGreaterThan(-1);

    // Find the .catch( that closes the persist mutation chain.
    const persistCatchIdx = source.indexOf(".catch(", persistStart);
    expect(persistCatchIdx).toBeGreaterThan(-1);

    const persistBlock = source.slice(persistStart, persistCatchIdx);
    expect(persistBlock).not.toContain("optimisticResponse");
  });

  it("does not carry optimisticResponse in the handleSwipe mutation", () => {
    // Static assertion: handleSwipe can return InputValidationError (a typed
    // GraphQL error variant). Apollo v3 does not reliably roll back optimistic
    // writes on typed GraphQL errors — only on network errors. So no
    // `optimisticResponse` must appear in the handleSwipe call.
    // See .claude/rules/pagination.md § "Drop `optimisticResponse` for mutations
    // that can fail with typed GraphQL errors".
    //
    // Strategy: the handleSwipe call is in the `applySwipe` callback. Find the
    // region between `handleSwipe({` and the `.catch(` that follows it, and
    // assert no `optimisticResponse` key appears there.
    const source = LearnClient.toString();

    const swipeStart = source.indexOf("handleSwipe({");
    expect(swipeStart).toBeGreaterThan(-1);

    const swipeCatchIdx = source.indexOf(".catch(", swipeStart);
    expect(swipeCatchIdx).toBeGreaterThan(-1);

    const swipeBlock = source.slice(swipeStart, swipeCatchIdx);
    expect(swipeBlock).not.toHaveLength(0);
    // The anchor is present by construction; a call-body token proves the slice spans the mutation.
    expect(swipeBlock).toContain("variables:");
    expect(swipeBlock).not.toContain("optimisticResponse");
  });
});

describe("<LearnClient> LearnActionBar integration", () => {
  it("renders LearnActionBar when cards exist", () => {
    renderLearnClient([]);
    expect(screen.getByTestId("learn-action-bar")).toBeInTheDocument();
  });

  it("removes LearnActionBar when the session queue empties", async () => {
    const user = userEvent.setup();
    const swipe = makeSwipeMock(4);
    renderLearnClient([swipe.mock]);

    expect(screen.getByTestId("learn-action-bar")).toHaveAttribute("data-disabled", "false");

    await user.click(screen.getByRole("button", { name: "Rate as Easy" }));

    await waitFor(() => {
      expect(screen.queryByTestId("learn-action-bar")).not.toBeInTheDocument();
    });
  });

  it("does not render rating buttons once the caught-up state is reached", async () => {
    const user = userEvent.setup();
    const swipe = makeSwipeMock(4);
    renderLearnClient([swipe.mock], [CARD_1]);

    await user.click(screen.getByRole("button", { name: "Rate as Easy" }));

    await waitFor(() => {
      expect(
        screen.getByRole("heading", { name: "Today's learning is complete" }),
      ).toBeInTheDocument();
    });
    expect(screen.queryByRole("button", { name: "Rate as Easy" })).not.toBeInTheDocument();
  });

  it("enables the rating buttons from the start in FLIP_TO_REVEAL mode (reveal is optional)", () => {
    renderLearnClient([], [CARD_1], { displayMode: "FLIP_TO_REVEAL" });

    // Front-only no longer gates rating: the buttons are enabled immediately so
    // the learner can swipe / rate without first revealing the answer.
    expect(screen.getByRole("button", { name: "Rate as Again" })).toBeEnabled();
    expect(screen.getByRole("button", { name: "Rate as Hard" })).toBeEnabled();
    expect(screen.getByRole("button", { name: "Rate as Easy" })).toBeEnabled();
    expect(screen.getByTestId("learn-action-bar")).toHaveAttribute("data-disabled", "false");
  });

  it("enables the rating buttons from the start in ALWAYS_VISIBLE mode", () => {
    renderLearnClient([], [CARD_1], { displayMode: "ALWAYS_VISIBLE" });

    expect(screen.getByRole("button", { name: "Rate as Again" })).toBeEnabled();
    expect(screen.getByRole("button", { name: "Rate as Hard" })).toBeEnabled();
    expect(screen.getByRole("button", { name: "Rate as Easy" })).toBeEnabled();
    expect(screen.getByTestId("learn-action-bar")).toHaveAttribute("data-disabled", "false");
  });
});

// ---------------------------------------------------------------------------
// Practice mode phase switch
//
// When the daily learn queue is exhausted, AllCaughtUp offers a "Study again"
// action that hands the screen to PracticeClient (FSRS-safe re-study). The
// PracticeClient is mocked to a stub marker above; these tests assert only the
// learn-client-side phase transition.
// ---------------------------------------------------------------------------
describe("<LearnClient> practice mode phase switch", () => {
  it("shows a Study again button on the caught-up screen when the queue starts empty", () => {
    renderLearnClient([], []);

    expect(
      screen.getByRole("heading", { name: "Today's learning is complete" }),
    ).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Study again" })).toBeInTheDocument();
    // Still on the learn phase — the practice stub is not rendered yet.
    expect(screen.queryByTestId("practice-client")).not.toBeInTheDocument();
  });

  it("renders PracticeClient with the cardgroupId after clicking Study again", async () => {
    const user = userEvent.setup();
    renderLearnClient([], []);

    await user.click(screen.getByRole("button", { name: "Study again" }));

    expect(screen.getByTestId("practice-client")).toHaveTextContent(`Practice mode for ${CG_ID}`);
    // The caught-up learn screen is gone — we are fully in practice mode.
    expect(
      screen.queryByRole("heading", { name: "Today's learning is complete" }),
    ).not.toBeInTheDocument();
  });
});

// ---------------------------------------------------------------------------
// onSwipe identity-stability test
//
// Asserts that the `onSwipe` callback passed to SwipeCardStack as `onCardSwiped`
// keeps the same reference across re-renders caused by queue state updates.
//
// Regression guard: onSwipe must keep a stable callback identity across
// re-renders caused by queue mutations. The callback's dep array is
// `[cardgroupId, handleSwipe]` — it does not include `queue`. Queue mutations
// are performed via `setQueue((current) => ...)` functional-update callbacks
// that always read the latest state, so no `queue` closure capture is needed.
// If a future change adds `queue` to the deps, every swipe would mint a fresh
// function and this assertion would fail.
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
        variables: { input: { cardId: CARD_1.id, cardgroupId: CG_ID, rating: 4 } },
      },
      result: {
        data: {
          handleSwipe: {
            __typename: "HandleSwipeSuccess" as const,
            response: {
              __typename: "SwipeResponse" as const,
              performanceMode: "DIFFICULT",
            },
          },
        },
      },
    };

    renderWithIntl(
      <MockedProvider mocks={[swipeMock, makeDefaultPersistMock(), ...makeDefaultPrefetchMocks()]}>
        <LearnClient
          cardgroupId={CG_ID}
          initialCards={[CARD_1, CARD_2]}
          displayMode="FLIP_TO_REVEAL"
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
    // re-renders. If `queue` were added to the useCallback dep array, every
    // queue state update would produce a new function and this assertion would
    // fail. Queue mutations use functional setState so no `queue` dep is needed.
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
    cefrLevel: null,
    userCardState: userCardState("2026-04-30T00:00:00Z", 0),
    cardgroupId: CG_ID,
  }));
}

type PrefetchCard = ReturnType<typeof makeQueue>[number];

/**
 * Builds a LearnNextDueCards mock that returns `cards` and tracks call count.
 * Optional `delay` (ms) holds the response open, allowing tests to fire the
 * effect multiple times before the first response resolves — used to verify
 * the in-flight guard prevents concurrent prefetch requests. Optional
 * `cardgroupId` targets a deck other than the default one, for tests that
 * switch the active cardgroup mid-session.
 */
function makePrefetchMock(cards: PrefetchCard[], opts?: { delay?: number; cardgroupId?: string }) {
  let calls = 0;
  const mock: {
    request: { query: typeof LearnNextDueCardsDocument; variables: object };
    result: () => { data: { learnNextDueCards: PrefetchCard[]; me: null } };
    delay?: number;
  } = {
    request: {
      query: LearnNextDueCardsDocument,
      variables: { cardgroupId: opts?.cardgroupId ?? CG_ID, limit: LEARN_PAGE_LIMIT },
    },
    result: () => {
      calls += 1;
      return { data: { learnNextDueCards: cards, me: null } };
    },
  };
  if (opts?.delay !== undefined) {
    mock.delay = opts.delay;
  }
  return { mock, callCount: () => calls };
}

/**
 * Builds a successful `HandleSwipe` mock for `cardId` (rating 4 / Easy) that
 * records whether its result fn ran. Tests await `wasCalled()` when they must
 * order a later event after the mutation actually resolves: the optimistic
 * queue advance is synchronous, so waiting on the rendered queue would let the
 * next step race the mutation's continuation.
 */
function makeSwipeSuccessMock(cardId: string) {
  let called = false;
  return {
    mock: {
      request: {
        query: HandleSwipeDocument,
        variables: { input: { cardId, cardgroupId: CG_ID, rating: 4 } },
      },
      result: () => {
        called = true;
        return {
          data: {
            handleSwipe: {
              __typename: "HandleSwipeSuccess" as const,
              response: {
                __typename: "SwipeResponse" as const,
                performanceMode: "DIFFICULT",
              },
            },
          },
        };
      },
    },
    wasCalled: () => called,
  };
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
    // The first card's swipe will fire HandleSwipe. Under the new contract the
    // server response does NOT carry a replacement queue; the optimistic delete
    // is the sole driver of queue advancement, so the post-swipe queue is just
    // the leftover four.
    const swipe = {
      request: {
        query: HandleSwipeDocument,
        variables: { input: { cardId: "q-1", cardgroupId: CG_ID, rating: 4 } },
      },
      result: {
        data: {
          handleSwipe: {
            __typename: "HandleSwipeSuccess" as const,
            response: {
              __typename: "SwipeResponse" as const,
              performanceMode: "DIFFICULT",
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
      cefrLevel: null,
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
        variables: { input: { cardId: "q-1", cardgroupId: CG_ID, rating: 4 } },
      },
      result: {
        data: {
          handleSwipe: {
            __typename: "HandleSwipeSuccess" as const,
            response: {
              __typename: "SwipeResponse" as const,
              performanceMode: "DIFFICULT",
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

  it("does not re-append a still-in-flight swiped card returned by a racing prefetch", async () => {
    // Regression for issue #789. `onSwipe` optimistically removes the swiped
    // card then awaits `handleSwipe`. The removal shrinks the queue past the
    // threshold and fires a network-only `LearnNextDueCards` prefetch WHILE the
    // swipe mutation is still in flight. If the backend read beats the FSRS
    // write commit, the just-swiped card still reads as "due" and comes back in
    // the batch; because the dedup `seen` set is built from the post-removal
    // queue (which no longer holds the card), it would be re-appended — a
    // duplicate FSRS review. The session-scoped swiped-id filter drops it while
    // still merging a genuinely new card from the same batch.
    const initial = makeQueue(PREFETCH_THRESHOLD + 1); // q-1..q-6 — above threshold, no mount prefetch
    const swipedCard = initial[0] as PrefetchCard; // q-1, "Front 1"
    const freshCard: PrefetchCard = {
      __typename: "Card" as const,
      id: "p-new",
      front: "Prefetched New",
      back: "Prefetched Back",
      cefrLevel: null,
      userCardState: userCardState("2026-04-30T00:00:00Z", 0),
      cardgroupId: CG_ID,
    };
    // The racing prefetch returns the just-swiped card AND a genuinely new card.
    const prefetch = makePrefetchMock([swipedCard, freshCard]);
    // Hold `handleSwipe` open so the prefetch resolves and merges while the swipe
    // mutation is still pending (the FSRS write has not committed).
    const swipe = {
      request: {
        query: HandleSwipeDocument,
        variables: { input: { cardId: "q-1", cardgroupId: CG_ID, rating: 4 } },
      },
      delay: 80,
      result: {
        data: {
          handleSwipe: {
            __typename: "HandleSwipeSuccess" as const,
            response: {
              __typename: "SwipeResponse" as const,
              performanceMode: "DIFFICULT",
            },
          },
        },
      },
    };

    const user = userEvent.setup();
    renderLearnClient([prefetch.mock, swipe], initial, { skipDefaultPrefetchMocks: true });

    // Swipe q-1 — queue shrinks 6 → 5, crossing the threshold and firing the
    // racing prefetch while `handleSwipe` is still pending.
    await user.click(screen.getByRole("button", { name: "Rate as Easy" }));

    // The racing prefetch fires and resolves.
    await waitFor(() => {
      expect(prefetch.callCount()).toBe(1);
    });

    // The genuinely new card IS merged (proving the merge ran and dedup filtered
    // ONLY the swiped id)...
    await waitFor(() => {
      const latest = capturedCardSnapshots.at(-1);
      expect(latest?.map((c) => c.id)).toContain("p-new");
    });

    // ...but the just-swiped, still-in-flight card is NOT re-appended.
    const merged = capturedCardSnapshots.at(-1);
    const ids = merged?.map((c) => c.id) ?? [];
    expect(ids).not.toContain("q-1");
  });

  it("does not re-fire prefetch on tail swipes once the due pool is exhausted", async () => {
    // Perf regression for issue #789. Once `LearnNextDueCards` returns nothing
    // due, the pool is exhausted; every subsequent tail swipe (queue 5 → 4 → …)
    // would otherwise fire another redundant network-only 20-card query that
    // returns nothing new. The exhaustion guard suppresses those until a swipe
    // succeeds (which may make a rated card due again).
    //
    // Only ONE prefetch mock is supplied (the mount fetch). A second, unmatched
    // prefetch dispatched by the tail swipe would trip the file-wide
    // MockedProvider leak spy (`assertNoLeaks` in the shared afterEach) — the
    // same negative-assertion idiom the "does not prefetch when queue is above
    // threshold / empty" tests above rely on.
    const initial = makeQueue(PREFETCH_THRESHOLD); // q-1..q-5 — at threshold
    const prefetch = makePrefetchMock([]); // mount fetch → nothing due → exhausted
    const swipe = {
      request: {
        query: HandleSwipeDocument,
        variables: { input: { cardId: "q-1", cardgroupId: CG_ID, rating: 4 } },
      },
      result: {
        data: {
          handleSwipe: {
            __typename: "HandleSwipeSuccess" as const,
            response: {
              __typename: "SwipeResponse" as const,
              performanceMode: "DIFFICULT",
            },
          },
        },
      },
    };

    const user = userEvent.setup();
    renderLearnClient([prefetch.mock, swipe], initial, { skipDefaultPrefetchMocks: true });

    // Mount prefetch fires once and finds nothing due.
    await waitFor(() => {
      expect(prefetch.callCount()).toBe(1);
    });
    // Let the resolved prefetch's `.then` run so the exhaustion guard is set
    // before the swipe re-crosses the threshold.
    await act(async () => {
      await new Promise((resolve) => setTimeout(resolve, 0));
    });

    // Swipe the head card — queue shrinks 5 → 4, re-crossing the threshold. With
    // no exhaustion guard this dispatches a second, unmatched prefetch.
    await user.click(screen.getByRole("button", { name: "Rate as Easy" }));

    // Queue advanced to the next card.
    await waitFor(() => {
      expect(screen.getByText("Front 2")).toBeInTheDocument();
    });
    // Give any (buggy) second prefetch dispatch a chance to surface as a leak.
    await act(async () => {
      await new Promise((resolve) => setTimeout(resolve, 0));
    });

    // Still exactly the one mount prefetch — the tail swipe fired no further
    // query (a second dispatch also trips `assertNoLeaks` in afterEach).
    expect(prefetch.callCount()).toBe(1);
  });

  it("resumes prefetch after a successful swipe clears the exhaustion guard", async () => {
    // Recovery counterpart to the suppression test above (regression guard for
    // issue #789). Once the pool is exhausted (`exhaustedRef` set), a swipe that
    // SUCCEEDS must re-open prefetching: the exhaustion verdict is a
    // point-in-time snapshot that newly-due cards can outdate, so
    // `HandleSwipeSuccess` resets `exhaustedRef`. The NEXT tail swipe that
    // re-crosses the threshold then dispatches a fresh prefetch. If the
    // `exhaustedRef.current = false` reset on `HandleSwipeSuccess` were removed,
    // the guard would stay set for the rest of the session and this second
    // prefetch would never fire — the assertions below go red.
    //
    // Three `LearnNextDueCards` mocks are consumed in order:
    //  1. mount fetch → nothing due → exhausted;
    //  2. recovery fetch (after the second swipe) → a genuinely due card,
    //     proving prefetch resumed;
    //  3. merging that single card leaves the queue at 4 (still ≤ threshold), so
    //     the effect fires once more — a terminating empty fetch that
    //     re-exhausts the pool and stops the cascade (an unmatched fourth
    //     dispatch would trip `assertNoLeaks` in the shared afterEach).
    // This mirrors the "warns and re-allows prefetch after a failed attempt"
    // recovery convention (second prefetch mock + a swipe that re-crosses the
    // threshold), extended with the extra swipe the exhaustion guard requires.
    const initial = makeQueue(PREFETCH_THRESHOLD); // q-1..q-5 — at threshold
    const mountPrefetch = makePrefetchMock([]); // mount fetch → nothing due → exhausted
    const recoveryCard: PrefetchCard = {
      __typename: "Card" as const,
      id: "r-recovery",
      front: "Recovered Due Card",
      back: "Recovered Back",
      cefrLevel: null,
      userCardState: userCardState("2026-04-30T00:00:00Z", 0),
      cardgroupId: CG_ID,
    };
    const recovery = makePrefetchMock([recoveryCard]); // after the 2nd swipe → a due card
    const terminator = makePrefetchMock([]); // post-merge fire re-exhausts, ending the cascade
    // Each swipe mock tracks whether its result fn ran so the test can wait for
    // the FIRST swipe's mutation to actually resolve (which clears the guard)
    // before dispatching the second swipe — the optimistic queue advance is
    // synchronous and would otherwise let the second swipe race ahead of the
    // guard reset.
    const swipe1 = makeSwipeSuccessMock("q-1");
    const swipe2 = makeSwipeSuccessMock("q-2");

    const user = userEvent.setup();
    renderLearnClient(
      [mountPrefetch.mock, recovery.mock, terminator.mock, swipe1.mock, swipe2.mock],
      initial,
      { skipDefaultPrefetchMocks: true },
    );

    // Mount prefetch fires once and finds nothing due → exhaustion guard set.
    await waitFor(() => {
      expect(mountPrefetch.callCount()).toBe(1);
    });
    // Let the resolved mount prefetch's `.then` run so `exhaustedRef` is set.
    await act(async () => {
      await new Promise((resolve) => setTimeout(resolve, 0));
    });

    // First swipe (q-1) succeeds. The optimistic removal shrinks 5 → 4 and
    // re-fires the effect, but the guard is still set at that instant, so no
    // prefetch fires here — the swipe's `HandleSwipeSuccess` then clears the guard.
    await user.click(screen.getByRole("button", { name: "Rate as Easy" }));
    // Wait for the first swipe's mutation to actually resolve (its result fn
    // runs)...
    await waitFor(() => {
      expect(swipe1.wasCalled()).toBe(true);
    });
    // ...then drain microtasks so the `HandleSwipeSuccess` continuation clears
    // `exhaustedRef` BEFORE the second swipe re-crosses the threshold. Without
    // this ordering the guard would still be set when the second swipe fires.
    await act(async () => {
      await new Promise((resolve) => setTimeout(resolve, 0));
    });
    // Queue advanced to the next card (q-2 is now head).
    await waitFor(() => {
      expect(screen.getByText("Front 2")).toBeInTheDocument();
    });
    // The first swipe did NOT resume prefetch — the guard held while it ran.
    expect(recovery.callCount()).toBe(0);

    // Second swipe (q-2) shrinks 4 → 3, re-crossing the threshold with the guard
    // now cleared. This is the dispatch the exhaustion guard would suppress if it
    // never reset — so it must fire the recovery prefetch.
    await user.click(screen.getByRole("button", { name: "Rate as Easy" }));

    // The recovery prefetch fires (its result fn runs)...
    await waitFor(() => {
      expect(recovery.callCount()).toBe(1);
    });
    // ...and the due card it returns is merged into the queue, proving prefetch
    // resumed after the successful swipe cleared the exhaustion guard.
    await waitFor(() => {
      const latest = capturedCardSnapshots.at(-1);
      expect(latest?.map((c) => c.id)).toContain("r-recovery");
    });
  });

  it("does not re-append a card whose prefetch snapshot predates its swipe commit", async () => {
    // Settled-swipe counterpart to the in-flight test above. Here the swipe
    // mutation resolves FIRST and the racing prefetch — whose server read ran
    // before the FSRS write committed — resolves afterwards, still carrying the
    // swiped card. A filter keyed on in-flight ids alone would have released the
    // id by then and let the card back into the queue, where rating it a second
    // time records a duplicate same-day FSRS review. The server never returns a
    // same-day-swiped card (`StartOfLearnDay` in
    // backend/internal/domain/learn_day.go), so the session-scoped swiped-id set
    // treats such a batch entry as the stale read it is.
    const initial = makeQueue(PREFETCH_THRESHOLD + 1); // q-1..q-6 — above threshold, no mount prefetch
    const swipedCard = initial[0] as PrefetchCard; // q-1, "Front 1"
    const freshCard: PrefetchCard = {
      __typename: "Card" as const,
      id: "p-late",
      front: "Late Prefetched",
      back: "Late Back",
      cefrLevel: null,
      userCardState: userCardState("2026-04-30T00:00:00Z", 0),
      cardgroupId: CG_ID,
    };
    // The stale batch is held open long enough for the swipe mutation to settle
    // first; the settle is awaited explicitly below rather than inferred from
    // the delay, so the ordering the regression depends on is deterministic.
    const prefetch = makePrefetchMock([swipedCard, freshCard], { delay: 300 });
    // No delay — `handleSwipe` settles before the prefetch response lands.
    const swipe = makeSwipeSuccessMock("q-1");

    const user = userEvent.setup();
    renderLearnClient([prefetch.mock, swipe.mock], initial, { skipDefaultPrefetchMocks: true });

    // Swipe q-1 — queue shrinks 6 → 5, crossing the threshold and firing the
    // prefetch whose snapshot still lists q-1 as due.
    await user.click(screen.getByRole("button", { name: "Rate as Easy" }));

    // Wait for the mutation to resolve, then drain microtasks so its
    // continuation has fully run before the stale batch merges.
    await waitFor(() => {
      expect(swipe.wasCalled()).toBe(true);
    });
    await act(async () => {
      await new Promise((resolve) => setTimeout(resolve, 0));
    });

    await waitFor(
      () => {
        expect(prefetch.callCount()).toBe(1);
      },
      { timeout: 1000 },
    );

    // The genuinely new card IS merged, proving the merge ran...
    await waitFor(() => {
      const latest = capturedCardSnapshots.at(-1);
      expect(latest?.map((c) => c.id)).toContain("p-late");
    });

    // ...while the settled, already-swiped card stays out of the queue and is
    // never rendered again.
    const merged = capturedCardSnapshots.at(-1);
    expect(merged?.map((c) => c.id) ?? []).not.toContain("q-1");
    expect(screen.queryByText("Front 1")).not.toBeInTheDocument();
  });

  it("resets the swiped-session filter when the active cardgroup changes", async () => {
    // The swiped-id set is scoped to one deck's session: a different cardgroup
    // has its own due pool, so an id recorded against the previous deck must not
    // suppress a legitimate prefetch result after the switch. This mirrors the
    // reset of `exhaustedRef` in the same guard.
    const OTHER_CG_ID = "cg-2";
    const initial = makeQueue(PREFETCH_THRESHOLD + 1); // q-1..q-6 — above threshold
    const swipedCard = initial[0] as PrefetchCard; // q-1, "Front 1"
    // Deck A's prefetch (fired by the optimistic removal) finds nothing due.
    const prefetchA = makePrefetchMock([]);
    // Deck B returns a card with the id swiped in deck A. Reusing the id is the
    // point of the fixture — the filter must not carry across decks.
    const prefetchB = makePrefetchMock([{ ...swipedCard, cardgroupId: OTHER_CG_ID }], {
      cardgroupId: OTHER_CG_ID,
    });
    const swipe = makeSwipeSuccessMock("q-1");
    // The persist-last-viewed effect fires once per cardgroup, so both decks
    // need a no-op mock or the file-wide leak spy records an unmatched request.
    const persistB = {
      request: {
        query: SetLastViewedCardgroupDocument,
        variables: { cardgroupId: OTHER_CG_ID },
      },
      result: {
        data: {
          setLastViewedCardgroup: {
            __typename: "SetLastViewedCardgroupSuccess" as const,
            user: {
              __typename: "User" as const,
              id: "u-default",
              lastViewedCardgroup: { __typename: "Cardgroup" as const, id: OTHER_CG_ID },
            },
          },
        },
      },
    };

    const mocks = [prefetchA.mock, prefetchB.mock, swipe.mock, makeDefaultPersistMock(), persistB];

    const user = userEvent.setup();
    const { rerender } = renderWithIntl(
      <MockedProvider mocks={mocks as never}>
        <LearnClient cardgroupId={CG_ID} initialCards={initial} displayMode="ALWAYS_VISIBLE" />
      </MockedProvider>,
    );

    // Swipe q-1 in deck A — queue shrinks 6 → 5 and deck A's prefetch resolves
    // empty, recording q-1 in the session set along the way.
    await user.click(screen.getByRole("button", { name: "Rate as Easy" }));
    await waitFor(() => {
      expect(prefetchA.callCount()).toBe(1);
    });
    // Let the resolved prefetch's `.then` and the in-flight reset run before the
    // deck switch re-evaluates the effect.
    await act(async () => {
      await new Promise((resolve) => setTimeout(resolve, 0));
    });

    rerender(
      <MockedProvider mocks={mocks as never}>
        <LearnClient
          cardgroupId={OTHER_CG_ID}
          initialCards={initial}
          displayMode="ALWAYS_VISIBLE"
        />
      </MockedProvider>,
    );

    // The deck change resets the guards, so the queue (still at threshold)
    // dispatches deck B's prefetch.
    await waitFor(() => {
      expect(prefetchB.callCount()).toBe(1);
    });
    // q-1 is appended: the previous deck's swipe no longer filters it.
    await waitFor(() => {
      const latest = capturedCardSnapshots.at(-1);
      expect(latest?.map((c) => c.id)).toContain("q-1");
    });
  });

  it("keeps a swiped id filtered out of a later prefetch within the same JST learn day", async () => {
    // Same-day control for the rollover test below. The clock stays inside one
    // JST learn day, so the swiped-id set must survive and drop q-1 from the
    // second batch while still admitting that batch's genuinely new card.
    const user = userEvent.setup();
    vi.useFakeTimers({ shouldAdvanceTime: true });
    // 2026-07-20T14:00:00Z is 23:00 JST — well inside the 2026-07-20 learn day.
    vi.setSystemTime(new Date("2026-07-20T14:00:00.000Z"));

    const initial = makeQueue(PREFETCH_THRESHOLD + 1); // q-1..q-6 — above threshold, no mount prefetch
    const swipedCard = initial[0] as PrefetchCard; // q-1, "Front 1"
    const makeFresh = (id: string): PrefetchCard => ({
      __typename: "Card" as const,
      id,
      front: `Fresh ${id}`,
      back: `Fresh back ${id}`,
      cefrLevel: null,
      userCardState: userCardState("2026-04-30T00:00:00Z", 0),
      cardgroupId: CG_ID,
    });
    // First batch tops the queue back above threshold so no cascade follows.
    const firstPrefetch = makePrefetchMock([makeFresh("p-1")]);
    // Second batch re-offers the already-swiped q-1 alongside a new card.
    const secondPrefetch = makePrefetchMock([swipedCard, makeFresh("p-2")]);

    renderLearnClient(
      [
        firstPrefetch.mock,
        secondPrefetch.mock,
        makeSwipeSuccessMock("q-1").mock,
        makeSwipeSuccessMock("q-2").mock,
      ],
      initial,
      { skipDefaultPrefetchMocks: true },
    );

    // Swipe q-1: queue 6 → 5 fires the first prefetch, which merges p-1 back to 6.
    await user.click(screen.getByRole("button", { name: "Rate as Easy" }));
    await waitFor(() => {
      expect(firstPrefetch.callCount()).toBe(1);
    });
    await waitFor(() => {
      const latest = capturedCardSnapshots.at(-1);
      expect(latest?.map((c) => c.id)).toContain("p-1");
    });

    // Swipe q-2: queue 6 → 5 fires the second prefetch, still on the same day.
    await user.click(screen.getByRole("button", { name: "Rate as Easy" }));
    await waitFor(() => {
      expect(secondPrefetch.callCount()).toBe(1);
    });
    await waitFor(() => {
      const latest = capturedCardSnapshots.at(-1);
      expect(latest?.map((c) => c.id)).toContain("p-2");
    });

    // The merge ran (p-2 landed) but q-1 stayed filtered — same-day behaviour
    // is unchanged.
    const merged = capturedCardSnapshots.at(-1);
    expect(merged?.map((c) => c.id) ?? []).not.toContain("q-1");

    vi.useRealTimers();
  });

  it("clears the swiped-id set and the exhaustion verdict when the JST learn day rolls over", async () => {
    // A learner studying at 23:55 JST keeps the tab open past midnight. The
    // server's learn day advances at 15:00 UTC and legitimately re-serves cards
    // swiped on the previous day, so both session guards must be dropped: the
    // swiped-id set (or every re-served card is filtered out of the merge) and
    // the exhaustion verdict (or the effect never dispatches the refill query).
    const user = userEvent.setup();
    vi.useFakeTimers({ shouldAdvanceTime: true });
    // 2026-07-20T14:55:00Z is 23:55 JST — five minutes before the rollover.
    vi.setSystemTime(new Date("2026-07-20T14:55:00.000Z"));

    // Three cards, so the queue can actually reach zero before the refill lands:
    // the caught-up screen renders only on `queue.length === 0`, so a fixture
    // that stops short of an empty queue would assert its absence vacuously.
    const initial = makeQueue(3); // q-1..q-3 — below threshold, mount prefetch fires
    const swipedCard = initial[0] as PrefetchCard; // q-1, "Front 1"
    const mountPrefetch = makePrefetchMock([]); // nothing else due today → exhausted
    // The new learn day re-serves q-1, held open long enough for the final swipe
    // to drain the queue to zero first — so the caught-up screen is genuinely on
    // screen when the merge decides whether q-1 survives the swiped-id filter.
    const newDayPrefetch = makePrefetchMock([swipedCard], { delay: 300 });
    const terminator = makePrefetchMock([]); // post-merge fire re-exhausts, ending the cascade
    // q-1's mutation is held open past the second swipe. Its `HandleSwipeSuccess`
    // continuation clears the exhaustion verdict on its own, so letting it settle
    // early would mask whether the rollover reset did any work.
    const heldSwipe = { ...makeSwipeSuccessMock("q-1").mock, delay: 250 };
    const rolloverSwipe = makeSwipeSuccessMock("q-2");
    const drainSwipe = makeSwipeSuccessMock("q-3");

    renderLearnClient(
      [
        mountPrefetch.mock,
        newDayPrefetch.mock,
        terminator.mock,
        heldSwipe,
        rolloverSwipe.mock,
        drainSwipe.mock,
      ],
      initial,
      { skipDefaultPrefetchMocks: true },
    );

    // Mount prefetch finds nothing due → exhaustion guard set.
    await waitFor(() => {
      expect(mountPrefetch.callCount()).toBe(1);
    });
    await act(async () => {
      await new Promise((resolve) => setTimeout(resolve, 0));
    });

    // Swipe q-1 inside the old learn day: the id enters the session set and the
    // queue drops to 2, but the exhaustion guard suppresses any prefetch.
    await user.click(screen.getByRole("button", { name: "Rate as Easy" }));

    // JST midnight passes while that mutation is still in flight.
    vi.setSystemTime(new Date("2026-07-20T15:01:00.000Z"));

    // The next tail swipe re-runs the prefetch effect, which now reads a new
    // learn-day key and drops both guards before its threshold checks.
    await user.click(screen.getByRole("button", { name: "Rate as Easy" }));

    // Drain the last card while that refill is still in flight. The queue is now
    // empty, so the caught-up screen is what the learner sees — the state the
    // issue reports as stuck.
    await user.click(screen.getByRole("button", { name: "Rate as Easy" }));
    expect(
      screen.getByRole("heading", { name: "Today's learning is complete" }),
    ).toBeInTheDocument();

    // The refill query fires — proof the exhaustion verdict was cleared.
    await waitFor(() => {
      expect(newDayPrefetch.callCount()).toBe(1);
    });
    // q-1 is merged back in — proof the swiped-id set was cleared.
    await waitFor(() => {
      const latest = capturedCardSnapshots.at(-1);
      expect(latest?.map((c) => c.id)).toContain("q-1");
    });
    // ...so the session is no longer declared complete. Without the rollover
    // reset the merge yields zero additions and the queue stays empty, leaving
    // this heading on screen.
    expect(
      screen.queryByRole("heading", { name: "Today's learning is complete" }),
    ).not.toBeInTheDocument();

    // Let the held-open first mutation and the terminating prefetch settle so
    // teardown does not race an in-flight request.
    await waitFor(
      () => {
        expect(terminator.callCount()).toBe(1);
      },
      { timeout: 2000 },
    );
    await act(async () => {
      await new Promise((resolve) => setTimeout(resolve, 400));
    });

    vi.useRealTimers();
  });

  it("keeps a card swiped after the rollover filtered out of the prefetch that swipe fires", async () => {
    // The rollover reset must run BEFORE the swipe records its id, not only
    // inside the prefetch effect. The first post-rollover effect run is the one
    // the boundary-crossing swipe itself triggers, so an effect-only reset wipes
    // that swipe's freshly-recorded id and reopens the double-rate race: the
    // card's `last_review` still predates the new learn day while its mutation
    // is in flight, so the server's REVIEW window legitimately returns it, and
    // an empty swiped-id set lets it back into the queue to be rated twice.
    const user = userEvent.setup();
    vi.useFakeTimers({ shouldAdvanceTime: true });
    // 2026-07-20T14:59:00Z is 23:59 JST — the last minute of the old learn day.
    vi.setSystemTime(new Date("2026-07-20T14:59:00.000Z"));

    const initial = makeQueue(PREFETCH_THRESHOLD + 1); // q-1..q-6 — above threshold, no mount prefetch
    const swipedCard = initial[0] as PrefetchCard; // q-1, "Front 1"
    const freshCard: PrefetchCard = {
      __typename: "Card" as const,
      id: "p-new-day",
      front: "New Day Card",
      back: "New Day Back",
      cefrLevel: null,
      userCardState: userCardState("2026-04-30T00:00:00Z", 0),
      cardgroupId: CG_ID,
    };
    // The batch the crossing swipe fires re-offers that same swipe's card
    // alongside a genuinely new one, so the merge is observable either way.
    const rolloverPrefetch = makePrefetchMock([swipedCard, freshCard]);
    const swipe = makeSwipeSuccessMock("q-1");

    renderLearnClient([rolloverPrefetch.mock, swipe.mock], initial, {
      skipDefaultPrefetchMocks: true,
    });

    // JST midnight passes before the learner swipes again, so this swipe belongs
    // to the new learn day and its id must survive the rollover reset.
    vi.setSystemTime(new Date("2026-07-20T15:00:01.000Z"));

    // Swipe q-1 — queue shrinks 6 → 5, crossing the threshold and firing the
    // prefetch whose batch still lists q-1 as due.
    await user.click(screen.getByRole("button", { name: "Rate as Easy" }));

    await waitFor(() => {
      expect(rolloverPrefetch.callCount()).toBe(1);
    });
    // The genuinely new card IS merged, proving the merge ran (and that the
    // exhaustion verdict did not suppress the query)...
    await waitFor(() => {
      const latest = capturedCardSnapshots.at(-1);
      expect(latest?.map((c) => c.id)).toContain("p-new-day");
    });
    // ...while the card swiped after the rollover stays out of the queue.
    const merged = capturedCardSnapshots.at(-1);
    expect(merged?.map((c) => c.id) ?? []).not.toContain("q-1");
    expect(screen.queryByText("Front 1")).not.toBeInTheDocument();

    vi.useRealTimers();
  });
});

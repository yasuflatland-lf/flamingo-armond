// @vitest-environment jsdom
import { readFileSync } from "node:fs";
import { join } from "node:path";
import { MockedProvider } from "@apollo/client/testing/react";
import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { GraphQLError } from "graphql";
import { type RefObject, useImperativeHandle, useRef } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { CardContent, type SwipeCardData } from "@/components/learn/swipe-card";
import type { SwipeCardStackHandle } from "@/components/learn/swipe-card-stack";
import { PracticeTodaysCardsDocument } from "@/generated/graphql";
import { renderWithIntl } from "@/test/render-with-intl";
import {
  type ApolloMockLeakSpyResult,
  installApolloMockLeakSpy,
} from "../../../../__tests__/utils/mock-apollo-paginated";
import { PracticeClient } from "./practice-client";

// ---------------------------------------------------------------------------
// SwipeCardStack mock — same shape as learn-client.test.tsx.
//
// React 19 passes refs as plain props, so the mock accepts a `ref` prop and
// wires it to useImperativeHandle. triggerSwipe calls onCardSwiped with the
// first card in the `cards` array. The real CardContent is rendered so the
// active card's front text is queryable without the next/dynamic AnimatedCard
// chunk.
// ---------------------------------------------------------------------------
type SwipeCardStackOnCardSwiped = Parameters<
  typeof import("@/components/learn/swipe-card-stack")["SwipeCardStack"]
>[0]["onCardSwiped"];

vi.mock("@/components/learn/swipe-card-stack", () => ({
  SwipeCardStack: (props: {
    cards: SwipeCardData[];
    onCardSwiped: SwipeCardStackOnCardSwiped;
    completedCount?: number;
    ref?: RefObject<SwipeCardStackHandle | null>;
  }) => {
    const activeCardRef = useRef(props.cards[0] ?? null);
    activeCardRef.current = props.cards[0] ?? null;

    useImperativeHandle(props.ref, () => ({
      triggerSwipe: (direction: "left" | "right" | "down") => {
        const card = activeCardRef.current;
        if (card) props.onCardSwiped(card, direction);
      },
    }));

    const activeCard = props.cards[0];
    if (!activeCard) return <div>Stack empty</div>;
    return <CardContent card={activeCard} revealed={true} />;
  },
}));

// ---------------------------------------------------------------------------
// LearnActionBar mock — exposes onRate + disabled, identical to learn-client.test.tsx.
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
// File-wide MockedProvider leak spy. The PracticeClient must never fire any
// mutation. `operationNames` lists three names: the query "PracticeTodaysCards"
// plus the two swipe-path mutations "HandleSwipe" and "SetLastViewedCardgroup".
// The two mutation names are INCLUDED deliberately — with NO mutation mocks
// provided, any stray mutation fired by the practice path surfaces as an
// unmatched request that `assertNoLeaks` catches as a leak.
// ---------------------------------------------------------------------------
let leakSpy: ApolloMockLeakSpyResult;

beforeEach(() => {
  vi.useRealTimers();
  leakSpy = installApolloMockLeakSpy({
    operationNames: ["PracticeTodaysCards", "HandleSwipe", "SetLastViewedCardgroup"],
  });
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

function makeCard(id: string, front: string, back: string): SwipeCardData & { __typename: "Card" } {
  return {
    __typename: "Card" as const,
    id,
    front,
    back,
    cefrLevel: null,
    userCardState: userCardState("2026-04-30T00:00:00Z", 0),
    cardgroupId: CG_ID,
  };
}

/** Build a PracticeTodaysCards mock returning `cards`, tracking call count. */
function makePracticeMock(cards: ReturnType<typeof makeCard>[], opts?: { delay?: number }) {
  let calls = 0;
  const mock = {
    request: {
      query: PracticeTodaysCardsDocument,
      variables: { cardgroupId: CG_ID },
    },
    ...(opts?.delay !== undefined ? { delay: opts.delay } : {}),
    result: () => {
      calls += 1;
      return { data: { practiceTodaysCards: cards } };
    },
  };
  return { mock, callCount: () => calls };
}

/** Build a PracticeTodaysCards mock that fails with a network error. */
function makePracticeErrorMock() {
  return {
    request: {
      query: PracticeTodaysCardsDocument,
      variables: { cardgroupId: CG_ID },
    },
    error: new Error("network down"),
  };
}

/**
 * Build a PracticeTodaysCards mock that fails with a FIELD-level BAD_USER_INPUT
 * GraphQL error (e.g. opening practice for a deleted cardgroup, so the backend
 * rejects with `extensions.field == "cardgroupId"`). For this shape
 * `getBackendErrorBanner` returns `undefined`, so the component must render its
 * own fallback copy instead of an empty banner.
 */
function makePracticeFieldErrorMock() {
  return {
    request: {
      query: PracticeTodaysCardsDocument,
      variables: { cardgroupId: CG_ID },
    },
    result: {
      errors: [
        new GraphQLError("cardgroup not found", {
          extensions: { code: "BAD_USER_INPUT", field: "cardgroupId" },
        }),
      ],
    },
  };
}

function renderPractice(mocks: unknown[]) {
  renderWithIntl(
    <MockedProvider mocks={mocks as never}>
      <PracticeClient cardgroupId={CG_ID} />
    </MockedProvider>,
  );
}

/**
 * Assert the structured `[PracticeClient] query failed` log fired and that its
 * payload omits a `message` key (PII rule — backend messages may echo
 * user-authored card content). Returns the payload so a caller can make
 * additional shape assertions (e.g. `cardgroupId`).
 */
function expectMessageFreeFailureLog(
  consoleErrorSpy: ReturnType<typeof vi.spyOn>,
): Record<string, unknown> {
  const logCall = consoleErrorSpy.mock.calls.find(
    (call: unknown[]) => call[0] === "[PracticeClient] query failed",
  );
  expect(logCall).toBeDefined();
  const payload = logCall?.[1] as Record<string, unknown>;
  expect(payload).not.toHaveProperty("message");
  return payload;
}

describe("<PracticeClient>", () => {
  it("renders the card stack and the persistent practice banner for a non-empty pool", async () => {
    const { mock } = makePracticeMock([
      makeCard("c-1", "Hello", "Hola"),
      makeCard("c-2", "Bye", "Adios"),
      makeCard("c-3", "Yes", "Si"),
    ]);
    renderPractice([mock]);

    await waitFor(() => {
      expect(screen.getByText("Hello")).toBeInTheDocument();
    });
    expect(screen.getByText("Practice — swipes aren't recorded")).toBeInTheDocument();
  });

  it("shows the empty terminal state with no Study again button when the pool is empty", async () => {
    const { mock } = makePracticeMock([]);
    renderPractice([mock]);

    await waitFor(() => {
      expect(
        screen.getByRole("heading", { name: "No cards practiced today yet" }),
      ).toBeInTheDocument();
    });
    expect(screen.queryByRole("button", { name: "Study again" })).not.toBeInTheDocument();
    // No practice swipes can be recorded, so no mutation must have fired.
    leakSpy.assertNoLeaks();
  });

  it("advances the round locally without firing any mutation; retiring all cards completes the round", async () => {
    const user = userEvent.setup();
    const { mock } = makePracticeMock([
      makeCard("c-1", "Hello", "Hola"),
      makeCard("c-2", "Bye", "Adios"),
    ]);
    renderPractice([mock]);

    await waitFor(() => {
      expect(screen.getByText("Hello")).toBeInTheDocument();
    });

    // Swipe Easy retires the active card. Repeat until the queue is empty.
    await user.click(screen.getByRole("button", { name: "Rate as Easy" }));
    await waitFor(() => {
      expect(screen.getByText("Bye")).toBeInTheDocument();
    });
    await user.click(screen.getByRole("button", { name: "Rate as Easy" }));

    await waitFor(() => {
      expect(screen.getByRole("heading", { name: "Practice complete" })).toBeInTheDocument();
    });
    expect(screen.getByRole("button", { name: "Study again" })).toBeInTheDocument();

    // The whole flow must not have fired any mutation (leak spy clean).
    leakSpy.assertNoLeaks();
  });

  it("re-queues the card on Again so the round does not complete", async () => {
    const user = userEvent.setup();
    // Two-card pool: swiping the active card Again re-inserts it, so the round
    // can never empty from a single Again swipe.
    const { mock } = makePracticeMock([
      makeCard("c-1", "Hello", "Hola"),
      makeCard("c-2", "Bye", "Adios"),
    ]);
    renderPractice([mock]);

    await waitFor(() => {
      expect(screen.getByText("Hello")).toBeInTheDocument();
    });

    await user.click(screen.getByRole("button", { name: "Rate as Again" }));

    // The next card becomes active, but the round is NOT complete — the swiped
    // card was re-queued, so the queue is still non-empty and the banner stays.
    await waitFor(() => {
      expect(screen.getByText("Bye")).toBeInTheDocument();
    });
    expect(screen.queryByRole("heading", { name: "Practice complete" })).not.toBeInTheDocument();
    expect(screen.getByText("Practice — swipes aren't recorded")).toBeInTheDocument();
    leakSpy.assertNoLeaks();
  });

  it("restarts the round and re-fetches a fresh pool when Study again is clicked", async () => {
    const user = userEvent.setup();
    const first = makePracticeMock([makeCard("c-1", "Hello", "Hola")]);
    const second = makePracticeMock([makeCard("c-9", "Restart", "Reinicio")]);
    // Two mocks for the same request: the first satisfies the initial mount,
    // the second satisfies the refetch triggered by Study again.
    renderPractice([first.mock, second.mock]);

    await waitFor(() => {
      expect(screen.getByText("Hello")).toBeInTheDocument();
    });

    // Retire the only card → round complete.
    await user.click(screen.getByRole("button", { name: "Rate as Easy" }));
    await waitFor(() => {
      expect(screen.getByRole("heading", { name: "Practice complete" })).toBeInTheDocument();
    });

    // Study again triggers a second fetch and re-seeds the round.
    await user.click(screen.getByRole("button", { name: "Study again" }));

    await waitFor(() => {
      expect(second.callCount()).toBe(1);
    });
    await waitFor(() => {
      expect(screen.getByText("Restart")).toBeInTheDocument();
    });
    leakSpy.assertNoLeaks();
  });

  it("renders the skeleton while the initial fetch is in flight, before data resolves", async () => {
    // A delayed mock keeps the first request pending so the skeleton branch is
    // observable before the card stack renders.
    const { mock } = makePracticeMock([makeCard("c-1", "Hello", "Hola")], { delay: 30 });
    renderPractice([mock]);

    // While loading with an empty local queue, the LearnSkeleton is shown.
    expect(screen.getByLabelText("Loading flashcards")).toBeInTheDocument();
    expect(screen.queryByText("Hello")).not.toBeInTheDocument();

    // Once the fetch settles, the stack replaces the skeleton.
    await waitFor(() => {
      expect(screen.getByText("Hello")).toBeInTheDocument();
    });
    expect(screen.queryByLabelText("Loading flashcards")).not.toBeInTheDocument();
    leakSpy.assertNoLeaks();
  });

  it("shows an error banner with a Retry button on initial-load failure, and Retry recovers", async () => {
    const consoleErrorSpy = vi.spyOn(console, "error").mockImplementation(() => {});
    const success = makePracticeMock([makeCard("c-1", "Hello", "Hola")]);
    // First request errors (initial load); the second satisfies the Retry.
    renderPractice([makePracticeErrorMock(), success.mock]);

    await waitFor(() => {
      expect(screen.getByRole("alert")).toBeInTheDocument();
    });
    const retry = screen.getByRole("button", { name: "Retry" });
    expect(retry).toBeInTheDocument();

    // Structured log fired once, keyed on the error, WITHOUT a `message` key
    // (PII rule — backend messages may echo user-authored content).
    expect(expectMessageFreeFailureLog(consoleErrorSpy)).toMatchObject({ cardgroupId: CG_ID });

    // Retry re-runs the query; the second (successful) mock recovers the stack.
    const user = userEvent.setup();
    await user.click(retry);

    await waitFor(() => {
      expect(screen.getByText("Hello")).toBeInTheDocument();
    });
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
    leakSpy.assertNoLeaks();
    consoleErrorSpy.mockRestore();
  });

  it("shows an error banner with Retry (not a silent completion screen) when Study-again refetch fails", async () => {
    const consoleErrorSpy = vi.spyOn(console, "error").mockImplementation(() => {});
    const user = userEvent.setup();
    const first = makePracticeMock([makeCard("c-1", "Hello", "Hola")]);
    // First mount succeeds; the Study-again refetch errors.
    renderPractice([first.mock, makePracticeErrorMock()]);

    await waitFor(() => {
      expect(screen.getByText("Hello")).toBeInTheDocument();
    });

    // Retire the only card → round complete.
    await user.click(screen.getByRole("button", { name: "Rate as Easy" }));
    await waitFor(() => {
      expect(screen.getByRole("heading", { name: "Practice complete" })).toBeInTheDocument();
    });

    // Study again triggers the failing refetch.
    await user.click(screen.getByRole("button", { name: "Study again" }));

    // The failure surfaces as the error banner + Retry, NOT a silent fall-back
    // to the completion screen.
    await waitFor(() => {
      expect(screen.getByRole("alert")).toBeInTheDocument();
    });
    expect(screen.getByRole("button", { name: "Retry" })).toBeInTheDocument();
    expect(screen.queryByRole("heading", { name: "Practice complete" })).not.toBeInTheDocument();

    expectMessageFreeFailureLog(consoleErrorSpy);

    leakSpy.assertNoLeaks();
    consoleErrorSpy.mockRestore();
  });

  it("renders fallback banner copy (not an empty box) on a field-level BAD_USER_INPUT error", async () => {
    const consoleErrorSpy = vi.spyOn(console, "error").mockImplementation(() => {});
    // A field-level BAD_USER_INPUT (e.g. deleted cardgroup) makes
    // `getBackendErrorBanner` return undefined; the component must fall back to
    // its own copy so the alert is never an empty red box.
    renderPractice([makePracticeFieldErrorMock()]);

    await waitFor(() => {
      expect(screen.getByRole("alert")).toBeInTheDocument();
    });

    // The banner shows the non-empty fallback string, and a Retry is offered.
    const alert = screen.getByRole("alert");
    expect(alert).toHaveTextContent("Could not load today's practice cards. Please try again.");
    expect(alert.textContent?.trim().length ?? 0).toBeGreaterThan(0);
    expect(screen.getByRole("button", { name: "Retry" })).toBeInTheDocument();

    // The structured failure log still fires, keyed on the error, message-free.
    expectMessageFreeFailureLog(consoleErrorSpy);

    leakSpy.assertNoLeaks();
    consoleErrorSpy.mockRestore();
  });

  it("shows the skeleton (not the completion screen) while a Study-again restart is in flight", async () => {
    const user = userEvent.setup();
    const first = makePracticeMock([makeCard("c-1", "Hello", "Hola")]);
    // The Study-again refetch is delayed so its in-flight window is observable.
    const second = makePracticeMock([makeCard("c-9", "Restart", "Reinicio")], { delay: 40 });
    renderPractice([first.mock, second.mock]);

    await waitFor(() => {
      expect(screen.getByText("Hello")).toBeInTheDocument();
    });

    // Retire the only card → round complete.
    await user.click(screen.getByRole("button", { name: "Rate as Easy" }));
    await waitFor(() => {
      expect(screen.getByRole("heading", { name: "Practice complete" })).toBeInTheDocument();
    });

    // Study again clears the queue and refetches. While the (delayed) refetch is
    // in flight, the skeleton is shown — the stale completion screen must NOT
    // reappear (that would make the button look like a no-op and invite a
    // double-click).
    await user.click(screen.getByRole("button", { name: "Study again" }));

    await waitFor(() => {
      expect(screen.getByLabelText("Loading flashcards")).toBeInTheDocument();
    });
    expect(screen.queryByRole("heading", { name: "Practice complete" })).not.toBeInTheDocument();

    // The new round eventually renders once the refetch settles.
    await waitFor(() => {
      expect(screen.getByText("Restart")).toBeInTheDocument();
    });
    leakSpy.assertNoLeaks();
  });
});

// ---------------------------------------------------------------------------
// Static source guards — practice mode is FSRS-safe by construction.
//
// The imports in practice-client.tsx are module-scoped, so `.toString()` on the
// component cannot see them. Read the source file directly and assert the
// swipe-mutation surface never appears.
// ---------------------------------------------------------------------------
describe("PracticeClient FSRS-safe source guards", () => {
  const source = readFileSync(
    join(process.cwd(), "src/app/learn/[cardgroupId]/practice-client.tsx"),
    "utf8",
  );

  it("never references the HandleSwipe mutation", () => {
    expect(source).not.toContain("HandleSwipeMutation");
  });

  it("never imports or calls useMutation", () => {
    expect(source).not.toContain("useMutation");
  });

  it("never declares an optimisticResponse", () => {
    expect(source).not.toContain("optimisticResponse");
  });
});

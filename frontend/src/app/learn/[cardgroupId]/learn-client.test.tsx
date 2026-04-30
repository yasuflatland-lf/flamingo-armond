// @vitest-environment jsdom
import { MockedProvider } from "@apollo/client/testing/react";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { GraphQLError } from "graphql";
import { describe, expect, it } from "vitest";
import { HandleSwipeDocument } from "@/generated/graphql";
import { LearnClient } from "./learn-client";

const CG_ID = "cg-1";

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

function renderLearnClient(mocks: unknown[], initialCards = [CARD_1]) {
  render(
    <MockedProvider mocks={mocks as never}>
      <LearnClient cardgroupId={CG_ID} initialCards={initialCards} />
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

  it("rolls back the card and shows an error when handleSwipe fails", async () => {
    const user = userEvent.setup();
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
  });

  it("renders an empty-card state with a manage cards link", () => {
    renderLearnClient([], []);

    expect(screen.getByText("No cards to learn")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Manage cards" })).toHaveAttribute(
      "href",
      `/cardgroups/${CG_ID}/cards`,
    );
  });
});

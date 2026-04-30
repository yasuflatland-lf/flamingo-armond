// @vitest-environment jsdom
import { MockedProvider } from "@apollo/client/testing/react";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { expect, it } from "vitest";
import { LearnClient } from "@/app/learn/[cardgroupId]/learn-client";
import { HandleSwipeDocument } from "@/generated/graphql";

const CG_ID = "cg-1";

const CARD = {
  __typename: "Card" as const,
  id: "c-1",
  front: "Hello",
  back: "Hola",
  due: "2026-04-30T00:00:00Z",
  state: 0,
  cardgroupId: CG_ID,
};

const NEXT_CARD = {
  __typename: "Card" as const,
  id: "c-2",
  front: "Next",
  back: "Siguiente",
  due: "2026-04-30T00:00:00Z",
  state: 1,
  cardgroupId: CG_ID,
};

it("renders the adaptive mode badge from the swipe response", async () => {
  const user = userEvent.setup();
  const mock = {
    request: {
      query: HandleSwipeDocument,
      variables: { input: { cardId: CARD.id, cardgroupId: CG_ID, mode: 4 } },
    },
    result: {
      data: {
        handleSwipe: {
          __typename: "SwipeResponse" as const,
          nextCards: [NEXT_CARD],
          performanceMode: 3,
          metrics: {
            __typename: "PerformanceMetrics" as const,
            successRate: 0.87,
            avgDifficulty: 0.38,
            retentionRate: 0.9,
            studyStreak: 4,
            lapseRate: 0.05,
            reviewCount: 32,
          },
        },
      },
    },
  };

  render(
    <MockedProvider mocks={[mock]}>
      <LearnClient cardgroupId={CG_ID} initialCards={[CARD]} />
    </MockedProvider>,
  );

  await user.click(screen.getByRole("button", { name: "Easy" }));

  await waitFor(() => {
    expect(screen.getByText("Mode: Easy")).toBeInTheDocument();
  });
  expect(screen.getByText("87% success")).toBeInTheDocument();
  expect(screen.getByText("Strong run, the next batch can move faster.")).toBeInTheDocument();
});

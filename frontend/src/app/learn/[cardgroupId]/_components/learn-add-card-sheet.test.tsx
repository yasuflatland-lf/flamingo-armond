// @vitest-environment jsdom
import { MockedProvider } from "@apollo/client/testing/react";
import { act, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import { renderWithIntl } from "@/test/render-with-intl";
import { CreateCardDocument } from "@/generated/graphql";
import { LearnAddCardSheet } from "./learn-add-card-sheet";

const userCardState = (due: string, state: number) => ({
  __typename: "UserCardState" as const,
  due,
  state,
});

function dispatchAddCard(cardgroupId: string) {
  const event = new CustomEvent("flamingo:add-card", {
    cancelable: true,
    detail: { cardgroupId },
  });
  act(() => {
    window.dispatchEvent(event);
  });
  return event;
}

afterEach(() => {
  vi.restoreAllMocks();
});

describe("<LearnAddCardSheet>", () => {
  it("opens the add-card drawer when the event targets this cardgroup", async () => {
    renderWithIntl(
      <MockedProvider mocks={[]}>
        <LearnAddCardSheet cardgroupId="cg-1" />
      </MockedProvider>,
    );

    const event = dispatchAddCard("cg-1");

    expect(event.defaultPrevented).toBe(true);
    expect(await screen.findByRole("textbox", { name: /front/i })).toBeInTheDocument();
    expect(screen.getByRole("textbox", { name: /back/i })).toBeInTheDocument();
  });

  it("ignores an add-card event for a different cardgroup", () => {
    renderWithIntl(
      <MockedProvider mocks={[]}>
        <LearnAddCardSheet cardgroupId="cg-1" />
      </MockedProvider>,
    );

    const event = dispatchAddCard("cg-2");

    expect(event.defaultPrevented).toBe(false);
    expect(screen.queryByRole("textbox", { name: /front/i })).toBeNull();
  });

  it("creates a card and closes the drawer on success", async () => {
    const user = userEvent.setup();
    const createMock = {
      request: {
        query: CreateCardDocument,
        variables: { input: { cardgroupId: "cg-1", front: "Hello", back: "World" } },
      },
      result: {
        data: {
          createCard: {
            __typename: "CreateCardSuccess" as const,
            card: {
              __typename: "Card" as const,
              id: "card-new",
              front: "Hello",
              back: "World",
              userCardState: userCardState("2026-05-23T00:00:00Z", 0),
              cardgroupId: "cg-1",
            },
          },
        },
      },
    };

    renderWithIntl(
      <MockedProvider mocks={[createMock]}>
        <LearnAddCardSheet cardgroupId="cg-1" />
      </MockedProvider>,
    );

    dispatchAddCard("cg-1");

    await user.type(await screen.findByRole("textbox", { name: /front/i }), "Hello");
    await user.type(screen.getByRole("textbox", { name: /back/i }), "World");
    await user.click(screen.getByRole("button", { name: /^add$/i }));

    await waitFor(() => {
      expect(screen.queryByRole("textbox", { name: /front/i })).toBeNull();
    });
  });
});

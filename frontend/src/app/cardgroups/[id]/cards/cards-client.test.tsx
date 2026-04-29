// @vitest-environment jsdom
import { MockedProvider } from "@apollo/client/testing/react";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { GraphQLError } from "graphql";
import { describe, expect, it } from "vitest";
import {
  CardsByCardgroupDocument,
  CreateCardDocument,
  DeleteCardDocument,
  UpdateCardDocument,
} from "@/generated/graphql";
import { CardsClient } from "./cards-client";

const CG_ID = "cg-1";

const CARD_1 = {
  __typename: "Card" as const,
  id: "c-1",
  front: "Hello",
  back: "Hola",
  due: "2024-06-15",
  state: 0,
  cardgroupId: CG_ID,
};

const CARD_2 = {
  __typename: "Card" as const,
  id: "c-2",
  front: "Bye",
  back: "Adios",
  due: "2024-06-15",
  state: 0,
  cardgroupId: CG_ID,
};

const NEW_CARD = {
  __typename: "Card" as const,
  id: "c-3",
  front: "Cat",
  back: "Gato",
  due: "2024-06-16",
  state: 0,
  cardgroupId: CG_ID,
};

function makeCreateMock(
  input: { cardgroupId: string; front: string; back: string },
  card = NEW_CARD,
) {
  return {
    request: { query: CreateCardDocument, variables: { input } },
    result: {
      data: {
        createCard: { __typename: "CreateCardPayload" as const, card },
      },
    },
  };
}

function makeUpdateMock(id: string, input: { front: string; back: string }, card = CARD_1) {
  return {
    request: { query: UpdateCardDocument, variables: { id, input } },
    result: {
      data: {
        updateCard: { __typename: "UpdateCardPayload" as const, card },
      },
    },
  };
}

function makeDeleteMock(id: string) {
  return {
    request: { query: DeleteCardDocument, variables: { id } },
    result: { data: { deleteCard: true } },
  };
}

function renderClient(
  mocks: unknown[],
  initialCards = [CARD_1, CARD_2],
  options: { errorPolicy?: boolean } = {},
) {
  const defaultOptions = options.errorPolicy
    ? { mutate: { errorPolicy: "all" as const } }
    : undefined;

  render(
    <MockedProvider mocks={mocks as never} defaultOptions={defaultOptions}>
      <CardsClient cardgroupId={CG_ID} initialCards={initialCards} />
    </MockedProvider>,
  );
}

describe("<CardsClient>", () => {
  it("renders existing cards", () => {
    renderClient([]);
    expect(screen.getByText("Hello")).toBeInTheDocument();
    expect(screen.getByText("Bye")).toBeInTheDocument();
  });

  it("add success appends a new row", async () => {
    const user = userEvent.setup();
    const mock = makeCreateMock({ cardgroupId: CG_ID, front: "Cat", back: "Gato" });

    // Pre-populate cache with existing cards so the update callback can read it.
    const cacheMocks = [
      {
        request: {
          query: CardsByCardgroupDocument,
          variables: { cardgroupId: CG_ID },
        },
        result: { data: { cardsByCardgroup: [CARD_1, CARD_2] } },
      },
      mock,
    ];

    renderClient(cacheMocks);

    const addFrontInput = screen.getAllByLabelText(/front/i)[0] as HTMLElement;
    const addBackInput = screen.getAllByLabelText(/back/i)[0] as HTMLElement;

    // The add form is first
    await user.click(addFrontInput);
    await user.type(addFrontInput, "Cat");
    await user.click(addBackInput);
    await user.type(addBackInput, "Gato");

    await user.click(screen.getByRole("button", { name: /^add$/i }));

    await waitFor(() => {
      expect(screen.getByText("Cat")).toBeInTheDocument();
    });
  });

  it("add BAD_USER_INPUT(field=front) shows inline error", async () => {
    const user = userEvent.setup();

    const mock = {
      request: {
        query: CreateCardDocument,
        variables: { input: { cardgroupId: CG_ID, front: "x", back: "y" } },
      },
      result: {
        errors: [
          new GraphQLError("front is too short", {
            extensions: { code: "BAD_USER_INPUT", field: "front" },
          }),
        ],
      },
    };

    renderClient([mock], [], { errorPolicy: true });

    const frontInput = screen.getByLabelText(/front/i);
    const backInput = screen.getByLabelText(/back/i);

    await user.type(frontInput, "x");
    await user.type(backInput, "y");
    await user.click(screen.getByRole("button", { name: /^add$/i }));

    await waitFor(() => {
      expect(screen.getByText("front is too short")).toBeInTheDocument();
    });
  });

  it("edit toggle shows edit form and cancel reverts to view", async () => {
    const user = userEvent.setup();
    renderClient([]);

    // Click Edit on first card row
    const firstEditBtn = screen.getAllByRole("button", { name: /edit/i })[0] as HTMLElement;
    await user.click(firstEditBtn);

    // Edit form appears alongside the add form; edit input is the second Front input
    // [0] = add form (empty), [1] = edit form (prefilled)
    const editFrontInput = screen.getAllByLabelText(/front/i)[1] as HTMLElement;
    expect(editFrontInput).toHaveValue("Hello");

    // Cancel reverts
    await user.click(screen.getByRole("button", { name: /cancel/i }));

    await waitFor(() => {
      expect(screen.queryByRole("button", { name: /cancel/i })).not.toBeInTheDocument();
    });
    expect(screen.getByText("Hello")).toBeInTheDocument();
  });

  it("edit save calls updateCard mutation and shows updated values", async () => {
    const user = userEvent.setup();
    const updatedCard = { ...CARD_1, front: "Hello updated", back: "Hola updated" };
    const mock = makeUpdateMock(
      "c-1",
      { front: "Hello updated", back: "Hola updated" },
      updatedCard,
    );

    renderClient([mock]);

    const firstEditBtn = screen.getAllByRole("button", { name: /edit/i })[0] as HTMLElement;
    await user.click(firstEditBtn);

    // [0] = add form, [1] = edit form
    const editFrontInput = screen.getAllByLabelText(/front/i)[1] as HTMLElement;
    await user.clear(editFrontInput);
    await user.type(editFrontInput, "Hello updated");

    const editBackInput = screen.getAllByLabelText(/back/i)[1] as HTMLElement;
    await user.clear(editBackInput);
    await user.type(editBackInput, "Hola updated");

    await user.click(screen.getByRole("button", { name: /^save$/i }));

    await waitFor(() => {
      expect(screen.getByText("Hello updated")).toBeInTheDocument();
    });
  });

  it("delete dialog confirm removes the row and calls mutation", async () => {
    const user = userEvent.setup();
    const mock = makeDeleteMock("c-1");

    renderClient([mock]);

    expect(screen.getByText("Hello")).toBeInTheDocument();

    const firstDeleteBtn = screen.getAllByRole("button", { name: /delete/i })[0] as HTMLElement;
    await user.click(firstDeleteBtn);

    // Confirm button inside the dialog
    const dialog = await screen.findByRole("alertdialog");
    const confirmBtn = within(dialog).getByRole("button", { name: /delete/i });
    await user.click(confirmBtn);

    await waitFor(() => {
      expect(screen.queryByText("Hello")).not.toBeInTheDocument();
    });
  });

  it("UNAUTHENTICATED on delete shows banner", async () => {
    const user = userEvent.setup();

    const mock = {
      request: { query: DeleteCardDocument, variables: { id: "c-1" } },
      result: {
        errors: [new GraphQLError("Unauthenticated", { extensions: { code: "UNAUTHENTICATED" } })],
      },
    };

    renderClient([mock], [CARD_1], { errorPolicy: true });

    const deleteBtn = screen.getByRole("button", { name: /delete/i });
    await user.click(deleteBtn);

    const dialog = await screen.findByRole("alertdialog");
    const confirmBtn = within(dialog).getByRole("button", { name: /delete/i });
    await user.click(confirmBtn);

    await waitFor(() => {
      expect(screen.getByText("Your session expired. Please sign in again.")).toBeInTheDocument();
    });
  });

  it("submit button is disabled while mutation in flight", async () => {
    const user = userEvent.setup();

    // Use a never-resolving mock so the mutation stays in-flight.
    const mock = {
      request: {
        query: CreateCardDocument,
        variables: { input: { cardgroupId: CG_ID, front: "Slow", back: "Lento" } },
      },
      result: () =>
        new Promise<never>(() => {
          // never resolves
        }),
    };

    renderClient([mock], []);

    const frontInput = screen.getByLabelText(/front/i);
    const backInput = screen.getByLabelText(/back/i);

    await user.type(frontInput, "Slow");
    await user.type(backInput, "Lento");

    const addBtn = screen.getByRole("button", { name: /^add$/i });
    await user.click(addBtn);

    await waitFor(() => {
      expect(screen.getByRole("button", { name: /saving/i })).toBeDisabled();
    });
  });

  it("edit save failure keeps the row in edit mode and shows the inline error", async () => {
    const user = userEvent.setup();

    const mock = {
      request: {
        query: UpdateCardDocument,
        variables: { id: "c-1", input: { front: "", back: "Hola" } },
      },
      result: {
        errors: [
          new GraphQLError("front is required", {
            extensions: { code: "BAD_USER_INPUT", field: "front" },
          }),
        ],
      },
    };

    renderClient([mock], [CARD_1], { errorPolicy: true });

    // Open edit form for the card
    const editBtn = screen.getByRole("button", { name: /edit/i });
    await user.click(editBtn);

    // [0] = add form, [1] = edit form
    const editFrontInput = screen.getAllByLabelText(/front/i)[1] as HTMLElement;
    await user.clear(editFrontInput);
    // Leave front empty so the server returns BAD_USER_INPUT

    await user.click(screen.getByRole("button", { name: /^save$/i }));

    await waitFor(() => {
      expect(screen.getByText("front is required")).toBeInTheDocument();
    });

    // Edit form must still be open (inputs still visible)
    expect(screen.getAllByLabelText(/front/i)).toHaveLength(2);
    // No navigation — save button still present
    expect(screen.getByRole("button", { name: /^save$/i })).toBeInTheDocument();
  });

  it("delete failure surfaces the banner", async () => {
    const user = userEvent.setup();

    const mock = {
      request: { query: DeleteCardDocument, variables: { id: "c-1" } },
      result: {
        errors: [new GraphQLError("Unauthenticated", { extensions: { code: "UNAUTHENTICATED" } })],
      },
    };

    renderClient([mock], [CARD_1], { errorPolicy: true });

    const deleteBtn = screen.getByRole("button", { name: /delete/i });
    await user.click(deleteBtn);

    const dialog = await screen.findByRole("alertdialog");
    const confirmBtn = within(dialog).getByRole("button", { name: /delete/i });
    await user.click(confirmBtn);

    await waitFor(() => {
      expect(screen.getByText("Your session expired. Please sign in again.")).toBeInTheDocument();
    });
  });

  it("add-card form is reset after successful create", async () => {
    const user = userEvent.setup();
    const mock = makeCreateMock({ cardgroupId: CG_ID, front: "Cat", back: "Gato" });

    const cacheMocks = [
      {
        request: {
          query: CardsByCardgroupDocument,
          variables: { cardgroupId: CG_ID },
        },
        result: { data: { cardsByCardgroup: [CARD_1, CARD_2] } },
      },
      mock,
    ];

    renderClient(cacheMocks);

    // Fill the add form (idPrefix="add-")
    const addFrontInput = screen.getAllByLabelText(/front/i)[0] as HTMLElement;
    const addBackInput = screen.getAllByLabelText(/back/i)[0] as HTMLElement;

    await user.click(addFrontInput);
    await user.type(addFrontInput, "Cat");
    await user.click(addBackInput);
    await user.type(addBackInput, "Gato");

    await user.click(screen.getByRole("button", { name: /^add$/i }));

    // New card row appears
    await waitFor(() => {
      expect(screen.getByText("Cat")).toBeInTheDocument();
    });

    // Add form inputs must be cleared (remounted via key)
    const resetFrontInput = screen.getAllByLabelText(/front/i)[0] as HTMLInputElement;
    const resetBackInput = screen.getAllByLabelText(/back/i)[0] as HTMLInputElement;
    expect(resetFrontInput.value).toBe("");
    expect(resetBackInput.value).toBe("");
  });
});

// @vitest-environment jsdom
import type { MockedResponse } from "@apollo/client/testing";
import { MockedProvider } from "@apollo/client/testing/react";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { GraphQLError } from "graphql";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { DeleteCardgroupDocument, UpdateCardgroupDocument } from "@/generated/graphql";
import { EditCardgroupClient } from "./edit-cardgroup-client";

const mockPush = vi.fn();
const mockRefresh = vi.fn();

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: mockPush, refresh: mockRefresh }),
}));

const CARDGROUP = { id: "cg-1", name: "Spanish Vocab" };

function makeUpdateMock(
  variables: { id: string; input: { name: string } },
  result: MockedResponse["result"],
): MockedResponse {
  return { request: { query: UpdateCardgroupDocument, variables }, result };
}

function makeDeleteMock(
  variables: { id: string },
  result: MockedResponse["result"],
): MockedResponse {
  return { request: { query: DeleteCardgroupDocument, variables }, result };
}

function renderClient(mocks: MockedResponse[] = [], errorPolicy?: "all" | "none" | "ignore") {
  const defaultOptions = errorPolicy ? { mutate: { errorPolicy } } : undefined;
  render(
    <MockedProvider mocks={mocks} defaultOptions={defaultOptions}>
      <EditCardgroupClient cardgroup={CARDGROUP} />
    </MockedProvider>,
  );
}

describe("<EditCardgroupClient>", () => {
  beforeEach(() => {
    mockPush.mockClear();
    mockRefresh.mockClear();
  });

  it("edit success navigates to detail page", async () => {
    const user = userEvent.setup();
    const mocks = [
      makeUpdateMock(
        { id: "cg-1", input: { name: "Spanish Vocab" } },
        {
          data: {
            updateCardgroup: {
              __typename: "UpdateCardgroupPayload",
              cardgroup: {
                __typename: "Cardgroup",
                id: "cg-1",
                name: "Spanish Vocab",
                updatedAt: "2024-06-15T10:00:00.000Z",
              },
            },
          },
        },
      ),
    ];
    renderClient(mocks);

    await user.click(screen.getByRole("button", { name: /save/i }));

    await waitFor(() => {
      expect(mockPush).toHaveBeenCalledWith("/cardgroups/cg-1");
    });
  });

  it("edit BAD_USER_INPUT field=name shows inline error", async () => {
    const user = userEvent.setup();
    const mocks = [
      makeUpdateMock(
        { id: "cg-1", input: { name: "Spanish Vocab" } },
        {
          errors: [
            new GraphQLError("name already exists", {
              extensions: { code: "BAD_USER_INPUT", field: "name" },
            }),
          ],
        },
      ),
    ];
    renderClient(mocks, "all");

    await user.click(screen.getByRole("button", { name: /save/i }));

    await waitFor(() => {
      expect(screen.getByText("name already exists")).toBeInTheDocument();
    });
    expect(mockPush).not.toHaveBeenCalled();
  });

  it("delete dialog opens when Delete button is clicked", async () => {
    const user = userEvent.setup();
    renderClient();

    await user.click(screen.getByRole("button", { name: /^delete$/i }));

    await waitFor(() => {
      expect(screen.getByRole("alertdialog")).toBeInTheDocument();
      expect(screen.getByText("Delete cardgroup")).toBeInTheDocument();
    });
  });

  it("cancel closes the delete dialog without firing mutation", async () => {
    const user = userEvent.setup();
    const deleteCalled = vi.fn();
    const mocks: MockedResponse[] = [
      {
        request: { query: DeleteCardgroupDocument, variables: { id: "cg-1" } },
        result: () => {
          deleteCalled();
          return { data: { deleteCardgroup: true } };
        },
      },
    ];
    renderClient(mocks);

    await user.click(screen.getByRole("button", { name: /^delete$/i }));
    await waitFor(() => {
      expect(screen.getByRole("alertdialog")).toBeInTheDocument();
    });

    await user.click(screen.getByRole("button", { name: /^cancel$/i }));

    await waitFor(() => {
      expect(screen.queryByRole("alertdialog")).not.toBeInTheDocument();
    });
    expect(deleteCalled).not.toHaveBeenCalled();
  });

  it("confirm fires DeleteCardgroupMutation, closes dialog, and navigates to /cardgroups", async () => {
    const user = userEvent.setup();
    const deleteCalled = vi.fn();
    const mocks: MockedResponse[] = [
      {
        request: { query: DeleteCardgroupDocument, variables: { id: "cg-1" } },
        result: () => {
          deleteCalled();
          return { data: { deleteCardgroup: true } };
        },
      },
    ];
    renderClient(mocks);

    await user.click(screen.getByRole("button", { name: /^delete$/i }));
    await waitFor(() => {
      expect(screen.getByRole("alertdialog")).toBeInTheDocument();
    });

    const dialogDeleteBtns = screen
      .getAllByRole("button", { name: /^delete$/i })
      .filter((el) => el.closest("[role='alertdialog']"));
    const confirmBtn = dialogDeleteBtns[0];
    if (!confirmBtn) throw new Error("Delete confirm button not found in dialog");
    await user.click(confirmBtn);

    await waitFor(() => {
      expect(deleteCalled).toHaveBeenCalledOnce();
    });

    await waitFor(() => {
      expect(screen.queryByRole("alertdialog")).not.toBeInTheDocument();
    });

    await waitFor(() => {
      expect(mockPush).toHaveBeenCalledWith("/cardgroups");
    });
  });

  it("delete UNAUTHENTICATED shows banner error and dialog stays open", async () => {
    const user = userEvent.setup();
    const mocks = [
      makeDeleteMock(
        { id: "cg-1" },
        {
          errors: [
            new GraphQLError("Unauthenticated", {
              extensions: { code: "UNAUTHENTICATED" },
            }),
          ],
        },
      ),
    ];
    renderClient(mocks, "all");

    await user.click(screen.getByRole("button", { name: /^delete$/i }));
    await waitFor(() => {
      expect(screen.getByRole("alertdialog")).toBeInTheDocument();
    });

    const dialogDeleteBtns = screen
      .getAllByRole("button", { name: /^delete$/i })
      .filter((el) => el.closest("[role='alertdialog']"));
    const confirmBtn = dialogDeleteBtns[0];
    if (!confirmBtn) throw new Error("Delete confirm button not found in dialog");
    await user.click(confirmBtn);

    await waitFor(() => {
      expect(screen.getByText("Your session expired. Please sign in again.")).toBeInTheDocument();
    });
    expect(screen.getByRole("alertdialog")).toBeInTheDocument();
    expect(mockPush).not.toHaveBeenCalled();
  });

  it("update network rejection shows error banner and does not navigate", async () => {
    const user = userEvent.setup();
    const mocks: MockedResponse[] = [
      {
        request: {
          query: UpdateCardgroupDocument,
          variables: { id: "cg-1", input: { name: "Spanish Vocab" } },
        },
        error: new Error("Network error: failed to fetch"),
      },
    ];
    renderClient(mocks);

    await user.click(screen.getByRole("button", { name: /save/i }));

    await waitFor(() => {
      expect(screen.getByText("Could not reach the server. Please try again.")).toBeInTheDocument();
    });
    expect(mockPush).not.toHaveBeenCalled();
  });

  it("delete network rejection shows error banner, dialog stays open, and does not navigate", async () => {
    const user = userEvent.setup();
    const mocks: MockedResponse[] = [
      {
        request: { query: DeleteCardgroupDocument, variables: { id: "cg-1" } },
        error: new Error("Network error: failed to fetch"),
      },
    ];
    renderClient(mocks);

    await user.click(screen.getByRole("button", { name: /^delete$/i }));
    await waitFor(() => {
      expect(screen.getByRole("alertdialog")).toBeInTheDocument();
    });

    const dialogDeleteBtns = screen
      .getAllByRole("button", { name: /^delete$/i })
      .filter((el) => el.closest("[role='alertdialog']"));
    const confirmBtn = dialogDeleteBtns[0];
    if (!confirmBtn) throw new Error("Delete confirm button not found in dialog");
    await user.click(confirmBtn);

    await waitFor(() => {
      expect(screen.getByText("Could not reach the server. Please try again.")).toBeInTheDocument();
    });
    expect(screen.getByRole("alertdialog")).toBeInTheDocument();
    expect(mockPush).not.toHaveBeenCalled();
  });
});

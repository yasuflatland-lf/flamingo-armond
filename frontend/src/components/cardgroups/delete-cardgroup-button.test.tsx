// @vitest-environment jsdom
import { InMemoryCache } from "@apollo/client";
import type { MockedResponse } from "@apollo/client/testing";
import { MockedProvider } from "@apollo/client/testing/react";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { GraphQLError } from "graphql";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { CARDGROUPS_DEFAULT_VARS } from "@/app/cardgroups/queries";
import { DeleteCardgroupDocument, MyCardgroupsConnectionDocument } from "@/generated/graphql";
import {
  type ApolloMockLeakSpyResult,
  installApolloMockLeakSpy,
} from "../../../__tests__/utils/mock-apollo-paginated";
import { DeleteCardgroupButton } from "./delete-cardgroup-button";

const CG_ID = "cg-1";
const CG_NAME = "Spanish Vocab";

function makeDeleteMock(
  variables: { id: string },
  result: MockedResponse["result"],
): MockedResponse {
  return { request: { query: DeleteCardgroupDocument, variables }, result };
}

function renderButton(
  mocks: MockedResponse[] = [],
  cache?: InMemoryCache,
  errorPolicy?: "all" | "none" | "ignore",
) {
  const defaultOptions = errorPolicy ? { mutate: { errorPolicy } } : undefined;
  render(
    <MockedProvider mocks={mocks} cache={cache} defaultOptions={defaultOptions}>
      <DeleteCardgroupButton id={CG_ID} name={CG_NAME} />
    </MockedProvider>,
  );
}

function getDialogDeleteButton() {
  const btn = screen
    .getAllByRole("button", { name: /^delete$/i })
    .find((el) => el.closest("[role='alertdialog']"));
  if (!btn) throw new Error("Delete confirm button not found in dialog");
  return btn;
}

let leakSpy: ApolloMockLeakSpyResult;

beforeEach(() => {
  leakSpy = installApolloMockLeakSpy({ operationNames: ["DeleteCardgroup"] });
});

afterEach(() => {
  vi.restoreAllMocks();
  leakSpy.assertNoLeaks();
  leakSpy.teardown();
});

describe("<DeleteCardgroupButton>", () => {
  it("renders a Trash icon button with an aria-label that includes the name", () => {
    renderButton();
    expect(screen.getByRole("button", { name: `Delete cardgroup ${CG_NAME}` })).toBeInTheDocument();
  });

  it("opens the confirm dialog when the Trash button is clicked", async () => {
    const user = userEvent.setup();
    renderButton();

    await user.click(screen.getByRole("button", { name: `Delete cardgroup ${CG_NAME}` }));

    await waitFor(() => {
      expect(screen.getByRole("alertdialog")).toBeInTheDocument();
    });
    expect(screen.getByText("Delete cardgroup")).toBeInTheDocument();
    expect(
      screen.getByText(
        `This will permanently delete "${CG_NAME}" and all its cards. This cannot be undone.`,
      ),
    ).toBeInTheDocument();
  });

  it("Cancel closes the dialog without firing the mutation", async () => {
    const user = userEvent.setup();
    let deleteCalls = 0;
    const mocks: MockedResponse[] = [
      {
        request: { query: DeleteCardgroupDocument, variables: { id: CG_ID } },
        result: () => {
          deleteCalls += 1;
          return { data: { deleteCardgroup: true } };
        },
      },
    ];
    renderButton(mocks);

    await user.click(screen.getByRole("button", { name: `Delete cardgroup ${CG_NAME}` }));
    await waitFor(() => {
      expect(screen.getByRole("alertdialog")).toBeInTheDocument();
    });

    await user.click(screen.getByRole("button", { name: /^cancel$/i }));

    await waitFor(() => {
      expect(screen.queryByRole("alertdialog")).not.toBeInTheDocument();
    });
    expect(deleteCalls).toBe(0);
  });

  it("Confirm fires DeleteCardgroupMutation and closes the dialog", async () => {
    const user = userEvent.setup();
    let deleteCalls = 0;
    const mocks: MockedResponse[] = [
      {
        request: { query: DeleteCardgroupDocument, variables: { id: CG_ID } },
        result: () => {
          deleteCalls += 1;
          return { data: { deleteCardgroup: true } };
        },
      },
    ];
    renderButton(mocks);

    await user.click(screen.getByRole("button", { name: `Delete cardgroup ${CG_NAME}` }));
    await waitFor(() => {
      expect(screen.getByRole("alertdialog")).toBeInTheDocument();
    });

    await user.click(getDialogDeleteButton());

    await waitFor(() => {
      expect(deleteCalls).toBe(1);
    });
    await waitFor(() => {
      expect(screen.queryByRole("alertdialog")).not.toBeInTheDocument();
    });
  });

  it("removes the cardgroup from the Connection cache and decrements totalCount on success", async () => {
    const user = userEvent.setup();
    const cache = new InMemoryCache();
    const cg1 = {
      __typename: "Cardgroup" as const,
      id: CG_ID,
      name: CG_NAME,
      updatedAt: "2024-06-15T10:00:00.000Z",
    };
    const cg2 = {
      __typename: "Cardgroup" as const,
      id: "cg-2",
      name: "Math",
      updatedAt: "2024-05-20T08:00:00.000Z",
    };
    cache.writeQuery({
      query: MyCardgroupsConnectionDocument,
      variables: CARDGROUPS_DEFAULT_VARS,
      data: {
        myCardgroupsConnection: {
          __typename: "CardgroupConnection",
          edges: [
            { __typename: "CardgroupEdge", cursor: cg1.id, node: cg1 },
            { __typename: "CardgroupEdge", cursor: cg2.id, node: cg2 },
          ],
          pageInfo: {
            __typename: "PageInfo",
            hasNextPage: false,
            hasPreviousPage: false,
            startCursor: cg1.id,
            endCursor: cg2.id,
          },
          totalCount: 2,
        },
      },
    });
    const mocks: MockedResponse[] = [
      makeDeleteMock({ id: CG_ID }, { data: { deleteCardgroup: true } }),
    ];
    renderButton(mocks, cache);

    await user.click(screen.getByRole("button", { name: `Delete cardgroup ${CG_NAME}` }));
    await waitFor(() => {
      expect(screen.getByRole("alertdialog")).toBeInTheDocument();
    });
    await user.click(getDialogDeleteButton());

    await waitFor(() => {
      expect(screen.queryByRole("alertdialog")).not.toBeInTheDocument();
    });

    const conn = cache.readQuery({
      query: MyCardgroupsConnectionDocument,
      variables: CARDGROUPS_DEFAULT_VARS,
    });
    expect(conn?.myCardgroupsConnection.edges).toHaveLength(1);
    expect(conn?.myCardgroupsConnection.edges[0]?.node).not.toBeNull();
    expect(conn?.myCardgroupsConnection.edges[0]?.node.id).toBe("cg-2");
    expect(conn?.myCardgroupsConnection.totalCount).toBe(1);
  });

  it("UNAUTHENTICATED error keeps the dialog open and shows a banner", async () => {
    const user = userEvent.setup();
    const mocks: MockedResponse[] = [
      makeDeleteMock(
        { id: CG_ID },
        {
          errors: [
            new GraphQLError("Unauthenticated", {
              extensions: { code: "UNAUTHENTICATED" },
            }),
          ],
        },
      ),
    ];
    renderButton(mocks, undefined, "all");

    await user.click(screen.getByRole("button", { name: `Delete cardgroup ${CG_NAME}` }));
    await waitFor(() => {
      expect(screen.getByRole("alertdialog")).toBeInTheDocument();
    });
    await user.click(getDialogDeleteButton());

    await waitFor(() => {
      expect(screen.getByText("Your session expired. Please sign in again.")).toBeInTheDocument();
    });
    expect(screen.getByRole("alertdialog")).toBeInTheDocument();
  });

  it("network rejection keeps the dialog open, shows a banner, and logs structurally", async () => {
    const user = userEvent.setup();
    const consoleErrorSpy = vi.spyOn(console, "error").mockImplementation(() => {});
    const mocks: MockedResponse[] = [
      {
        request: { query: DeleteCardgroupDocument, variables: { id: CG_ID } },
        error: new Error("Network error: failed to fetch"),
      },
    ];
    renderButton(mocks);

    await user.click(screen.getByRole("button", { name: `Delete cardgroup ${CG_NAME}` }));
    await waitFor(() => {
      expect(screen.getByRole("alertdialog")).toBeInTheDocument();
    });
    await user.click(getDialogDeleteButton());

    await waitFor(() => {
      expect(screen.getByText("Could not reach the server. Please try again.")).toBeInTheDocument();
    });
    expect(screen.getByRole("alertdialog")).toBeInTheDocument();

    expect(consoleErrorSpy).toHaveBeenCalledWith(
      "[delete-cardgroup-button] delete rejection",
      expect.objectContaining({ id: CG_ID, err: expect.anything() }),
    );
  });
});

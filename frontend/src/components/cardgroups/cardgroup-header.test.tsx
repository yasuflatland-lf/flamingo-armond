// @vitest-environment jsdom
import type { MockedResponse } from "@apollo/client/testing";
import { MockedProvider } from "@apollo/client/testing/react";
import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { GraphQLError } from "graphql";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { DeleteCardgroupDocument, UpdateCardgroupDocument } from "@/generated/graphql";
import { renderWithIntl } from "@/test/render-with-intl";
import { CardgroupHeader } from "./cardgroup-header";

const mockPush = vi.fn();
const mockRefresh = vi.fn();

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: mockPush, refresh: mockRefresh }),
}));

const CARDGROUP = { id: "cg-1", name: "Spanish Vocab" };

function makeDeleteMock(
  variables: { id: string },
  result: MockedResponse["result"],
): MockedResponse {
  return { request: { query: DeleteCardgroupDocument, variables }, result };
}

function makeUpdateMock(
  variables: { id: string; input: { name: string } },
  result: MockedResponse["result"],
  delay?: MockedResponse["delay"],
): MockedResponse {
  return { request: { query: UpdateCardgroupDocument, variables }, result, delay };
}

function renderHeader(mocks: MockedResponse[] = [], totalCount = 5) {
  renderWithIntl(
    <MockedProvider mocks={mocks}>
      <CardgroupHeader cardgroup={CARDGROUP} totalCount={totalCount} />
    </MockedProvider>,
  );
}

describe("<CardgroupHeader>", () => {
  beforeEach(() => {
    mockPush.mockClear();
    mockRefresh.mockClear();
  });

  it("renders the cardgroup name as h1", () => {
    renderHeader();
    expect(screen.getByRole("heading", { level: 1, name: /spanish vocab/i })).toBeInTheDocument();
  });

  it("renders a Badge with the totalCount", () => {
    renderHeader([], 42);
    expect(screen.getByText("42 cards")).toBeInTheDocument();
  });

  it("renders the kebab trigger button", () => {
    renderHeader();
    expect(screen.getByRole("button", { name: /cardgroup options/i })).toBeInTheDocument();
  });

  it("kebab menu opens Rename and Delete cardgroup items", async () => {
    const user = userEvent.setup();
    renderHeader();

    await user.click(screen.getByRole("button", { name: /cardgroup options/i }));

    await waitFor(() => {
      expect(screen.getByRole("menuitem", { name: /rename/i })).toBeInTheDocument();
      expect(screen.getByRole("menuitem", { name: /delete cardgroup/i })).toBeInTheDocument();
    });
  });

  it("clicking Rename opens the Rename cardgroup FormSheet", async () => {
    const user = userEvent.setup();
    renderHeader();

    await user.click(screen.getByRole("button", { name: /cardgroup options/i }));
    await waitFor(() => {
      expect(screen.getByRole("menuitem", { name: /rename/i })).toBeInTheDocument();
    });
    await user.click(screen.getByRole("menuitem", { name: /rename/i }));

    await waitFor(() => {
      expect(screen.getByRole("heading", { name: /rename cardgroup/i })).toBeInTheDocument();
    });
  });

  it("does not dismiss the rename FormSheet while save is submitting", async () => {
    const user = userEvent.setup();
    renderHeader([
      makeUpdateMock(
        { id: "cg-1", input: { name: "Spanish Vocab" } },
        {
          data: {
            updateCardgroup: {
              __typename: "UpdateCardgroupSuccess" as const,
              cardgroup: {
                __typename: "Cardgroup" as const,
                id: "cg-1",
                name: "Spanish Vocab",
                updatedAt: "2024-06-15T10:00:00.000Z",
              },
            },
          },
        },
        Infinity,
      ),
    ]);

    await user.click(screen.getByRole("button", { name: /cardgroup options/i }));
    await waitFor(() => {
      expect(screen.getByRole("menuitem", { name: /rename/i })).toBeInTheDocument();
    });
    await user.click(screen.getByRole("menuitem", { name: /rename/i }));
    await user.click(screen.getByRole("button", { name: /^save$/i }));

    await waitFor(() => {
      expect(screen.getByRole("button", { name: /saving/i })).toBeInTheDocument();
    });
    await user.keyboard("{Escape}");

    expect(screen.getByRole("heading", { name: /rename cardgroup/i })).toBeInTheDocument();
  });

  it("clicking Delete cardgroup opens the AlertDialog", async () => {
    const user = userEvent.setup();
    renderHeader();

    await user.click(screen.getByRole("button", { name: /cardgroup options/i }));
    await waitFor(() => {
      expect(screen.getByRole("menuitem", { name: /delete cardgroup/i })).toBeInTheDocument();
    });
    await user.click(screen.getByRole("menuitem", { name: /delete cardgroup/i }));

    await waitFor(() => {
      expect(screen.getByRole("alertdialog")).toBeInTheDocument();
      expect(
        screen.getByRole("heading", { level: 2, name: /^delete cardgroup$/i }),
      ).toBeInTheDocument();
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
    renderHeader(mocks);

    await user.click(screen.getByRole("button", { name: /cardgroup options/i }));
    await waitFor(() =>
      expect(screen.getByRole("menuitem", { name: /delete cardgroup/i })).toBeInTheDocument(),
    );
    await user.click(screen.getByRole("menuitem", { name: /delete cardgroup/i }));
    await waitFor(() => expect(screen.getByRole("alertdialog")).toBeInTheDocument());

    await user.click(screen.getByRole("button", { name: /^cancel$/i }));

    await waitFor(() => {
      expect(screen.queryByRole("alertdialog")).not.toBeInTheDocument();
    });
    expect(deleteCalled).not.toHaveBeenCalled();
  });

  it("confirm fires DeleteCardgroupMutation and navigates to /cardgroups", async () => {
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
    renderHeader(mocks);

    await user.click(screen.getByRole("button", { name: /cardgroup options/i }));
    await waitFor(() =>
      expect(screen.getByRole("menuitem", { name: /delete cardgroup/i })).toBeInTheDocument(),
    );
    await user.click(screen.getByRole("menuitem", { name: /delete cardgroup/i }));
    await waitFor(() => expect(screen.getByRole("alertdialog")).toBeInTheDocument());

    const confirmBtn = screen
      .getAllByRole("button", { name: /^delete$/i })
      .find((el) => el.closest("[role='alertdialog']"));
    if (!confirmBtn) throw new Error("Delete confirm button not found in dialog");
    await user.click(confirmBtn);

    await waitFor(() => {
      expect(deleteCalled).toHaveBeenCalledOnce();
    });
    await waitFor(() => {
      expect(mockPush).toHaveBeenCalledWith("/cardgroups");
    });
    expect(mockRefresh).toHaveBeenCalledTimes(1);
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
    renderHeader(mocks);

    await user.click(screen.getByRole("button", { name: /cardgroup options/i }));
    await waitFor(() =>
      expect(screen.getByRole("menuitem", { name: /delete cardgroup/i })).toBeInTheDocument(),
    );
    await user.click(screen.getByRole("menuitem", { name: /delete cardgroup/i }));
    await waitFor(() => expect(screen.getByRole("alertdialog")).toBeInTheDocument());

    const confirmBtn = screen
      .getAllByRole("button", { name: /^delete$/i })
      .find((el) => el.closest("[role='alertdialog']"));
    if (!confirmBtn) throw new Error("Delete confirm button not found in dialog");
    await user.click(confirmBtn);

    await waitFor(() => {
      expect(screen.getByText("Your session expired. Please sign in again.")).toBeInTheDocument();
    });
    expect(screen.getByRole("alertdialog")).toBeInTheDocument();
    expect(mockPush).not.toHaveBeenCalled();
  });

  it("delete network rejection shows error banner and dialog stays open", async () => {
    const user = userEvent.setup();
    const mocks: MockedResponse[] = [
      {
        request: { query: DeleteCardgroupDocument, variables: { id: "cg-1" } },
        error: new Error("Network error: failed to fetch"),
      },
    ];
    renderHeader(mocks);

    await user.click(screen.getByRole("button", { name: /cardgroup options/i }));
    await waitFor(() =>
      expect(screen.getByRole("menuitem", { name: /delete cardgroup/i })).toBeInTheDocument(),
    );
    await user.click(screen.getByRole("menuitem", { name: /delete cardgroup/i }));
    await waitFor(() => expect(screen.getByRole("alertdialog")).toBeInTheDocument());

    const confirmBtn = screen
      .getAllByRole("button", { name: /^delete$/i })
      .find((el) => el.closest("[role='alertdialog']"));
    if (!confirmBtn) throw new Error("Delete confirm button not found in dialog");
    await user.click(confirmBtn);

    await waitFor(() => {
      expect(screen.getByText("Could not reach the server. Please try again.")).toBeInTheDocument();
    });
    expect(screen.getByRole("alertdialog")).toBeInTheDocument();
    expect(mockPush).not.toHaveBeenCalled();
  });
});

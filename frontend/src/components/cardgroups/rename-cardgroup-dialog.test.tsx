// @vitest-environment jsdom
import type { MockedResponse } from "@apollo/client/testing";
import { MockedProvider } from "@apollo/client/testing/react";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { UpdateCardgroupDocument } from "@/generated/graphql";
import { RenameCardgroupDialog } from "./rename-cardgroup-dialog";

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

function renderDialog(mocks: MockedResponse[] = [], open = true) {
  const onOpenChange = vi.fn();
  render(
    <MockedProvider mocks={mocks}>
      <RenameCardgroupDialog cardgroup={CARDGROUP} open={open} onOpenChange={onOpenChange} />
    </MockedProvider>,
  );
  return { onOpenChange };
}

describe("<RenameCardgroupDialog>", () => {
  beforeEach(() => {
    mockPush.mockClear();
    mockRefresh.mockClear();
  });

  it("renders the dialog title when open", () => {
    renderDialog();
    expect(screen.getByRole("heading", { name: /rename cardgroup/i })).toBeInTheDocument();
  });

  it("does not render dialog content when closed", () => {
    renderDialog([], false);
    expect(screen.queryByRole("heading", { name: /rename cardgroup/i })).not.toBeInTheDocument();
  });

  it("save success closes dialog and refreshes the route", async () => {
    const user = userEvent.setup();
    const { onOpenChange } = renderDialog([
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
      ),
    ]);

    await user.click(screen.getByRole("button", { name: /^save$/i }));

    await waitFor(() => {
      expect(mockRefresh).toHaveBeenCalledTimes(1);
    });
    expect(onOpenChange).toHaveBeenCalledWith(false);
    expect(mockPush).not.toHaveBeenCalled();
  });

  it("InputValidationError field=name shows inline error and does not close", async () => {
    const user = userEvent.setup();
    const { onOpenChange } = renderDialog([
      makeUpdateMock(
        { id: "cg-1", input: { name: "Spanish Vocab" } },
        {
          data: {
            updateCardgroup: {
              __typename: "InputValidationError" as const,
              field: "name",
              message: "name already exists",
            },
          },
        },
      ),
    ]);

    await user.click(screen.getByRole("button", { name: /^save$/i }));

    await waitFor(() => {
      expect(screen.getByText("name already exists")).toBeInTheDocument();
    });
    expect(mockRefresh).not.toHaveBeenCalled();
    expect(onOpenChange).not.toHaveBeenCalledWith(false);
  });

  it("network rejection shows error banner and does not close", async () => {
    const user = userEvent.setup();
    const { onOpenChange } = renderDialog([
      {
        request: {
          query: UpdateCardgroupDocument,
          variables: { id: "cg-1", input: { name: "Spanish Vocab" } },
        },
        error: new Error("Network error: failed to fetch"),
      },
    ]);

    await user.click(screen.getByRole("button", { name: /^save$/i }));

    await waitFor(() => {
      expect(screen.getByText("Could not reach the server. Please try again.")).toBeInTheDocument();
    });
    expect(mockRefresh).not.toHaveBeenCalled();
    expect(onOpenChange).not.toHaveBeenCalledWith(false);
  });
});

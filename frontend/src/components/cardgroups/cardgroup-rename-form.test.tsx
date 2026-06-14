// @vitest-environment jsdom
import type { MockedResponse } from "@apollo/client/testing";
import { MockedProvider } from "@apollo/client/testing/react";
import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { GraphQLError } from "graphql";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { UpdateCardgroupDocument } from "@/generated/graphql";
import { renderWithIntl } from "@/test/render-with-intl";
import { CardgroupRenameForm } from "./cardgroup-rename-form";

const mockRefresh = vi.fn();

vi.mock("next/navigation", () => ({
  useRouter: () => ({ refresh: mockRefresh }),
}));

const CARDGROUP = { id: "cg-1", name: "Spanish Vocab" };

function makeUpdateMock(
  variables: { id: string; input: { name: string } },
  result: MockedResponse["result"],
): MockedResponse {
  return { request: { query: UpdateCardgroupDocument, variables }, result };
}

function renderForm(mocks: MockedResponse[] = []) {
  const onSaved = vi.fn();
  renderWithIntl(
    <MockedProvider mocks={mocks}>
      <CardgroupRenameForm cardgroup={CARDGROUP} onSaved={onSaved} />
    </MockedProvider>,
  );
  return { onSaved };
}

describe("<CardgroupRenameForm>", () => {
  beforeEach(() => {
    mockRefresh.mockClear();
  });

  it("save success calls onSaved and refreshes the route", async () => {
    const user = userEvent.setup();
    const { onSaved } = renderForm([
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
    expect(onSaved).toHaveBeenCalledTimes(1);
  });

  it("InputValidationError field=name renders the inline name error and keeps the form open", async () => {
    const user = userEvent.setup();
    const { onSaved } = renderForm([
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
    expect(onSaved).not.toHaveBeenCalled();
  });

  it("network rejection shows the banner and keeps the form open", async () => {
    const user = userEvent.setup();
    const { onSaved } = renderForm([
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
    expect(onSaved).not.toHaveBeenCalled();
  });

  it("UNAUTHENTICATED rejection shows the session-expired banner and keeps the form open", async () => {
    const user = userEvent.setup();
    const { onSaved } = renderForm([
      makeUpdateMock(
        { id: "cg-1", input: { name: "Spanish Vocab" } },
        {
          errors: [
            new GraphQLError("Unauthenticated", {
              extensions: { code: "UNAUTHENTICATED" },
            }),
          ],
        },
      ),
    ]);

    await user.click(screen.getByRole("button", { name: /^save$/i }));

    await waitFor(() => {
      expect(screen.getByText("Your session expired. Please sign in again.")).toBeInTheDocument();
    });
    expect(mockRefresh).not.toHaveBeenCalled();
    expect(onSaved).not.toHaveBeenCalled();
  });
});

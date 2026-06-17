// @vitest-environment jsdom
import { MockedProvider } from "@apollo/client/testing/react";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { NextIntlClientProvider } from "next-intl";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import enMessages from "../../../../../../messages/en.json";

const push = vi.fn();
const refresh = vi.fn();
vi.mock("next/navigation", () => ({
  useRouter: () => ({ push, refresh }),
}));

import {
  AdminDeleteMasterMutation,
  AdminPublishMasterMutation,
  AdminUnpublishMasterMutation,
} from "../../queries";
import { MasterEditHeader } from "./master-edit-header";
import type { AdminMasterDeck } from "./queries";

const DECK: AdminMasterDeck = {
  __typename: "MasterCardgroup",
  id: "m-1",
  name: "Spanish A1",
  description: null,
  language: null,
  level: null,
  category: null,
  coverImageUrl: null,
  source: null,
  version: 1,
  status: "DRAFT",
  isDefaultStarter: false,
  sortOrder: 0,
  cardCount: 5,
  createdAt: "2026-01-01T00:00:00Z",
  updatedAt: "2026-01-01T00:00:00Z",
};

function renderHeader(deck: AdminMasterDeck, mocks: ReadonlyArray<unknown> = []) {
  return render(
    <MockedProvider mocks={mocks as never}>
      <NextIntlClientProvider locale="en" messages={enMessages} timeZone="UTC">
        <MasterEditHeader master={deck} />
      </NextIntlClientProvider>
    </MockedProvider>,
  );
}

beforeEach(() => {
  push.mockClear();
  refresh.mockClear();
});
afterEach(() => vi.clearAllMocks());

describe("MasterEditHeader", () => {
  it("renders the deck name, draft badge, and card count", () => {
    renderHeader(DECK);
    expect(screen.getByRole("heading", { level: 1, name: "Spanish A1" })).toBeInTheDocument();
    expect(screen.getByTestId("master-edit-status-badge")).toHaveTextContent("Draft");
    expect(screen.getByText("5 cards")).toBeInTheDocument();
  });

  it("disables publish and shows the hint for an empty draft", () => {
    renderHeader({ ...DECK, cardCount: 0 });
    expect(screen.getByTestId("master-edit-publish")).toBeDisabled();
    expect(screen.getByTestId("master-edit-empty-hint")).toBeInTheDocument();
  });

  it("opens the deck-settings dialog with the metadata form (no publish/delete sections)", async () => {
    const user = userEvent.setup();
    renderHeader(DECK);
    await user.click(screen.getByTestId("master-edit-deck-settings"));
    expect(await screen.findByTestId("master-field-name")).toBeInTheDocument();
    // Metadata-only: the form's in-dialog publish/delete sections are absent.
    expect(screen.queryByTestId("master-publish-section")).not.toBeInTheDocument();
    expect(screen.queryByTestId("master-row-delete-trigger")).not.toBeInTheDocument();
  });

  it("publishes the deck and refreshes the route", async () => {
    const user = userEvent.setup();
    const mocks = [
      {
        request: { query: AdminPublishMasterMutation, variables: { id: "m-1" } },
        result: {
          data: {
            adminPublishMasterCardgroup: {
              __typename: "PublishMasterCardgroupSuccess",
              master: { ...DECK, status: "PUBLISHED" },
            },
          },
        },
      },
    ];
    renderHeader(DECK, mocks);
    await user.click(screen.getByTestId("master-edit-publish"));
    await waitFor(() => expect(refresh).toHaveBeenCalledTimes(1));
  });

  it("unpublishes the deck and refreshes the route", async () => {
    const user = userEvent.setup();
    const mocks = [
      {
        request: { query: AdminUnpublishMasterMutation, variables: { id: "m-1" } },
        result: {
          data: {
            adminUnpublishMasterCardgroup: { ...DECK, status: "DRAFT" },
          },
        },
      },
    ];
    renderHeader({ ...DECK, status: "PUBLISHED" }, mocks);
    await user.click(screen.getByTestId("master-edit-publish"));
    await waitFor(() => expect(refresh).toHaveBeenCalledTimes(1));
  });

  it("delete confirm carries the destructive variant and navigates to the list on success", async () => {
    const user = userEvent.setup();
    const mocks = [
      {
        request: { query: AdminDeleteMasterMutation, variables: { id: "m-1" } },
        result: { data: { adminDeleteMasterCardgroup: true } },
      },
    ];
    renderHeader(DECK, mocks);
    await user.click(screen.getByTestId("master-edit-delete"));
    const confirm = await screen.findByTestId("master-delete-dialog-confirm");
    expect(confirm.className).toContain("bg-destructive");
    await user.click(confirm);
    await waitFor(() => expect(push).toHaveBeenCalledWith("/admin/masters"));
  });
});

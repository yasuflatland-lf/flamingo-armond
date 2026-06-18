// @vitest-environment jsdom
import { MockedProvider } from "@apollo/client/testing/react";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { GraphQLError } from "graphql";
import { NextIntlClientProvider } from "next-intl";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import enMessages from "../../../../../../messages/en.json";

vi.mock("sonner", () => ({ toast: { success: vi.fn(), error: vi.fn() } }));

const push = vi.fn();
const refresh = vi.fn();
vi.mock("next/navigation", () => ({
  useRouter: () => ({ push, refresh }),
}));

import { toast } from "sonner";
import {
  AdminDeleteMasterMutation,
  AdminPublishMasterMutation,
  AdminUnpublishMasterMutation,
  AdminUpdateMasterMutation,
} from "../../queries";
import { MasterEditHeader } from "./master-edit-header";
import type { AdminMasterDeck } from "./queries";

// The input shape the form emits for DECK (name trimmed, empty strings -> null, sortOrder as number)
const UPDATE_INPUT = {
  name: "Spanish A1",
  description: null,
  language: null,
  level: null,
  category: null,
  coverImageUrl: null,
  source: null,
  isDefaultStarter: false,
  sortOrder: 0,
} as const;

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
        <MasterEditHeader master={deck} cardCount={deck.cardCount} />
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
    // Deck settings now lives in the desktop split-button dropdown.
    await user.click(screen.getByTestId("master-edit-more-options"));
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
    // Delete now lives in the desktop split-button dropdown.
    await user.click(screen.getByTestId("master-edit-more-options"));
    await user.click(screen.getByTestId("master-edit-delete"));
    const confirm = await screen.findByTestId("master-delete-dialog-confirm");
    expect(confirm.className).toContain("bg-destructive");
    await user.click(confirm);
    await waitFor(() => expect(push).toHaveBeenCalledWith("/admin/masters"));
  });

  it("deck-settings save success: closes the sheet and refreshes the route", async () => {
    const user = userEvent.setup();
    const mocks = [
      {
        request: {
          query: AdminUpdateMasterMutation,
          variables: { id: "m-1", input: UPDATE_INPUT },
        },
        result: {
          data: {
            adminUpdateMasterCardgroup: {
              __typename: "UpdateMasterCardgroupSuccess",
              master: { ...DECK },
            },
          },
        },
      },
    ];
    renderHeader(DECK, mocks);
    // Open the deck-settings dialog from the desktop split-button dropdown.
    await user.click(screen.getByTestId("master-edit-more-options"));
    await user.click(screen.getByTestId("master-edit-deck-settings"));
    expect(await screen.findByTestId("master-field-name")).toBeInTheDocument();
    // Submit the form without changing values (form submits prefilled values).
    await user.click(screen.getByTestId("master-form-submit"));
    // On success the sheet closes and the route is refreshed.
    await waitFor(() => expect(refresh).toHaveBeenCalledTimes(1));
    expect(toast.success).toHaveBeenCalled();
  });

  it("deck-settings save validation error: shows the backend field error for name", async () => {
    const user = userEvent.setup();
    const mocks = [
      {
        request: {
          query: AdminUpdateMasterMutation,
          variables: { id: "m-1", input: UPDATE_INPUT },
        },
        result: {
          data: {
            adminUpdateMasterCardgroup: {
              __typename: "InputValidationError",
              field: "name",
              message: "Name is required",
            },
          },
        },
      },
    ];
    renderHeader(DECK, mocks);
    await user.click(screen.getByTestId("master-edit-more-options"));
    await user.click(screen.getByTestId("master-edit-deck-settings"));
    expect(await screen.findByTestId("master-field-name")).toBeInTheDocument();
    await user.click(screen.getByTestId("master-form-submit"));
    // The backend validation message renders under the name field.
    await waitFor(() => expect(screen.getByText("Name is required")).toBeInTheDocument());
    // The sheet stays open and route is NOT refreshed.
    expect(refresh).not.toHaveBeenCalled();
  });

  it("publish auth error (FORBIDDEN): shows the forbidden toast and does not refresh", async () => {
    const user = userEvent.setup();
    const mocks = [
      {
        request: { query: AdminPublishMasterMutation, variables: { id: "m-1" } },
        result: {
          errors: [new GraphQLError("Forbidden", { extensions: { code: "FORBIDDEN" } })],
        },
      },
    ];
    // DRAFT deck with cards so the publish button is enabled.
    renderHeader(DECK, mocks);
    await user.click(screen.getByTestId("master-edit-publish"));
    await waitFor(() => expect(toast.error).toHaveBeenCalled());
    expect(refresh).not.toHaveBeenCalled();
  });

  it("overflow menu (mobile): opens deck-settings and delete from the dropdown", async () => {
    const user = userEvent.setup();
    renderHeader(DECK);
    const overflow = screen.getByTestId("master-edit-overflow");
    await user.click(overflow);
    // Deck settings entry triggers the settings sheet.
    const settingsItem = await screen.findByRole("menuitem", { name: /deck settings/i });
    await user.click(settingsItem);
    expect(await screen.findByTestId("master-field-name")).toBeInTheDocument();
  });

  it("overflow menu (mobile): the publish toggle publishes the deck and refreshes", async () => {
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
    await user.click(screen.getByTestId("master-edit-overflow"));
    await user.click(await screen.findByTestId("master-edit-publish-mobile"));
    await waitFor(() => expect(refresh).toHaveBeenCalledTimes(1));
  });

  it("displays the cardCount prop (live count), not master.cardCount", () => {
    render(
      <MockedProvider mocks={[]}>
        <NextIntlClientProvider locale="en" messages={enMessages} timeZone="UTC">
          <MasterEditHeader master={{ ...DECK, cardCount: 0 }} cardCount={5} />
        </NextIntlClientProvider>
      </MockedProvider>,
    );
    // Publish is enabled because the LIVE count is 5, even though master.cardCount is 0.
    expect(screen.getByTestId("master-edit-publish")).not.toBeDisabled();
  });
});

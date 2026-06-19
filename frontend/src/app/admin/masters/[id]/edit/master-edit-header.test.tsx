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

const onBatchImport = vi.fn();

function renderHeader(
  overrides: Partial<AdminMasterDeck> & { cardCount?: number } = {},
  mocks: ReadonlyArray<unknown> = [],
) {
  const { cardCount, ...deckOverrides } = overrides;
  const deck = { ...DECK, ...deckOverrides };
  const resolvedCardCount = cardCount ?? deck.cardCount;
  return render(
    <MockedProvider mocks={mocks as never}>
      <NextIntlClientProvider locale="en" messages={enMessages} timeZone="UTC">
        <MasterEditHeader
          master={deck}
          cardCount={resolvedCardCount}
          onBatchImport={onBatchImport}
        />
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
    renderHeader();
    expect(screen.getByRole("heading", { level: 1, name: "Spanish A1" })).toBeInTheDocument();
    expect(screen.getByTestId("master-edit-status-badge")).toHaveTextContent("Draft");
    // Card count renders in both the mobile and desktop meta rows.
    expect(screen.getAllByText("5 cards").length).toBeGreaterThan(0);
  });

  it("disables publish and shows the hint for an empty draft", () => {
    renderHeader({ cardCount: 0 });
    expect(screen.getByTestId("master-edit-publish")).toBeDisabled();
    expect(screen.getByTestId("master-edit-empty-hint")).toBeInTheDocument();
  });

  it("opens the deck-settings dialog with the metadata form (no publish/delete sections)", async () => {
    const user = userEvent.setup();
    renderHeader();
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
    renderHeader({}, mocks);
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
    renderHeader({ status: "PUBLISHED" }, mocks);
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
    renderHeader({}, mocks);
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
    renderHeader({}, mocks);
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
    renderHeader({}, mocks);
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
    renderHeader({}, mocks);
    await user.click(screen.getByTestId("master-edit-publish"));
    await waitFor(() => expect(toast.error).toHaveBeenCalled());
    expect(refresh).not.toHaveBeenCalled();
  });

  it("mobile: overflow menu opens deck-settings sheet", async () => {
    const user = userEvent.setup();
    renderHeader();
    const overflow = screen.getByTestId("master-edit-overflow");
    await user.click(overflow);
    // Deck settings entry triggers the settings sheet.
    const settingsItem = await screen.findByTestId("master-edit-deck-settings-mobile");
    await user.click(settingsItem);
    expect(await screen.findByTestId("master-field-name")).toBeInTheDocument();
  });

  it("mobile: status chip opens the publish toggle and calls the mutation", async () => {
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
    renderHeader({ status: "PUBLISHED", cardCount: 1246 }, mocks);
    await user.click(screen.getByTestId("master-edit-status-chip"));
    const toggle = screen.getByTestId("master-edit-publish-mobile");
    expect(toggle).toHaveTextContent(/Unpublish/);
    await user.click(toggle);
    await waitFor(() => expect(refresh).toHaveBeenCalledTimes(1));
  });

  it("mobile: status chip publish item is disabled for an empty draft, with the hint", async () => {
    const user = userEvent.setup();
    renderHeader({ status: "DRAFT", cardCount: 0 });
    expect(screen.getByTestId("master-edit-empty-hint")).toBeInTheDocument();
    await user.click(screen.getByTestId("master-edit-status-chip"));
    expect(screen.getByTestId("master-edit-publish-mobile")).toHaveAttribute(
      "aria-disabled",
      "true",
    );
  });

  it("mobile: overflow menu hosts batch import alongside Settings and Delete (no publish)", async () => {
    const user = userEvent.setup();
    renderHeader({ status: "PUBLISHED", cardCount: 5 });
    await user.click(screen.getByTestId("master-edit-overflow"));
    expect(screen.getByTestId("master-edit-import-mobile")).toBeInTheDocument();
    expect(screen.getByTestId("master-edit-deck-settings-mobile")).toBeInTheDocument();
    expect(screen.getByTestId("master-edit-delete-mobile")).toBeInTheDocument();
    // Publish stays on the status chip, not the overflow menu.
    expect(screen.queryByTestId("master-edit-publish-mobile")).toBeNull();
  });

  it("mobile: overflow 'Batch import' item calls onBatchImport", async () => {
    const user = userEvent.setup();
    renderHeader({ status: "PUBLISHED", cardCount: 5 });
    await user.click(screen.getByTestId("master-edit-overflow"));
    await user.click(screen.getByTestId("master-edit-import-mobile"));
    expect(onBatchImport).toHaveBeenCalledTimes(1);
  });

  it("renders the inline back link to the masters list", () => {
    renderHeader({ status: "DRAFT", cardCount: 3 });
    expect(screen.getByRole("link", { name: /back to masters/i })).toHaveAttribute(
      "href",
      "/admin/masters",
    );
  });

  it("displays the cardCount prop (live count), not master.cardCount", () => {
    // master.cardCount is 0 but the live cardCount prop is 5 — publish must be enabled.
    // renderHeader spreads overrides onto DECK; here we explicitly pass cardCount=5 while
    // the base DECK.cardCount would be 0 if we change it. To isolate, render directly.
    const { container } = render(
      <MockedProvider mocks={[]}>
        <NextIntlClientProvider locale="en" messages={enMessages} timeZone="UTC">
          <MasterEditHeader
            master={{ ...DECK, cardCount: 0 }}
            cardCount={5}
            onBatchImport={onBatchImport}
          />
        </NextIntlClientProvider>
      </MockedProvider>,
    );
    // Publish is enabled because the LIVE count is 5, even though master.cardCount is 0.
    const publishBtn = container.querySelector('[data-testid="master-edit-publish"]');
    expect(publishBtn).not.toBeDisabled();
  });
});

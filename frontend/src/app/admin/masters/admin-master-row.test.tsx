// @vitest-environment jsdom

import { MockedProvider } from "@apollo/client/testing/react";
import { fireEvent, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { renderWithIntl } from "@/test/render-with-intl";
import { type AdminMasterListItem, AdminMasterRow } from "./admin-master-row";
import { AdminPublishMasterMutation, AdminUnpublishMasterMutation } from "./queries";

vi.mock("sonner", () => ({ toast: { success: vi.fn(), error: vi.fn() } }));

const BASE: AdminMasterListItem = {
  id: "m-1",
  version: 1,
  name: "Spanish A1",
  description: null,
  language: null,
  level: null,
  category: null,
  coverImageUrl: null,
  source: null,
  isDefaultStarter: false,
  sortOrder: 0,
  status: "DRAFT",
  cardCount: 42,
};

function publishedNode() {
  return {
    __typename: "MasterCardgroup" as const,
    ...BASE,
    status: "PUBLISHED" as const,
    version: 2,
  };
}

function renderRow(
  master: AdminMasterListItem,
  mocks: ReadonlyArray<unknown> = [],
  onEdit = vi.fn(),
) {
  renderWithIntl(
    <MockedProvider mocks={mocks as never}>
      <ul>
        <AdminMasterRow master={master} onEdit={onEdit} />
      </ul>
    </MockedProvider>,
  );
  return { onEdit };
}

describe("AdminMasterRow", () => {
  it("shows the name, card count, and an enabled Publish button for a DRAFT with cards", () => {
    renderRow(BASE);
    expect(screen.getByText("Spanish A1")).toBeInTheDocument();
    expect(screen.getByTestId("master-row-publish-toggle")).not.toHaveAttribute(
      "aria-disabled",
      "true",
    );
    expect(screen.getByTestId("master-row-publish-toggle")).toHaveAttribute(
      "aria-pressed",
      "false",
    );
  });

  it("disables Publish and shows a hint for a DRAFT with 0 cards", () => {
    renderRow({ ...BASE, cardCount: 0 });
    expect(screen.getByTestId("master-row-publish-toggle")).toHaveAttribute(
      "aria-disabled",
      "true",
    );
    expect(screen.getByTestId("master-publish-empty-hint")).toBeInTheDocument();
  });

  it("does not fire the mutation when clicking a disabled empty-deck Publish", async () => {
    renderRow({ ...BASE, cardCount: 0 });
    fireEvent.click(screen.getByTestId("master-row-publish-toggle"));
    const { toast } = await import("sonner");
    expect(toast.success).not.toHaveBeenCalled();
    expect(toast.error).not.toHaveBeenCalled();
    expect(screen.getByTestId("master-publish-empty-hint")).toBeInTheDocument();
  });

  it("invokes onEdit when the Edit button is clicked", async () => {
    const user = userEvent.setup();
    const { onEdit } = renderRow(BASE);
    await user.click(screen.getByTestId("master-row-edit"));
    expect(onEdit).toHaveBeenCalledWith("m-1");
  });

  it("publishes a DRAFT with cards and toasts success", async () => {
    const user = userEvent.setup();
    const mocks = [
      {
        request: { query: AdminPublishMasterMutation, variables: { id: "m-1" } },
        result: {
          data: {
            adminPublishMasterCardgroup: {
              __typename: "PublishMasterCardgroupSuccess",
              master: publishedNode(),
            },
          },
        },
      },
    ];
    renderRow(BASE, mocks);
    await user.click(screen.getByTestId("master-row-publish-toggle"));
    const { toast } = await import("sonner");
    await waitFor(() => expect(toast.success).toHaveBeenCalled());
  });

  it("renders an Unpublish toggle for a PUBLISHED master", () => {
    renderRow({ ...BASE, status: "PUBLISHED" });
    expect(screen.getByTestId("master-row-publish-toggle")).toHaveAccessibleName(
      /unpublish|非公開/i,
    );
    expect(screen.getByTestId("master-row-publish-toggle")).toHaveAttribute("aria-pressed", "true");
  });

  it("toasts an error when publish returns MasterCardgroupEmptyError", async () => {
    const user = userEvent.setup();
    const mocks = [
      {
        request: { query: AdminPublishMasterMutation, variables: { id: "m-1" } },
        result: {
          data: {
            adminPublishMasterCardgroup: {
              __typename: "MasterCardgroupEmptyError",
              message: "deck is empty",
            },
          },
        },
      },
    ];
    renderRow(BASE, mocks);
    await user.click(screen.getByTestId("master-row-publish-toggle"));
    const { toast } = await import("sonner");
    await waitFor(() => expect(toast.error).toHaveBeenCalled());
  });

  it("unpublishes a PUBLISHED master and toasts success", async () => {
    const user = userEvent.setup();
    const mocks = [
      {
        request: { query: AdminUnpublishMasterMutation, variables: { id: "m-1" } },
        result: {
          data: {
            adminUnpublishMasterCardgroup: {
              __typename: "MasterCardgroup",
              ...BASE,
              status: "DRAFT",
            },
          },
        },
      },
    ];
    renderRow({ ...BASE, status: "PUBLISHED" }, mocks);
    await user.click(screen.getByTestId("master-row-publish-toggle"));
    const { toast } = await import("sonner");
    await waitFor(() => expect(toast.success).toHaveBeenCalled());
  });
});

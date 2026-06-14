// @vitest-environment jsdom

import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { renderWithIntl } from "@/test/render-with-intl";
import { type AdminMasterListItem, AdminMasterRow } from "./admin-master-row";

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

function renderRow(master: AdminMasterListItem, onEdit = vi.fn()) {
  renderWithIntl(
    <ul>
      <AdminMasterRow master={master} onEdit={onEdit} />
    </ul>,
  );
  return { onEdit };
}

describe("AdminMasterRow", () => {
  it("shows the name, card count, and the Draft status badge", () => {
    renderRow(BASE);
    expect(screen.getByText("Spanish A1")).toBeInTheDocument();
    expect(screen.getByText("42 cards")).toBeInTheDocument();
    expect(screen.getByTestId("master-row-status-badge")).toHaveTextContent("Draft");
  });

  it("shows the Published status badge for a PUBLISHED master", () => {
    renderRow({ ...BASE, status: "PUBLISHED" });
    expect(screen.getByTestId("master-row-status-badge")).toHaveTextContent("Published");
  });

  it("renders no publish toggle — the publish action lives in the Edit panel", () => {
    renderRow(BASE);
    expect(screen.queryByTestId("master-row-publish-toggle")).not.toBeInTheDocument();
  });

  it("invokes onEdit when the Edit button is clicked", async () => {
    const user = userEvent.setup();
    const { onEdit } = renderRow(BASE);
    await user.click(screen.getByTestId("master-row-edit"));
    expect(onEdit).toHaveBeenCalledWith("m-1");
  });
});

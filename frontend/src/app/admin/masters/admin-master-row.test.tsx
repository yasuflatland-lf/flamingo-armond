// @vitest-environment jsdom

import { screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
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

function renderRow(master: AdminMasterListItem) {
  renderWithIntl(
    <ul>
      <AdminMasterRow master={master} />
    </ul>,
  );
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

  it("links the whole row to the edit route", () => {
    renderRow(BASE);
    const link = screen.getByRole("link", { name: /edit spanish a1/i });
    expect(link).toHaveAttribute("href", "/admin/masters/m-1/edit");
  });
});

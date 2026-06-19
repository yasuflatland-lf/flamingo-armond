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
  it("shows the name, card count, and the Draft status badge with a muted dot", () => {
    renderRow(BASE);
    expect(screen.getByText("Spanish A1")).toBeInTheDocument();
    expect(screen.getByText("42 cards")).toBeInTheDocument();
    const badge = screen.getByTestId("master-row-status-badge");
    expect(badge).toHaveTextContent("Draft");
    // Draft decks carry a muted status dot, not the published green.
    expect(badge.querySelector('[aria-hidden="true"]')?.className).toContain("bg-muted-foreground");
  });

  it("shows the Published status badge with the green status dot for a PUBLISHED master", () => {
    renderRow({ ...BASE, status: "PUBLISHED" });
    const badge = screen.getByTestId("master-row-status-badge");
    expect(badge).toHaveTextContent("Published");
    // Published decks carry the green (--success) status dot.
    expect(badge.querySelector('[aria-hidden="true"]')?.className).toContain("bg-success");
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

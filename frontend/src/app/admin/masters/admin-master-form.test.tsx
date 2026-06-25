// @vitest-environment jsdom

import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { renderWithIntl } from "@/test/render-with-intl";
import { AdminMasterForm } from "./admin-master-form";
import type { AdminMasterListItem } from "./admin-master-row";

const EXISTING: AdminMasterListItem = {
  id: "m-1",
  version: 3,
  name: "Spanish A1",
  description: "Beginner",
  isDefaultStarter: false,
  sortOrder: 5,
  status: "DRAFT",
  cardCount: 10,
};

describe("AdminMasterForm", () => {
  it("renders the four editable fields in create mode", () => {
    renderWithIntl(<AdminMasterForm mode="create" submitting={false} submit={vi.fn()} />);
    for (const id of ["name", "description", "isDefaultStarter", "sortOrder"]) {
      expect(screen.getByTestId(`master-field-${id}`)).toBeInTheDocument();
    }
  });

  it("does not render the removed metadata fields", () => {
    renderWithIntl(<AdminMasterForm mode="create" submitting={false} submit={vi.fn()} />);
    for (const id of ["language", "level", "category", "coverImageUrl", "source"]) {
      expect(screen.queryByTestId(`master-field-${id}`)).not.toBeInTheDocument();
    }
  });

  it("blocks submit when the name is empty", async () => {
    const submit = vi.fn();
    const user = userEvent.setup();
    renderWithIntl(<AdminMasterForm mode="create" submitting={false} submit={submit} />);
    await user.click(screen.getByTestId("master-form-submit"));
    expect(submit).not.toHaveBeenCalled();
  });

  it("submits trimmed values in create mode", async () => {
    const submit = vi.fn().mockResolvedValue(undefined);
    const user = userEvent.setup();
    renderWithIntl(<AdminMasterForm mode="create" submitting={false} submit={submit} />);
    await user.type(screen.getByTestId("master-field-name"), "  New Deck  ");
    await user.type(screen.getByTestId("master-field-sortOrder"), "7");
    await user.click(screen.getByTestId("master-form-submit"));
    await waitFor(() => expect(submit).toHaveBeenCalledTimes(1));
    const firstCallArg = submit.mock.calls[0]?.[0];
    expect(firstCallArg).toMatchObject({ name: "New Deck", sortOrder: 7 });
  });

  it("blocks submit when sortOrder is not a whole number", async () => {
    const submit = vi.fn();
    const user = userEvent.setup();
    renderWithIntl(<AdminMasterForm mode="create" submitting={false} submit={submit} />);
    await user.type(screen.getByTestId("master-field-name"), "Deck");
    await user.type(screen.getByTestId("master-field-sortOrder"), "1.5");
    await user.click(screen.getByTestId("master-form-submit"));
    expect(submit).not.toHaveBeenCalled();
  });

  it("prefills edit mode fields", () => {
    renderWithIntl(
      <AdminMasterForm mode="edit" master={EXISTING} submitting={false} submit={vi.fn()} />,
    );
    expect(screen.getByTestId("master-field-name")).toHaveValue("Spanish A1");
  });

  it("collapses empty optional fields to null and empty sortOrder to null", async () => {
    const submit = vi.fn().mockResolvedValue(undefined);
    const user = userEvent.setup();
    renderWithIntl(<AdminMasterForm mode="create" submitting={false} submit={submit} />);
    await user.type(screen.getByTestId("master-field-name"), "Only Name");
    await user.click(screen.getByTestId("master-form-submit"));
    await waitFor(() => expect(submit).toHaveBeenCalledTimes(1));
    expect(submit.mock.calls[0]?.[0]).toMatchObject({
      name: "Only Name",
      description: null,
      sortOrder: null,
      isDefaultStarter: false,
    });
  });

  it("surfaces a field validation error from the parent", () => {
    renderWithIntl(
      <AdminMasterForm
        mode="edit"
        master={EXISTING}
        submitting={false}
        submit={vi.fn()}
        validationError={{ field: "name", message: "name is taken" }}
      />,
    );
    expect(screen.getByText("name is taken")).toBeInTheDocument();
  });

  it("renders no Publishing section in create mode", () => {
    renderWithIntl(<AdminMasterForm mode="create" submitting={false} submit={vi.fn()} />);
    expect(screen.queryByTestId("master-publish-section")).not.toBeInTheDocument();
  });
});

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
  language: "es",
  level: "A1",
  category: "language",
  coverImageUrl: null,
  source: null,
  isDefaultStarter: false,
  sortOrder: 5,
  status: "DRAFT",
  cardCount: 10,
};

describe("AdminMasterForm", () => {
  it("renders all nine editable fields in create mode", () => {
    renderWithIntl(<AdminMasterForm mode="create" submitting={false} submit={vi.fn()} />);
    for (const id of [
      "name",
      "description",
      "language",
      "level",
      "category",
      "coverImageUrl",
      "source",
      "isDefaultStarter",
      "sortOrder",
    ]) {
      expect(screen.getByTestId(`master-field-${id}`)).toBeInTheDocument();
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

  it("prefills edit mode and shows the Danger zone", () => {
    renderWithIntl(
      <AdminMasterForm
        mode="edit"
        master={EXISTING}
        submitting={false}
        submit={vi.fn()}
        onDelete={vi.fn()}
      />,
    );
    expect(screen.getByTestId("master-field-name")).toHaveValue("Spanish A1");
    expect(screen.getByTestId("master-row-delete-trigger")).toBeInTheDocument();
  });

  it("calls onDelete after confirming in the dialog", async () => {
    const onDelete = vi.fn().mockResolvedValue(undefined);
    const user = userEvent.setup();
    renderWithIntl(
      <AdminMasterForm
        mode="edit"
        master={EXISTING}
        submitting={false}
        submit={vi.fn()}
        onDelete={onDelete}
      />,
    );
    await user.click(screen.getByTestId("master-row-delete-trigger"));
    await user.click(screen.getByTestId("master-delete-dialog-confirm"));
    await waitFor(() => expect(onDelete).toHaveBeenCalledWith("m-1"));
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
      language: null,
      level: null,
      category: null,
      coverImageUrl: null,
      source: null,
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
});

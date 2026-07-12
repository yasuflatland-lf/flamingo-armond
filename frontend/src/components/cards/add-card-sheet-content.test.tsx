// @vitest-environment happy-dom
import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { FormSheet } from "@/components/ui/form-sheet";
import { renderWithIntl } from "@/test/render-with-intl";
import { AddCardSheetContent } from "./add-card-sheet-content";

type Props = Parameters<typeof AddCardSheetContent>[0];

function renderContent(props: Partial<Props> = {}) {
  const onOpenChange = vi.fn();
  const merged: Props = {
    idPrefix: "add-card-",
    submit: vi.fn().mockResolvedValue(undefined),
    submitting: false,
    error: null,
    validationError: null,
    onDirtyChange: vi.fn(),
    ...props,
  };
  const result = renderWithIntl(
    <FormSheet title="Add card" open onOpenChange={onOpenChange}>
      <AddCardSheetContent {...merged} />
    </FormSheet>,
  );
  return { ...result, onOpenChange, props: merged };
}

describe("<AddCardSheetContent>", () => {
  it("renders the create form with the front and back fields", () => {
    renderContent();

    expect(screen.getByRole("textbox", { name: /front/i })).toBeInTheDocument();
    expect(screen.getByRole("textbox", { name: /back/i })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /^add$/i })).toBeInTheDocument();
  });

  it("namespaces the field ids with the supplied idPrefix", () => {
    renderContent({ idPrefix: "learn-add-card-" });

    expect(screen.getByRole("textbox", { name: /front/i })).toHaveAttribute(
      "id",
      "learn-add-card-front-field",
    );
  });

  it("shows the added-count row when addedCount is greater than zero", () => {
    renderContent({ addedCount: 2 });

    const row = screen.getByTestId("add-card-added-count");
    expect(row).toBeInTheDocument();
    expect(row).toHaveTextContent("2 added");
  });

  it("hides the added-count row when addedCount is zero", () => {
    renderContent({ addedCount: 0 });

    expect(screen.queryByTestId("add-card-added-count")).toBeNull();
  });

  it("hides the added-count row when addedCount is omitted (learn path)", () => {
    renderContent();

    expect(screen.queryByTestId("add-card-added-count")).toBeNull();
  });

  it("reports isDirty via onDirtyChange when the user types into a field", async () => {
    const user = userEvent.setup();
    const { props } = renderContent();

    // Mounts clean.
    expect(props.onDirtyChange).toHaveBeenLastCalledWith(false);

    await user.type(screen.getByRole("textbox", { name: /front/i }), "Hello");

    await waitFor(() => {
      expect(props.onDirtyChange).toHaveBeenLastCalledWith(true);
    });
  });

  it("requests the sheet to close when Cancel is clicked", async () => {
    const user = userEvent.setup();
    const { onOpenChange } = renderContent();

    await user.click(screen.getByRole("button", { name: /cancel/i }));

    expect(onOpenChange).toHaveBeenCalledWith(false);
  });
});

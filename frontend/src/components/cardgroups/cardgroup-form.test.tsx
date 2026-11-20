// @vitest-environment happy-dom
import { MockedProvider } from "@apollo/client/testing/react";
import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { renderWithIntl } from "@/test/render-with-intl";
import { CardgroupForm } from "./cardgroup-form";

// Wrap with MockedProvider to mirror the profile-form.test.tsx recipe.
// The form itself does not run a mutation, but the parent's submit might.
function renderForm(props: Partial<Parameters<typeof CardgroupForm>[0]> = {}) {
  const submit = vi.fn().mockResolvedValue(undefined);
  renderWithIntl(
    <MockedProvider mocks={[]}>
      <CardgroupForm
        mode={props.mode ?? "create"}
        defaultValues={props.defaultValues ?? { name: "" }}
        submit={props.submit ?? submit}
        submitting={props.submitting}
        validationError={props.validationError}
        secondarySlot={props.secondarySlot}
        onDirtyChange={props.onDirtyChange}
      />
    </MockedProvider>,
  );
  return { submit };
}

describe("<CardgroupForm>", () => {
  it("shows name input populated with defaultValues.name", () => {
    renderForm({ defaultValues: { name: "My Group" } });
    expect(screen.getByDisplayValue("My Group")).toBeInTheDocument();
  });

  it("shows empty input when defaultValues.name is empty", () => {
    renderForm({ defaultValues: { name: "" } });
    const input = screen.getByRole("textbox");
    expect(input).toHaveValue("");
  });

  it("submitting empty name shows inline required error", async () => {
    const user = userEvent.setup();
    renderForm({ defaultValues: { name: "" } });

    const input = screen.getByRole("textbox");
    await user.click(input);
    await user.tab();

    await waitFor(() => {
      expect(screen.getByText("name is required")).toBeInTheDocument();
    });
  });

  it("submitting a name with more than 100 graphemes shows length error", async () => {
    const user = userEvent.setup();
    renderForm({ defaultValues: { name: "" } });

    const input = screen.getByRole("textbox");
    await user.click(input);
    await user.paste("x".repeat(101));
    await user.tab();

    await waitFor(() => {
      expect(screen.getByText("name must be at most 100 characters")).toBeInTheDocument();
    });
  });

  it("uses mode-based default label: Create in create mode", () => {
    renderForm({ mode: "create" });
    expect(screen.getByRole("button", { name: /create/i })).toBeInTheDocument();
  });

  it("uses mode-based default label: Save in edit mode", () => {
    renderForm({ mode: "edit", defaultValues: { name: "Existing" } });
    expect(screen.getByRole("button", { name: /save/i })).toBeInTheDocument();
  });

  it("calls submit with the current name value", async () => {
    const user = userEvent.setup();
    const submit = vi.fn().mockResolvedValue(undefined);
    renderForm({ mode: "create", defaultValues: { name: "Hello" }, submit });

    await user.click(screen.getByRole("button", { name: /create/i }));

    await waitFor(() => {
      expect(submit).toHaveBeenCalledWith({ name: "Hello" });
    });
  });

  it("renders secondarySlot next to the submit button", () => {
    renderForm({
      mode: "edit",
      defaultValues: { name: "Group" },
      secondarySlot: <button type="button">Delete</button>,
    });
    expect(screen.getByRole("button", { name: /delete/i })).toBeInTheDocument();
  });

  it("disables submit button when submitting=true", () => {
    renderForm({ mode: "edit", defaultValues: { name: "Group" }, submitting: true });
    const btn = screen.getByRole("button", { name: /saving/i });
    expect(btn).toBeDisabled();
  });

  it("validationError with field=name renders the server message as inline field error", () => {
    renderForm({
      defaultValues: { name: "duplicate" },
      validationError: { field: "name", message: "name already taken by another group" },
    });

    const msg = screen.getByText("name already taken by another group");
    expect(msg).toBeInTheDocument();
    expect(msg.className).toMatch(/text-destructive/);
  });

  it("validationError with field !== name renders nothing under the name field", () => {
    renderForm({
      defaultValues: { name: "x" },
      validationError: { field: "description", message: "description is too long" },
    });

    expect(screen.queryByText("description is too long")).not.toBeInTheDocument();
  });

  it("reports isDirty via onDirtyChange when the name field is edited", async () => {
    const user = userEvent.setup();
    const onDirtyChange = vi.fn();
    renderForm({ defaultValues: { name: "" }, onDirtyChange });

    // Mounts clean.
    expect(onDirtyChange).toHaveBeenLastCalledWith(false);

    await user.type(screen.getByLabelText(/name/i), "New group");
    await waitFor(() => {
      expect(onDirtyChange).toHaveBeenLastCalledWith(true);
    });
  });
});

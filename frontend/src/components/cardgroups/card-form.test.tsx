// @vitest-environment jsdom
import { CombinedGraphQLErrors } from "@apollo/client/errors";
import { MockedProvider } from "@apollo/client/testing/react";
import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { GraphQLError } from "graphql";
import { describe, expect, it, vi } from "vitest";
import { renderWithIntl } from "@/test/render-with-intl";
import { CardForm } from "./card-form";

function renderForm(props: Partial<Parameters<typeof CardForm>[0]> = {}) {
  const submit = vi.fn().mockResolvedValue(undefined);
  renderWithIntl(
    <MockedProvider mocks={[]}>
      <CardForm
        mode={props.mode ?? "create"}
        defaultValues={props.defaultValues ?? { front: "", back: "" }}
        submit={props.submit ?? submit}
        submitLabel={props.submitLabel}
        submitting={props.submitting}
        error={props.error}
        onCancel={props.onCancel}
      />
    </MockedProvider>,
  );
  return { submit };
}

describe("<CardForm>", () => {
  it("renders front and back inputs with default values", () => {
    renderForm({ defaultValues: { front: "Dog", back: "Perro" } });
    expect(screen.getByDisplayValue("Dog")).toBeInTheDocument();
    expect(screen.getByDisplayValue("Perro")).toBeInTheDocument();
  });

  it("submitting empty front shows inline required error", async () => {
    const user = userEvent.setup();
    renderForm();

    const frontInput = screen.getByLabelText(/front/i);
    await user.click(frontInput);
    await user.tab();

    await waitFor(() => {
      expect(screen.getByText("front is required")).toBeInTheDocument();
    });
  });

  it("submitting empty back shows inline required error", async () => {
    const user = userEvent.setup();
    renderForm();

    const backInput = screen.getByLabelText(/back/i);
    await user.click(backInput);
    await user.tab();

    await waitFor(() => {
      expect(screen.getByText("back is required")).toBeInTheDocument();
    });
  });

  it("renders backend BAD_USER_INPUT field error under the front field", () => {
    const gqlError = new GraphQLError("front already exists", {
      extensions: { code: "BAD_USER_INPUT", field: "front" },
    });
    const combinedError = new CombinedGraphQLErrors({ errors: [gqlError] });

    renderForm({ defaultValues: { front: "duplicate", back: "something" }, error: combinedError });

    expect(screen.getByText("front already exists")).toBeInTheDocument();
  });

  it("renders generic banner for a plain network Error", () => {
    const networkError = new Error("network down");
    renderForm({ defaultValues: { front: "x", back: "y" }, error: networkError });

    expect(screen.getByText("Could not reach the server. Please try again.")).toBeInTheDocument();
    expect(screen.getByRole("alert")).toBeInTheDocument();
  });

  it("uses mode-based default label: Add in create mode", () => {
    renderForm({ mode: "create" });
    expect(screen.getByRole("button", { name: /^add$/i })).toBeInTheDocument();
  });

  it("uses mode-based default label: Save in edit mode", () => {
    renderForm({ mode: "edit", defaultValues: { front: "x", back: "y" } });
    expect(screen.getByRole("button", { name: /^save$/i })).toBeInTheDocument();
  });

  it("calls submit with front and back values", async () => {
    const user = userEvent.setup();
    const submit = vi.fn().mockResolvedValue(undefined);
    renderForm({ defaultValues: { front: "Cat", back: "Gato" }, submit });

    await user.click(screen.getByRole("button", { name: /^add$/i }));

    await waitFor(() => {
      expect(submit).toHaveBeenCalledWith({ front: "Cat", back: "Gato" });
    });
  });

  it("renders Cancel button when onCancel is provided", () => {
    const onCancel = vi.fn();
    renderForm({ onCancel });
    expect(screen.getByRole("button", { name: /cancel/i })).toBeInTheDocument();
  });

  it("calls onCancel when Cancel button is clicked", async () => {
    const user = userEvent.setup();
    const onCancel = vi.fn();
    renderForm({ onCancel });

    await user.click(screen.getByRole("button", { name: /cancel/i }));
    expect(onCancel).toHaveBeenCalledOnce();
  });

  it("disables submit button when submitting=true", () => {
    renderForm({ submitting: true });
    const btn = screen.getByRole("button", { name: /saving/i });
    expect(btn).toBeDisabled();
  });
});

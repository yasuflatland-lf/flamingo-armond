// @vitest-environment jsdom
import { CombinedGraphQLErrors } from "@apollo/client/errors";
import { MockedProvider } from "@apollo/client/testing/react";
import { useForm } from "@tanstack/react-form";
import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { GraphQLError } from "graphql";
import { describe, expect, it, vi } from "vitest";
import { renderWithIntl } from "@/test/render-with-intl";
import { CardgroupForm } from "./cardgroup-form";

/**
 * Test-only harness that renders CardgroupForm and additionally exposes
 * form.state.isSubmitSuccessful in the DOM via a sibling form.Subscribe node.
 * This is needed because CardgroupForm owns `form` internally and does not
 * expose it as a ref — the Subscribe selector is the only clean external probe.
 */
function CardgroupFormWithStatus({
  submit,
}: {
  submit: (values: { name: string }) => Promise<void>;
}) {
  const form = useForm({
    defaultValues: { name: "Test Group" },
    onSubmit: async ({ value }) => {
      await submit(value).catch((err) => {
        console.error("[cardgroup-form] submit rejected", err);
        throw err; // keep formState.isSubmitSuccessful correct
      });
    },
  });

  return (
    <>
      <form
        onSubmit={(e) => {
          e.preventDefault();
          e.stopPropagation();
          form.handleSubmit().catch(() => {
            // swallow re-thrown rejection to avoid unhandled browser promise rejection
          });
        }}
      >
        <button type="submit">Create</button>
      </form>
      <form.Subscribe selector={(state) => state.isSubmitSuccessful}>
        {(isSubmitSuccessful) => (
          <div data-testid="is-submit-successful">{String(isSubmitSuccessful)}</div>
        )}
      </form.Subscribe>
    </>
  );
}

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
        submitLabel={props.submitLabel}
        submitting={props.submitting}
        error={props.error}
        validationError={props.validationError}
        secondarySlot={props.secondarySlot}
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

  it("renders backend BAD_USER_INPUT field error under the name field", () => {
    // Build a CombinedGraphQLErrors that carries a BAD_USER_INPUT with field:"name".
    const gqlError = new GraphQLError("name already exists", {
      extensions: { code: "BAD_USER_INPUT", field: "name" },
    });
    // CombinedGraphQLErrors wraps an array of GraphQLErrors.
    const combinedError = new CombinedGraphQLErrors({ errors: [gqlError] });

    renderForm({ defaultValues: { name: "duplicate" }, error: combinedError });

    expect(screen.getByText("name already exists")).toBeInTheDocument();
    const errorEl = screen.getByText("name already exists");
    expect(errorEl.className).toMatch(/text-destructive/);
  });

  it("renders generic banner for a plain network Error (non-CombinedGraphQLErrors)", () => {
    const networkError = new Error("network down");
    renderForm({ defaultValues: { name: "anything" }, error: networkError });

    expect(screen.getByText("Could not reach the server. Please try again.")).toBeInTheDocument();
    const banner = screen.getByRole("alert");
    expect(banner).toBeInTheDocument();
  });

  it("uses mode-based default label: Create in create mode", () => {
    renderForm({ mode: "create" });
    expect(screen.getByRole("button", { name: /create/i })).toBeInTheDocument();
  });

  it("uses mode-based default label: Save in edit mode", () => {
    renderForm({ mode: "edit", defaultValues: { name: "Existing" } });
    expect(screen.getByRole("button", { name: /save/i })).toBeInTheDocument();
  });

  it("uses custom submitLabel when provided", () => {
    renderForm({ mode: "create", submitLabel: "Add Group" });
    expect(screen.getByRole("button", { name: /add group/i })).toBeInTheDocument();
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
    // The validationError prop takes precedence over fieldErrors.name when
    // validationError.field === "name". FieldError picks backendError over
    // zodErrors, so the server message appears under the Name label.
    renderForm({
      defaultValues: { name: "duplicate" },
      validationError: { field: "name", message: "name already taken by another group" },
    });

    const msg = screen.getByText("name already taken by another group");
    expect(msg).toBeInTheDocument();
    expect(msg.className).toMatch(/text-destructive/);
  });

  it("validationError with field !== name falls back to fieldErrors.name from error prop", () => {
    // When validationError.field is not "name", the form expression
    // `validationError?.field === "name" ? validationError.message : fieldErrors.name`
    // falls through to fieldErrors.name. The error prop carries a BAD_USER_INPUT
    // with field:"name" so fieldErrors.name should be rendered.
    const gqlError = new GraphQLError("name already exists", {
      extensions: { code: "BAD_USER_INPUT", field: "name" },
    });
    const combinedError = new CombinedGraphQLErrors({ errors: [gqlError] });

    renderForm({
      defaultValues: { name: "duplicate" },
      error: combinedError,
      validationError: { field: "description", message: "description is too long" },
    });

    // The BAD_USER_INPUT field error for "name" is shown (not the validationError message).
    expect(screen.getByText("name already exists")).toBeInTheDocument();
    expect(screen.queryByText("description is too long")).not.toBeInTheDocument();
  });

  it("validationError=null preserves existing error/fieldErrors behaviour", () => {
    // When validationError is null, the form falls back to fieldErrors.name
    // derived from the error prop, matching the legacy Tier C behavior.
    const gqlError = new GraphQLError("name conflict", {
      extensions: { code: "BAD_USER_INPUT", field: "name" },
    });
    const combinedError = new CombinedGraphQLErrors({ errors: [gqlError] });

    renderForm({
      defaultValues: { name: "conflict" },
      error: combinedError,
      validationError: null,
    });

    expect(screen.getByText("name conflict")).toBeInTheDocument();
    const errorEl = screen.getByText("name conflict");
    expect(errorEl.className).toMatch(/text-destructive/);
  });

  it("keeps formState.isSubmitSuccessful=false after a rejecting submit (regression: issue #111)", async () => {
    const user = userEvent.setup();
    const submit = vi.fn().mockRejectedValue(new Error("network down"));

    renderWithIntl(
      <MockedProvider mocks={[]}>
        <CardgroupFormWithStatus submit={submit} />
      </MockedProvider>,
    );

    // isSubmitSuccessful starts false
    expect(screen.getByTestId("is-submit-successful")).toHaveTextContent("false");

    await user.click(screen.getByRole("button", { name: /create/i }));

    // After a rejected submit, isSubmitSuccessful must remain false — not flip to true.
    await waitFor(() => {
      expect(submit).toHaveBeenCalledOnce();
    });
    expect(screen.getByTestId("is-submit-successful")).toHaveTextContent("false");
  });
});

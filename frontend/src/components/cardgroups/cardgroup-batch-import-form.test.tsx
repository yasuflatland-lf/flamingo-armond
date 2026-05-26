// @vitest-environment jsdom
import { ApolloClient } from "@apollo/client";
import type { MockedResponse } from "@apollo/client/testing";
import { MockedProvider } from "@apollo/client/testing/react";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import {
  CardsByCardgroupConnectionDocument,
  ImportCardsDocument,
  ValidateCardImportDocument,
} from "@/generated/graphql";
import { CardgroupBatchImportForm, resolveStep1Button } from "./cardgroup-batch-import-form";

const CARDGROUP_ID = "cg-1";
const CARDGROUP_NAME = "Spanish Vocab";

// encodePayload mirrors the component: UTF-8 safe base64.
function encodePayload(text: string): string {
  return btoa(unescape(encodeURIComponent(text)));
}

const TWO_LINE_TEXT = "apple\tred fruit\nbanana\tyellow fruit";

function validateMock(text: string, result: MockedResponse["result"]): MockedResponse {
  return {
    request: {
      query: ValidateCardImportDocument,
      variables: { input: { payload: encodePayload(text) } },
    },
    result,
  };
}

function importMock(text: string, result: MockedResponse["result"]): MockedResponse {
  return {
    request: {
      query: ImportCardsDocument,
      variables: { input: { cardgroupId: CARDGROUP_ID, payload: encodePayload(text) } },
    },
    result,
  };
}

const VALID_RESULT: MockedResponse["result"] = {
  data: {
    validateCardImport: {
      __typename: "CardImportValidationResult" as const,
      valid: true,
      parsedCards: [
        { __typename: "ParsedCard" as const, front: "apple", back: "red fruit", line: 1 },
        { __typename: "ParsedCard" as const, front: "banana", back: "yellow fruit", line: 2 },
      ],
      errors: [],
    },
  },
};

const INVALID_RESULT: MockedResponse["result"] = {
  data: {
    validateCardImport: {
      __typename: "CardImportValidationResult" as const,
      valid: false,
      parsedCards: [],
      errors: [
        {
          __typename: "CardImportError" as const,
          line: 1,
          message: "missing tab separator",
        },
      ],
    },
  },
};

function renderForm(mocks: MockedResponse[] = []) {
  const onImported = vi.fn();
  const utils = render(
    <MockedProvider mocks={mocks}>
      <CardgroupBatchImportForm
        cardgroupId={CARDGROUP_ID}
        cardgroupName={CARDGROUP_NAME}
        onImported={onImported}
      />
    </MockedProvider>,
  );
  return { onImported, ...utils };
}

async function typePayload(user: ReturnType<typeof userEvent.setup>, text: string) {
  const textarea = screen.getByLabelText(/cards to import/i);
  await user.click(textarea);
  // userEvent.type interprets tab/newline; paste sets the literal value.
  await user.paste(text);
  return textarea;
}

// Advance to step 2 by typing valid text, validating, and clicking Continue.
async function advanceToStep2(user: ReturnType<typeof userEvent.setup>) {
  await typePayload(user, TWO_LINE_TEXT);
  await user.click(screen.getByRole("button", { name: /^validate$/i }));
  const continueButton = await screen.findByRole("button", { name: /continue/i });
  await waitFor(() => expect(continueButton).toBeEnabled());
  await user.click(continueButton);
}

describe("resolveStep1Button", () => {
  it("empty: text blank -> Validate, no action, disabled", () => {
    expect(
      resolveStep1Button({ hasText: false, validating: false, result: null, isStale: false }),
    ).toEqual({ label: "Validate", action: null, disabled: true });
  });

  it("ready: text present, not yet validated -> Validate, validate action, enabled", () => {
    expect(
      resolveStep1Button({ hasText: true, validating: false, result: null, isStale: false }),
    ).toEqual({ label: "Validate", action: "validate", disabled: false });
  });

  it("validating: request in flight -> Validating..., no action, disabled", () => {
    expect(
      resolveStep1Button({ hasText: true, validating: true, result: null, isStale: false }),
    ).toEqual({ label: "Validating...", action: null, disabled: true });
  });

  it("valid + fresh: -> Continue, continue action, enabled", () => {
    expect(
      resolveStep1Button({
        hasText: true,
        validating: false,
        result: { valid: true, parsedCards: [{ front: "a", back: "b", line: 1 }], errors: [] },
        isStale: false,
      }),
    ).toEqual({ label: "Continue →", action: "continue", disabled: false });
  });

  it("valid but stale (edited since validate): -> Validate, validate action, enabled", () => {
    expect(
      resolveStep1Button({
        hasText: true,
        validating: false,
        result: { valid: true, parsedCards: [{ front: "a", back: "b", line: 1 }], errors: [] },
        isStale: true,
      }),
    ).toEqual({ label: "Validate", action: "validate", disabled: false });
  });

  it("invalid: -> Validate (re-run), validate action, enabled", () => {
    expect(
      resolveStep1Button({
        hasText: true,
        validating: false,
        result: { valid: false, parsedCards: [], errors: [{ line: 1, message: "x" }] },
        isStale: false,
      }),
    ).toEqual({ label: "Validate", action: "validate", disabled: false });
  });
});

describe("<CardgroupBatchImportForm>", () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("shows the fixed target name and no cardgroup selector", () => {
    renderForm();
    expect(screen.getByText(CARDGROUP_NAME)).toBeInTheDocument();
    expect(screen.queryByRole("combobox")).not.toBeInTheDocument();
  });

  it("empty state: the forward button reads Validate and is disabled", () => {
    renderForm();
    const button = screen.getByRole("button", { name: /^validate$/i });
    expect(button).toBeDisabled();
  });

  it("valid validate: shows valid status, a collapsed preview, and a Continue button", async () => {
    const user = userEvent.setup();
    renderForm([validateMock(TWO_LINE_TEXT, VALID_RESULT)]);

    await typePayload(user, TWO_LINE_TEXT);
    await user.click(screen.getByRole("button", { name: /^validate$/i }));

    // Valid status with the check glyph and parsed count.
    await waitFor(() => {
      expect(screen.getByRole("status")).toHaveTextContent(/✓ Valid — 2 cards parsed/i);
    });
    // Collapsible trigger is present and collapsed: the preview row is hidden.
    const trigger = screen.getByRole("button", { name: /show preview \(2\)/i });
    expect(trigger).toBeInTheDocument();
    expect(screen.queryByText("apple")).not.toBeInTheDocument();
    // Forward action is now Continue.
    expect(screen.getByRole("button", { name: /continue/i })).toBeEnabled();

    // Expanding the collapsible reveals the preview rows.
    await user.click(trigger);
    await waitFor(() => {
      expect(screen.getByText("apple")).toBeInTheDocument();
    });
    expect(screen.getByText("banana")).toBeInTheDocument();
  });

  it("editing the textarea after a valid validate reverts the button to Validate", async () => {
    const user = userEvent.setup();
    renderForm([validateMock(TWO_LINE_TEXT, VALID_RESULT)]);

    await typePayload(user, TWO_LINE_TEXT);
    await user.click(screen.getByRole("button", { name: /^validate$/i }));
    await waitFor(() => {
      expect(screen.getByRole("button", { name: /continue/i })).toBeEnabled();
    });

    // Append text — validatedPayload now differs from payloadText.
    await user.click(screen.getByLabelText(/cards to import/i));
    await user.paste("\ncherry\tcherry");

    await waitFor(() => {
      expect(screen.queryByRole("button", { name: /continue/i })).not.toBeInTheDocument();
    });
    expect(screen.getByRole("button", { name: /^validate$/i })).toBeEnabled();
  });

  it("invalid validate: shows invalid status, an open collapsible, error rows, button stays Validate", async () => {
    const user = userEvent.setup();
    renderForm([validateMock("broken line", INVALID_RESULT)]);

    await typePayload(user, "broken line");
    await user.click(screen.getByRole("button", { name: /^validate$/i }));

    await waitFor(() => {
      expect(screen.getByRole("status")).toHaveTextContent(/✕ Invalid — 1 error/i);
    });
    // Errors visible immediately (collapsible open by default when invalid).
    expect(screen.getByText(/missing tab separator/i)).toBeInTheDocument();
    const alerts = screen.getAllByRole("alert");
    expect(alerts.some((el) => /missing tab separator/i.test(el.textContent ?? ""))).toBe(true);
    // Forward action stays Validate; no Continue.
    expect(screen.queryByRole("button", { name: /continue/i })).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: /^validate$/i })).toBeEnabled();
  });

  it("Continue advances to step 2 with a confirm heading and a back affordance", async () => {
    const user = userEvent.setup();
    renderForm([validateMock(TWO_LINE_TEXT, VALID_RESULT)]);

    await advanceToStep2(user);

    // Confirm heading uses the parsed count + group name.
    expect(
      screen.getByRole("heading", { name: /import 2 cards into spanish vocab\?/i }),
    ).toBeInTheDocument();
    // Step counter reads 2 of 2.
    expect(screen.getByText(/2 of 2/i)).toBeInTheDocument();
    // The completed step 1 is a reachable back button in the breadcrumb.
    expect(screen.getByRole("button", { name: /paste & review/i })).toBeInTheDocument();
    // The import button reads "Import N cards".
    expect(screen.getByRole("button", { name: /import 2 cards/i })).toBeEnabled();
  });

  it("the back link returns to step 1 preserving the textarea content", async () => {
    const user = userEvent.setup();
    renderForm([validateMock(TWO_LINE_TEXT, VALID_RESULT)]);

    await advanceToStep2(user);

    await user.click(screen.getByRole("button", { name: /back to edit/i }));

    // Back on step 1: textarea content preserved.
    const textarea = screen.getByLabelText(/cards to import/i);
    expect(textarea).toHaveValue(TWO_LINE_TEXT);
    expect(screen.getByText(/1 of 2/i)).toBeInTheDocument();
  });

  it("clicking the completed breadcrumb step also returns to step 1", async () => {
    const user = userEvent.setup();
    renderForm([validateMock(TWO_LINE_TEXT, VALID_RESULT)]);

    await advanceToStep2(user);

    await user.click(screen.getByRole("button", { name: /paste & review/i }));

    const textarea = screen.getByLabelText(/cards to import/i);
    expect(textarea).toHaveValue(TWO_LINE_TEXT);
    expect(screen.getByText(/1 of 2/i)).toBeInTheDocument();
  });

  it("successful import calls onImported and refetches the cards list", async () => {
    const user = userEvent.setup();
    // Spy on the Apollo client's refetchQueries so we can assert the cards
    // connection document is the refetch target without needing a live
    // observer in this harness.
    const refetchSpy = vi
      .spyOn(ApolloClient.prototype, "refetchQueries")
      // biome-ignore lint/suspicious/noExplicitAny: test stub for refetchQueries return
      .mockResolvedValue([] as any);

    const { onImported } = renderForm([
      validateMock(TWO_LINE_TEXT, VALID_RESULT),
      importMock(TWO_LINE_TEXT, {
        data: {
          importCards: {
            __typename: "ImportCardsPayload" as const,
            inserted: 2,
            updated: 0,
            errors: [],
          },
        },
      }),
    ]);

    await advanceToStep2(user);
    await user.click(screen.getByRole("button", { name: /import 2 cards/i }));

    await waitFor(() => {
      expect(onImported).toHaveBeenCalledTimes(1);
    });
    expect(refetchSpy).toHaveBeenCalledTimes(1);
    expect(refetchSpy).toHaveBeenCalledWith(
      expect.objectContaining({ include: [CardsByCardgroupConnectionDocument] }),
    );
  });

  it("validate transport/network error shows a banner and leaves the forward action disabled", async () => {
    const user = userEvent.setup();
    renderForm([
      {
        request: {
          query: ValidateCardImportDocument,
          variables: { input: { payload: encodePayload(TWO_LINE_TEXT) } },
        },
        error: new Error("network error"),
      },
    ]);

    await typePayload(user, TWO_LINE_TEXT);
    await user.click(screen.getByRole("button", { name: /^validate$/i }));

    await waitFor(() => {
      expect(screen.getByRole("alert")).toBeInTheDocument();
    });
    // No preview rows should be rendered.
    expect(screen.queryByText("apple")).not.toBeInTheDocument();
    expect(screen.queryByText("banana")).not.toBeInTheDocument();
    // No Continue forward action — validation failed, so we stay on step 1.
    expect(screen.queryByRole("button", { name: /continue/i })).not.toBeInTheDocument();
  });

  it("import with error rows stays on step 2 and shows the result banner; Done then closes", async () => {
    const user = userEvent.setup();
    const { onImported } = renderForm([
      validateMock(TWO_LINE_TEXT, VALID_RESULT),
      importMock(TWO_LINE_TEXT, {
        data: {
          importCards: {
            __typename: "ImportCardsPayload" as const,
            inserted: 1,
            updated: 0,
            errors: [
              {
                __typename: "CardImportError" as const,
                line: 2,
                message: "duplicate front",
              },
            ],
          },
        },
      }),
    ]);

    await advanceToStep2(user);
    await user.click(screen.getByRole("button", { name: /import 2 cards/i }));

    await waitFor(() => {
      expect(screen.getByText(/duplicate front/i)).toBeInTheDocument();
    });
    expect(screen.getByText(/1 inserted, 0 updated/i)).toBeInTheDocument();
    // onImported NOT called on a partial-failure import.
    expect(onImported).not.toHaveBeenCalled();
    // Still on step 2.
    expect(screen.getByText(/2 of 2/i)).toBeInTheDocument();
    // Result banner has role=status.
    const status = screen.getAllByRole("status");
    expect(status.some((el) => /1 inserted/i.test(el.textContent ?? ""))).toBe(true);

    // Done closes the sheet.
    await user.click(screen.getByRole("button", { name: /^done$/i }));
    expect(onImported).toHaveBeenCalledTimes(1);
  });

  it("all-failed import shows the destructive 'no cards persisted' banner; Done still closes", async () => {
    const user = userEvent.setup();
    const refetchSpy = vi
      .spyOn(ApolloClient.prototype, "refetchQueries")
      // biome-ignore lint/suspicious/noExplicitAny: test stub for refetchQueries return
      .mockResolvedValue([] as any);

    const { onImported } = renderForm([
      validateMock(TWO_LINE_TEXT, VALID_RESULT),
      importMock(TWO_LINE_TEXT, {
        data: {
          importCards: {
            __typename: "ImportCardsPayload" as const,
            inserted: 0,
            updated: 0,
            errors: [
              {
                __typename: "CardImportError" as const,
                line: 1,
                message: "constraint violation",
              },
            ],
          },
        },
      }),
    ]);

    await advanceToStep2(user);
    await user.click(screen.getByRole("button", { name: /import 2 cards/i }));

    // Destructive all-failed banner instead of the green partial-success one.
    await waitFor(() => {
      expect(screen.getByText(/import failed: no cards persisted/i)).toBeInTheDocument();
    });
    // The result section is announced as a status region.
    const status = screen.getAllByRole("status");
    expect(
      status.some((el) => /import failed: no cards persisted/i.test(el.textContent ?? "")),
    ).toBe(true);
    // Error rows render.
    expect(screen.getByText(/constraint violation/i)).toBeInTheDocument();
    // onImported NOT called automatically on an all-failed import.
    expect(onImported).not.toHaveBeenCalled();
    expect(refetchSpy).toHaveBeenCalledTimes(1);

    // Done is unconditional and closes the sheet.
    await user.click(screen.getByRole("button", { name: /^done$/i }));
    expect(onImported).toHaveBeenCalledTimes(1);
  });

  it("import transport/network error shows a banner and stays on step 2 for retry", async () => {
    const user = userEvent.setup();
    const { onImported } = renderForm([
      validateMock(TWO_LINE_TEXT, VALID_RESULT),
      {
        request: {
          query: ImportCardsDocument,
          variables: {
            input: { cardgroupId: CARDGROUP_ID, payload: encodePayload(TWO_LINE_TEXT) },
          },
        },
        error: new Error("network error"),
      },
    ]);

    await advanceToStep2(user);
    await user.click(screen.getByRole("button", { name: /import 2 cards/i }));

    // handleImport's outer catch sets bannerError -> role=alert banner.
    await waitFor(() => {
      expect(screen.getByRole("alert")).toBeInTheDocument();
    });
    // No success callback on a rejected import.
    expect(onImported).not.toHaveBeenCalled();
    // Still on the confirm step.
    expect(screen.getByText(/2 of 2/i)).toBeInTheDocument();
    // The import button is still present and enabled for a retry.
    expect(screen.getByRole("button", { name: /import 2 cards/i })).toBeEnabled();
  });

  it("post-import '← Back to edit' returns to step 1 and re-requires a validate", async () => {
    const user = userEvent.setup();
    renderForm([
      validateMock(TWO_LINE_TEXT, VALID_RESULT),
      importMock(TWO_LINE_TEXT, {
        data: {
          importCards: {
            __typename: "ImportCardsPayload" as const,
            inserted: 1,
            updated: 0,
            errors: [
              {
                __typename: "CardImportError" as const,
                line: 2,
                message: "duplicate front",
              },
            ],
          },
        },
      }),
    ]);

    await advanceToStep2(user);
    await user.click(screen.getByRole("button", { name: /import 2 cards/i }));

    // Result banner visible on step 2 (partial failure).
    await waitFor(() => {
      expect(screen.getByText(/duplicate front/i)).toBeInTheDocument();
    });

    // The result panel's back button returns to step 1.
    await user.click(screen.getByRole("button", { name: /back to edit/i }));

    // Step 1, and the forward button reverted to Validate (stale guard cleared
    // validatedPayload after the import).
    expect(screen.getByText(/1 of 2/i)).toBeInTheDocument();
    const validateButton = screen.getByRole("button", { name: /^validate$/i });
    expect(validateButton).toBeEnabled();
    expect(screen.queryByRole("button", { name: /continue/i })).not.toBeInTheDocument();
    // Textarea content preserved.
    expect(screen.getByLabelText(/cards to import/i)).toHaveValue(TWO_LINE_TEXT);
  });
});

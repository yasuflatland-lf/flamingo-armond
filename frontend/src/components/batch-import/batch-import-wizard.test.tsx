// @vitest-environment jsdom
import { ApolloClient } from "@apollo/client";
import type { MockedResponse } from "@apollo/client/testing";
import { MockedProvider } from "@apollo/client/testing/react";
import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import {
  CardsByCardgroupConnectionDocument,
  ValidateCardImportDocument,
} from "@/generated/graphql";
import { renderWithIntl } from "@/test/render-with-intl";
import { BatchImportWizard, type ImportResult, resolveStep1Button } from "./batch-import-wizard";

const TARGET_ID = "tgt-1";
const TARGET_NAME = "Spanish Vocab";

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
        { __typename: "CardImportError" as const, line: 1, message: "missing tab separator" },
      ],
    },
  },
};

const OK_IMPORT: ImportResult = { inserted: 2, updated: 0, errors: [] };

function renderWizard(
  opts: {
    mocks?: MockedResponse[];
    onImport?: (payload: string) => Promise<ImportResult | null>;
    importing?: boolean;
  } = {},
) {
  const onImported = vi.fn();
  const onCancel = vi.fn();
  const onImport = opts.onImport ?? vi.fn(async () => OK_IMPORT);
  const utils = renderWithIntl(
    <MockedProvider mocks={opts.mocks ?? []}>
      <BatchImportWizard
        onImport={onImport}
        importing={opts.importing ?? false}
        refetchDocument={CardsByCardgroupConnectionDocument}
        targetId={TARGET_ID}
        targetName={TARGET_NAME}
        onImported={onImported}
        onCancel={onCancel}
      />
    </MockedProvider>,
  );
  return { onImported, onCancel, onImport, ...utils };
}

async function typePayload(user: ReturnType<typeof userEvent.setup>, text: string) {
  const textarea = screen.getByLabelText(/cards to import/i);
  await user.click(textarea);
  await user.paste(text);
  return textarea;
}

async function advanceToStep2(user: ReturnType<typeof userEvent.setup>) {
  await typePayload(user, TWO_LINE_TEXT);
  await user.click(screen.getByRole("button", { name: /^validate$/i }));
  const importButton = await screen.findByRole("button", { name: /^import$/i });
  await waitFor(() => expect(importButton).toBeEnabled());
  await user.click(importButton);
}

describe("resolveStep1Button", () => {
  it("empty: text blank -> validate, no action, disabled", () => {
    expect(
      resolveStep1Button({ hasText: false, validating: false, result: null, isStale: false }),
    ).toEqual({ labelKey: "validate", action: null, disabled: true });
  });

  it("ready: text present, not yet validated -> validate, validate action, enabled", () => {
    expect(
      resolveStep1Button({ hasText: true, validating: false, result: null, isStale: false }),
    ).toEqual({ labelKey: "validate", action: "validate", disabled: false });
  });

  it("validating: request in flight -> validating, no action, disabled", () => {
    expect(
      resolveStep1Button({ hasText: true, validating: true, result: null, isStale: false }),
    ).toEqual({ labelKey: "validating", action: null, disabled: true });
  });

  it("valid + fresh: -> import, continue action, enabled", () => {
    expect(
      resolveStep1Button({
        hasText: true,
        validating: false,
        result: { valid: true, parsedCards: [], errors: [] },
        isStale: false,
      }),
    ).toEqual({ labelKey: "import", action: "continue", disabled: false });
  });

  it("valid but stale (edited since validate): -> validate, validate action, enabled", () => {
    expect(
      resolveStep1Button({
        hasText: true,
        validating: false,
        result: { valid: true, parsedCards: [], errors: [] },
        isStale: true,
      }),
    ).toEqual({ labelKey: "validate", action: "validate", disabled: false });
  });

  it("invalid: -> validate (re-run), validate action, enabled", () => {
    expect(
      resolveStep1Button({
        hasText: true,
        validating: false,
        result: { valid: false, parsedCards: [], errors: [{ line: 1, message: "x" }] },
        isStale: false,
      }),
    ).toEqual({ labelKey: "validate", action: "validate", disabled: false });
  });
});

describe("<BatchImportWizard>", () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("empty state: the forward button reads Validate and is disabled", () => {
    renderWizard();
    const btn = screen.getByTestId("batch-import-step1-btn");
    expect(btn).toHaveTextContent(/validate/i);
    expect(btn).toBeDisabled();
  });

  it("step 1 progress bar caption reads Paste & review", () => {
    renderWizard();
    expect(screen.getByText(/paste & review/i)).toBeInTheDocument();
  });

  it("accessible name of the textarea contains both the label and the separator rule", () => {
    renderWizard();
    expect(screen.getByLabelText(/cards to import/i)).toHaveAccessibleName(
      /cards to import.*separate each pair with a tab/i,
    );
    // The accessible name comes from a single <label> whose text includes the Tab-separator hint.
    expect(document.querySelectorAll('label[for="batch-import-payload"]')).toHaveLength(1);
  });

  it("valid validate: shows valid status, a collapsed preview, and an Import button", async () => {
    const user = userEvent.setup();
    renderWizard({ mocks: [validateMock(TWO_LINE_TEXT, VALID_RESULT)] });
    await typePayload(user, TWO_LINE_TEXT);
    await user.click(screen.getByRole("button", { name: /^validate$/i }));
    expect(await screen.findByTestId("batch-import-validate-status")).toBeInTheDocument();
    expect(screen.getByRole("status")).toHaveTextContent(/✓ Valid — 2 cards parsed/i);
    expect(await screen.findByRole("button", { name: /^import$/i })).toBeEnabled();
    // Preview is collapsed by default.
    expect(screen.queryByText("apple")).toBeNull();
    // Expanding the collapsible reveals the preview rows.
    await user.click(screen.getByRole("button", { name: /show preview \(2\)/i }));
    await waitFor(() => {
      expect(screen.getByText("apple")).toBeInTheDocument();
    });
  });

  it("editing the textarea after a valid validate reverts the button to Validate", async () => {
    const user = userEvent.setup();
    renderWizard({ mocks: [validateMock(TWO_LINE_TEXT, VALID_RESULT)] });
    await typePayload(user, TWO_LINE_TEXT);
    await user.click(screen.getByRole("button", { name: /^validate$/i }));
    await waitFor(() => {
      expect(screen.getByRole("button", { name: /^import$/i })).toBeEnabled();
    });
    // Append text — validatedPayload now differs from payloadText.
    await user.click(screen.getByLabelText(/cards to import/i));
    await user.paste("\ncherry\tcherry");
    await waitFor(() => {
      expect(screen.queryByRole("button", { name: /^import$/i })).not.toBeInTheDocument();
    });
    expect(screen.getByRole("button", { name: /^validate$/i })).toBeEnabled();
  });

  it("invalid validate: shows invalid status and an open error list; button stays Validate", async () => {
    const user = userEvent.setup();
    renderWizard({ mocks: [validateMock(TWO_LINE_TEXT, INVALID_RESULT)] });
    await typePayload(user, TWO_LINE_TEXT);
    await user.click(screen.getByRole("button", { name: /^validate$/i }));
    expect(await screen.findByText(/missing tab separator/i)).toBeInTheDocument();
    expect(screen.getByRole("status")).toHaveTextContent(/✕ Invalid — 1 error/i);
    expect(screen.getByTestId("batch-import-step1-btn")).toHaveTextContent(/validate/i);
  });

  it("Import advances to step 2 with a confirm heading naming the target", async () => {
    const user = userEvent.setup();
    renderWizard({ mocks: [validateMock(TWO_LINE_TEXT, VALID_RESULT)] });
    await typePayload(user, TWO_LINE_TEXT);
    await user.click(screen.getByRole("button", { name: /^validate$/i }));
    const importButton = await screen.findByRole("button", { name: /^import$/i });
    await waitFor(() => expect(importButton).toBeEnabled());
    await user.click(importButton);
    expect(await screen.findByRole("heading")).toHaveTextContent(TARGET_NAME);
  });

  it("step 2 back button is disabled while importing is true", async () => {
    const user = userEvent.setup();
    renderWizard({ mocks: [validateMock(TWO_LINE_TEXT, VALID_RESULT)], importing: true });
    await typePayload(user, TWO_LINE_TEXT);
    await user.click(screen.getByRole("button", { name: /^validate$/i }));
    const importButton = await screen.findByRole("button", { name: /^import$/i });
    await waitFor(() => expect(importButton).toBeEnabled());
    await user.click(importButton);
    expect(screen.getByRole("button", { name: /paste & review/i })).toBeDisabled();
  });

  it("successful import calls onImport, refetches the connection, and calls onImported", async () => {
    const user = userEvent.setup();
    const refetchSpy = vi
      .spyOn(ApolloClient.prototype, "refetchQueries")
      // biome-ignore lint/suspicious/noExplicitAny: test stub for refetchQueries return
      .mockResolvedValue({} as any);
    const onImport = vi.fn(async () => OK_IMPORT);
    const { onImported } = renderWizard({
      mocks: [validateMock(TWO_LINE_TEXT, VALID_RESULT)],
      onImport,
    });
    await advanceToStep2(user);
    const confirm = await screen.findByTestId("batch-import-confirm-btn");
    await user.click(confirm);
    await waitFor(() => expect(onImport).toHaveBeenCalledWith(encodePayload(TWO_LINE_TEXT)));
    expect(refetchSpy).toHaveBeenCalledWith({ include: [CardsByCardgroupConnectionDocument] });
    await waitFor(() => expect(onImported).toHaveBeenCalledTimes(1));
  });

  it("import with error rows stays on step 2 and does not call onImported", async () => {
    const user = userEvent.setup();
    vi.spyOn(ApolloClient.prototype, "refetchQueries")
      // biome-ignore lint/suspicious/noExplicitAny: test stub for refetchQueries return
      .mockResolvedValue({} as any);
    const onImport = vi.fn(
      async (): Promise<ImportResult> => ({
        inserted: 1,
        updated: 0,
        errors: [{ line: 2, message: "boom", kind: "HARD" as const }],
      }),
    );
    const { onImported } = renderWizard({
      mocks: [validateMock(TWO_LINE_TEXT, VALID_RESULT)],
      onImport,
    });
    await advanceToStep2(user);
    await user.click(await screen.findByTestId("batch-import-confirm-btn"));
    expect(await screen.findByText(/boom/i)).toBeInTheDocument();
    expect(onImported).not.toHaveBeenCalled();
  });

  it("import with a duplicate (warning) row shows amber styling and a warning summary; Done closes", async () => {
    const user = userEvent.setup();
    vi.spyOn(ApolloClient.prototype, "refetchQueries")
      // biome-ignore lint/suspicious/noExplicitAny: test stub for refetchQueries return
      .mockResolvedValue([] as any);
    const onImport = vi.fn(
      async (): Promise<ImportResult> => ({
        inserted: 1,
        updated: 0,
        errors: [{ line: 2, message: "duplicate front", kind: "DUPLICATE" as const }],
      }),
    );
    const { onImported } = renderWizard({
      mocks: [validateMock(TWO_LINE_TEXT, VALID_RESULT)],
      onImport,
    });
    await advanceToStep2(user);
    await user.click(await screen.findByTestId("batch-import-confirm-btn"));
    await waitFor(() => expect(screen.getByText(/duplicate front/i)).toBeInTheDocument());
    // Duplicate rows are counted and labelled as warnings, not errors (amber, not red).
    const warningRow = screen.getByText(/duplicate front/i).closest("li");
    expect(warningRow).toHaveClass("bg-amber-50");
    expect(warningRow).not.toHaveClass("bg-destructive/10");
    // The result summary counts the duplicate as a warning, not an error.
    const statuses = screen.getAllByRole("status");
    expect(
      statuses.some((el) => /1 inserted, 0 updated, 1 warning/i.test(el.textContent ?? "")),
    ).toBe(true);
    // onImported NOT called on a partial result; Done closes the sheet.
    expect(onImported).not.toHaveBeenCalled();
    await user.click(screen.getByRole("button", { name: /^done$/i }));
    expect(onImported).toHaveBeenCalledTimes(1);
  });

  it("clicking the completed progress-bar segment also returns to step 1", async () => {
    const user = userEvent.setup();
    renderWizard({ mocks: [validateMock(TWO_LINE_TEXT, VALID_RESULT)] });
    await advanceToStep2(user);
    await user.click(screen.getByRole("button", { name: /paste & review/i }));
    // Back on step 1: textarea content preserved.
    expect(screen.getByTestId("batch-import-payload")).toHaveValue(TWO_LINE_TEXT);
  });

  it("validate transport/network error shows a banner and leaves the forward action disabled", async () => {
    const user = userEvent.setup();
    renderWizard({
      mocks: [
        {
          request: {
            query: ValidateCardImportDocument,
            variables: { input: { payload: encodePayload(TWO_LINE_TEXT) } },
          },
          error: new Error("network error"),
        },
      ],
    });
    await typePayload(user, TWO_LINE_TEXT);
    await user.click(screen.getByRole("button", { name: /^validate$/i }));
    await waitFor(() => {
      expect(screen.getByRole("alert")).toBeInTheDocument();
    });
    // No preview rows rendered after a network error.
    expect(screen.queryByText("apple")).not.toBeInTheDocument();
    // Forward action is not in the import-enabled state.
    expect(screen.queryByRole("button", { name: /^import$/i })).not.toBeInTheDocument();
  });

  it("all-failed import shows the destructive banner; Done still closes", async () => {
    const user = userEvent.setup();
    vi.spyOn(ApolloClient.prototype, "refetchQueries")
      // biome-ignore lint/suspicious/noExplicitAny: test stub for refetchQueries return
      .mockResolvedValue({} as any);
    const onImport = vi.fn(
      async (): Promise<ImportResult> => ({
        inserted: 0,
        updated: 0,
        errors: [{ line: 1, message: "nope", kind: "HARD" as const }],
      }),
    );
    const { onImported } = renderWizard({
      mocks: [validateMock(TWO_LINE_TEXT, VALID_RESULT)],
      onImport,
    });
    await advanceToStep2(user);
    await user.click(await screen.findByTestId("batch-import-confirm-btn"));
    await waitFor(() =>
      expect(screen.getByText(/import failed: no cards persisted/i)).toBeInTheDocument(),
    );
    expect(onImported).not.toHaveBeenCalled();
    await user.click(await screen.findByRole("button", { name: /done/i }));
    expect(onImported).toHaveBeenCalledTimes(1);
  });

  it("import rejection shows a banner and stays on step 2 for retry", async () => {
    const user = userEvent.setup();
    const onImport = vi.fn(async () => {
      throw new Error("network down");
    });
    const { onImported } = renderWizard({
      mocks: [validateMock(TWO_LINE_TEXT, VALID_RESULT)],
      onImport,
    });
    await advanceToStep2(user);
    await user.click(await screen.findByTestId("batch-import-confirm-btn"));
    // handleImport's outer catch sets bannerError -> role=alert banner.
    await waitFor(() => {
      expect(screen.getByRole("alert")).toBeInTheDocument();
    });
    expect(await screen.findByTestId("batch-import-confirm-btn")).toBeInTheDocument();
    expect(onImported).not.toHaveBeenCalled();
  });

  it("post-import '← Back to edit' returns to step 1 and re-requires a validate", async () => {
    const user = userEvent.setup();
    vi.spyOn(ApolloClient.prototype, "refetchQueries")
      // biome-ignore lint/suspicious/noExplicitAny: test stub for refetchQueries return
      .mockResolvedValue([] as any);
    // Partial failure keeps the sheet open on the result panel.
    const onImport = vi.fn(
      async (): Promise<ImportResult> => ({
        inserted: 1,
        updated: 0,
        errors: [{ line: 2, message: "duplicate front", kind: "DUPLICATE" as const }],
      }),
    );
    renderWizard({ mocks: [validateMock(TWO_LINE_TEXT, VALID_RESULT)], onImport });
    await advanceToStep2(user);
    await user.click(await screen.findByTestId("batch-import-confirm-btn"));
    // Result banner visible.
    await waitFor(() => {
      expect(screen.getByText(/duplicate front/i)).toBeInTheDocument();
    });
    // Back to edit from the result panel returns to step 1.
    await user.click(screen.getByRole("button", { name: /back to edit/i }));
    // setValidatedPayload(null) ran on success, so the forward button reverts to Validate.
    const validateButton = screen.getByRole("button", { name: /^validate$/i });
    expect(validateButton).toBeEnabled();
    expect(screen.queryByRole("button", { name: /^import$/i })).not.toBeInTheDocument();
  });

  it("the back link returns to step 1 preserving the textarea content", async () => {
    const user = userEvent.setup();
    renderWizard({ mocks: [validateMock(TWO_LINE_TEXT, VALID_RESULT)] });
    await advanceToStep2(user);
    await user.click(await screen.findByRole("button", { name: /back to edit/i }));
    expect(screen.getByTestId("batch-import-payload")).toHaveValue(TWO_LINE_TEXT);
  });

  it("step 1: clicking Back to Cardgroup invokes onCancel", async () => {
    const user = userEvent.setup();
    const { onCancel } = renderWizard();
    await user.click(screen.getByRole("button", { name: /back to cardgroup/i }));
    expect(onCancel).toHaveBeenCalledTimes(1);
  });
});

describe("BatchImportWizard footer layout", () => {
  // Pins the WizardFooter left→right slot order across all three footers:
  // Cancel→primary (step 1), Back→Import (step 2 confirm), Back→Done (step 2 result).
  function assertBefore(left: HTMLElement, right: HTMLElement) {
    expect(left.compareDocumentPosition(right) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
  }

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("step 1: Back to Cardgroup sits before the primary action", () => {
    renderWizard();
    assertBefore(
      screen.getByRole("button", { name: /back to cardgroup/i }),
      screen.getByTestId("batch-import-step1-btn"),
    );
  });

  it("step 2 (confirm): Back to edit sits before the Import action", async () => {
    const user = userEvent.setup();
    renderWizard({ mocks: [validateMock(TWO_LINE_TEXT, VALID_RESULT)] });
    await advanceToStep2(user);
    assertBefore(
      await screen.findByRole("button", { name: /back to edit/i }),
      screen.getByTestId("batch-import-confirm-btn"),
    );
  });

  it("step 2 (result): Back to edit sits before Done", async () => {
    const user = userEvent.setup();
    vi.spyOn(ApolloClient.prototype, "refetchQueries")
      // biome-ignore lint/suspicious/noExplicitAny: test stub for refetchQueries return
      .mockResolvedValue({} as any);
    // Leave a non-empty error row so the result view stays open (no onImported close).
    const onImport = vi.fn(
      async (): Promise<ImportResult> => ({
        inserted: 1,
        updated: 0,
        errors: [{ line: 2, message: "boom", kind: "HARD" as const }],
      }),
    );
    renderWizard({ mocks: [validateMock(TWO_LINE_TEXT, VALID_RESULT)], onImport });
    await advanceToStep2(user);
    await user.click(await screen.findByTestId("batch-import-confirm-btn"));
    assertBefore(
      await screen.findByRole("button", { name: /back to edit/i }),
      screen.getByRole("button", { name: /done/i }),
    );
  });
});

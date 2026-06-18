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

  it("valid validate: shows valid status, a collapsed preview, and an Import button", async () => {
    const user = userEvent.setup();
    renderWizard({ mocks: [validateMock(TWO_LINE_TEXT, VALID_RESULT)] });
    await typePayload(user, TWO_LINE_TEXT);
    await user.click(screen.getByRole("button", { name: /^validate$/i }));
    expect(await screen.findByTestId("batch-import-validate-status")).toBeInTheDocument();
    expect(await screen.findByRole("button", { name: /^import$/i })).toBeEnabled();
    expect(screen.queryByText("apple")).toBeNull();
  });

  it("invalid validate: shows invalid status and an open error list; button stays Validate", async () => {
    const user = userEvent.setup();
    renderWizard({ mocks: [validateMock(TWO_LINE_TEXT, INVALID_RESULT)] });
    await typePayload(user, TWO_LINE_TEXT);
    await user.click(screen.getByRole("button", { name: /^validate$/i }));
    expect(await screen.findByText(/missing tab separator/i)).toBeInTheDocument();
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

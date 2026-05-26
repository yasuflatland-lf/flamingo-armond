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
import { CardgroupBatchImportForm } from "./cardgroup-batch-import-form";

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

describe("<CardgroupBatchImportForm>", () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("shows the fixed target name and no cardgroup selector", () => {
    renderForm();
    expect(screen.getByText(CARDGROUP_NAME)).toBeInTheDocument();
    expect(screen.queryByRole("combobox")).not.toBeInTheDocument();
  });

  it("validate renders the preview rows", async () => {
    const user = userEvent.setup();
    renderForm([validateMock(TWO_LINE_TEXT, VALID_RESULT)]);

    await typePayload(user, TWO_LINE_TEXT);
    await user.click(screen.getByRole("button", { name: /^validate$/i }));

    await waitFor(() => {
      expect(screen.getByText("apple")).toBeInTheDocument();
    });
    expect(screen.getByText("banana")).toBeInTheDocument();
    expect(screen.getByText(/2 cards parsed/i)).toBeInTheDocument();
  });

  it("Import is disabled until a successful validate", async () => {
    const user = userEvent.setup();
    renderForm([validateMock(TWO_LINE_TEXT, VALID_RESULT)]);

    const importButton = screen.getByRole("button", { name: /^import$/i });
    expect(importButton).toBeDisabled();

    await typePayload(user, TWO_LINE_TEXT);
    // Still disabled before validation runs.
    expect(importButton).toBeDisabled();

    await user.click(screen.getByRole("button", { name: /^validate$/i }));

    await waitFor(() => {
      expect(importButton).toBeEnabled();
    });
  });

  it("editing the textarea after validation re-disables Import (stale guard)", async () => {
    const user = userEvent.setup();
    renderForm([validateMock(TWO_LINE_TEXT, VALID_RESULT)]);

    await typePayload(user, TWO_LINE_TEXT);
    await user.click(screen.getByRole("button", { name: /^validate$/i }));

    const importButton = screen.getByRole("button", { name: /^import$/i });
    await waitFor(() => {
      expect(importButton).toBeEnabled();
    });

    // Append text — validatedPayload now differs from payloadText.
    await user.click(screen.getByLabelText(/cards to import/i));
    await user.paste("\ncherry\tcherry");

    await waitFor(() => {
      expect(importButton).toBeDisabled();
    });
  });

  it("parse-error rows render with role=alert", async () => {
    const user = userEvent.setup();
    const text = "broken line";
    renderForm([
      validateMock(text, {
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
      }),
    ]);

    await typePayload(user, text);
    await user.click(screen.getByRole("button", { name: /^validate$/i }));

    await waitFor(() => {
      expect(screen.getByText(/missing tab separator/i)).toBeInTheDocument();
    });
    const alerts = screen.getAllByRole("alert");
    expect(alerts.some((el) => /missing tab separator/i.test(el.textContent ?? ""))).toBe(true);
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

    await typePayload(user, TWO_LINE_TEXT);
    await user.click(screen.getByRole("button", { name: /^validate$/i }));
    await waitFor(() => {
      expect(screen.getByRole("button", { name: /^import$/i })).toBeEnabled();
    });

    await user.click(screen.getByRole("button", { name: /^import$/i }));

    await waitFor(() => {
      expect(onImported).toHaveBeenCalledTimes(1);
    });
    expect(refetchSpy).toHaveBeenCalledTimes(1);
    expect(refetchSpy).toHaveBeenCalledWith(
      expect.objectContaining({ include: [CardsByCardgroupConnectionDocument] }),
    );
  });

  it("validate transport/network error shows a banner and leaves Import disabled", async () => {
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
    // Import must remain disabled because validation failed.
    expect(screen.getByRole("button", { name: /^import$/i })).toBeDisabled();
  });

  it("import with error rows keeps the form open and shows the result banner", async () => {
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

    await typePayload(user, TWO_LINE_TEXT);
    await user.click(screen.getByRole("button", { name: /^validate$/i }));
    await waitFor(() => {
      expect(screen.getByRole("button", { name: /^import$/i })).toBeEnabled();
    });

    await user.click(screen.getByRole("button", { name: /^import$/i }));

    await waitFor(() => {
      expect(screen.getByText(/duplicate front/i)).toBeInTheDocument();
    });
    expect(screen.getByText(/1 inserted, 0 updated/i)).toBeInTheDocument();
    expect(onImported).not.toHaveBeenCalled();
    // Result banner has role=status.
    const status = screen.getAllByRole("status");
    expect(status.some((el) => /1 inserted/i.test(el.textContent ?? ""))).toBe(true);
  });
});

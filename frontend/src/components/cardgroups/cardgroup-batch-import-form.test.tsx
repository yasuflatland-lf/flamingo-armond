// @vitest-environment jsdom
import { ApolloClient } from "@apollo/client";
import type { MockedResponse } from "@apollo/client/testing";
import { MockedProvider } from "@apollo/client/testing/react";
import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import {
  CardsByCardgroupConnectionDocument,
  ImportCardsDocument,
  ValidateCardImportDocument,
} from "@/generated/graphql";
import { renderWithIntl } from "@/test/render-with-intl";
import { CardgroupBatchImportForm } from "./cardgroup-batch-import-form";

const CARDGROUP_ID = "cg-1";
const CARDGROUP_NAME = "Spanish Vocab";
const TWO_LINE_TEXT = "apple\tred fruit\nbanana\tyellow fruit";

function encodePayload(text: string): string {
  return btoa(unescape(encodeURIComponent(text)));
}

const validateMock: MockedResponse = {
  request: {
    query: ValidateCardImportDocument,
    variables: { input: { payload: encodePayload(TWO_LINE_TEXT) } },
  },
  result: {
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
  },
};

const importMock: MockedResponse = {
  request: {
    query: ImportCardsDocument,
    variables: { input: { cardgroupId: CARDGROUP_ID, payload: encodePayload(TWO_LINE_TEXT) } },
  },
  result: {
    data: {
      importCards: {
        __typename: "ImportCardsPayload" as const,
        inserted: 2,
        updated: 0,
        errors: [],
      },
    },
  },
};

describe("<CardgroupBatchImportForm> wiring", () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("imports through the cardgroup mutation and refetches the cards connection", async () => {
    const user = userEvent.setup();
    const refetchSpy = vi
      .spyOn(ApolloClient.prototype, "refetchQueries")
      // biome-ignore lint/suspicious/noExplicitAny: test stub for refetchQueries return
      .mockResolvedValue({} as any);
    const onImported = vi.fn();
    renderWithIntl(
      <MockedProvider mocks={[validateMock, importMock]}>
        <CardgroupBatchImportForm
          cardgroupId={CARDGROUP_ID}
          cardgroupName={CARDGROUP_NAME}
          onImported={onImported}
        />
      </MockedProvider>,
    );

    const textarea = screen.getByLabelText(/cards to import/i);
    await user.click(textarea);
    await user.paste(TWO_LINE_TEXT);
    await user.click(screen.getByRole("button", { name: /^validate$/i }));
    const importButton = await screen.findByRole("button", { name: /^import$/i });
    await waitFor(() => expect(importButton).toBeEnabled());
    await user.click(importButton);
    await user.click(await screen.findByTestId("batch-import-confirm-btn"));

    await waitFor(() => expect(onImported).toHaveBeenCalledTimes(1));
    expect(refetchSpy).toHaveBeenCalledWith({ include: [CardsByCardgroupConnectionDocument] });
  });
});

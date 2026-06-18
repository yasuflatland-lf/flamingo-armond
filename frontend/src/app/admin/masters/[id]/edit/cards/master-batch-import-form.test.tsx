// @vitest-environment jsdom
import { readFileSync } from "node:fs";
import { join } from "node:path";
import { ApolloClient } from "@apollo/client";
import type { MockedResponse } from "@apollo/client/testing";
import { MockedProvider } from "@apollo/client/testing/react";
import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import {
  AdminImportMasterCardsDocument,
  AdminMasterCardsConnectionDocument,
  ValidateCardImportDocument,
} from "@/generated/graphql";
import { encodePayload } from "@/test/batch-import-test-utils";
import { renderWithIntl } from "@/test/render-with-intl";
import { MasterBatchImportForm } from "./master-batch-import-form";

const MASTER_ID = "m-1";
const DECK_NAME = "Core 2000";
const TWO_LINE_TEXT = "apple\tred fruit\nbanana\tyellow fruit";

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
    query: AdminImportMasterCardsDocument,
    variables: { input: { masterCardgroupId: MASTER_ID, payload: encodePayload(TWO_LINE_TEXT) } },
  },
  result: {
    data: {
      adminImportMasterCards: {
        __typename: "ImportCardsPayload" as const,
        inserted: 2,
        updated: 0,
        errors: [],
      },
    },
  },
};

describe("<MasterBatchImportForm> wiring", () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("imports through the admin master mutation and refetches the master-cards connection", async () => {
    const user = userEvent.setup();
    const refetchSpy = vi
      .spyOn(ApolloClient.prototype, "refetchQueries")
      // biome-ignore lint/suspicious/noExplicitAny: test stub for refetchQueries return
      .mockResolvedValue([] as any);
    const onImported = vi.fn();
    renderWithIntl(
      <MockedProvider mocks={[validateMock, importMock]}>
        <MasterBatchImportForm masterId={MASTER_ID} deckName={DECK_NAME} onImported={onImported} />
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
    expect(refetchSpy).toHaveBeenCalledWith({ include: [AdminMasterCardsConnectionDocument] });
  });

  it("wires the admin master mutation, not the cardgroup import mutation (static-source guard)", () => {
    const src = readFileSync(
      join(process.cwd(), "src/app/admin/masters/[id]/edit/cards/master-batch-import-form.tsx"),
      "utf8",
    );
    expect(src).toContain("adminImportMasterCards");
    expect(src).not.toContain("importCards(");
  });
});

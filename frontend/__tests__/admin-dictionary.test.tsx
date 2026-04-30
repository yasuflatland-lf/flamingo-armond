// @vitest-environment jsdom
import { MockedProvider } from "@apollo/client/testing/react";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { GraphQLError } from "graphql";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { DictionaryImportClient } from "@/app/admin/dictionary/dictionary-client";
import {
  MyCardgroupsDocument,
  UpsertDictionaryDocument,
  ValidateDictionaryDocument,
} from "@/generated/graphql";

// Compute the expected base64 payload for "hello\tworld\nfoo\tbar"
// encodePayload mirrors the inline helper in dictionary-client.tsx:
//   btoa(unescape(encodeURIComponent(text)))
function encodePayload(text: string): string {
  return btoa(unescape(encodeURIComponent(text)));
}

const PAYLOAD_TEXT = "hello\tworld\nfoo\tbar";
const ENCODED_PAYLOAD = encodePayload(PAYLOAD_TEXT);

const CARDGROUPS_MOCK = {
  request: {
    query: MyCardgroupsDocument,
    variables: {},
  },
  result: {
    data: {
      myCardgroups: [
        {
          __typename: "Cardgroup",
          id: "cg-1",
          name: "My Cards",
          updatedAt: "2024-01-01T00:00:00Z",
        },
      ],
    },
  },
};

const VALIDATE_SUCCESS_MOCK = {
  request: {
    query: ValidateDictionaryDocument,
    variables: { input: { payload: ENCODED_PAYLOAD } },
  },
  result: {
    data: {
      validateDictionary: {
        __typename: "DictionaryValidationResult",
        valid: true,
        parsedWords: [
          { __typename: "ParsedWord", front: "hello", back: "world", line: 1 },
          { __typename: "ParsedWord", front: "foo", back: "bar", line: 2 },
        ],
        errors: [],
      },
    },
  },
};

const VALIDATE_WITH_ERRORS_MOCK = {
  request: {
    query: ValidateDictionaryDocument,
    variables: { input: { payload: ENCODED_PAYLOAD } },
  },
  result: {
    data: {
      validateDictionary: {
        __typename: "DictionaryValidationResult",
        valid: false,
        parsedWords: [{ __typename: "ParsedWord", front: "good", back: "row", line: 1 }],
        errors: [{ __typename: "DictionaryValidationError", line: 2, message: "missing back" }],
      },
    },
  },
};

const UPSERT_SUCCESS_MOCK = {
  request: {
    query: UpsertDictionaryDocument,
    variables: { input: { cardgroupId: "cg-1", payload: ENCODED_PAYLOAD } },
  },
  result: {
    data: {
      upsertDictionary: {
        __typename: "UpsertDictionaryPayload",
        inserted: 2,
        updated: 0,
        errors: [],
      },
    },
  },
};

const UPSERT_FORBIDDEN_MOCK = {
  request: {
    query: UpsertDictionaryDocument,
    variables: { input: { cardgroupId: "cg-1", payload: ENCODED_PAYLOAD } },
  },
  result: {
    errors: [
      // Use uppercase "FORBIDDEN" in the message so classifyError's
      // err.message.includes("FORBIDDEN") branch triggers correctly.
      // The new CombinedGraphQLErrors type exposes .errors not .graphQLErrors,
      // so the graphQLErrors branch in classifyError does not fire.
      new GraphQLError("FORBIDDEN", { extensions: { code: "FORBIDDEN" } }),
    ],
  },
};

let consoleErrorSpy: ReturnType<typeof vi.spyOn>;
let consoleWarnSpy: ReturnType<typeof vi.spyOn>;

beforeEach(() => {
  consoleErrorSpy = vi.spyOn(console, "error").mockImplementation(() => {});
  consoleWarnSpy = vi.spyOn(console, "warn").mockImplementation(() => {});
});

afterEach(() => {
  consoleErrorSpy.mockRestore();
  consoleWarnSpy.mockRestore();
});

function renderComponent(mocks: object[]) {
  render(
    <MockedProvider mocks={mocks as never}>
      <DictionaryImportClient />
    </MockedProvider>,
  );
}

describe("DictionaryImportClient — validate → import flow", () => {
  // T1: Happy path: validate then import
  it("validates input, shows preview, enables Import, then imports and shows result", async () => {
    const user = userEvent.setup({ delay: null });
    renderComponent([CARDGROUPS_MOCK, VALIDATE_SUCCESS_MOCK, UPSERT_SUCCESS_MOCK]);

    // Wait for cardgroup selector to populate.
    const select = await screen.findByRole("combobox", { name: /target cardgroup/i });
    await user.selectOptions(select, "cg-1");

    // Type payload into textarea.
    const textarea = screen.getByRole("textbox", { name: /dictionary payload/i });
    await user.type(textarea, PAYLOAD_TEXT);

    // Import button must be disabled before validation.
    const importBtn = screen.getByRole("button", { name: /^import$/i });
    expect(importBtn).toBeDisabled();

    // Click Validate.
    await user.click(screen.getByRole("button", { name: /^validate$/i }));

    // Preview table rows appear.
    expect(await screen.findByText("hello")).toBeInTheDocument();
    expect(screen.getByText("world")).toBeInTheDocument();
    expect(screen.getByText("foo")).toBeInTheDocument();
    expect(screen.getByText("bar")).toBeInTheDocument();

    // Validation summary shows valid status.
    expect(screen.getByRole("status")).toHaveTextContent("Valid");

    // Import button is now enabled.
    expect(importBtn).not.toBeDisabled();

    // Click Import.
    await user.click(importBtn);

    // Success message appears.
    const successStatus = await screen.findByRole("status", {
      name: (_, el) => el.textContent?.includes("Import complete") ?? false,
    });
    expect(successStatus).toHaveTextContent("2 inserted");
    expect(successStatus).toHaveTextContent("0 updated");
  });

  // T2: Validation surfaces errors; Import stays disabled when valid is false
  it("shows parsed words and parse errors; Import remains disabled when valid is false", async () => {
    const user = userEvent.setup({ delay: null });
    renderComponent([CARDGROUPS_MOCK, VALIDATE_WITH_ERRORS_MOCK]);

    // Wait for cardgroup selector and select a cardgroup.
    const select = await screen.findByRole("combobox", { name: /target cardgroup/i });
    await user.selectOptions(select, "cg-1");

    // Type payload into textarea.
    const textarea = screen.getByRole("textbox", { name: /dictionary payload/i });
    await user.type(textarea, PAYLOAD_TEXT);

    // Click Validate.
    await user.click(screen.getByRole("button", { name: /^validate$/i }));

    // The successfully-parsed word is shown in the preview table.
    expect(await screen.findByText("good")).toBeInTheDocument();
    expect(screen.getByText("row")).toBeInTheDocument();

    // The parse error is rendered and visually distinguishable (shown in errors list).
    expect(screen.getByText(/missing back/i)).toBeInTheDocument();
    // The error list item includes the line number prefix.
    expect(screen.getByText(/line 2/i)).toBeInTheDocument();

    // Validation summary reflects invalid status.
    expect(screen.getByRole("status")).toHaveTextContent("Invalid");

    // Import button must remain disabled: valid is false so canImport is false.
    const importBtn = screen.getByRole("button", { name: /^import$/i });
    expect(importBtn).toBeDisabled();
  });

  // T3: Import returns FORBIDDEN → "Admin role required." banner
  it("shows 'Admin role required.' when upsertDictionary returns FORBIDDEN", async () => {
    const user = userEvent.setup({ delay: null });
    renderComponent([CARDGROUPS_MOCK, VALIDATE_SUCCESS_MOCK, UPSERT_FORBIDDEN_MOCK]);

    // Select cardgroup.
    const select = await screen.findByRole("combobox", { name: /target cardgroup/i });
    await user.selectOptions(select, "cg-1");

    // Type payload.
    const textarea = screen.getByRole("textbox", { name: /dictionary payload/i });
    await user.type(textarea, PAYLOAD_TEXT);

    // Validate first.
    await user.click(screen.getByRole("button", { name: /^validate$/i }));
    await screen.findByText("hello");

    // Click Import.
    const importBtn = screen.getByRole("button", { name: /^import$/i });
    expect(importBtn).not.toBeDisabled();
    await user.click(importBtn);

    // FORBIDDEN banner appears.
    const alert = await screen.findByRole("alert");
    expect(alert).toHaveTextContent("Admin role required.");
  });
});

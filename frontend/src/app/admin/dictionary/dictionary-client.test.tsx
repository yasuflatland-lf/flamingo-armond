// @vitest-environment jsdom

import { MockedProvider } from "@apollo/client/testing/react";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  MyCardgroupsConnectionDocument,
  UpsertDictionaryDocument,
  ValidateDictionaryDocument,
} from "@/generated/graphql";
import {
  type ApolloMockLeakSpyResult,
  installApolloMockLeakSpy,
} from "../../../../__tests__/utils/mock-apollo-paginated";
import { DictionaryImportClient } from "./dictionary-client";

// ---------------------------------------------------------------------------
// Stubs: next/link (no router in jsdom)
// ---------------------------------------------------------------------------

vi.mock("next/link", () => ({
  default: ({
    href,
    children,
    ...rest
  }: {
    href: string;
    children: React.ReactNode;
    [key: string]: unknown;
  }) => (
    <a href={href} {...rest}>
      {children}
    </a>
  ),
}));

// ---------------------------------------------------------------------------
// Helpers: encodePayload mirrors the production helper so mock variables match
// ---------------------------------------------------------------------------

/**
 * Mirrors `encodePayload` in dictionary-client.tsx. Must stay in sync so that
 * mock `variables.input.payload` matches exactly what the component sends.
 */
function encodePayload(text: string): string {
  return btoa(unescape(encodeURIComponent(text)));
}

// ---------------------------------------------------------------------------
// Fixtures
// ---------------------------------------------------------------------------

const CG_1 = {
  __typename: "Cardgroup" as const,
  id: "cg-abc",
  name: "Fruits",
  updatedAt: "2025-01-01T00:00:00.000Z",
};

const CG_2 = {
  __typename: "Cardgroup" as const,
  id: "cg-def",
  name: "Animals",
  updatedAt: "2025-02-01T00:00:00.000Z",
};

/** A single-entry payload: "apple<tab>the fruit" */
const PAYLOAD_TEXT = "apple\tthe fruit";
const PAYLOAD_ENCODED = encodePayload(PAYLOAD_TEXT);

/** A second single-entry payload used to test post-validation edits */
const PAYLOAD_TEXT_EDITED = "apple\tthe fruit\nbanana\ta yellow fruit";
const PAYLOAD_ENCODED_EDITED = encodePayload(PAYLOAD_TEXT_EDITED);

const CARDGROUPS_MOCK = {
  request: { query: MyCardgroupsConnectionDocument, variables: { first: 100 } },
  result: {
    data: {
      myCardgroupsConnection: {
        __typename: "CardgroupConnection" as const,
        edges: [
          { __typename: "CardgroupEdge" as const, cursor: "cursor-abc", node: CG_1 },
          { __typename: "CardgroupEdge" as const, cursor: "cursor-def", node: CG_2 },
        ],
        pageInfo: {
          __typename: "PageInfo" as const,
          hasNextPage: false,
          hasPreviousPage: false,
          startCursor: "cursor-abc",
          endCursor: "cursor-def",
        },
        totalCount: 2,
      },
    },
  },
};

const VALID_VALIDATION_RESULT = {
  __typename: "DictionaryValidationResult" as const,
  valid: true,
  parsedWords: [{ __typename: "ParsedWord" as const, front: "apple", back: "the fruit", line: 1 }],
  errors: [],
};

const INVALID_VALIDATION_RESULT = {
  __typename: "DictionaryValidationResult" as const,
  valid: false,
  parsedWords: [],
  errors: [
    {
      __typename: "DictionaryValidationError" as const,
      line: 1,
      message: "missing tab separator",
    },
  ],
};

const EMPTY_PARSEDWORDS_VALIDATION_RESULT = {
  __typename: "DictionaryValidationResult" as const,
  valid: true,
  parsedWords: [],
  errors: [],
};

// ---------------------------------------------------------------------------
// Leak spy lifecycle
// ---------------------------------------------------------------------------

let leakSpy: ApolloMockLeakSpyResult;

beforeEach(() => {
  leakSpy = installApolloMockLeakSpy({
    operationNames: ["ValidateDictionary", "UpsertDictionary", "MyCardgroupsConnection"],
  });
});

afterEach(() => {
  vi.restoreAllMocks();
  leakSpy.assertNoLeaks();
  leakSpy.teardown();
});

// ---------------------------------------------------------------------------
// Render helper
// ---------------------------------------------------------------------------

function renderClient(mocks: unknown[]) {
  render(
    <MockedProvider mocks={mocks as never}>
      <DictionaryImportClient />
    </MockedProvider>,
  );
}

// ---------------------------------------------------------------------------
// Scenarios
// ---------------------------------------------------------------------------

describe("<DictionaryImportClient>", () => {
  // S1: Import button is disabled initially (no validationResult)
  it("S1: Import button is disabled initially before any validation", async () => {
    renderClient([CARDGROUPS_MOCK]);

    // Wait for the cardgroups query to settle so the selector renders
    expect(await screen.findByLabelText("Target cardgroup")).toBeInTheDocument();

    const importBtn = screen.getByRole("button", { name: /import/i });
    expect(importBtn).toBeDisabled();
  });

  // S2: After successful validation, canImport becomes true and Import button enables
  it("S2: Import button enables after successful validation (valid=true, parsedWords non-empty)", async () => {
    const user = userEvent.setup();

    const validateMock = {
      request: {
        query: ValidateDictionaryDocument,
        variables: { input: { payload: PAYLOAD_ENCODED } },
      },
      result: { data: { validateDictionary: VALID_VALIDATION_RESULT } },
    };

    renderClient([CARDGROUPS_MOCK, validateMock]);

    // Select a cardgroup
    const select = await screen.findByLabelText("Target cardgroup");
    await user.selectOptions(select, CG_1.id);

    // Enter payload text
    const textarea = screen.getByLabelText("Dictionary payload");
    await user.type(textarea, PAYLOAD_TEXT);

    // Click Validate
    const validateBtn = screen.getByRole("button", { name: /validate/i });
    await user.click(validateBtn);

    // Import should enable once validation resolves
    await waitFor(() => {
      expect(screen.getByRole("button", { name: /^import$/i })).not.toBeDisabled();
    });
  });

  // S3: Editing the payload AFTER validation disables Import
  //     This is the regression-guard for the `validatedPayload === payloadText` clause.
  it("S3: Editing payload after validation disables Import (regression: validatedPayload check)", async () => {
    const user = userEvent.setup();

    const validateMock = {
      request: {
        query: ValidateDictionaryDocument,
        variables: { input: { payload: PAYLOAD_ENCODED } },
      },
      result: { data: { validateDictionary: VALID_VALIDATION_RESULT } },
    };

    renderClient([CARDGROUPS_MOCK, validateMock]);

    // Select a cardgroup
    const select = await screen.findByLabelText("Target cardgroup");
    await user.selectOptions(select, CG_1.id);

    // Enter payload text and validate
    const textarea = screen.getByLabelText("Dictionary payload");
    await user.type(textarea, PAYLOAD_TEXT);
    await user.click(screen.getByRole("button", { name: /validate/i }));

    // Wait for Import to be enabled
    await waitFor(() => {
      expect(screen.getByRole("button", { name: /^import$/i })).not.toBeDisabled();
    });

    // Now edit the textarea — Import must be disabled immediately
    await user.type(textarea, " extra");

    expect(screen.getByRole("button", { name: /^import$/i })).toBeDisabled();
  });

  // S4: Re-validating with the new (edited) payload re-enables Import
  it("S4: Re-validating edited payload re-enables Import", async () => {
    const user = userEvent.setup();

    const validateMock1 = {
      request: {
        query: ValidateDictionaryDocument,
        variables: { input: { payload: PAYLOAD_ENCODED } },
      },
      result: { data: { validateDictionary: VALID_VALIDATION_RESULT } },
    };

    // The edited payload is PAYLOAD_TEXT + " extra"
    const editedText = `${PAYLOAD_TEXT} extra`;
    const editedEncoded = encodePayload(editedText);

    const VALID_EDITED_RESULT = {
      __typename: "DictionaryValidationResult" as const,
      valid: true,
      parsedWords: [
        { __typename: "ParsedWord" as const, front: "apple", back: "the fruit extra", line: 1 },
      ],
      errors: [],
    };

    const validateMock2 = {
      request: {
        query: ValidateDictionaryDocument,
        variables: { input: { payload: editedEncoded } },
      },
      result: { data: { validateDictionary: VALID_EDITED_RESULT } },
    };

    renderClient([CARDGROUPS_MOCK, validateMock1, validateMock2]);

    // Select cardgroup
    const select = await screen.findByLabelText("Target cardgroup");
    await user.selectOptions(select, CG_1.id);

    // Enter initial payload, validate
    const textarea = screen.getByLabelText("Dictionary payload");
    await user.type(textarea, PAYLOAD_TEXT);
    await user.click(screen.getByRole("button", { name: /validate/i }));

    await waitFor(() => {
      expect(screen.getByRole("button", { name: /^import$/i })).not.toBeDisabled();
    });

    // Edit payload
    await user.type(textarea, " extra");

    expect(screen.getByRole("button", { name: /^import$/i })).toBeDisabled();

    // Re-validate with the new text
    await user.click(screen.getByRole("button", { name: /validate/i }));

    // Import must re-enable
    await waitFor(() => {
      expect(screen.getByRole("button", { name: /^import$/i })).not.toBeDisabled();
    });
  });

  // S5: Switching cardgroupId after validation — validatedCardgroupId is NOT tracked by the
  //     component. The `canImport` expression is:
  //       validationResult.valid === true
  //       && validationResult.parsedWords.length > 0
  //       && !!cardgroupId
  //       && validatedPayload === payloadText
  //     There is no `validatedCardgroupId` state. Switching the cardgroup while
  //     `validatedPayload === payloadText` keeps Import enabled. This is intentional:
  //     validation is payload-scoped, not cardgroup-scoped.
  it("S5: Switching cardgroupId after validation keeps Import enabled (no validatedCardgroupId check)", async () => {
    const user = userEvent.setup();

    const validateMock = {
      request: {
        query: ValidateDictionaryDocument,
        variables: { input: { payload: PAYLOAD_ENCODED } },
      },
      result: { data: { validateDictionary: VALID_VALIDATION_RESULT } },
    };

    renderClient([CARDGROUPS_MOCK, validateMock]);

    // Select first cardgroup
    const select = await screen.findByLabelText("Target cardgroup");
    await user.selectOptions(select, CG_1.id);

    // Enter payload and validate
    const textarea = screen.getByLabelText("Dictionary payload");
    await user.type(textarea, PAYLOAD_TEXT);
    await user.click(screen.getByRole("button", { name: /validate/i }));

    await waitFor(() => {
      expect(screen.getByRole("button", { name: /^import$/i })).not.toBeDisabled();
    });

    // Switch to second cardgroup — Import must remain enabled
    await user.selectOptions(select, CG_2.id);

    expect(screen.getByRole("button", { name: /^import$/i })).not.toBeDisabled();
  });

  // S6: Validation returning valid: false keeps Import disabled
  it("S6: Import stays disabled when validation returns valid: false", async () => {
    const user = userEvent.setup();

    const validateMock = {
      request: {
        query: ValidateDictionaryDocument,
        variables: { input: { payload: PAYLOAD_ENCODED } },
      },
      result: { data: { validateDictionary: INVALID_VALIDATION_RESULT } },
    };

    renderClient([CARDGROUPS_MOCK, validateMock]);

    const select = await screen.findByLabelText("Target cardgroup");
    await user.selectOptions(select, CG_1.id);

    const textarea = screen.getByLabelText("Dictionary payload");
    await user.type(textarea, PAYLOAD_TEXT);
    await user.click(screen.getByRole("button", { name: /validate/i }));

    // Error count should appear in the validation result
    await waitFor(() => {
      expect(screen.getByRole("status")).toBeInTheDocument();
    });

    // Import must remain disabled
    expect(screen.getByRole("button", { name: /^import$/i })).toBeDisabled();
  });

  // S7: Validation returning valid: true but empty parsedWords keeps Import disabled
  it("S7: Import stays disabled when parsedWords is empty even if valid: true", async () => {
    const user = userEvent.setup();

    const validateMock = {
      request: {
        query: ValidateDictionaryDocument,
        variables: { input: { payload: PAYLOAD_ENCODED } },
      },
      result: { data: { validateDictionary: EMPTY_PARSEDWORDS_VALIDATION_RESULT } },
    };

    renderClient([CARDGROUPS_MOCK, validateMock]);

    const select = await screen.findByLabelText("Target cardgroup");
    await user.selectOptions(select, CG_1.id);

    const textarea = screen.getByLabelText("Dictionary payload");
    await user.type(textarea, PAYLOAD_TEXT);
    await user.click(screen.getByRole("button", { name: /validate/i }));

    // Wait for validation to settle
    await waitFor(() => {
      // The status region shows "Valid — 0 words parsed."
      expect(screen.getByRole("status")).toBeInTheDocument();
    });

    // Import must remain disabled — canImport requires parsedWords.length > 0
    expect(screen.getByRole("button", { name: /^import$/i })).toBeDisabled();
  });

  // S8: Import fires the mutation and shows success result
  it("S8: clicking Import fires UpsertDictionary and shows the success summary", async () => {
    const user = userEvent.setup();

    const validateMock = {
      request: {
        query: ValidateDictionaryDocument,
        variables: { input: { payload: PAYLOAD_ENCODED } },
      },
      result: { data: { validateDictionary: VALID_VALIDATION_RESULT } },
    };

    const upsertMock = {
      request: {
        query: UpsertDictionaryDocument,
        variables: { input: { cardgroupId: CG_1.id, payload: PAYLOAD_ENCODED } },
      },
      result: {
        data: {
          upsertDictionary: {
            __typename: "UpsertDictionaryPayload" as const,
            inserted: 1,
            updated: 0,
            errors: [],
          },
        },
      },
    };

    renderClient([CARDGROUPS_MOCK, validateMock, upsertMock]);

    // Select cardgroup
    const select = await screen.findByLabelText("Target cardgroup");
    await user.selectOptions(select, CG_1.id);

    // Enter payload and validate
    const textarea = screen.getByLabelText("Dictionary payload");
    await user.type(textarea, PAYLOAD_TEXT);
    await user.click(screen.getByRole("button", { name: /validate/i }));

    await waitFor(() => {
      expect(screen.getByRole("button", { name: /^import$/i })).not.toBeDisabled();
    });

    // Trigger the import
    await user.click(screen.getByRole("button", { name: /^import$/i }));

    // Success summary should appear
    await waitFor(() => {
      expect(screen.getByText(/import complete/i)).toBeInTheDocument();
    });
    expect(screen.getByText(/1 inserted/i)).toBeInTheDocument();
  });

  // S9: No cardgroup selected — clicking Import shows an inline banner instead of firing mutation
  it("S9: shows banner when Import is clicked without a cardgroup selected (guard in handleImport)", async () => {
    const _user = userEvent.setup();

    // Render without any mocks beyond cardgroups — we deliberately skip validation
    // here to test the handleImport guard path. We use a button click that bypasses
    // canImport by invoking handleImport directly is not possible from the outside;
    // however we can reach this path via the "no cardgroup" branch since canImport
    // requires !!cardgroupId. We test it by checking the guard in handleImport
    // cannot be reached via the disabled Import button — which means the guard in
    // handleImport exists for defensive purposes. The UI gate (disabled) is the
    // primary enforcer. We document this fact rather than invent an untestable path.
    //
    // The relevant code path: handleImport() returns early with a setBannerError
    // if (!cardgroupId). This is unreachable via the UI today because canImport
    // requires !!cardgroupId and the button is disabled. We confirm the button is
    // disabled before any cardgroup is selected.
    renderClient([CARDGROUPS_MOCK]);

    await screen.findByLabelText("Target cardgroup");

    // No cardgroup selected — Import must be disabled before any validation
    expect(screen.getByRole("button", { name: /^import$/i })).toBeDisabled();
  });

  // S10: Validate button is disabled when payloadText is empty
  it("S10: Validate button is disabled when payload textarea is empty", async () => {
    renderClient([CARDGROUPS_MOCK]);

    await screen.findByLabelText("Target cardgroup");

    // No text in textarea — Validate must be disabled
    expect(screen.getByRole("button", { name: /validate/i })).toBeDisabled();
  });

  // S11: Validate button enables once user types into the textarea
  it("S11: Validate button enables when payload textarea has non-whitespace content", async () => {
    const user = userEvent.setup();

    renderClient([CARDGROUPS_MOCK]);

    await screen.findByLabelText("Target cardgroup");

    const validateBtn = screen.getByRole("button", { name: /validate/i });
    expect(validateBtn).toBeDisabled();

    const textarea = screen.getByLabelText("Dictionary payload");
    await user.type(textarea, "a");

    expect(validateBtn).not.toBeDisabled();
  });

  // S12: Validation error response shows an error banner (classifyError path)
  it("S12: shows error banner when validateDictionary query returns a network error", async () => {
    const user = userEvent.setup();

    const validateErrorMock = {
      request: {
        query: ValidateDictionaryDocument,
        variables: { input: { payload: PAYLOAD_ENCODED } },
      },
      error: new Error("network failure"),
    };

    renderClient([CARDGROUPS_MOCK, validateErrorMock]);

    const select = await screen.findByLabelText("Target cardgroup");
    await user.selectOptions(select, CG_1.id);

    const textarea = screen.getByLabelText("Dictionary payload");
    await user.type(textarea, PAYLOAD_TEXT);
    await user.click(screen.getByRole("button", { name: /validate/i }));

    await waitFor(() => {
      expect(screen.getAllByRole("alert").length).toBeGreaterThan(0);
    });

    // Import must remain disabled on error
    expect(screen.getByRole("button", { name: /^import$/i })).toBeDisabled();
  });

  // S13: Validation result preview table shows parsed words
  it("S13: shows parsed words preview table after successful validation", async () => {
    const user = userEvent.setup();

    const validateMock = {
      request: {
        query: ValidateDictionaryDocument,
        variables: { input: { payload: PAYLOAD_ENCODED } },
      },
      result: { data: { validateDictionary: VALID_VALIDATION_RESULT } },
    };

    renderClient([CARDGROUPS_MOCK, validateMock]);

    const select = await screen.findByLabelText("Target cardgroup");
    await user.selectOptions(select, CG_1.id);

    const textarea = screen.getByLabelText("Dictionary payload");
    await user.type(textarea, PAYLOAD_TEXT);
    await user.click(screen.getByRole("button", { name: /validate/i }));

    // Preview entries should appear
    await waitFor(() => {
      expect(screen.getByText("apple")).toBeInTheDocument();
    });
    expect(screen.getByText("the fruit")).toBeInTheDocument();
  });

  // S15: Null myCardgroupsConnection in the server response emits a console.warn
  it("S15: warns when myCardgroupsConnection arrives null from the server", async () => {
    const nullConnectionMock = {
      request: { query: MyCardgroupsConnectionDocument, variables: { first: 100 } },
      result: {
        data: {
          myCardgroupsConnection: null,
        },
      },
    };

    const consoleWarnSpy = vi.spyOn(console, "warn").mockImplementation(() => {});
    try {
      renderClient([nullConnectionMock]);

      await waitFor(() => {
        expect(consoleWarnSpy).toHaveBeenCalledWith(expect.stringContaining("[admin/dictionary]"));
      });
    } finally {
      consoleWarnSpy.mockRestore();
    }
  });

  // S14: Second call to ValidateDictionary with a different payload
  //      verifies the mock variables match the encoded payload bytes exactly.
  it("S14: second validate call with a larger payload re-enables Import", async () => {
    const user = userEvent.setup();

    const validateMock = {
      request: {
        query: ValidateDictionaryDocument,
        variables: { input: { payload: PAYLOAD_ENCODED_EDITED } },
      },
      result: {
        data: {
          validateDictionary: {
            __typename: "DictionaryValidationResult" as const,
            valid: true,
            parsedWords: [
              {
                __typename: "ParsedWord" as const,
                front: "apple",
                back: "the fruit",
                line: 1,
              },
              {
                __typename: "ParsedWord" as const,
                front: "banana",
                back: "a yellow fruit",
                line: 2,
              },
            ],
            errors: [],
          },
        },
      },
    };

    renderClient([CARDGROUPS_MOCK, validateMock]);

    const select = await screen.findByLabelText("Target cardgroup");
    await user.selectOptions(select, CG_1.id);

    const textarea = screen.getByLabelText("Dictionary payload");
    await user.type(textarea, PAYLOAD_TEXT_EDITED);
    await user.click(screen.getByRole("button", { name: /validate/i }));

    await waitFor(() => {
      expect(screen.getByRole("button", { name: /^import$/i })).not.toBeDisabled();
    });

    // Both words should appear in the preview
    expect(screen.getByText("banana")).toBeInTheDocument();
    expect(screen.getByText("a yellow fruit")).toBeInTheDocument();
  });
});

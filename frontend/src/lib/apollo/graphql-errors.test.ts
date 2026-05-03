import { CombinedGraphQLErrors } from "@apollo/client/errors";
import { describe, expect, test } from "vitest";
import { isUnauthenticatedGraphQLError, tryGetDuplicateCardInfo } from "./graphql-errors";

const PREFIX = "GraphQL errors: ";

function makeErr(payload: unknown): Error {
  return new Error(PREFIX + JSON.stringify(payload));
}

describe("isUnauthenticatedGraphQLError", () => {
  test("non-Error value returns false", () => {
    expect(isUnauthenticatedGraphQLError(null)).toBe(false);
    expect(isUnauthenticatedGraphQLError("some string")).toBe(false);
    expect(isUnauthenticatedGraphQLError(42)).toBe(false);
    expect(isUnauthenticatedGraphQLError({})).toBe(false);
  });

  test("Error with wrong prefix returns false", () => {
    expect(isUnauthenticatedGraphQLError(new Error("Network error"))).toBe(false);
    expect(isUnauthenticatedGraphQLError(new Error("UNAUTHENTICATED"))).toBe(false);
    expect(isUnauthenticatedGraphQLError(new Error("GraphQL errors UNAUTHENTICATED"))).toBe(false);
  });

  test("Error with correct prefix but malformed JSON returns false", () => {
    expect(isUnauthenticatedGraphQLError(new Error(`${PREFIX}not-json`))).toBe(false);
    expect(isUnauthenticatedGraphQLError(new Error(`${PREFIX}{unclosed`))).toBe(false);
  });

  test("Error with correct prefix + valid JSON but no UNAUTHENTICATED code returns false", () => {
    const err = makeErr([{ extensions: { code: "FORBIDDEN" } }]);
    expect(isUnauthenticatedGraphQLError(err)).toBe(false);
  });

  test("Error with correct prefix + valid JSON + missing extensions returns false", () => {
    const err = makeErr([{ message: "something went wrong" }]);
    expect(isUnauthenticatedGraphQLError(err)).toBe(false);
  });

  test("Error with correct prefix + UNAUTHENTICATED code returns true", () => {
    const err = makeErr([{ extensions: { code: "UNAUTHENTICATED" } }]);
    expect(isUnauthenticatedGraphQLError(err)).toBe(true);
  });

  test("mixed errors array with at least one UNAUTHENTICATED returns true", () => {
    const err = makeErr([
      { extensions: { code: "FORBIDDEN" } },
      { extensions: { code: "UNAUTHENTICATED" } },
      { message: "another error" },
    ]);
    expect(isUnauthenticatedGraphQLError(err)).toBe(true);
  });
});

function makeCombined(
  errors: Array<{ message?: string; extensions?: Record<string, unknown> }>,
): CombinedGraphQLErrors {
  return new CombinedGraphQLErrors({
    errors: errors.map((e) => ({ message: e.message ?? "error", ...e })),
  });
}

describe("tryGetDuplicateCardInfo", () => {
  test("match: correct code, reason, and payload returns DuplicateCardInfo", () => {
    const err = makeCombined([
      {
        message: "card with same front exists in this cardgroup",
        extensions: {
          code: "BAD_USER_INPUT",
          field: "front",
          reason: "CARD_DUPLICATE_FRONT",
          existingCardId: "abc-123",
          existingBack: "existing back text",
        },
      },
    ]);
    expect(tryGetDuplicateCardInfo(err)).toEqual({
      existingCardId: "abc-123",
      existingBack: "existing back text",
    });
  });

  test("no match (wrong code): BAD_USER_INPUT without reason discriminator returns null", () => {
    const err = makeCombined([
      {
        message: "some bad input",
        extensions: { code: "BAD_USER_INPUT", field: "front" },
      },
    ]);
    expect(tryGetDuplicateCardInfo(err)).toBeNull();
  });

  test("no match (wrong reason): BAD_USER_INPUT with a different reason returns null", () => {
    const err = makeCombined([
      {
        message: "other validation error",
        extensions: {
          code: "BAD_USER_INPUT",
          field: "front",
          reason: "SOME_OTHER_REASON",
          existingCardId: "abc-123",
          existingBack: "existing back text",
        },
      },
    ]);
    expect(tryGetDuplicateCardInfo(err)).toBeNull();
  });

  test("no match (other error type): UNAUTHENTICATED entry returns null", () => {
    const err = makeCombined([
      {
        message: "Not authenticated",
        extensions: { code: "UNAUTHENTICATED" },
      },
    ]);
    expect(tryGetDuplicateCardInfo(err)).toBeNull();
  });

  test("no match (non-Combined error): plain Error returns null", () => {
    expect(tryGetDuplicateCardInfo(new Error("hi"))).toBeNull();
  });

  test("no match (non-Combined): null returns null", () => {
    expect(tryGetDuplicateCardInfo(null)).toBeNull();
  });

  test("shape mismatch: missing existingBack returns null", () => {
    const err = makeCombined([
      {
        message: "duplicate",
        extensions: {
          code: "BAD_USER_INPUT",
          reason: "CARD_DUPLICATE_FRONT",
          existingCardId: "abc-123",
        },
      },
    ]);
    expect(tryGetDuplicateCardInfo(err)).toBeNull();
  });

  test("shape mismatch: non-string existingCardId returns null", () => {
    const err = makeCombined([
      {
        message: "duplicate",
        extensions: {
          code: "BAD_USER_INPUT",
          reason: "CARD_DUPLICATE_FRONT",
          existingCardId: 42,
          existingBack: "some back",
        },
      },
    ]);
    expect(tryGetDuplicateCardInfo(err)).toBeNull();
  });

  test("shape mismatch: empty-string existingBack returns null", () => {
    const err = makeCombined([
      {
        message: "duplicate",
        extensions: {
          code: "BAD_USER_INPUT",
          reason: "CARD_DUPLICATE_FRONT",
          existingCardId: "abc-123",
          existingBack: "",
        },
      },
    ]);
    expect(tryGetDuplicateCardInfo(err)).toBeNull();
  });

  test("multi-entry: only one entry matches, returns that entry's payload", () => {
    const err = makeCombined([
      {
        message: "session expired",
        extensions: { code: "UNAUTHENTICATED" },
      },
      {
        message: "card with same front exists in this cardgroup",
        extensions: {
          code: "BAD_USER_INPUT",
          field: "front",
          reason: "CARD_DUPLICATE_FRONT",
          existingCardId: "xyz-789",
          existingBack: "the matching back",
        },
      },
      {
        message: "other field error",
        extensions: { code: "BAD_USER_INPUT", field: "back" },
      },
    ]);
    expect(tryGetDuplicateCardInfo(err)).toEqual({
      existingCardId: "xyz-789",
      existingBack: "the matching back",
    });
  });
});

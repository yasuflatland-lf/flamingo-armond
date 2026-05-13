import { describe, expect, test, vi } from "vitest";
import { isForbiddenGraphQLError, isUnauthenticatedGraphQLError } from "./graphql-errors";

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

  test("malformed JSON logs console.warn with error name", () => {
    const warnSpy = vi.spyOn(console, "warn").mockImplementation(() => {});
    const result = isUnauthenticatedGraphQLError(new Error(`${PREFIX}not-json`));
    expect(result).toBe(false);
    expect(warnSpy).toHaveBeenCalledWith("[graphql-errors] failed to parse GraphQL error message", {
      name: "SyntaxError",
    });
    warnSpy.mockRestore();
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

describe("isForbiddenGraphQLError", () => {
  test("non-Error value returns false", () => {
    expect(isForbiddenGraphQLError(null)).toBe(false);
    expect(isForbiddenGraphQLError("some string")).toBe(false);
  });

  test("Error with wrong prefix returns false", () => {
    expect(isForbiddenGraphQLError(new Error("FORBIDDEN"))).toBe(false);
  });

  test("Error with UNAUTHENTICATED code (not FORBIDDEN) returns false", () => {
    const err = makeErr([{ extensions: { code: "UNAUTHENTICATED" } }]);
    expect(isForbiddenGraphQLError(err)).toBe(false);
  });

  test("Error with FORBIDDEN code returns true", () => {
    const err = makeErr([{ extensions: { code: "FORBIDDEN" } }]);
    expect(isForbiddenGraphQLError(err)).toBe(true);
  });

  test("mixed errors array with at least one FORBIDDEN returns true", () => {
    const err = makeErr([
      { extensions: { code: "UNAUTHENTICATED" } },
      { extensions: { code: "FORBIDDEN" } },
    ]);
    expect(isForbiddenGraphQLError(err)).toBe(true);
  });
});

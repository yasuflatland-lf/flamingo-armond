import { CombinedGraphQLErrors } from "@apollo/client/errors";
import { describe, expect, test, vi } from "vitest";
import {
  isForbiddenGraphQLError,
  isUnauthenticatedGraphQLError,
  liftGraphQLCodes,
} from "./graphql-errors";

const PREFIX = "GraphQL errors: ";

function makeErr(payload: unknown): Error {
  return new Error(PREFIX + JSON.stringify(payload));
}

/** Build a CombinedGraphQLErrors from a plain errors array. */
function makeCombinedError(
  errors: Array<{ message: string; extensions?: Record<string, unknown> }>,
): CombinedGraphQLErrors {
  return new CombinedGraphQLErrors({ errors });
}

describe("liftGraphQLCodes", () => {
  test("returns extension codes from a CombinedGraphQLErrors, skipping entries without a code", () => {
    const err = makeCombinedError([
      { message: "Not allowed", extensions: { code: "FORBIDDEN" } },
      // Entry without an extensions.code — must be skipped silently.
      { message: "Floating message", extensions: {} },
    ]);
    expect(liftGraphQLCodes(err)).toEqual(["FORBIDDEN"]);
  });

  test("returns multiple codes preserving order", () => {
    const err = makeCombinedError([
      { message: "Session expired", extensions: { code: "UNAUTHENTICATED" } },
      { message: "Not allowed", extensions: { code: "FORBIDDEN" } },
    ]);
    expect(liftGraphQLCodes(err)).toEqual(["UNAUTHENTICATED", "FORBIDDEN"]);
  });

  test("returns empty array for a non-CombinedGraphQLErrors (plain) Error", () => {
    expect(liftGraphQLCodes(new Error("network down"))).toEqual([]);
  });

  test("ignores a v3-shaped Error that carries .graphQLErrors but is not a CombinedGraphQLErrors", () => {
    // Guards the Apollo v3 → v4 migration: the old shape attached an array
    // named `graphQLErrors` to a plain Error. v4 surfaces a typed
    // CombinedGraphQLErrors instance instead. liftGraphQLCodes must NOT match
    // the v3 shape — doing so would mask a real migration regression.
    const v3Shaped = Object.assign(new Error("transport"), {
      graphQLErrors: [{ extensions: { code: "FORBIDDEN" } }],
    });
    expect(liftGraphQLCodes(v3Shaped)).toEqual([]);
  });

  test("returns empty array for null / undefined", () => {
    expect(liftGraphQLCodes(null)).toEqual([]);
    expect(liftGraphQLCodes(undefined)).toEqual([]);
  });

  test("returns empty array for a primitive (string / number)", () => {
    expect(liftGraphQLCodes("oops")).toEqual([]);
    expect(liftGraphQLCodes(42)).toEqual([]);
  });

  test("ignores non-string extensions.code values", () => {
    // The shape narrows on `typeof code === 'string'`; numeric or object
    // values must be skipped so the warn payload stays a string[] enum.
    const err = makeCombinedError([
      { message: "Bad code", extensions: { code: 500 } },
      { message: "Good code", extensions: { code: "INTERNAL" } },
    ]);
    expect(liftGraphQLCodes(err)).toEqual(["INTERNAL"]);
  });
});

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

  test("correct prefix + non-array JSON payload returns false", () => {
    expect(isUnauthenticatedGraphQLError(new Error(`${PREFIX}{}`))).toBe(false);
    expect(isUnauthenticatedGraphQLError(new Error(`${PREFIX}42`))).toBe(false);
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

  test("Error with correct prefix but malformed JSON returns false", () => {
    expect(isForbiddenGraphQLError(new Error(`${PREFIX}not-json`))).toBe(false);
    expect(isForbiddenGraphQLError(new Error(`${PREFIX}{unclosed`))).toBe(false);
  });

  test("malformed JSON logs console.warn with error name", () => {
    const warnSpy = vi.spyOn(console, "warn").mockImplementation(() => {});
    const result = isForbiddenGraphQLError(new Error(`${PREFIX}not-json`));
    expect(result).toBe(false);
    expect(warnSpy).toHaveBeenCalledWith("[graphql-errors] failed to parse GraphQL error message", {
      name: "SyntaxError",
    });
    warnSpy.mockRestore();
  });

  test("Error with correct prefix + valid JSON array + entry missing extensions returns false", () => {
    const err = makeErr([{ message: "something went wrong" }]);
    expect(isForbiddenGraphQLError(err)).toBe(false);
  });

  test("correct prefix + non-array JSON payload returns false", () => {
    expect(isForbiddenGraphQLError(new Error(`${PREFIX}{}`))).toBe(false);
    expect(isForbiddenGraphQLError(new Error(`${PREFIX}42`))).toBe(false);
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

import { CombinedGraphQLErrors } from "@apollo/client/errors";
import { describe, expect, it } from "vitest";
import { getBackendErrorBanner, getBackendFieldErrors } from "./errors";

/** Build a CombinedGraphQLErrors from a plain errors array. */
function makeCombinedError(
  errors: Array<{
    message: string;
    extensions?: Record<string, unknown>;
  }>,
): CombinedGraphQLErrors {
  return new CombinedGraphQLErrors({ errors });
}

const NETWORK_MESSAGE = "Could not reach the server. Please try again.";

describe("getBackendFieldErrors", () => {
  it("returns empty map for a non-CombinedGraphQLErrors network error", () => {
    const err = new Error("Network request failed");
    expect(getBackendFieldErrors(err)).toEqual({});
  });

  it("returns field map for BAD_USER_INPUT with a field extension", () => {
    const err = makeCombinedError([
      {
        message: "Display name is too short",
        extensions: { code: "BAD_USER_INPUT", field: "displayName" },
      },
    ]);
    expect(getBackendFieldErrors(err)).toEqual({
      displayName: "Display name is too short",
    });
  });

  it("returns empty map for BAD_USER_INPUT without a field extension", () => {
    const err = makeCombinedError([
      {
        message: "Invalid input",
        extensions: { code: "BAD_USER_INPUT" },
      },
    ]);
    expect(getBackendFieldErrors(err)).toEqual({});
  });

  it("returns empty map for UNAUTHENTICATED errors", () => {
    const err = makeCombinedError([
      { message: "Session expired", extensions: { code: "UNAUTHENTICATED" } },
    ]);
    expect(getBackendFieldErrors(err)).toEqual({});
  });

  it("returns empty map for INTERNAL errors", () => {
    const err = makeCombinedError([
      { message: "Something went wrong", extensions: { code: "INTERNAL" } },
    ]);
    expect(getBackendFieldErrors(err)).toEqual({});
  });

  it("returns only field errors from a mix of field and INTERNAL errors", () => {
    const err = makeCombinedError([
      {
        message: "Bio is too long",
        extensions: { code: "BAD_USER_INPUT", field: "bio" },
      },
      { message: "Something went wrong", extensions: { code: "INTERNAL" } },
    ]);
    expect(getBackendFieldErrors(err)).toEqual({ bio: "Bio is too long" });
  });
});

describe("getBackendErrorBanner", () => {
  it("returns undefined for a falsy error", () => {
    expect(getBackendErrorBanner(undefined)).toBeUndefined();
    expect(getBackendErrorBanner(null)).toBeUndefined();
  });

  it("returns generic network message for a non-CombinedGraphQLErrors error", () => {
    const err = new Error("Network request failed");
    expect(getBackendErrorBanner(err)).toBe(NETWORK_MESSAGE);
  });

  it("returns undefined when all errors are field-level BAD_USER_INPUT", () => {
    const err = makeCombinedError([
      {
        message: "Display name is required",
        extensions: { code: "BAD_USER_INPUT", field: "displayName" },
      },
    ]);
    expect(getBackendErrorBanner(err)).toBeUndefined();
  });

  it("returns the message for BAD_USER_INPUT without a field extension", () => {
    const err = makeCombinedError([
      {
        message: "Invalid input provided",
        extensions: { code: "BAD_USER_INPUT" },
      },
    ]);
    expect(getBackendErrorBanner(err)).toBe("Invalid input provided");
  });

  it("returns the session-expired message for UNAUTHENTICATED", () => {
    const err = makeCombinedError([
      { message: "Not authenticated", extensions: { code: "UNAUTHENTICATED" } },
    ]);
    expect(getBackendErrorBanner(err)).toBe("Your session expired. Please sign in again.");
  });

  it("INTERNAL takes priority over UNAUTHENTICATED when INTERNAL appears first", () => {
    const err = makeCombinedError([
      { message: "Internal server error", extensions: { code: "INTERNAL" } },
      { message: "Not authenticated", extensions: { code: "UNAUTHENTICATED" } },
    ]);
    expect(getBackendErrorBanner(err)).toBe("Internal server error");
  });

  it("returns INTERNAL message from a mix of field error and INTERNAL", () => {
    const err = makeCombinedError([
      {
        message: "Bio is too long",
        extensions: { code: "BAD_USER_INPUT", field: "bio" },
      },
      { message: "Unexpected server failure", extensions: { code: "INTERNAL" } },
    ]);
    expect(getBackendErrorBanner(err)).toBe("Unexpected server failure");
  });

  it("INTERNAL takes priority over UNAUTHENTICATED when UNAUTHENTICATED appears first", () => {
    const err = makeCombinedError([
      { message: "Not authenticated", extensions: { code: "UNAUTHENTICATED" } },
      { message: "Internal server error", extensions: { code: "INTERNAL" } },
    ]);
    expect(getBackendErrorBanner(err)).toBe("Internal server error");
  });

  it("returns INTERNAL banner and field map when BAD_USER_INPUT(field) and INTERNAL coexist", () => {
    const err = makeCombinedError([
      {
        message: "Name is required",
        extensions: { code: "BAD_USER_INPUT", field: "name" },
      },
      { message: "Internal server error", extensions: { code: "INTERNAL" } },
    ]);
    expect(getBackendErrorBanner(err)).toBe("Internal server error");
    expect(getBackendFieldErrors(err)).toEqual({ name: "Name is required" });
  });

  it("returns UNAUTHENTICATED banner and field map when BAD_USER_INPUT(field) and UNAUTHENTICATED coexist", () => {
    const err = makeCombinedError([
      { message: "Not authenticated", extensions: { code: "UNAUTHENTICATED" } },
      {
        message: "Name is required",
        extensions: { code: "BAD_USER_INPUT", field: "name" },
      },
    ]);
    expect(getBackendErrorBanner(err)).toBe("Your session expired. Please sign in again.");
    expect(getBackendFieldErrors(err)).toEqual({ name: "Name is required" });
  });
});

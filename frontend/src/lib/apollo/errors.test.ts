import { CombinedGraphQLErrors } from "@apollo/client/errors";
import { afterEach, describe, expect, it, vi } from "vitest";
import {
  classifyAndLogAuthOutcome,
  classifyMutationAuthError,
  getBackendErrorBanner,
  getBackendFieldErrors,
  mutationAuthBanner,
} from "./errors";

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

describe("classifyMutationAuthError", () => {
  it("classifies a FORBIDDEN error as forbidden", () => {
    const err = makeCombinedError([{ message: "Forbidden", extensions: { code: "FORBIDDEN" } }]);
    expect(classifyMutationAuthError(err)).toBe("forbidden");
  });

  it("classifies an UNAUTHENTICATED error as unauthenticated", () => {
    const err = makeCombinedError([
      { message: "Not authenticated", extensions: { code: "UNAUTHENTICATED" } },
    ]);
    expect(classifyMutationAuthError(err)).toBe("unauthenticated");
  });

  it("classifies any other GraphQL error code as other", () => {
    const badInput = makeCombinedError([
      { message: "Invalid input", extensions: { code: "BAD_USER_INPUT" } },
    ]);
    const internal = makeCombinedError([
      { message: "Server error", extensions: { code: "INTERNAL" } },
    ]);
    expect(classifyMutationAuthError(badInput)).toBe("other");
    expect(classifyMutationAuthError(internal)).toBe("other");
  });

  it("classifies a non-CombinedGraphQLErrors network error as other", () => {
    expect(classifyMutationAuthError(new Error("Network request failed"))).toBe("other");
  });

  it("classifies a falsy error as other", () => {
    expect(classifyMutationAuthError(null)).toBe("other");
    expect(classifyMutationAuthError(undefined)).toBe("other");
  });

  it("returns the kind of the first auth error in the array (mirrors classifyQueryError)", () => {
    const forbiddenFirst = makeCombinedError([
      { message: "Forbidden", extensions: { code: "FORBIDDEN" } },
      { message: "Not authenticated", extensions: { code: "UNAUTHENTICATED" } },
    ]);
    const unauthenticatedFirst = makeCombinedError([
      { message: "Not authenticated", extensions: { code: "UNAUTHENTICATED" } },
      { message: "Forbidden", extensions: { code: "FORBIDDEN" } },
    ]);
    expect(classifyMutationAuthError(forbiddenFirst)).toBe("forbidden");
    expect(classifyMutationAuthError(unauthenticatedFirst)).toBe("unauthenticated");
  });
});

describe("mutationAuthBanner", () => {
  const copy = {
    forbidden: "You do not have permission.",
    unauthenticated: "Your session expired.",
    fallback: "Something went wrong.",
  };

  it("returns the forbidden copy for a FORBIDDEN error", () => {
    const err = makeCombinedError([{ message: "Forbidden", extensions: { code: "FORBIDDEN" } }]);
    expect(mutationAuthBanner(err, copy)).toBe(copy.forbidden);
  });

  it("returns the unauthenticated copy for an UNAUTHENTICATED error", () => {
    const err = makeCombinedError([
      { message: "Not authenticated", extensions: { code: "UNAUTHENTICATED" } },
    ]);
    expect(mutationAuthBanner(err, copy)).toBe(copy.unauthenticated);
  });

  it("returns the fallback copy for any other GraphQL error", () => {
    const err = makeCombinedError([
      { message: "Invalid input", extensions: { code: "BAD_USER_INPUT" } },
    ]);
    expect(mutationAuthBanner(err, copy)).toBe(copy.fallback);
  });

  it("returns the fallback copy for a network error", () => {
    expect(mutationAuthBanner(new Error("Network request failed"), copy)).toBe(copy.fallback);
  });
});

describe("classifyAndLogAuthOutcome", () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("returns an auth outcome with kind forbidden for a FORBIDDEN error without warning", () => {
    const warn = vi.spyOn(console, "warn").mockImplementation(() => {});
    const err = makeCombinedError([{ message: "Forbidden", extensions: { code: "FORBIDDEN" } }]);
    expect(classifyAndLogAuthOutcome(err, "useScope", "doThing")).toEqual({
      status: "auth",
      kind: "forbidden",
    });
    expect(warn).not.toHaveBeenCalled();
  });

  it("returns an auth outcome with kind unauthenticated for an UNAUTHENTICATED error without warning", () => {
    const warn = vi.spyOn(console, "warn").mockImplementation(() => {});
    const err = makeCombinedError([
      { message: "Not authenticated", extensions: { code: "UNAUTHENTICATED" } },
    ]);
    expect(classifyAndLogAuthOutcome(err, "useScope", "doThing")).toEqual({
      status: "auth",
      kind: "unauthenticated",
    });
    expect(warn).not.toHaveBeenCalled();
  });

  it("returns a rejected outcome and warns with the scoped prefix for a non-auth GraphQL error", () => {
    const warn = vi.spyOn(console, "warn").mockImplementation(() => {});
    const err = makeCombinedError([
      { message: "Invalid input", extensions: { code: "BAD_USER_INPUT" } },
    ]);
    expect(classifyAndLogAuthOutcome(err, "useScope", "doThing")).toEqual({ status: "rejected" });
    expect(warn).toHaveBeenCalledTimes(1);
    expect(warn).toHaveBeenCalledWith("[useScope] doThing rejected", {
      name: "CombinedGraphQLErrors",
      codes: ["BAD_USER_INPUT"],
    });
  });

  it("returns a rejected outcome and warns with name unknown for a non-Error throw", () => {
    const warn = vi.spyOn(console, "warn").mockImplementation(() => {});
    expect(classifyAndLogAuthOutcome("boom", "useScope", "doThing")).toEqual({
      status: "rejected",
    });
    expect(warn).toHaveBeenCalledWith("[useScope] doThing rejected", {
      name: "unknown",
      codes: [],
    });
  });

  it("merges caller-supplied extra context into the warn payload", () => {
    const warn = vi.spyOn(console, "warn").mockImplementation(() => {});
    const err = new Error("Network request failed");
    expect(
      classifyAndLogAuthOutcome(err, "useMasterMutations", "deleteMaster", { masterId: "m-1" }),
    ).toEqual({ status: "rejected" });
    expect(warn).toHaveBeenCalledWith("[useMasterMutations] deleteMaster rejected", {
      masterId: "m-1",
      name: "Error",
      codes: [],
    });
  });
});

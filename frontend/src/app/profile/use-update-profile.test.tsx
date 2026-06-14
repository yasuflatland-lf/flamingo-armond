// @vitest-environment jsdom
import type { MockedResponse } from "@apollo/client/testing";
import { MockedProvider } from "@apollo/client/testing/react";
import { act, renderHook } from "@testing-library/react";
import { GraphQLError } from "graphql";
import { createElement, type ReactNode } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { UpdateProfileDocument } from "@/generated/graphql";
import { type UpdateProfileOutcome, useUpdateProfile } from "./use-update-profile";

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

const INPUT = { displayName: "Alice" };
const REQUEST = { query: UpdateProfileDocument, variables: { input: INPUT } };

function renderUpdateProfile(mocks: MockedResponse[]) {
  return renderHook(() => useUpdateProfile(), {
    wrapper: ({ children }: { children: ReactNode }) =>
      createElement(MockedProvider, { mocks }, children),
  });
}

type RenderResult = ReturnType<typeof renderUpdateProfile>["result"];

async function runSubmit(result: RenderResult): Promise<UpdateProfileOutcome> {
  let outcome: UpdateProfileOutcome | undefined;
  await act(async () => {
    outcome = await result.current.submit(INPUT);
  });
  return outcome as UpdateProfileOutcome;
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

describe("useUpdateProfile", () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("returns success when the server resolves UpdateProfileSuccess", async () => {
    const mocks = [
      {
        request: REQUEST,
        result: {
          data: {
            updateProfile: {
              __typename: "UpdateProfileSuccess",
              user: {
                __typename: "User",
                id: "user-1",
                displayName: "Alice",
                bio: null,
                avatarUrl: null,
              },
            },
          },
        },
      },
    ];

    const { result } = renderUpdateProfile(mocks);
    const outcome = await runSubmit(result);

    expect(outcome).toEqual({ status: "success" });
  });

  it("returns validation with the field + message on InputValidationError", async () => {
    const mocks = [
      {
        request: REQUEST,
        result: {
          data: {
            updateProfile: {
              __typename: "InputValidationError",
              field: "displayName",
              message: "displayName must be 1-50 characters",
            },
          },
        },
      },
    ];

    const { result } = renderUpdateProfile(mocks);
    const outcome = await runSubmit(result);

    expect(outcome).toEqual({
      status: "validation",
      field: "displayName",
      message: "displayName must be 1-50 characters",
    });
  });

  it("returns unauthenticated when the mutation rejects with UNAUTHENTICATED", async () => {
    const mocks = [
      {
        request: REQUEST,
        result: {
          errors: [
            new GraphQLError("Unauthenticated", { extensions: { code: "UNAUTHENTICATED" } }),
          ],
        },
      },
    ];

    const { result } = renderUpdateProfile(mocks);
    const outcome = await runSubmit(result);

    expect(outcome).toEqual({ status: "unauthenticated" });
  });

  it("returns rejected with the backend banner on a non-auth GraphQL error", async () => {
    const mocks = [
      {
        request: REQUEST,
        result: {
          errors: [
            new GraphQLError("Something went wrong on the server", {
              extensions: { code: "INTERNAL" },
            }),
          ],
        },
      },
    ];

    const { result } = renderUpdateProfile(mocks);
    const outcome = await runSubmit(result);

    expect(outcome).toEqual({
      status: "rejected",
      banner: "Something went wrong on the server",
    });
  });

  it("returns rejected with the generic network banner on a transport error", async () => {
    const warnSpy = vi.spyOn(console, "warn").mockImplementation(() => {});
    const mocks = [{ request: REQUEST, error: new Error("network down") }];

    const { result } = renderUpdateProfile(mocks);
    const outcome = await runSubmit(result);

    expect(outcome).toEqual({
      status: "rejected",
      banner: "Could not reach the server. Please try again.",
    });
    // Redaction contract: the structured warn must not leak err.message.
    const warnArgs = warnSpy.mock.calls.flat().map((a) => JSON.stringify(a));
    expect(warnArgs.some((a) => a.includes("network down"))).toBe(false);
  });

  it("returns unexpected on an unknown / future union variant", async () => {
    const warnSpy = vi.spyOn(console, "warn").mockImplementation(() => {});
    const mocks = [
      {
        request: REQUEST,
        result: { data: { updateProfile: { __typename: "SomeFutureVariant" } } },
      },
    ];

    const { result } = renderUpdateProfile(mocks);
    const outcome = await runSubmit(result);

    expect(outcome).toEqual({ status: "unexpected" });
    expect(warnSpy).toHaveBeenCalled();
  });
});

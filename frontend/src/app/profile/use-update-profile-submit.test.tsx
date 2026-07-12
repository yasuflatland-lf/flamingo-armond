// @vitest-environment happy-dom
import type { MockedResponse } from "@apollo/client/testing";
import { MockedProvider } from "@apollo/client/testing/react";
import { act, renderHook } from "@testing-library/react";
import { GraphQLError } from "graphql";
import { NextIntlClientProvider } from "next-intl";
import type { ReactNode } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { UpdateProfileDocument } from "@/generated/graphql";
import enMessages from "../../../messages/en.json";
import { useUpdateProfileSubmit } from "./use-update-profile-submit";

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

const INPUT = { displayName: "Alice" };
const REQUEST = { query: UpdateProfileDocument, variables: { input: INPUT } };

const SESSION_EXPIRED = "Session over — sign back in.";
const REJECTION_LABEL = "[test] update profile rejected";

function renderSubmit(mocks: MockedResponse[], onSuccess: () => void) {
  return renderHook(
    () =>
      useUpdateProfileSubmit({
        onSuccess,
        sessionExpiredMessage: SESSION_EXPIRED,
        rejectionLabel: REJECTION_LABEL,
      }),
    {
      wrapper: ({ children }: { children: ReactNode }) => (
        <NextIntlClientProvider locale="en" messages={enMessages} timeZone="UTC">
          <MockedProvider mocks={mocks}>{children}</MockedProvider>
        </NextIntlClientProvider>
      ),
    },
  );
}

type RenderResult = ReturnType<typeof renderSubmit>["result"];

async function runSubmit(result: RenderResult): Promise<{ threw: boolean; error?: unknown }> {
  let threw = false;
  let error: unknown;
  await act(async () => {
    try {
      await result.current.submit(INPUT);
    } catch (err) {
      threw = true;
      error = err;
    }
  });
  return { threw, error };
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

describe("useUpdateProfileSubmit", () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("calls onSuccess and leaves error state clear on UpdateProfileSuccess", async () => {
    const onSuccess = vi.fn();
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

    const { result } = renderSubmit(mocks, onSuccess);
    const { threw } = await runSubmit(result);

    expect(threw).toBe(false);
    expect(onSuccess).toHaveBeenCalledOnce();
    expect(result.current.validationError).toBeNull();
    expect(result.current.bannerMessage).toBeNull();
    expect(result.current.fieldErrors).toEqual({});
  });

  it("maps InputValidationError to validationError + fieldErrors, no onSuccess", async () => {
    const onSuccess = vi.fn();
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

    const { result } = renderSubmit(mocks, onSuccess);
    const { threw } = await runSubmit(result);

    expect(threw).toBe(false);
    expect(onSuccess).not.toHaveBeenCalled();
    expect(result.current.validationError).toEqual({
      field: "displayName",
      message: "displayName must be 1-50 characters",
    });
    expect(result.current.fieldErrors).toEqual({
      displayName: "displayName must be 1-50 characters",
    });
    expect(result.current.bannerMessage).toBeNull();
  });

  it("maps UNAUTHENTICATED to the caller-supplied sessionExpiredMessage banner", async () => {
    const onSuccess = vi.fn();
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

    const { result } = renderSubmit(mocks, onSuccess);
    const { threw } = await runSubmit(result);

    expect(threw).toBe(false);
    expect(result.current.bannerMessage).toBe(SESSION_EXPIRED);
    expect(result.current.validationError).toBeNull();
  });

  it("maps an unknown union variant to the generic Common banner", async () => {
    vi.spyOn(console, "warn").mockImplementation(() => {});
    const onSuccess = vi.fn();
    const mocks = [
      {
        request: REQUEST,
        result: { data: { updateProfile: { __typename: "SomeFutureVariant" } } },
      },
    ];

    const { result } = renderSubmit(mocks, onSuccess);
    const { threw } = await runSubmit(result);

    expect(threw).toBe(false);
    expect(result.current.bannerMessage).toBe(enMessages.Common.somethingWentWrong);
  });

  it("sets the backend banner AND re-throws rejectionLabel on a non-auth GraphQL error", async () => {
    const onSuccess = vi.fn();
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

    const { result } = renderSubmit(mocks, onSuccess);
    const { threw, error } = await runSubmit(result);

    expect(threw).toBe(true);
    expect((error as Error).message).toBe(REJECTION_LABEL);
    expect(result.current.bannerMessage).toBe("Something went wrong on the server");
  });

  it("falls back to the generic Common banner when the transport error yields no backend banner", async () => {
    vi.spyOn(console, "warn").mockImplementation(() => {});
    const onSuccess = vi.fn();
    const mocks = [{ request: REQUEST, error: new Error("network down") }];

    const { result } = renderSubmit(mocks, onSuccess);
    const { threw, error } = await runSubmit(result);

    expect(threw).toBe(true);
    expect((error as Error).message).toBe(REJECTION_LABEL);
    // getBackendErrorBanner maps a raw transport error to the connectivity copy.
    expect(result.current.bannerMessage).toBe("Could not reach the server. Please try again.");
  });

  it("reset() clears validationError and bannerMessage", async () => {
    const onSuccess = vi.fn();
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

    const { result } = renderSubmit(mocks, onSuccess);
    await runSubmit(result);

    expect(result.current.validationError).not.toBeNull();

    act(() => {
      result.current.reset();
    });

    expect(result.current.validationError).toBeNull();
    expect(result.current.bannerMessage).toBeNull();
    expect(result.current.fieldErrors).toEqual({});
  });
});

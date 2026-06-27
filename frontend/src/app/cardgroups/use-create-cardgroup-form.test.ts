// @vitest-environment happy-dom
import type { MockedResponse } from "@apollo/client/testing";
import { MockedProvider } from "@apollo/client/testing/react";
import { act, renderHook } from "@testing-library/react";
import { GraphQLError } from "graphql";
import { createElement, type ReactNode } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { CreateCardgroupMutation } from "./queries";
import { useCreateCardgroupForm } from "./use-create-cardgroup-form";

const CREATED_CARDGROUP = {
  __typename: "Cardgroup" as const,
  id: "cg-1",
  name: "My Group",
  updatedAt: "2026-04-30T00:00:00Z",
};

function successMock(name: string): MockedResponse {
  return {
    request: { query: CreateCardgroupMutation, variables: { input: { name } } },
    result: {
      data: {
        createCardgroup: {
          __typename: "CreateCardgroupSuccess",
          cardgroup: { ...CREATED_CARDGROUP, name },
        },
      },
    },
  };
}

function render(mocks: MockedResponse[]) {
  return renderHook(() => useCreateCardgroupForm(), {
    wrapper: ({ children }: { children: ReactNode }) =>
      createElement(MockedProvider, { mocks }, children),
  });
}

afterEach(() => {
  vi.restoreAllMocks();
});

describe("useCreateCardgroupForm.submit", () => {
  it("returns success and sets no error state on CreateCardgroupSuccess", async () => {
    const { result } = render([successMock("My Group")]);

    let outcome: unknown;
    await act(async () => {
      outcome = await result.current.submit({ name: "My Group" });
    });

    expect(outcome).toEqual({ status: "success", cardgroupId: "cg-1" });
    expect(result.current.validationError).toBeNull();
    expect(result.current.authError).toBeNull();
    expect(result.current.limitError).toBeNull();
    expect(result.current.unexpectedError).toBeNull();
  });

  it("sets validationError on InputValidationError", async () => {
    const mocks: MockedResponse[] = [
      {
        request: { query: CreateCardgroupMutation, variables: { input: { name: "Dup" } } },
        result: {
          data: {
            createCardgroup: {
              __typename: "InputValidationError",
              field: "name",
              message: "name already exists",
            },
          },
        },
      },
    ];
    const { result } = render(mocks);

    let outcome: unknown;
    await act(async () => {
      outcome = await result.current.submit({ name: "Dup" });
    });

    expect(outcome).toEqual({
      status: "validation",
      field: "name",
      message: "name already exists",
    });
    expect(result.current.validationError).toEqual({
      field: "name",
      message: "name already exists",
    });
    expect(result.current.unexpectedError).toBeNull();
  });

  it("sets authError 'unauthenticated' when the mutation rejects with UNAUTHENTICATED", async () => {
    const mocks: MockedResponse[] = [
      {
        request: { query: CreateCardgroupMutation, variables: { input: { name: "My Group" } } },
        result: {
          errors: [
            new GraphQLError("Unauthenticated", { extensions: { code: "UNAUTHENTICATED" } }),
          ],
        },
      },
    ];
    const { result } = render(mocks);

    let outcome: unknown;
    await act(async () => {
      outcome = await result.current.submit({ name: "My Group" });
    });

    expect(outcome).toEqual({ status: "auth", kind: "unauthenticated" });
    expect(result.current.authError).toBe("unauthenticated");
  });

  it("sets authError 'forbidden' when the mutation rejects with FORBIDDEN", async () => {
    const mocks: MockedResponse[] = [
      {
        request: { query: CreateCardgroupMutation, variables: { input: { name: "My Group" } } },
        result: { errors: [new GraphQLError("Forbidden", { extensions: { code: "FORBIDDEN" } })] },
      },
    ];
    const { result } = render(mocks);

    let outcome: unknown;
    await act(async () => {
      outcome = await result.current.submit({ name: "My Group" });
    });

    expect(outcome).toEqual({ status: "auth", kind: "forbidden" });
    expect(result.current.authError).toBe("forbidden");
  });

  it("sets limitError on CardgroupLimitReachedError", async () => {
    const mocks: MockedResponse[] = [
      {
        request: { query: CreateCardgroupMutation, variables: { input: { name: "My Group" } } },
        result: {
          data: {
            createCardgroup: {
              __typename: "CardgroupLimitReachedError",
              message: "cardgroup limit reached",
              limit: 5,
              current: 5,
            },
          },
        },
      },
    ];
    const { result } = render(mocks);

    let outcome: unknown;
    await act(async () => {
      outcome = await result.current.submit({ name: "My Group" });
    });

    expect(outcome).toEqual({ status: "limit", limit: 5, current: 5 });
    expect(result.current.limitError).toEqual({ limit: 5, current: 5 });
  });

  it("sets unexpectedError 'unexpected' on an unknown payload variant", async () => {
    vi.spyOn(console, "warn").mockImplementation(() => {});
    const mocks: MockedResponse[] = [
      {
        request: { query: CreateCardgroupMutation, variables: { input: { name: "My Group" } } },
        result: { data: { createCardgroup: { __typename: "SomeFutureVariant" } as never } },
      },
    ];
    const { result } = render(mocks);

    let outcome: unknown;
    await act(async () => {
      outcome = await result.current.submit({ name: "My Group" });
    });

    expect(outcome).toEqual({ status: "unexpected" });
    expect(result.current.unexpectedError).toBe("unexpected");
  });

  it("sets unexpectedError 'unexpected' on a null payload (partial-response null bubble)", async () => {
    vi.spyOn(console, "warn").mockImplementation(() => {});
    const mocks: MockedResponse[] = [
      {
        request: { query: CreateCardgroupMutation, variables: { input: { name: "My Group" } } },
        result: { data: { createCardgroup: null as never } },
      },
    ];
    const { result } = render(mocks);

    let outcome: unknown;
    await act(async () => {
      outcome = await result.current.submit({ name: "My Group" });
    });

    expect(outcome).toEqual({ status: "unexpected" });
    expect(result.current.unexpectedError).toBe("unexpected");
  });

  it("sets unexpectedError 'rejected' on a transport error", async () => {
    vi.spyOn(console, "warn").mockImplementation(() => {});
    const mocks: MockedResponse[] = [
      {
        request: { query: CreateCardgroupMutation, variables: { input: { name: "My Group" } } },
        error: new Error("network down"),
      },
    ];
    const { result } = render(mocks);

    let outcome: unknown;
    await act(async () => {
      outcome = await result.current.submit({ name: "My Group" });
    });

    expect(outcome).toEqual({ status: "rejected" });
    expect(result.current.unexpectedError).toBe("rejected");
  });

  it("clears prior error state before the next submit (success after validation)", async () => {
    const mocks: MockedResponse[] = [
      {
        request: { query: CreateCardgroupMutation, variables: { input: { name: "Dup" } } },
        result: {
          data: {
            createCardgroup: {
              __typename: "InputValidationError",
              field: "name",
              message: "name already exists",
            },
          },
        },
      },
      successMock("Fresh"),
    ];
    const { result } = render(mocks);

    await act(async () => {
      await result.current.submit({ name: "Dup" });
    });
    expect(result.current.validationError).not.toBeNull();

    let outcome: unknown;
    await act(async () => {
      outcome = await result.current.submit({ name: "Fresh" });
    });

    expect(outcome).toEqual({ status: "success", cardgroupId: "cg-1" });
    expect(result.current.validationError).toBeNull();
  });
});

describe("useCreateCardgroupForm.reset", () => {
  it("clears all error state", async () => {
    const mocks: MockedResponse[] = [
      {
        request: { query: CreateCardgroupMutation, variables: { input: { name: "Dup" } } },
        result: {
          data: {
            createCardgroup: {
              __typename: "InputValidationError",
              field: "name",
              message: "name already exists",
            },
          },
        },
      },
    ];
    const { result } = render(mocks);

    await act(async () => {
      await result.current.submit({ name: "Dup" });
    });
    expect(result.current.validationError).not.toBeNull();

    act(() => {
      result.current.reset();
    });

    expect(result.current.validationError).toBeNull();
    expect(result.current.authError).toBeNull();
    expect(result.current.limitError).toBeNull();
    expect(result.current.unexpectedError).toBeNull();
  });
});

// @vitest-environment happy-dom
import type { MockedResponse } from "@apollo/client/testing";
import { MockedProvider } from "@apollo/client/testing/react";
import { act, renderHook } from "@testing-library/react";
import { GraphQLError } from "graphql";
import { createElement, type ReactNode } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";
import {
  AdminCreateRoleMutation,
  AdminDeleteRoleMutation,
  AdminUpdateRoleMutation,
} from "./queries";
import { useRoleMutations } from "./use-role-mutations";

function roleNode(id: string, name: string) {
  return { __typename: "Role" as const, id, name };
}

function render(mocks: MockedResponse[]) {
  return renderHook(() => useRoleMutations(), {
    wrapper: ({ children }: { children: ReactNode }) =>
      createElement(MockedProvider, { mocks }, children),
  });
}

afterEach(() => {
  vi.restoreAllMocks();
});

describe("useRoleMutations.createRole", () => {
  it("returns success on CreateRoleSuccess", async () => {
    const mocks: MockedResponse[] = [
      {
        request: { query: AdminCreateRoleMutation, variables: { name: "moderator" } },
        result: {
          data: {
            createRole: { __typename: "CreateRoleSuccess", role: roleNode("r-1", "moderator") },
          },
        },
      },
    ];
    const { result } = render(mocks);

    let outcome: unknown;
    await act(async () => {
      outcome = await result.current.createRole({ name: "moderator" });
    });

    expect(outcome).toEqual({ status: "success" });
  });

  it("trims and lower-cases the name before mutating", async () => {
    // The mock only matches the normalized variable, so a passing outcome proves
    // the hook normalized "  Moderator  " to "moderator".
    const mocks: MockedResponse[] = [
      {
        request: { query: AdminCreateRoleMutation, variables: { name: "moderator" } },
        result: {
          data: {
            createRole: { __typename: "CreateRoleSuccess", role: roleNode("r-1", "moderator") },
          },
        },
      },
    ];
    const { result } = render(mocks);

    let outcome: unknown;
    await act(async () => {
      outcome = await result.current.createRole({ name: "  Moderator  " });
    });

    expect(outcome).toEqual({ status: "success" });
  });

  it("returns validation on InputValidationError", async () => {
    const mocks: MockedResponse[] = [
      {
        request: { query: AdminCreateRoleMutation, variables: { name: "admin" } },
        result: {
          data: {
            createRole: {
              __typename: "InputValidationError",
              field: "name",
              message: "role name already exists",
            },
          },
        },
      },
    ];
    const { result } = render(mocks);

    let outcome: unknown;
    await act(async () => {
      outcome = await result.current.createRole({ name: "admin" });
    });

    expect(outcome).toEqual({
      status: "validation",
      field: "name",
      message: "role name already exists",
    });
  });

  it("returns auth(forbidden) when the mutation rejects with FORBIDDEN", async () => {
    const mocks: MockedResponse[] = [
      {
        request: { query: AdminCreateRoleMutation, variables: { name: "moderator" } },
        result: { errors: [new GraphQLError("Forbidden", { extensions: { code: "FORBIDDEN" } })] },
      },
    ];
    const { result } = render(mocks);

    let outcome: unknown;
    await act(async () => {
      outcome = await result.current.createRole({ name: "moderator" });
    });

    expect(outcome).toEqual({ status: "auth", kind: "forbidden" });
  });

  it("returns rejected on a transport error", async () => {
    vi.spyOn(console, "warn").mockImplementation(() => {});
    const mocks: MockedResponse[] = [
      {
        request: { query: AdminCreateRoleMutation, variables: { name: "moderator" } },
        error: new Error("network down"),
      },
    ];
    const { result } = render(mocks);

    let outcome: unknown;
    await act(async () => {
      outcome = await result.current.createRole({ name: "moderator" });
    });

    expect(outcome).toEqual({ status: "rejected" });
  });

  it("returns unexpected and warns on an unknown payload variant", async () => {
    const warnSpy = vi.spyOn(console, "warn").mockImplementation(() => {});
    const mocks: MockedResponse[] = [
      {
        request: { query: AdminCreateRoleMutation, variables: { name: "moderator" } },
        result: { data: { createRole: { __typename: "SomeFutureVariant" } } },
      },
    ];
    const { result } = render(mocks);

    let outcome: unknown;
    await act(async () => {
      outcome = await result.current.createRole({ name: "moderator" });
    });

    expect(outcome).toEqual({ status: "unexpected" });
    expect(warnSpy).toHaveBeenCalledWith(
      expect.stringContaining("unexpected createRole payload"),
      expect.objectContaining({ typename: "SomeFutureVariant" }),
    );
  });
});

describe("useRoleMutations.updateRole", () => {
  it("returns success on UpdateRoleSuccess", async () => {
    const mocks: MockedResponse[] = [
      {
        request: { query: AdminUpdateRoleMutation, variables: { id: "r-1", name: "reviewer" } },
        result: {
          data: {
            updateRole: { __typename: "UpdateRoleSuccess", role: roleNode("r-1", "reviewer") },
          },
        },
      },
    ];
    const { result } = render(mocks);

    let outcome: unknown;
    await act(async () => {
      outcome = await result.current.updateRole("r-1", { name: "Reviewer" });
    });

    expect(outcome).toEqual({ status: "success" });
  });

  it("returns systemRole carrying the server message on CannotModifySystemRoleError", async () => {
    const mocks: MockedResponse[] = [
      {
        request: { query: AdminUpdateRoleMutation, variables: { id: "r-1", name: "reviewer" } },
        result: {
          data: {
            updateRole: {
              __typename: "CannotModifySystemRoleError",
              message: 'cannot rename system role "admin"',
              roleId: "r-1",
              roleName: "admin",
            },
          },
        },
      },
    ];
    const { result } = render(mocks);

    let outcome: unknown;
    await act(async () => {
      outcome = await result.current.updateRole("r-1", { name: "reviewer" });
    });

    expect(outcome).toEqual({
      status: "systemRole",
      message: 'cannot rename system role "admin"',
    });
  });

  it("returns auth(unauthenticated) when the mutation rejects with UNAUTHENTICATED", async () => {
    const mocks: MockedResponse[] = [
      {
        request: { query: AdminUpdateRoleMutation, variables: { id: "r-1", name: "reviewer" } },
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
      outcome = await result.current.updateRole("r-1", { name: "reviewer" });
    });

    expect(outcome).toEqual({ status: "auth", kind: "unauthenticated" });
  });

  it("returns unexpected and warns on an unknown payload variant", async () => {
    const warnSpy = vi.spyOn(console, "warn").mockImplementation(() => {});
    const mocks: MockedResponse[] = [
      {
        request: { query: AdminUpdateRoleMutation, variables: { id: "r-1", name: "reviewer" } },
        result: { data: { updateRole: { __typename: "SomeFutureVariant" } } },
      },
    ];
    const { result } = render(mocks);

    let outcome: unknown;
    await act(async () => {
      outcome = await result.current.updateRole("r-1", { name: "reviewer" });
    });

    expect(outcome).toEqual({ status: "unexpected" });
    expect(warnSpy).toHaveBeenCalledWith(
      expect.stringContaining("unexpected updateRole payload"),
      expect.objectContaining({ typename: "SomeFutureVariant" }),
    );
  });

  it("returns rejected on a transport error", async () => {
    vi.spyOn(console, "warn").mockImplementation(() => {});
    const mocks: MockedResponse[] = [
      {
        request: { query: AdminUpdateRoleMutation, variables: { id: "r-1", name: "reviewer" } },
        error: new Error("network down"),
      },
    ];
    const { result } = render(mocks);

    let outcome: unknown;
    await act(async () => {
      outcome = await result.current.updateRole("r-1", { name: "reviewer" });
    });

    expect(outcome).toEqual({ status: "rejected" });
  });
});

describe("useRoleMutations.deleteRole", () => {
  it("resolves the raw mutation result on success (no typed outcome)", async () => {
    const mocks: MockedResponse[] = [
      {
        request: { query: AdminDeleteRoleMutation, variables: { id: "r-1" } },
        result: { data: { deleteRole: true } },
      },
    ];
    const { result } = render(mocks);

    let res: { data?: { deleteRole?: boolean } | null } | undefined;
    await act(async () => {
      res = await result.current.deleteRole("r-1");
    });

    expect(res?.data?.deleteRole).toBe(true);
  });

  it("rejects so the client's onCommitFailed can classify the error", async () => {
    const mocks: MockedResponse[] = [
      {
        request: { query: AdminDeleteRoleMutation, variables: { id: "r-1" } },
        result: {
          errors: [new GraphQLError("Forbidden", { extensions: { code: "FORBIDDEN" } })],
        },
      },
    ];
    const { result } = render(mocks);

    let caught: unknown;
    await act(async () => {
      caught = await result.current.deleteRole("r-1").catch((err: unknown) => err);
    });

    expect(caught).toBeInstanceOf(Error);
  });
});

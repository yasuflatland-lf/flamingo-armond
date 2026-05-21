// @vitest-environment jsdom
import { MockedProvider } from "@apollo/client/testing/react";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { RoleOption, UserForEdit } from "@/app/admin/users/[id]/edit/admin-user-edit-client";
import { AdminUserEditClient } from "@/app/admin/users/[id]/edit/admin-user-edit-client";
import { AdminAssignRoleDocument, AdminRevokeRoleDocument } from "@/generated/graphql";

// ---------------------------------------------------------------------------
// Fixture IDs
// ---------------------------------------------------------------------------

const USER_ID = "user-1";
const SELF_ADMIN_USER_ID = "admin-self-1";

const ROLE_GENERAL_ID = "role-general";
const ROLE_ADMIN_ID = "role-admin";

// ---------------------------------------------------------------------------
// Shared fixture data
// ---------------------------------------------------------------------------

const ALL_ROLES: RoleOption[] = [
  { id: ROLE_GENERAL_ID, name: "general" },
  { id: ROLE_ADMIN_ID, name: "admin" },
];

function makeUser(userId: string, roleNames: string[]): UserForEdit {
  const roleMap: Record<string, string> = {
    general: ROLE_GENERAL_ID,
    admin: ROLE_ADMIN_ID,
  };
  return {
    id: userId,
    displayName: `User ${userId}`,
    bio: null,
    avatarUrl: null,
    roles: roleNames.map((n) => ({
      id: roleMap[n] ?? `role-${n}`,
      name: n,
    })),
  };
}

// ---------------------------------------------------------------------------
// console spy helpers — mirror admin-dictionary.test.tsx scaffolding
// ---------------------------------------------------------------------------

let consoleWarnSpy: ReturnType<typeof vi.spyOn>;
let consoleErrorSpy: ReturnType<typeof vi.spyOn>;

beforeEach(() => {
  consoleWarnSpy = vi.spyOn(console, "warn").mockImplementation(() => {});
  consoleErrorSpy = vi.spyOn(console, "error").mockImplementation(() => {});
});

afterEach(() => {
  consoleWarnSpy.mockRestore();
  consoleErrorSpy.mockRestore();
});

// ---------------------------------------------------------------------------
// Helper renderer — SSR-props mode (no AdminUserQuery / AdminRolesQuery needed)
// ---------------------------------------------------------------------------

function renderEdit(mocks: object[], user: UserForEdit, allRoles: RoleOption[] = ALL_ROLES) {
  render(
    <MockedProvider mocks={mocks as never}>
      <AdminUserEditClient user={user} allRoles={allRoles} />
    </MockedProvider>,
  );
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

describe("AdminUserEditClient — role assign / revoke flow", () => {
  // T1: Initial render — user has ["general"] role
  it("renders general checkbox checked and admin checkbox unchecked for a user with only general role", async () => {
    renderEdit([], makeUser(USER_ID, ["general"]));

    const generalCheckbox = screen.getByRole("checkbox", { name: /general/i });
    const adminCheckbox = screen.getByRole("checkbox", { name: /admin/i });

    expect(generalCheckbox).toBeChecked();
    expect(adminCheckbox).not.toBeChecked();
  });

  // T2: Assign — click unchecked admin checkbox → AdminAssignRoleMutation called once
  it("calls AdminAssignRoleMutation once and renders admin checkbox as checked after assign", async () => {
    const user = userEvent.setup({ delay: null });

    let assignCalls = 0;
    const assignResult = vi.fn(() => {
      assignCalls += 1;
      return {
        data: {
          assignRole: {
            __typename: "AssignRoleSuccess",
            user: {
              __typename: "User",
              id: USER_ID,
              displayName: `User ${USER_ID}`,
              bio: null,
              avatarUrl: null,
              roles: [
                { __typename: "Role", id: ROLE_GENERAL_ID, name: "general" },
                { __typename: "Role", id: ROLE_ADMIN_ID, name: "admin" },
              ],
            },
          },
        },
      };
    });

    const assignMock = {
      request: {
        query: AdminAssignRoleDocument,
        variables: { userId: USER_ID, roleId: ROLE_ADMIN_ID },
      },
      result: assignResult,
    };

    renderEdit([assignMock], makeUser(USER_ID, ["general"]));

    const adminCheckbox = screen.getByRole("checkbox", { name: /admin/i });
    expect(adminCheckbox).not.toBeChecked();

    // Click to assign admin role
    await user.click(adminCheckbox);

    // Mutation should have fired exactly once
    await waitFor(() => expect(assignCalls).toBe(1));

    // After mutation resolution, admin checkbox is checked (from server-truth response)
    await waitFor(() => {
      expect(screen.getByRole("checkbox", { name: /admin/i })).toBeChecked();
    });
  });

  // T3: Revoke — user starts with ["general", "admin"], click general to revoke it
  it("calls AdminRevokeRoleMutation once and renders general checkbox as unchecked after revoke", async () => {
    const user = userEvent.setup({ delay: null });

    let revokeCalls = 0;
    const revokeResult = vi.fn(() => {
      revokeCalls += 1;
      return {
        data: {
          revokeRole: {
            __typename: "RevokeRoleSuccess",
            user: {
              __typename: "User",
              id: USER_ID,
              displayName: `User ${USER_ID}`,
              bio: null,
              avatarUrl: null,
              roles: [{ __typename: "Role", id: ROLE_ADMIN_ID, name: "admin" }],
            },
          },
        },
      };
    });

    const revokeMock = {
      request: {
        query: AdminRevokeRoleDocument,
        variables: { userId: USER_ID, roleId: ROLE_GENERAL_ID },
      },
      result: revokeResult,
    };

    renderEdit([revokeMock], makeUser(USER_ID, ["general", "admin"]));

    // Both checkboxes should be checked
    const generalCheckbox = screen.getByRole("checkbox", { name: /general/i });
    expect(generalCheckbox).toBeChecked();
    expect(screen.getByRole("checkbox", { name: /admin/i })).toBeChecked();

    // Click to revoke general role
    await user.click(generalCheckbox);

    // Mutation should have fired exactly once
    await waitFor(() => expect(revokeCalls).toBe(1));

    // After mutation resolution, general checkbox is unchecked (server-truth response)
    await waitFor(() => {
      expect(screen.getByRole("checkbox", { name: /general/i })).not.toBeChecked();
    });
  });

  // T4: Self-demotion guard — typed CannotRevokeOwnAdminRoleError variant keeps
  //     checkbox checked (server truth) and shows the banner with .message.
  //     Because there is no optimistic response, the checkbox never flips; its
  //     state after the error matches the server-truth initial state (checked).
  it("shows CannotRevokeOwnAdminRoleError banner and keeps admin checkbox checked when revoking own admin role", async () => {
    const user = userEvent.setup({ delay: null });

    const selfDemotionRevokeMock = {
      request: {
        query: AdminRevokeRoleDocument,
        variables: { userId: SELF_ADMIN_USER_ID, roleId: ROLE_ADMIN_ID },
      },
      result: {
        data: {
          revokeRole: {
            __typename: "CannotRevokeOwnAdminRoleError",
            message: "cannot revoke own admin role",
          },
        },
      },
    };

    renderEdit([selfDemotionRevokeMock], makeUser(SELF_ADMIN_USER_ID, ["admin"]));

    // Admin checkbox should be checked initially
    const adminCheckbox = screen.getByRole("checkbox", { name: /admin/i });
    expect(adminCheckbox).toBeChecked();

    // Attempt to uncheck (revoke own admin role)
    await user.click(adminCheckbox);

    // Banner with typed-variant message must appear
    const banner = await screen.findByRole("alert");
    expect(banner).toHaveTextContent("cannot revoke own admin role");

    // Checkbox must still be checked — server truth was never changed (no optimistic write)
    await waitFor(() => {
      expect(screen.getByRole("checkbox", { name: /admin/i })).toBeChecked();
    });
  });

  // T5: MockedProvider leak — no "no more mocked responses" warning for admin mutations
  it("does not produce MockedProvider leak warnings for admin role mutations across all test interactions", async () => {
    // This test fires one assign and verifies no duplicate-request warnings.
    const user = userEvent.setup({ delay: null });

    const assignMock = {
      request: {
        query: AdminAssignRoleDocument,
        variables: { userId: USER_ID, roleId: ROLE_ADMIN_ID },
      },
      result: {
        data: {
          assignRole: {
            __typename: "AssignRoleSuccess",
            user: {
              __typename: "User",
              id: USER_ID,
              displayName: `User ${USER_ID}`,
              bio: null,
              avatarUrl: null,
              roles: [
                { __typename: "Role", id: ROLE_GENERAL_ID, name: "general" },
                { __typename: "Role", id: ROLE_ADMIN_ID, name: "admin" },
              ],
            },
          },
        },
      },
    };

    renderEdit([assignMock], makeUser(USER_ID, ["general"]));

    const adminCheckbox = screen.getByRole("checkbox", { name: /admin/i });
    await user.click(adminCheckbox);

    // Allow all async work to settle
    await waitFor(() => {
      expect(screen.getByRole("checkbox", { name: /admin/i })).toBeChecked();
    });

    // Verify no "No more mocked responses" warnings were emitted for the admin mutations
    const adminMutationLeakWarnings = consoleWarnSpy.mock.calls.filter((args: unknown[]) =>
      args.some(
        (arg: unknown) =>
          typeof arg === "string" &&
          (arg.includes("AdminAssignRole") || arg.includes("AdminRevokeRole")),
      ),
    );
    expect(adminMutationLeakWarnings).toEqual([]);
  });
});

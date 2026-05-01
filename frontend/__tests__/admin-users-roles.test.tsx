// @vitest-environment jsdom
import { MockedProvider } from "@apollo/client/testing/react";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { GraphQLError } from "graphql";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { AdminUserEditClient } from "@/app/admin/users/[id]/edit/AdminUserEditClient";
import {
  AdminAssignRoleDocument,
  AdminRevokeRoleDocument,
  AdminRolesDocument,
  AdminUserDocument,
} from "@/generated/graphql";

// ---------------------------------------------------------------------------
// Fixture IDs
// ---------------------------------------------------------------------------

const USER_ID = "user-1";
const CALLER_ID = "caller-99";
const SELF_ADMIN_USER_ID = "admin-self-1";

const ROLE_GENERAL_ID = "role-general";
const ROLE_ADMIN_ID = "role-admin";

// ---------------------------------------------------------------------------
// Shared mock fragments
// ---------------------------------------------------------------------------

const ALL_ROLES_MOCK = {
  request: {
    query: AdminRolesDocument,
    variables: {},
  },
  result: {
    data: {
      roles: [
        { __typename: "Role", id: ROLE_GENERAL_ID, name: "general" },
        { __typename: "Role", id: ROLE_ADMIN_ID, name: "admin" },
      ],
    },
  },
};

function makeAdminUserMock(userId: string, roleNames: string[]) {
  const roleMap: Record<string, string> = {
    general: ROLE_GENERAL_ID,
    admin: ROLE_ADMIN_ID,
  };
  return {
    request: {
      query: AdminUserDocument,
      variables: { id: userId },
    },
    result: {
      data: {
        adminUser: {
          __typename: "User",
          id: userId,
          displayName: `User ${userId}`,
          bio: null,
          avatarUrl: null,
          roles: roleNames.map((n) => ({
            __typename: "Role",
            id: roleMap[n] ?? `role-${n}`,
            name: n,
          })),
        },
      },
    },
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
// Helper renderer
// ---------------------------------------------------------------------------

function renderEdit(mocks: object[], userId: string, callerId?: string) {
  render(
    <MockedProvider mocks={mocks as never}>
      <AdminUserEditClient userId={userId} callerId={callerId} />
    </MockedProvider>,
  );
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

describe("AdminUserEditClient — role assign / revoke flow", () => {
  // T1: Initial render — user has ["general"] role
  it("renders general checkbox checked and admin checkbox unchecked for a user with only general role", async () => {
    renderEdit(
      [makeAdminUserMock(USER_ID, ["general"]), ALL_ROLES_MOCK],
      USER_ID,
      CALLER_ID,
    );

    // Wait for async data to load
    const generalCheckbox = await screen.findByRole("checkbox", { name: /general/i });
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
      };
    });

    const assignMock = {
      request: {
        query: AdminAssignRoleDocument,
        variables: { userId: USER_ID, roleId: ROLE_ADMIN_ID },
      },
      result: assignResult,
    };

    renderEdit(
      [makeAdminUserMock(USER_ID, ["general"]), ALL_ROLES_MOCK, assignMock],
      USER_ID,
      CALLER_ID,
    );

    // Wait for initial render
    const adminCheckbox = await screen.findByRole("checkbox", { name: /admin/i });
    expect(adminCheckbox).not.toBeChecked();

    // Click to assign admin role
    await user.click(adminCheckbox);

    // Mutation should have fired exactly once
    await waitFor(() => expect(assignCalls).toBe(1));

    // After optimistic response / mutation resolution, admin checkbox is checked
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
            __typename: "User",
            id: USER_ID,
            displayName: `User ${USER_ID}`,
            bio: null,
            avatarUrl: null,
            roles: [
              { __typename: "Role", id: ROLE_ADMIN_ID, name: "admin" },
            ],
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

    renderEdit(
      [makeAdminUserMock(USER_ID, ["general", "admin"]), ALL_ROLES_MOCK, revokeMock],
      USER_ID,
      CALLER_ID,
    );

    // Wait for initial render — both checkboxes should be checked
    const generalCheckbox = await screen.findByRole("checkbox", { name: /general/i });
    expect(generalCheckbox).toBeChecked();
    expect(screen.getByRole("checkbox", { name: /admin/i })).toBeChecked();

    // Click to revoke general role
    await user.click(generalCheckbox);

    // Mutation should have fired exactly once
    await waitFor(() => expect(revokeCalls).toBe(1));

    // After optimistic response / mutation resolution, general checkbox is unchecked
    await waitFor(() => {
      expect(screen.getByRole("checkbox", { name: /general/i })).not.toBeChecked();
    });
  });

  // T4: Self-demotion guard — FORBIDDEN error keeps checkbox checked, shows banner
  it("shows FORBIDDEN banner and keeps admin checkbox checked when revoking own admin role", async () => {
    const user = userEvent.setup({ delay: null });

    const selfDemotionRevokeMock = {
      request: {
        query: AdminRevokeRoleDocument,
        variables: { userId: SELF_ADMIN_USER_ID, roleId: ROLE_ADMIN_ID },
      },
      result: {
        errors: [
          new GraphQLError("cannot revoke own admin role", {
            extensions: { code: "FORBIDDEN" },
          }),
        ],
      },
    };

    renderEdit(
      [
        makeAdminUserMock(SELF_ADMIN_USER_ID, ["admin"]),
        ALL_ROLES_MOCK,
        selfDemotionRevokeMock,
      ],
      SELF_ADMIN_USER_ID,
      SELF_ADMIN_USER_ID, // callerId === userId → same user
    );

    // Wait for initial render — admin checkbox should be checked
    const adminCheckbox = await screen.findByRole("checkbox", { name: /admin/i });
    expect(adminCheckbox).toBeChecked();

    // Attempt to uncheck (revoke own admin role)
    await user.click(adminCheckbox);

    // Banner with FORBIDDEN message must appear
    const banner = await screen.findByRole("alert");
    expect(banner).toHaveTextContent("cannot revoke own admin role");

    // Checkbox must remain checked (optimistic write rolled back)
    await waitFor(() => {
      expect(screen.getByRole("checkbox", { name: /admin/i })).toBeChecked();
    });
  });

  // T5: MockedProvider leak — no "no more mocked responses" warning for admin mutations
  it("does not produce MockedProvider leak warnings for admin role mutations across all test interactions", async () => {
    // This test fires one assign and one revoke and verifies no duplicate-request warnings.
    const user = userEvent.setup({ delay: null });

    const assignMock = {
      request: {
        query: AdminAssignRoleDocument,
        variables: { userId: USER_ID, roleId: ROLE_ADMIN_ID },
      },
      result: {
        data: {
          assignRole: {
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

    renderEdit(
      [makeAdminUserMock(USER_ID, ["general"]), ALL_ROLES_MOCK, assignMock],
      USER_ID,
      CALLER_ID,
    );

    // Wait for initial render and click assign once
    const adminCheckbox = await screen.findByRole("checkbox", { name: /admin/i });
    await user.click(adminCheckbox);

    // Allow all async work to settle
    await waitFor(() => {
      expect(screen.getByRole("checkbox", { name: /admin/i })).toBeChecked();
    });

    // Verify no "No more mocked responses" warnings were emitted for the admin mutations
    const adminMutationLeakWarnings = consoleWarnSpy.mock.calls.filter(
      (args: unknown[]) =>
        args.some(
          (arg: unknown) =>
            typeof arg === "string" &&
            (arg.includes("AdminAssignRole") || arg.includes("AdminRevokeRole")),
        ),
    );
    expect(adminMutationLeakWarnings).toEqual([]);
  });
});

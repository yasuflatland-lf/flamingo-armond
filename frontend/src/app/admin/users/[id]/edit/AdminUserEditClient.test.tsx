// @vitest-environment jsdom

import { MockedProvider } from "@apollo/client/testing/react";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";

import {
  AdminAssignRoleDocument,
  AdminRevokeRoleDocument,
  AdminUpdateUserDocument,
} from "@/generated/graphql";
import { AdminUserEditClient, type RoleOption, type UserForEdit } from "./AdminUserEditClient";

vi.mock("next/link", () => ({
  default: ({ href, children, ...rest }: { href: string; children: React.ReactNode }) => (
    <a href={href} {...rest}>
      {children}
    </a>
  ),
}));

const ADMIN_ROLE: RoleOption = { id: "r-admin", name: "admin" };
const MOD_ROLE: RoleOption = { id: "r-mod", name: "moderator" };

function makeUser(overrides: Partial<UserForEdit> = {}): UserForEdit {
  return {
    id: "u-1",
    displayName: "Alice",
    bio: "bio text",
    avatarUrl: null,
    roles: [{ id: ADMIN_ROLE.id, name: ADMIN_ROLE.name }],
    ...overrides,
  };
}

describe("AdminUserEditClient", () => {
  // Capture console.warn for MockedProvider leak detection — see
  // .claude/rules/pagination.md § "Capture console.warn for MockedProvider
  // leaks, then assert in teardown".
  let warnSpy: ReturnType<typeof vi.spyOn> | null = null;
  let errorSpy: ReturnType<typeof vi.spyOn> | null = null;

  afterEach(() => {
    warnSpy?.mockRestore();
    warnSpy = null;
    errorSpy?.mockRestore();
    errorSpy = null;
  });

  it("renders the heading and the role list", () => {
    render(
      <MockedProvider mocks={[]}>
        <AdminUserEditClient user={makeUser()} allRoles={[ADMIN_ROLE, MOD_ROLE]} />
      </MockedProvider>,
    );
    expect(screen.getByRole("heading", { name: /edit user/i })).toBeInTheDocument();
    expect(screen.getByRole("checkbox", { name: /admin/i })).toBeChecked();
    expect(screen.getByRole("checkbox", { name: /moderator/i })).not.toBeChecked();
    expect(screen.getByRole("link", { name: /cancel/i })).toHaveAttribute("href", "/admin/users");
  });

  // ---------- handleSave (adminUpdateUser) -----------------------------------

  describe("handleSave (adminUpdateUser)", () => {
    it("AdminUpdateUserSuccess — shows success banner", async () => {
      const user = userEvent.setup();
      const mocks = [
        {
          request: {
            query: AdminUpdateUserDocument,
            variables: {
              id: "u-1",
              input: { displayName: "Alice 2", bio: "bio text" },
            },
          },
          result: () => ({
            data: {
              adminUpdateUser: {
                __typename: "AdminUpdateUserSuccess" as const,
                user: {
                  __typename: "User" as const,
                  id: "u-1",
                  displayName: "Alice 2",
                  bio: "bio text",
                  avatarUrl: null,
                  roles: [{ __typename: "Role" as const, id: ADMIN_ROLE.id, name: "admin" }],
                },
              },
            },
          }),
        },
      ];

      render(
        <MockedProvider mocks={mocks}>
          <AdminUserEditClient user={makeUser()} allRoles={[ADMIN_ROLE]} />
        </MockedProvider>,
      );

      const input = screen.getByLabelText(/display name/i);
      await user.clear(input);
      await user.type(input, "Alice 2");
      await user.click(screen.getByRole("button", { name: /save changes/i }));

      await waitFor(() => {
        expect(screen.getByRole("status")).toHaveTextContent(/changes saved/i);
      });
      expect(screen.queryByRole("alert")).not.toBeInTheDocument();
    });

    it("InputValidationError — renders inline banner with message", async () => {
      const user = userEvent.setup();
      const mocks = [
        {
          request: {
            query: AdminUpdateUserDocument,
            variables: {
              id: "u-1",
              input: { displayName: "Alice 2", bio: "bio text" },
            },
          },
          result: () => ({
            data: {
              adminUpdateUser: {
                __typename: "InputValidationError" as const,
                field: "displayName",
                message: "display name is taken",
              },
            },
          }),
        },
      ];

      render(
        <MockedProvider mocks={mocks}>
          <AdminUserEditClient user={makeUser()} allRoles={[ADMIN_ROLE]} />
        </MockedProvider>,
      );

      const input = screen.getByLabelText(/display name/i);
      await user.clear(input);
      await user.type(input, "Alice 2");
      await user.click(screen.getByRole("button", { name: /save changes/i }));

      await waitFor(() => {
        expect(screen.getByRole("alert")).toHaveTextContent("display name is taken");
      });
      // No success banner.
      expect(screen.queryByRole("status")).not.toBeInTheDocument();
    });

    it("FORBIDDEN transport rejection — shows permission banner", async () => {
      const user = userEvent.setup();
      const forbiddenError = Object.assign(new Error("transport"), {
        graphQLErrors: [{ extensions: { code: "FORBIDDEN" } }],
      });
      const mocks = [
        {
          request: {
            query: AdminUpdateUserDocument,
            variables: {
              id: "u-1",
              input: { displayName: "Alice 2", bio: "bio text" },
            },
          },
          error: forbiddenError,
        },
      ];

      // The Apollo error path emits a console.error inside the React render
      // pipeline; silence it so test output stays clean.
      errorSpy = vi.spyOn(console, "error").mockImplementation(() => {});
      render(
        <MockedProvider mocks={mocks}>
          <AdminUserEditClient user={makeUser()} allRoles={[ADMIN_ROLE]} />
        </MockedProvider>,
      );

      const input = screen.getByLabelText(/display name/i);
      await user.clear(input);
      await user.type(input, "Alice 2");
      await user.click(screen.getByRole("button", { name: /save changes/i }));

      await waitFor(() => {
        expect(screen.getByRole("alert")).toHaveTextContent(/do not have permission/i);
      });
    });

    it("UNAUTHENTICATED transport rejection — shows session-expired banner", async () => {
      const user = userEvent.setup();
      const authError = Object.assign(new Error("transport"), {
        graphQLErrors: [{ extensions: { code: "UNAUTHENTICATED" } }],
      });
      const mocks = [
        {
          request: {
            query: AdminUpdateUserDocument,
            variables: {
              id: "u-1",
              input: { displayName: "Alice 2", bio: "bio text" },
            },
          },
          error: authError,
        },
      ];

      errorSpy = vi.spyOn(console, "error").mockImplementation(() => {});
      render(
        <MockedProvider mocks={mocks}>
          <AdminUserEditClient user={makeUser()} allRoles={[ADMIN_ROLE]} />
        </MockedProvider>,
      );

      const input = screen.getByLabelText(/display name/i);
      await user.clear(input);
      await user.type(input, "Alice 2");
      await user.click(screen.getByRole("button", { name: /save changes/i }));

      await waitFor(() => {
        expect(screen.getByRole("alert")).toHaveTextContent(/session has expired/i);
      });
    });

    it("unknown __typename — warns and shows degraded banner", async () => {
      const user = userEvent.setup();
      const mocks = [
        {
          request: {
            query: AdminUpdateUserDocument,
            variables: {
              id: "u-1",
              input: { displayName: "Alice 2", bio: "bio text" },
            },
          },
          result: () => ({
            data: {
              adminUpdateUser: {
                __typename: "FutureVariantClientDidNotKnowAbout",
              } as never,
            },
          }),
        },
      ];

      warnSpy = vi.spyOn(console, "warn").mockImplementation(() => {});
      render(
        <MockedProvider mocks={mocks}>
          <AdminUserEditClient user={makeUser()} allRoles={[ADMIN_ROLE]} />
        </MockedProvider>,
      );

      const input = screen.getByLabelText(/display name/i);
      await user.clear(input);
      await user.type(input, "Alice 2");
      await user.click(screen.getByRole("button", { name: /save changes/i }));

      await waitFor(() => expect(warnSpy).toHaveBeenCalled());

      expect(screen.getByRole("alert")).toHaveTextContent(/something went wrong/i);
      expect(warnSpy).toHaveBeenCalledWith(
        expect.stringContaining("unexpected save payload"),
        expect.objectContaining({ typename: "FutureVariantClientDidNotKnowAbout" }),
      );
    });

    it("null adminUpdateUser payload — warns and shows degraded banner", async () => {
      const user = userEvent.setup();
      const mocks = [
        {
          request: {
            query: AdminUpdateUserDocument,
            variables: {
              id: "u-1",
              input: { displayName: "Alice 2", bio: "bio text" },
            },
          },
          result: () => ({ data: { adminUpdateUser: null as never } }),
        },
      ];

      warnSpy = vi.spyOn(console, "warn").mockImplementation(() => {});
      render(
        <MockedProvider mocks={mocks}>
          <AdminUserEditClient user={makeUser()} allRoles={[ADMIN_ROLE]} />
        </MockedProvider>,
      );

      const input = screen.getByLabelText(/display name/i);
      await user.clear(input);
      await user.type(input, "Alice 2");
      await user.click(screen.getByRole("button", { name: /save changes/i }));

      await waitFor(() => expect(warnSpy).toHaveBeenCalled());
      expect(screen.getByRole("alert")).toHaveTextContent(/something went wrong/i);
      expect(warnSpy).toHaveBeenCalledWith(
        expect.stringContaining("unexpected save payload"),
        expect.objectContaining({ typename: null }),
      );
    });
  });

  // ---------- handleRoleToggle (assignRole) ----------------------------------

  describe("handleRoleToggle assignRole", () => {
    it("AssignRoleSuccess — updates local role state", async () => {
      const user = userEvent.setup();
      const mocks = [
        {
          request: {
            query: AdminAssignRoleDocument,
            variables: { userId: "u-1", roleId: MOD_ROLE.id },
          },
          result: () => ({
            data: {
              assignRole: {
                __typename: "AssignRoleSuccess" as const,
                user: {
                  __typename: "User" as const,
                  id: "u-1",
                  displayName: "Alice",
                  bio: "bio text",
                  avatarUrl: null,
                  roles: [
                    { __typename: "Role" as const, id: ADMIN_ROLE.id, name: "admin" },
                    { __typename: "Role" as const, id: MOD_ROLE.id, name: "moderator" },
                  ],
                },
              },
            },
          }),
        },
      ];

      render(
        <MockedProvider mocks={mocks}>
          <AdminUserEditClient user={makeUser()} allRoles={[ADMIN_ROLE, MOD_ROLE]} />
        </MockedProvider>,
      );

      await user.click(screen.getByRole("checkbox", { name: /moderator/i }));

      await waitFor(() => {
        expect(screen.getByRole("checkbox", { name: /moderator/i })).toBeChecked();
      });
    });

    it("InputValidationError — renders inline banner with .message", async () => {
      const user = userEvent.setup();
      const mocks = [
        {
          request: {
            query: AdminAssignRoleDocument,
            variables: { userId: "u-1", roleId: MOD_ROLE.id },
          },
          result: () => ({
            data: {
              assignRole: {
                __typename: "InputValidationError" as const,
                field: "roleId",
                message: "role does not exist",
              },
            },
          }),
        },
      ];

      render(
        <MockedProvider mocks={mocks}>
          <AdminUserEditClient user={makeUser()} allRoles={[ADMIN_ROLE, MOD_ROLE]} />
        </MockedProvider>,
      );

      await user.click(screen.getByRole("checkbox", { name: /moderator/i }));

      await waitFor(() => {
        expect(screen.getByRole("alert")).toHaveTextContent("role does not exist");
      });
      // Local role state unchanged.
      expect(screen.getByRole("checkbox", { name: /moderator/i })).not.toBeChecked();
    });

    it("FORBIDDEN transport rejection — shows permission banner per role", async () => {
      const user = userEvent.setup();
      const forbiddenError = Object.assign(new Error("transport"), {
        graphQLErrors: [{ extensions: { code: "FORBIDDEN" } }],
      });
      const mocks = [
        {
          request: {
            query: AdminAssignRoleDocument,
            variables: { userId: "u-1", roleId: MOD_ROLE.id },
          },
          error: forbiddenError,
        },
      ];

      errorSpy = vi.spyOn(console, "error").mockImplementation(() => {});
      render(
        <MockedProvider mocks={mocks}>
          <AdminUserEditClient user={makeUser()} allRoles={[ADMIN_ROLE, MOD_ROLE]} />
        </MockedProvider>,
      );

      await user.click(screen.getByRole("checkbox", { name: /moderator/i }));

      await waitFor(() => {
        expect(screen.getByRole("alert")).toHaveTextContent(/do not have permission/i);
      });
    });

    it("UNAUTHENTICATED transport rejection — shows session-expired banner per role", async () => {
      const user = userEvent.setup();
      const authError = Object.assign(new Error("transport"), {
        graphQLErrors: [{ extensions: { code: "UNAUTHENTICATED" } }],
      });
      const mocks = [
        {
          request: {
            query: AdminAssignRoleDocument,
            variables: { userId: "u-1", roleId: MOD_ROLE.id },
          },
          error: authError,
        },
      ];

      errorSpy = vi.spyOn(console, "error").mockImplementation(() => {});
      render(
        <MockedProvider mocks={mocks}>
          <AdminUserEditClient user={makeUser()} allRoles={[ADMIN_ROLE, MOD_ROLE]} />
        </MockedProvider>,
      );

      await user.click(screen.getByRole("checkbox", { name: /moderator/i }));

      await waitFor(() => {
        expect(screen.getByRole("alert")).toHaveTextContent(/session has expired/i);
      });
    });

    it("unknown __typename — warns and shows degraded banner", async () => {
      const user = userEvent.setup();
      const mocks = [
        {
          request: {
            query: AdminAssignRoleDocument,
            variables: { userId: "u-1", roleId: MOD_ROLE.id },
          },
          result: () => ({
            data: {
              assignRole: { __typename: "FutureVariantClientDidNotKnowAbout" } as never,
            },
          }),
        },
      ];

      warnSpy = vi.spyOn(console, "warn").mockImplementation(() => {});
      render(
        <MockedProvider mocks={mocks}>
          <AdminUserEditClient user={makeUser()} allRoles={[ADMIN_ROLE, MOD_ROLE]} />
        </MockedProvider>,
      );

      await user.click(screen.getByRole("checkbox", { name: /moderator/i }));

      await waitFor(() => expect(warnSpy).toHaveBeenCalled());
      expect(screen.getByRole("alert")).toHaveTextContent(/something went wrong/i);
      expect(warnSpy).toHaveBeenCalledWith(
        expect.stringContaining("unexpected role-toggle payload"),
        expect.objectContaining({ typename: "FutureVariantClientDidNotKnowAbout" }),
      );
    });

    it("null assignRole payload — warns and shows degraded banner", async () => {
      const user = userEvent.setup();
      const mocks = [
        {
          request: {
            query: AdminAssignRoleDocument,
            variables: { userId: "u-1", roleId: MOD_ROLE.id },
          },
          result: () => ({ data: { assignRole: null as never } }),
        },
      ];

      warnSpy = vi.spyOn(console, "warn").mockImplementation(() => {});
      render(
        <MockedProvider mocks={mocks}>
          <AdminUserEditClient user={makeUser()} allRoles={[ADMIN_ROLE, MOD_ROLE]} />
        </MockedProvider>,
      );

      await user.click(screen.getByRole("checkbox", { name: /moderator/i }));

      await waitFor(() => expect(warnSpy).toHaveBeenCalled());
      expect(screen.getByRole("alert")).toHaveTextContent(/something went wrong/i);
      expect(warnSpy).toHaveBeenCalledWith(
        expect.stringContaining("unexpected role-toggle payload"),
        expect.objectContaining({ typename: null }),
      );
    });
  });

  // ---------- handleRoleToggle (revokeRole) ----------------------------------

  describe("handleRoleToggle revokeRole", () => {
    it("RevokeRoleSuccess — updates local role state", async () => {
      const user = userEvent.setup();
      // Start with admin assigned; revoke it.
      const mocks = [
        {
          request: {
            query: AdminRevokeRoleDocument,
            variables: { userId: "u-1", roleId: ADMIN_ROLE.id },
          },
          result: () => ({
            data: {
              revokeRole: {
                __typename: "RevokeRoleSuccess" as const,
                user: {
                  __typename: "User" as const,
                  id: "u-1",
                  displayName: "Alice",
                  bio: "bio text",
                  avatarUrl: null,
                  roles: [],
                },
              },
            },
          }),
        },
      ];

      render(
        <MockedProvider mocks={mocks}>
          <AdminUserEditClient
            user={makeUser({ roles: [{ id: ADMIN_ROLE.id, name: "admin" }] })}
            allRoles={[ADMIN_ROLE, MOD_ROLE]}
          />
        </MockedProvider>,
      );

      await user.click(screen.getByRole("checkbox", { name: /admin/i }));

      await waitFor(() => {
        expect(screen.getByRole("checkbox", { name: /admin/i })).not.toBeChecked();
      });
    });

    it("CannotRevokeOwnAdminRoleError — renders inline banner with .message", async () => {
      const user = userEvent.setup();
      const mocks = [
        {
          request: {
            query: AdminRevokeRoleDocument,
            variables: { userId: "u-1", roleId: ADMIN_ROLE.id },
          },
          result: () => ({
            data: {
              revokeRole: {
                __typename: "CannotRevokeOwnAdminRoleError" as const,
                message: "cannot revoke your own admin role",
              },
            },
          }),
        },
      ];

      render(
        <MockedProvider mocks={mocks}>
          <AdminUserEditClient
            user={makeUser({ roles: [{ id: ADMIN_ROLE.id, name: "admin" }] })}
            allRoles={[ADMIN_ROLE]}
          />
        </MockedProvider>,
      );

      await user.click(screen.getByRole("checkbox", { name: /admin/i }));

      await waitFor(() => {
        expect(screen.getByRole("alert")).toHaveTextContent("cannot revoke your own admin role");
      });
      // Local state unchanged.
      expect(screen.getByRole("checkbox", { name: /admin/i })).toBeChecked();
    });

    it("InputValidationError on revoke — renders inline banner", async () => {
      const user = userEvent.setup();
      const mocks = [
        {
          request: {
            query: AdminRevokeRoleDocument,
            variables: { userId: "u-1", roleId: ADMIN_ROLE.id },
          },
          result: () => ({
            data: {
              revokeRole: {
                __typename: "InputValidationError" as const,
                field: "roleId",
                message: "user does not have this role",
              },
            },
          }),
        },
      ];

      render(
        <MockedProvider mocks={mocks}>
          <AdminUserEditClient
            user={makeUser({ roles: [{ id: ADMIN_ROLE.id, name: "admin" }] })}
            allRoles={[ADMIN_ROLE]}
          />
        </MockedProvider>,
      );

      await user.click(screen.getByRole("checkbox", { name: /admin/i }));

      await waitFor(() => {
        expect(screen.getByRole("alert")).toHaveTextContent("user does not have this role");
      });
    });

    it("unknown __typename on revoke — warns and shows degraded banner", async () => {
      const user = userEvent.setup();
      const mocks = [
        {
          request: {
            query: AdminRevokeRoleDocument,
            variables: { userId: "u-1", roleId: ADMIN_ROLE.id },
          },
          result: () => ({
            data: {
              revokeRole: { __typename: "FutureVariantClientDidNotKnowAbout" } as never,
            },
          }),
        },
      ];

      warnSpy = vi.spyOn(console, "warn").mockImplementation(() => {});
      render(
        <MockedProvider mocks={mocks}>
          <AdminUserEditClient
            user={makeUser({ roles: [{ id: ADMIN_ROLE.id, name: "admin" }] })}
            allRoles={[ADMIN_ROLE]}
          />
        </MockedProvider>,
      );

      await user.click(screen.getByRole("checkbox", { name: /admin/i }));

      await waitFor(() => expect(warnSpy).toHaveBeenCalled());
      expect(screen.getByRole("alert")).toHaveTextContent(/something went wrong/i);
      expect(warnSpy).toHaveBeenCalledWith(
        expect.stringContaining("unexpected role-toggle payload"),
        expect.objectContaining({ typename: "FutureVariantClientDidNotKnowAbout" }),
      );
    });

    it("null revokeRole payload — warns and shows degraded banner", async () => {
      const user = userEvent.setup();
      const mocks = [
        {
          request: {
            query: AdminRevokeRoleDocument,
            variables: { userId: "u-1", roleId: ADMIN_ROLE.id },
          },
          result: () => ({ data: { revokeRole: null as never } }),
        },
      ];

      warnSpy = vi.spyOn(console, "warn").mockImplementation(() => {});
      render(
        <MockedProvider mocks={mocks}>
          <AdminUserEditClient
            user={makeUser({ roles: [{ id: ADMIN_ROLE.id, name: "admin" }] })}
            allRoles={[ADMIN_ROLE]}
          />
        </MockedProvider>,
      );

      await user.click(screen.getByRole("checkbox", { name: /admin/i }));

      await waitFor(() => expect(warnSpy).toHaveBeenCalled());
      expect(screen.getByRole("alert")).toHaveTextContent(/something went wrong/i);
      expect(warnSpy).toHaveBeenCalledWith(
        expect.stringContaining("unexpected role-toggle payload"),
        expect.objectContaining({ typename: null }),
      );
    });
  });
});

// @vitest-environment jsdom

import { CombinedGraphQLErrors } from "@apollo/client/errors";
import { MockedProvider } from "@apollo/client/testing/react";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import { AdminAssignRoleDocument, AdminRevokeRoleDocument } from "@/generated/graphql";
import {
  type AdminUserListItem,
  type AdminUserRole,
  AdminUserRoleRow,
} from "./admin-user-role-row";

vi.mock("next/image", () => ({
  default: (props: { src: string; alt: string; width: number; height: number }) => (
    // biome-ignore lint/performance/noImgElement: jsdom-friendly stand-in for next/image
    // biome-ignore lint/a11y/useAltText: alt is forwarded from props
    <img {...props} />
  ),
}));

function makeCodedError(code: string): CombinedGraphQLErrors {
  return new CombinedGraphQLErrors({
    errors: [{ message: "transport", extensions: { code } }],
  });
}

const ADMIN_ROLE: AdminUserRole = { id: "r-admin", name: "admin" };
const MOD_ROLE: AdminUserRole = { id: "r-mod", name: "moderator" };

function makeUser(overrides: Partial<AdminUserListItem> = {}): AdminUserListItem {
  return {
    id: "u-1",
    displayName: "Alice",
    bio: "bio text",
    avatarUrl: null,
    roles: [ADMIN_ROLE],
    ...overrides,
  };
}

function renderRow({
  user = makeUser(),
  allRoles = [ADMIN_ROLE, MOD_ROLE],
  mocks = [],
  onEdit = vi.fn(),
}: {
  user?: AdminUserListItem;
  allRoles?: AdminUserRole[];
  mocks?: React.ComponentProps<typeof MockedProvider>["mocks"];
  onEdit?: (id: string) => void;
} = {}) {
  render(
    <MockedProvider mocks={mocks}>
      <ul>
        <AdminUserRoleRow user={user} allRoles={allRoles} onEdit={onEdit} />
      </ul>
    </MockedProvider>,
  );
  return { onEdit };
}

describe("AdminUserRoleRow", () => {
  let warnSpy: ReturnType<typeof vi.spyOn> | null = null;
  let errorSpy: ReturnType<typeof vi.spyOn> | null = null;

  afterEach(() => {
    warnSpy?.mockRestore();
    warnSpy = null;
    errorSpy?.mockRestore();
    errorSpy = null;
  });

  it("renders identity, inline roles, and an edit affordance", async () => {
    const user = userEvent.setup();
    const onEdit = vi.fn();
    renderRow({ onEdit });

    expect(screen.getByText("Alice")).toBeInTheDocument();
    expect(screen.getByText("bio text")).toBeInTheDocument();
    expect(screen.getByRole("checkbox", { name: /admin/i })).toBeChecked();
    expect(screen.getByRole("checkbox", { name: /moderator/i })).not.toBeChecked();

    await user.click(screen.getByRole("button", { name: /edit alice/i }));

    expect(onEdit).toHaveBeenCalledWith("u-1");
  });

  it("AssignRoleSuccess updates local role state", async () => {
    const user = userEvent.setup();
    const mocks = [
      {
        request: {
          query: AdminAssignRoleDocument,
          variables: { userId: "u-1", roleId: MOD_ROLE.id },
        },
        result: {
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
        },
      },
    ];

    renderRow({ mocks });

    await user.click(screen.getByRole("checkbox", { name: /moderator/i }));

    await waitFor(() => {
      expect(screen.getByRole("checkbox", { name: /moderator/i })).toBeChecked();
    });
  });

  it("InputValidationError on assign renders a per-role banner", async () => {
    const user = userEvent.setup();
    const mocks = [
      {
        request: {
          query: AdminAssignRoleDocument,
          variables: { userId: "u-1", roleId: MOD_ROLE.id },
        },
        result: {
          data: {
            assignRole: {
              __typename: "InputValidationError" as const,
              field: "roleId",
              message: "role does not exist",
            },
          },
        },
      },
    ];

    renderRow({ mocks });

    await user.click(screen.getByRole("checkbox", { name: /moderator/i }));

    await waitFor(() => {
      expect(screen.getByRole("alert")).toHaveTextContent("role does not exist");
    });
    expect(screen.getByRole("checkbox", { name: /moderator/i })).not.toBeChecked();
  });

  it("FORBIDDEN transport rejection shows permission copy per role", async () => {
    const user = userEvent.setup();
    const mocks = [
      {
        request: {
          query: AdminAssignRoleDocument,
          variables: { userId: "u-1", roleId: MOD_ROLE.id },
        },
        error: makeCodedError("FORBIDDEN"),
      },
    ];

    errorSpy = vi.spyOn(console, "error").mockImplementation(() => {});
    warnSpy = vi.spyOn(console, "warn").mockImplementation(() => {});
    renderRow({ mocks });

    await user.click(screen.getByRole("checkbox", { name: /moderator/i }));

    await waitFor(() => {
      expect(screen.getByRole("alert")).toHaveTextContent(/do not have permission/i);
    });
    expect(warnSpy).toHaveBeenCalledWith(
      expect.stringContaining("role-toggle rejected"),
      expect.objectContaining({ codes: ["FORBIDDEN"] }),
    );
  });

  it("UNAUTHENTICATED transport rejection shows session-expired copy per role", async () => {
    const user = userEvent.setup();
    const mocks = [
      {
        request: {
          query: AdminAssignRoleDocument,
          variables: { userId: "u-1", roleId: MOD_ROLE.id },
        },
        error: makeCodedError("UNAUTHENTICATED"),
      },
    ];

    errorSpy = vi.spyOn(console, "error").mockImplementation(() => {});
    warnSpy = vi.spyOn(console, "warn").mockImplementation(() => {});
    renderRow({ mocks });

    await user.click(screen.getByRole("checkbox", { name: /moderator/i }));

    await waitFor(() => {
      expect(screen.getByRole("alert")).toHaveTextContent(/session has expired/i);
    });
    expect(warnSpy).toHaveBeenCalledWith(
      expect.stringContaining("role-toggle rejected"),
      expect.objectContaining({ codes: ["UNAUTHENTICATED"] }),
    );
  });

  it("generic transport rejection warns without err.message and shows generic copy", async () => {
    const user = userEvent.setup();
    const mocks = [
      {
        request: {
          query: AdminAssignRoleDocument,
          variables: { userId: "u-1", roleId: MOD_ROLE.id },
        },
        error: new Error("network down"),
      },
    ];

    errorSpy = vi.spyOn(console, "error").mockImplementation(() => {});
    warnSpy = vi.spyOn(console, "warn").mockImplementation(() => {});
    renderRow({ mocks });

    await user.click(screen.getByRole("checkbox", { name: /moderator/i }));

    await waitFor(() => {
      expect(screen.getByRole("alert")).toHaveTextContent(/unexpected error occurred/i);
    });
    expect(warnSpy).toHaveBeenCalledWith(
      expect.stringContaining("role-toggle rejected"),
      expect.objectContaining({ name: "Error", codes: [] }),
    );
    expect(warnSpy).not.toHaveBeenCalledWith(
      expect.anything(),
      expect.objectContaining({ message: expect.anything() }),
    );
  });

  it("unknown assign payload warns and shows degraded copy", async () => {
    const user = userEvent.setup();
    const mocks = [
      {
        request: {
          query: AdminAssignRoleDocument,
          variables: { userId: "u-1", roleId: MOD_ROLE.id },
        },
        result: {
          data: {
            assignRole: { __typename: "FutureVariantClientDidNotKnowAbout" } as never,
          },
        },
      },
    ];

    warnSpy = vi.spyOn(console, "warn").mockImplementation(() => {});
    renderRow({ mocks });

    await user.click(screen.getByRole("checkbox", { name: /moderator/i }));

    await waitFor(() => expect(warnSpy).toHaveBeenCalled());
    expect(screen.getByRole("alert")).toHaveTextContent(/something went wrong/i);
    expect(warnSpy).toHaveBeenCalledWith(
      expect.stringContaining("unexpected role-toggle payload"),
      expect.objectContaining({ typename: "FutureVariantClientDidNotKnowAbout" }),
    );
  });

  it("null assign payload warns and shows degraded copy", async () => {
    const user = userEvent.setup();
    const mocks = [
      {
        request: {
          query: AdminAssignRoleDocument,
          variables: { userId: "u-1", roleId: MOD_ROLE.id },
        },
        result: { data: { assignRole: null as never } },
      },
    ];

    warnSpy = vi.spyOn(console, "warn").mockImplementation(() => {});
    renderRow({ mocks });

    await user.click(screen.getByRole("checkbox", { name: /moderator/i }));

    await waitFor(() => expect(warnSpy).toHaveBeenCalled());
    expect(screen.getByRole("alert")).toHaveTextContent(/something went wrong/i);
    expect(warnSpy).toHaveBeenCalledWith(
      expect.stringContaining("unexpected role-toggle payload"),
      expect.objectContaining({ typename: null }),
    );
  });

  it("RevokeRoleSuccess updates local role state", async () => {
    const user = userEvent.setup();
    const mocks = [
      {
        request: {
          query: AdminRevokeRoleDocument,
          variables: { userId: "u-1", roleId: ADMIN_ROLE.id },
        },
        result: {
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
        },
      },
    ];

    renderRow({ mocks });

    await user.click(screen.getByRole("checkbox", { name: /admin/i }));

    await waitFor(() => {
      expect(screen.getByRole("checkbox", { name: /admin/i })).not.toBeChecked();
    });
  });

  it("CannotRevokeOwnAdminRoleError renders the self-demotion guard message", async () => {
    const user = userEvent.setup();
    const mocks = [
      {
        request: {
          query: AdminRevokeRoleDocument,
          variables: { userId: "u-1", roleId: ADMIN_ROLE.id },
        },
        result: {
          data: {
            revokeRole: {
              __typename: "CannotRevokeOwnAdminRoleError" as const,
              message: "cannot revoke your own admin role",
            },
          },
        },
      },
    ];

    renderRow({ mocks, allRoles: [ADMIN_ROLE] });

    await user.click(screen.getByRole("checkbox", { name: /admin/i }));

    await waitFor(() => {
      expect(screen.getByRole("alert")).toHaveTextContent("cannot revoke your own admin role");
    });
    expect(screen.getByRole("checkbox", { name: /admin/i })).toBeChecked();
  });

  it("InputValidationError on revoke renders a per-role banner", async () => {
    const user = userEvent.setup();
    const mocks = [
      {
        request: {
          query: AdminRevokeRoleDocument,
          variables: { userId: "u-1", roleId: ADMIN_ROLE.id },
        },
        result: {
          data: {
            revokeRole: {
              __typename: "InputValidationError" as const,
              field: "roleId",
              message: "user does not have this role",
            },
          },
        },
      },
    ];

    renderRow({ mocks, allRoles: [ADMIN_ROLE] });

    await user.click(screen.getByRole("checkbox", { name: /admin/i }));

    await waitFor(() => {
      expect(screen.getByRole("alert")).toHaveTextContent("user does not have this role");
    });
  });

  it("unknown revoke payload warns and shows degraded copy", async () => {
    const user = userEvent.setup();
    const mocks = [
      {
        request: {
          query: AdminRevokeRoleDocument,
          variables: { userId: "u-1", roleId: ADMIN_ROLE.id },
        },
        result: {
          data: {
            revokeRole: { __typename: "FutureVariantClientDidNotKnowAbout" } as never,
          },
        },
      },
    ];

    warnSpy = vi.spyOn(console, "warn").mockImplementation(() => {});
    renderRow({ mocks, allRoles: [ADMIN_ROLE] });

    await user.click(screen.getByRole("checkbox", { name: /admin/i }));

    await waitFor(() => expect(warnSpy).toHaveBeenCalled());
    expect(screen.getByRole("alert")).toHaveTextContent(/something went wrong/i);
    expect(warnSpy).toHaveBeenCalledWith(
      expect.stringContaining("unexpected role-toggle payload"),
      expect.objectContaining({ typename: "FutureVariantClientDidNotKnowAbout" }),
    );
  });

  it("null revoke payload warns and shows degraded copy", async () => {
    const user = userEvent.setup();
    const mocks = [
      {
        request: {
          query: AdminRevokeRoleDocument,
          variables: { userId: "u-1", roleId: ADMIN_ROLE.id },
        },
        result: { data: { revokeRole: null as never } },
      },
    ];

    warnSpy = vi.spyOn(console, "warn").mockImplementation(() => {});
    renderRow({ mocks, allRoles: [ADMIN_ROLE] });

    await user.click(screen.getByRole("checkbox", { name: /admin/i }));

    await waitFor(() => expect(warnSpy).toHaveBeenCalled());
    expect(screen.getByRole("alert")).toHaveTextContent(/something went wrong/i);
    expect(warnSpy).toHaveBeenCalledWith(
      expect.stringContaining("unexpected role-toggle payload"),
      expect.objectContaining({ typename: null }),
    );
  });
});

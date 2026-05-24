// @vitest-environment jsdom

import { readFileSync } from "node:fs";
import { join } from "node:path";
import { CombinedGraphQLErrors } from "@apollo/client/errors";
import { MockedProvider } from "@apollo/client/testing/react";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import { AdminEditUserDocument } from "@/generated/graphql";
import { AdminUserProfileSheet } from "./admin-user-profile-sheet";
import type { AdminUserListItem, AdminUserRole } from "./admin-user-row";

const ADMIN_ROLE: AdminUserRole = { id: "r-admin", name: "admin" };
const MOD_ROLE: AdminUserRole = { id: "r-mod", name: "moderator" };

function makeCodedError(code: string): CombinedGraphQLErrors {
  return new CombinedGraphQLErrors({
    errors: [{ message: "transport", extensions: { code } }],
  });
}

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

function successPayload(roleIds: string[] = [ADMIN_ROLE.id]) {
  const roles = roleIds.map((id) => ({
    __typename: "Role" as const,
    id,
    name: id === ADMIN_ROLE.id ? ADMIN_ROLE.name : MOD_ROLE.name,
  }));
  return {
    __typename: "AdminEditUserSuccess" as const,
    user: {
      __typename: "User" as const,
      id: "u-1",
      displayName: "Alice 2",
      bio: "bio text",
      avatarUrl: null,
      roles,
    },
  };
}

function renderSheet({
  user = makeUser(),
  allRoles = [ADMIN_ROLE, MOD_ROLE],
  mocks = [],
  onDismiss = vi.fn(),
  onSaved = vi.fn(),
}: {
  user?: AdminUserListItem | null;
  allRoles?: AdminUserRole[];
  mocks?: React.ComponentProps<typeof MockedProvider>["mocks"];
  onDismiss?: () => void;
  onSaved?: () => void;
} = {}) {
  render(
    <MockedProvider mocks={mocks}>
      <AdminUserProfileSheet
        open
        user={user}
        allRoles={allRoles}
        loading={false}
        queryError={null}
        onDismiss={onDismiss}
        onSaved={onSaved}
      />
    </MockedProvider>,
  );
  return { onDismiss, onSaved };
}

describe("AdminUserProfileSheet", () => {
  let warnSpy: ReturnType<typeof vi.spyOn> | null = null;
  let errorSpy: ReturnType<typeof vi.spyOn> | null = null;

  afterEach(() => {
    warnSpy?.mockRestore();
    warnSpy = null;
    errorSpy?.mockRestore();
    errorSpy = null;
  });

  it("renders profile fields and staged role checkboxes", async () => {
    const user = userEvent.setup();
    renderSheet();

    expect(screen.getByRole("heading", { name: /edit user/i })).toBeInTheDocument();
    expect(screen.getByLabelText(/display name/i)).toHaveValue("Alice");
    expect(screen.getByLabelText(/bio/i)).toHaveValue("bio text");
    expect(screen.getByRole("checkbox", { name: /admin/i })).toBeChecked();
    expect(screen.getByRole("checkbox", { name: /moderator/i })).not.toBeChecked();

    await user.click(screen.getByRole("checkbox", { name: /moderator/i }));

    expect(screen.getByRole("checkbox", { name: /moderator/i })).toBeChecked();
  });

  it("validates required display name before submitting", async () => {
    const user = userEvent.setup();
    renderSheet();

    await user.clear(screen.getByLabelText(/display name/i));
    await user.click(screen.getByRole("button", { name: /save changes/i }));

    expect(screen.getByRole("alert")).toHaveTextContent("Display name is required.");
  });

  it("profile-only dirty save sends one adminEditUser mutation with final roles", async () => {
    const user = userEvent.setup();
    const onSaved = vi.fn();
    const mocks = [
      {
        request: {
          query: AdminEditUserDocument,
          variables: {
            id: "u-1",
            input: { displayName: "Alice 2", bio: "bio text", roleIds: [ADMIN_ROLE.id] },
          },
        },
        result: { data: { adminEditUser: successPayload() } },
      },
    ];

    renderSheet({ mocks, onSaved });

    await user.clear(screen.getByLabelText(/display name/i));
    await user.type(screen.getByLabelText(/display name/i), "Alice 2");
    await user.click(screen.getByRole("button", { name: /save changes/i }));

    await waitFor(() => {
      expect(onSaved).toHaveBeenCalledTimes(1);
    });
  });

  it("roles-only dirty save omits profile fields and sends final roleIds", async () => {
    const user = userEvent.setup();
    const onSaved = vi.fn();
    const mocks = [
      {
        request: {
          query: AdminEditUserDocument,
          variables: {
            id: "u-1",
            input: { roleIds: [ADMIN_ROLE.id, MOD_ROLE.id] },
          },
        },
        result: { data: { adminEditUser: successPayload([ADMIN_ROLE.id, MOD_ROLE.id]) } },
      },
    ];

    renderSheet({ mocks, onSaved });

    await user.click(screen.getByRole("checkbox", { name: /moderator/i }));
    await user.click(screen.getByRole("button", { name: /save changes/i }));

    await waitFor(() => {
      expect(onSaved).toHaveBeenCalledTimes(1);
    });
  });

  it("profile and roles dirty save sends a single combined mutation", async () => {
    const user = userEvent.setup();
    const onSaved = vi.fn();
    const mocks = [
      {
        request: {
          query: AdminEditUserDocument,
          variables: {
            id: "u-1",
            input: {
              displayName: "Alice 2",
              bio: "bio text",
              roleIds: [ADMIN_ROLE.id, MOD_ROLE.id],
            },
          },
        },
        result: { data: { adminEditUser: successPayload([ADMIN_ROLE.id, MOD_ROLE.id]) } },
      },
    ];

    renderSheet({ mocks, onSaved });

    await user.clear(screen.getByLabelText(/display name/i));
    await user.type(screen.getByLabelText(/display name/i), "Alice 2");
    await user.click(screen.getByRole("checkbox", { name: /moderator/i }));
    await user.click(screen.getByRole("button", { name: /save changes/i }));

    await waitFor(() => {
      expect(onSaved).toHaveBeenCalledTimes(1);
    });
  });

  it("InputValidationError renders the server message", async () => {
    const user = userEvent.setup();
    const mocks = [
      {
        request: {
          query: AdminEditUserDocument,
          variables: {
            id: "u-1",
            input: { displayName: "Alice 2", bio: "bio text", roleIds: [ADMIN_ROLE.id] },
          },
        },
        result: {
          data: {
            adminEditUser: {
              __typename: "InputValidationError" as const,
              field: "displayName",
              message: "display name is taken",
            },
          },
        },
      },
    ];

    renderSheet({ mocks });

    await user.clear(screen.getByLabelText(/display name/i));
    await user.type(screen.getByLabelText(/display name/i), "Alice 2");
    await user.click(screen.getByRole("button", { name: /save changes/i }));

    await waitFor(() => {
      expect(screen.getByRole("alert")).toHaveTextContent("display name is taken");
    });
  });

  it("CannotRevokeOwnAdminRoleError renders the self-demotion banner", async () => {
    const user = userEvent.setup();
    const mocks = [
      {
        request: {
          query: AdminEditUserDocument,
          variables: {
            id: "u-1",
            input: { roleIds: [] },
          },
        },
        result: {
          data: {
            adminEditUser: {
              __typename: "CannotRevokeOwnAdminRoleError" as const,
              message: "Cannot revoke your own admin role",
            },
          },
        },
      },
    ];

    renderSheet({ mocks, allRoles: [ADMIN_ROLE] });

    await user.click(screen.getByRole("checkbox", { name: /admin/i }));
    await user.click(screen.getByRole("button", { name: /save changes/i }));

    await waitFor(() => {
      expect(screen.getByRole("alert")).toHaveTextContent("Cannot revoke your own admin role");
    });
  });

  it("FORBIDDEN transport rejection shows permission copy without logging err.message", async () => {
    const user = userEvent.setup();
    const mocks = [
      {
        request: {
          query: AdminEditUserDocument,
          variables: {
            id: "u-1",
            input: { displayName: "Alice 2", bio: "bio text", roleIds: [ADMIN_ROLE.id] },
          },
        },
        error: makeCodedError("FORBIDDEN"),
      },
    ];

    errorSpy = vi.spyOn(console, "error").mockImplementation(() => {});
    warnSpy = vi.spyOn(console, "warn").mockImplementation(() => {});
    renderSheet({ mocks });

    await user.clear(screen.getByLabelText(/display name/i));
    await user.type(screen.getByLabelText(/display name/i), "Alice 2");
    await user.click(screen.getByRole("button", { name: /save changes/i }));

    await waitFor(() => {
      expect(screen.getByRole("alert")).toHaveTextContent(/do not have permission/i);
    });
    expect(warnSpy).toHaveBeenCalledWith(
      expect.stringContaining("adminEditUser rejected"),
      expect.objectContaining({ codes: ["FORBIDDEN"] }),
    );
    expect(warnSpy).not.toHaveBeenCalledWith(
      expect.anything(),
      expect.objectContaining({ message: expect.anything() }),
    );
  });

  it("UNAUTHENTICATED transport rejection shows session-expired copy", async () => {
    const user = userEvent.setup();
    const mocks = [
      {
        request: {
          query: AdminEditUserDocument,
          variables: {
            id: "u-1",
            input: { displayName: "Alice 2", bio: "bio text", roleIds: [ADMIN_ROLE.id] },
          },
        },
        error: makeCodedError("UNAUTHENTICATED"),
      },
    ];

    errorSpy = vi.spyOn(console, "error").mockImplementation(() => {});
    warnSpy = vi.spyOn(console, "warn").mockImplementation(() => {});
    renderSheet({ mocks });

    await user.clear(screen.getByLabelText(/display name/i));
    await user.type(screen.getByLabelText(/display name/i), "Alice 2");
    await user.click(screen.getByRole("button", { name: /save changes/i }));

    await waitFor(() => {
      expect(screen.getByRole("alert")).toHaveTextContent(/session has expired/i);
    });
    expect(warnSpy).toHaveBeenCalledWith(
      expect.stringContaining("adminEditUser rejected"),
      expect.objectContaining({ codes: ["UNAUTHENTICATED"] }),
    );
    // Mirror the FORBIDDEN test's negative assertion: err.message must not
    // leak into structured logs per the redact-err-message rule.
    expect(warnSpy).not.toHaveBeenCalledWith(
      expect.anything(),
      expect.objectContaining({ message: expect.anything() }),
    );
  });

  it("unrecognized error code falls back to generic unexpected-error copy", async () => {
    const user = userEvent.setup();
    const mocks = [
      {
        request: {
          query: AdminEditUserDocument,
          variables: {
            id: "u-1",
            input: { displayName: "Alice 2", bio: "bio text", roleIds: [ADMIN_ROLE.id] },
          },
        },
        error: makeCodedError("SOME_NEW_CODE"),
      },
    ];

    errorSpy = vi.spyOn(console, "error").mockImplementation(() => {});
    warnSpy = vi.spyOn(console, "warn").mockImplementation(() => {});
    renderSheet({ mocks });

    await user.clear(screen.getByLabelText(/display name/i));
    await user.type(screen.getByLabelText(/display name/i), "Alice 2");
    await user.click(screen.getByRole("button", { name: /save changes/i }));

    await waitFor(() => {
      expect(screen.getByRole("alert")).toHaveTextContent(/unexpected error/i);
    });
    expect(warnSpy).toHaveBeenCalledWith(
      expect.stringContaining("adminEditUser rejected"),
      expect.objectContaining({ codes: ["SOME_NEW_CODE"] }),
    );
    expect(warnSpy).not.toHaveBeenCalledWith(
      expect.anything(),
      expect.objectContaining({ message: expect.anything() }),
    );
  });

  it("shows the loading indicator while the lazy query is in flight", () => {
    render(
      <MockedProvider mocks={[]}>
        <AdminUserProfileSheet
          open
          user={null}
          allRoles={[]}
          loading
          queryError={null}
          onDismiss={vi.fn()}
          onSaved={vi.fn()}
        />
      </MockedProvider>,
    );

    expect(screen.getByTestId("admin-user-sheet-loading")).toHaveTextContent(/loading user/i);
    expect(screen.queryByLabelText(/display name/i)).not.toBeInTheDocument();
  });

  it("does not flash 'User not found.' while the sheet is closing", () => {
    render(
      <MockedProvider mocks={[]}>
        <AdminUserProfileSheet
          open={false}
          user={null}
          allRoles={[]}
          loading={false}
          queryError={null}
          onDismiss={vi.fn()}
          onSaved={vi.fn()}
        />
      </MockedProvider>,
    );

    expect(screen.queryByText(/user not found/i)).not.toBeInTheDocument();
  });

  it("does not configure optimisticResponse for adminEditUser", () => {
    const source = readFileSync(
      join(process.cwd(), "src/app/admin/users/admin-user-profile-sheet.tsx"),
      "utf8",
    );
    const start = source.indexOf("runEdit({");
    const end = source.indexOf("});", start);

    expect(start).toBeGreaterThanOrEqual(0);
    expect(source.slice(start, end)).not.toContain("optimisticResponse");
  });
});

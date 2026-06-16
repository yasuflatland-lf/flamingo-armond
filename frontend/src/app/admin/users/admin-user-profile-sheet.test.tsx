// @vitest-environment jsdom

import { readFileSync } from "node:fs";
import { join } from "node:path";
import { CombinedGraphQLErrors } from "@apollo/client/errors";
import { MockedProvider } from "@apollo/client/testing/react";
import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import { AdminEditUserDocument } from "@/generated/graphql";
import { renderWithIntl } from "@/test/render-with-intl";
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
    version: 41,
    displayName: "Alice",
    bio: "bio text",
    avatarUrl: null,
    lastSignInAt: null,
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
      version: 42,
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
  onReloadRequested = vi.fn(),
  onDelete,
}: {
  user?: AdminUserListItem | null;
  allRoles?: AdminUserRole[];
  mocks?: React.ComponentProps<typeof MockedProvider>["mocks"];
  onDismiss?: () => void;
  onSaved?: () => void;
  onReloadRequested?: () => void;
  onDelete?: (id: string) => Promise<void>;
} = {}) {
  renderWithIntl(
    <MockedProvider mocks={mocks}>
      <AdminUserProfileSheet
        open
        user={user}
        allRoles={allRoles}
        loading={false}
        queryError={null}
        onDismiss={onDismiss}
        onSaved={onSaved}
        onReloadRequested={onReloadRequested}
        onDelete={onDelete}
      />
    </MockedProvider>,
  );
  return { onDismiss, onSaved, onReloadRequested };
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
            expectedVersion: 41,
            displayName: "Alice 2",
            bio: "bio text",
            roleIds: [ADMIN_ROLE.id],
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

  it("ConcurrentUpdateError renders stale-edit copy and requests reload", async () => {
    const user = userEvent.setup();
    const onReloadRequested = vi.fn();
    const mocks = [
      {
        request: {
          query: AdminEditUserDocument,
          variables: {
            id: "u-1",
            expectedVersion: 41,
            displayName: "Alice 2",
            bio: "bio text",
            roleIds: [ADMIN_ROLE.id],
          },
        },
        result: {
          data: {
            adminEditUser: {
              __typename: "ConcurrentUpdateError" as const,
              message: "user has changed",
            },
          },
        },
      },
    ];

    renderSheet({ mocks, onReloadRequested });

    await user.clear(screen.getByLabelText(/display name/i));
    await user.type(screen.getByLabelText(/display name/i), "Alice 2");
    await user.click(screen.getByRole("button", { name: /save changes/i }));

    await waitFor(() => {
      expect(screen.getByRole("alert")).toHaveTextContent(
        "This user was changed by someone else. Reload and try again.",
      );
    });
    expect(onReloadRequested).toHaveBeenCalledTimes(1);
  });

  it("keeps the conflict banner when the reloaded user prop replaces the form", async () => {
    const user = userEvent.setup();
    const onReloadRequested = vi.fn();
    const mocks = [
      {
        request: {
          query: AdminEditUserDocument,
          variables: {
            id: "u-1",
            expectedVersion: 41,
            displayName: "Alice 2",
            bio: "bio text",
            roleIds: [ADMIN_ROLE.id],
          },
        },
        result: {
          data: {
            adminEditUser: {
              __typename: "ConcurrentUpdateError" as const,
              message: "user has changed",
            },
          },
        },
      },
    ];

    const renderTree = (u: AdminUserListItem) => (
      <MockedProvider mocks={mocks}>
        <AdminUserProfileSheet
          open
          user={u}
          allRoles={[ADMIN_ROLE, MOD_ROLE]}
          loading={false}
          queryError={null}
          onDismiss={vi.fn()}
          onSaved={vi.fn()}
          onReloadRequested={onReloadRequested}
        />
      </MockedProvider>
    );

    const { rerender } = renderWithIntl(renderTree(makeUser()));

    await user.clear(screen.getByLabelText(/display name/i));
    await user.type(screen.getByLabelText(/display name/i), "Alice 2");
    await user.click(screen.getByRole("button", { name: /save changes/i }));

    await waitFor(() => {
      expect(screen.getByRole("alert")).toHaveTextContent(
        "This user was changed by someone else. Reload and try again.",
      );
    });
    expect(onReloadRequested).toHaveBeenCalledTimes(1);

    // The parent refetch resolves: a fresh user object (bumped version, server
    // values) swaps into the form. The banner must survive that swap so the
    // admin still understands why their staged edits were replaced.
    rerender(renderTree(makeUser({ version: 99, displayName: "Server Name" })));

    await waitFor(() => {
      expect(screen.getByLabelText(/display name/i)).toHaveValue("Server Name");
    });
    expect(screen.getByRole("alert")).toHaveTextContent(
      "This user was changed by someone else. Reload and try again.",
    );

    // In production both the list refetch and the detail reload rewrite the
    // shared user cache entity, so the same-id user object can swap more than
    // once. The banner must survive every such swap, not just the first.
    rerender(renderTree(makeUser({ version: 100, displayName: "Server Name 2" })));

    await waitFor(() => {
      expect(screen.getByLabelText(/display name/i)).toHaveValue("Server Name 2");
    });
    expect(screen.getByRole("alert")).toHaveTextContent(
      "This user was changed by someone else. Reload and try again.",
    );
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
            expectedVersion: 41,
            roleIds: [ADMIN_ROLE.id, MOD_ROLE.id],
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
            expectedVersion: 41,
            displayName: "Alice 2",
            bio: "bio text",
            roleIds: [ADMIN_ROLE.id, MOD_ROLE.id],
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
            expectedVersion: 41,
            displayName: "Alice 2",
            bio: "bio text",
            roleIds: [ADMIN_ROLE.id],
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
            expectedVersion: 41,
            roleIds: [],
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
            expectedVersion: 41,
            displayName: "Alice 2",
            bio: "bio text",
            roleIds: [ADMIN_ROLE.id],
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
            expectedVersion: 41,
            displayName: "Alice 2",
            bio: "bio text",
            roleIds: [ADMIN_ROLE.id],
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
            expectedVersion: 41,
            displayName: "Alice 2",
            bio: "bio text",
            roleIds: [ADMIN_ROLE.id],
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
    renderWithIntl(
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
    renderWithIntl(
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

  it("omits the danger-zone delete button when onDelete is not provided", () => {
    renderSheet();
    expect(screen.queryByTestId("admin-delete-user-trigger")).not.toBeInTheDocument();
  });

  it("renders the danger-zone delete button when onDelete is provided", () => {
    renderSheet({ onDelete: vi.fn() });
    expect(screen.getByTestId("admin-delete-user-trigger")).toBeInTheDocument();
  });

  it("confirms deletion and calls onDelete with the user id", async () => {
    const user = userEvent.setup();
    const onDelete = vi.fn<(id: string) => Promise<void>>().mockResolvedValue(undefined);
    renderSheet({ onDelete });

    await user.click(screen.getByTestId("admin-delete-user-trigger"));
    await user.click(screen.getByTestId("admin-delete-user-confirm"));

    await waitFor(() => {
      expect(onDelete).toHaveBeenCalledWith("u-1");
    });
  });

  it("shows the forbidden copy and keeps the dialog open when onDelete rejects with FORBIDDEN", async () => {
    const user = userEvent.setup();
    warnSpy = vi.spyOn(console, "warn").mockImplementation(() => {});
    const onDelete = vi
      .fn<(id: string) => Promise<void>>()
      .mockRejectedValue(makeCodedError("FORBIDDEN"));
    renderSheet({ onDelete });

    await user.click(screen.getByTestId("admin-delete-user-trigger"));
    await user.click(screen.getByTestId("admin-delete-user-confirm"));

    await waitFor(() => {
      expect(screen.getByTestId("admin-delete-user-error")).toHaveTextContent(/cannot be deleted/i);
    });
    // The confirm button is still present — the dialog did not auto-close.
    expect(screen.getByTestId("admin-delete-user-confirm")).toBeInTheDocument();
    expect(warnSpy).toHaveBeenCalledWith(
      expect.stringContaining("adminDeleteUser rejected"),
      expect.objectContaining({ codes: ["FORBIDDEN"] }),
    );
  });
});

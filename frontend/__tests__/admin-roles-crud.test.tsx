// @vitest-environment jsdom
import { MockedProvider } from "@apollo/client/testing/react";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { GraphQLError } from "graphql";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { AdminRolesClient } from "@/app/admin/roles/AdminRolesClient";
import {
  AdminCreateRoleDocument,
  AdminDeleteRoleDocument,
  AdminUpdateRoleDocument,
} from "@/generated/graphql";

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

type RoleItem = { id: string; name: string };

// ---------------------------------------------------------------------------
// console spy helpers — mirrors admin-users-roles.test.tsx scaffolding
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

function renderClient(initialRoles: RoleItem[], mocks: object[]) {
  return render(
    <MockedProvider mocks={mocks as never}>
      <AdminRolesClient initialRoles={initialRoles} />
    </MockedProvider>,
  );
}

// ---------------------------------------------------------------------------
// Fixtures
// ---------------------------------------------------------------------------

const ADMIN_ROLE: RoleItem = { id: "r-admin", name: "admin" };
const MOD_ROLE: RoleItem = { id: "r-mod", name: "moderator" };

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

describe("AdminRolesClient", () => {
  // -------------------------------------------------------------------------
  // 1. Display
  // -------------------------------------------------------------------------

  it("renders the initial role list", () => {
    renderClient([ADMIN_ROLE, MOD_ROLE], []);

    expect(screen.getByText("admin")).toBeInTheDocument();
    expect(screen.getByText("moderator")).toBeInTheDocument();
  });

  it("disables Edit/Delete for the system role 'admin'", () => {
    renderClient([ADMIN_ROLE, MOD_ROLE], []);

    // Locate the admin list item by its containing <li>
    const adminItem = screen.getByText("admin").closest("li");
    expect(adminItem).toBeTruthy();

    const editBtn = within(adminItem as HTMLElement).getByRole("button", { name: /edit/i });
    const deleteBtn = within(adminItem as HTMLElement).getByRole("button", { name: /delete/i });

    expect(editBtn).toBeDisabled();
    expect(deleteBtn).toBeDisabled();

    // Non-system role buttons must be enabled
    const modItem = screen.getByText("moderator").closest("li");
    expect(modItem).toBeTruthy();
    const modEditBtn = within(modItem as HTMLElement).getByRole("button", { name: /edit/i });
    const modDeleteBtn = within(modItem as HTMLElement).getByRole("button", { name: /delete/i });

    expect(modEditBtn).not.toBeDisabled();
    expect(modDeleteBtn).not.toBeDisabled();
  });

  // -------------------------------------------------------------------------
  // 2. Create
  // -------------------------------------------------------------------------

  it("creates a new role and appends it to the list", async () => {
    const user = userEvent.setup();
    const initial = [ADMIN_ROLE, MOD_ROLE];

    const createMock = {
      request: { query: AdminCreateRoleDocument, variables: { name: "editor" } },
      result: vi.fn(() => ({
        data: {
          createRole: { __typename: "Role", id: "r-editor", name: "editor" },
        },
      })),
    };

    renderClient(initial, [createMock]);

    await user.type(screen.getByLabelText(/new role name/i), "editor");
    await user.click(screen.getByRole("button", { name: /add role/i }));

    await waitFor(() => {
      expect(screen.getByText("editor")).toBeInTheDocument();
    });

    expect(createMock.result).toHaveBeenCalledTimes(1);
  });

  it("clears the new-role input after a successful create", async () => {
    const user = userEvent.setup();
    const initial = [ADMIN_ROLE];

    const createMock = {
      request: { query: AdminCreateRoleDocument, variables: { name: "editor" } },
      result: vi.fn(() => ({
        data: {
          createRole: { __typename: "Role", id: "r-editor", name: "editor" },
        },
      })),
    };

    renderClient(initial, [createMock]);

    const input = screen.getByLabelText(/new role name/i);
    await user.type(input, "editor");
    await user.click(screen.getByRole("button", { name: /add role/i }));

    await waitFor(() => {
      expect(screen.getByText("editor")).toBeInTheDocument();
    });

    // Input must be cleared after successful create
    expect(screen.getByLabelText(/new role name/i)).toHaveValue("");
  });

  it("shows an error banner when createRole returns BAD_USER_INPUT (e.g., duplicate name)", async () => {
    const user = userEvent.setup();
    const initial = [ADMIN_ROLE];

    // BAD_USER_INPUT without a field extension — getBackendErrorBanner returns the message
    const createMock = {
      request: { query: AdminCreateRoleDocument, variables: { name: "admin" } },
      result: () => ({
        data: null,
        errors: [
          new GraphQLError("role name already exists", {
            extensions: { code: "BAD_USER_INPUT" },
          }),
        ],
      }),
    };

    renderClient(initial, [createMock]);

    await user.type(screen.getByLabelText(/new role name/i), "admin");
    await user.click(screen.getByRole("button", { name: /add role/i }));

    await waitFor(() => {
      const alert = screen.getByRole("alert");
      expect(alert).toHaveTextContent(/role name already exists/i);
    });
  });

  it("shows an error banner for empty new role name (client-side guard)", async () => {
    const user = userEvent.setup();
    renderClient([ADMIN_ROLE], []);

    // Submit without typing anything — client guard fires before mutation
    await user.click(screen.getByRole("button", { name: /add role/i }));

    const alert = screen.getByRole("alert");
    expect(alert).toHaveTextContent(/name is required/i);
  });

  // -------------------------------------------------------------------------
  // 3. Update
  // -------------------------------------------------------------------------

  it("starts inline edit when Edit is clicked, then saves the new name", async () => {
    const user = userEvent.setup();
    const initial = [ADMIN_ROLE, MOD_ROLE];

    const updateMock = {
      request: {
        query: AdminUpdateRoleDocument,
        variables: { id: "r-mod", name: "writer" },
      },
      result: vi.fn(() => ({
        data: {
          updateRole: { __typename: "Role", id: "r-mod", name: "writer" },
        },
      })),
    };

    renderClient(initial, [updateMock]);

    // Click Edit on "moderator"
    await user.click(screen.getByRole("button", { name: /edit moderator/i }));

    // Inline input appears — use data-testid to avoid ambiguity with the "New role name" input
    const editInput = screen.getByTestId("admin-role-edit-input");
    expect(editInput).toHaveValue("moderator");

    // Clear and type new name
    await user.clear(editInput);
    await user.type(editInput, "writer");
    await user.click(screen.getByRole("button", { name: /save/i }));

    await waitFor(() => {
      expect(screen.getByText("writer")).toBeInTheDocument();
    });

    expect(updateMock.result).toHaveBeenCalledTimes(1);
    // Inline edit input should be dismissed
    expect(screen.queryByTestId("admin-role-edit-input")).toBeNull();
  });

  it("cancels inline edit and restores the original name", async () => {
    const user = userEvent.setup();
    renderClient([ADMIN_ROLE, MOD_ROLE], []);

    // Open edit for "moderator"
    await user.click(screen.getByRole("button", { name: /edit moderator/i }));

    const editInput = screen.getByTestId("admin-role-edit-input");
    await user.clear(editInput);
    await user.type(editInput, "something-else");

    // Cancel — no mutation should fire
    await user.click(screen.getByRole("button", { name: /cancel/i }));

    // Inline input gone
    expect(screen.queryByTestId("admin-role-edit-input")).toBeNull();

    // Original name still shown
    expect(screen.getByText("moderator")).toBeInTheDocument();
    expect(screen.queryByText("something-else")).toBeNull();
  });

  it("shows an error banner when updateRole returns BAD_USER_INPUT", async () => {
    const user = userEvent.setup();
    const initial = [ADMIN_ROLE, MOD_ROLE];

    const updateMock = {
      request: {
        query: AdminUpdateRoleDocument,
        variables: { id: "r-mod", name: "admin" },
      },
      result: () => ({
        data: null,
        errors: [
          new GraphQLError("role name already exists", {
            extensions: { code: "BAD_USER_INPUT" },
          }),
        ],
      }),
    };

    renderClient(initial, [updateMock]);

    await user.click(screen.getByRole("button", { name: /edit moderator/i }));
    const editInput = screen.getByTestId("admin-role-edit-input");
    await user.clear(editInput);
    await user.type(editInput, "admin");
    await user.click(screen.getByRole("button", { name: /save/i }));

    await waitFor(() => {
      const alert = screen.getByRole("alert");
      expect(alert).toHaveTextContent(/role name already exists/i);
    });
  });

  // -------------------------------------------------------------------------
  // 4. Delete
  // -------------------------------------------------------------------------

  it("removes the role from the list on successful delete", async () => {
    const user = userEvent.setup();
    const initial = [ADMIN_ROLE, MOD_ROLE];

    const deleteMock = {
      request: { query: AdminDeleteRoleDocument, variables: { id: "r-mod" } },
      result: vi.fn(() => ({ data: { deleteRole: true } })),
    };

    renderClient(initial, [deleteMock]);

    await user.click(screen.getByRole("button", { name: /delete moderator/i }));

    await waitFor(() => {
      expect(screen.queryByText("moderator")).toBeNull();
    });

    expect(deleteMock.result).toHaveBeenCalledTimes(1);
    // admin still present
    expect(screen.getByText("admin")).toBeInTheDocument();
  });

  it("shows an error banner when deleteRole returns FORBIDDEN (server guard on non-admin role)", async () => {
    // This exercises the FORBIDDEN error path.
    // Even though the UI disables delete for the "admin" system role, a race
    // condition (e.g. a concurrent admin promotion) could cause the server to
    // return FORBIDDEN for a role that appeared deletable at render time.
    const user = userEvent.setup();

    const nonAdminRole: RoleItem = { id: "r-mod", name: "moderator" };
    const initial = [ADMIN_ROLE, nonAdminRole];

    const deleteMock = {
      request: { query: AdminDeleteRoleDocument, variables: { id: "r-mod" } },
      result: () => ({
        data: null,
        errors: [
          new GraphQLError("cannot delete a protected role", {
            extensions: { code: "FORBIDDEN" },
          }),
        ],
      }),
    };

    renderClient(initial, [deleteMock]);

    await user.click(screen.getByRole("button", { name: /delete moderator/i }));

    await waitFor(() => {
      const alert = screen.getByRole("alert");
      expect(alert).toHaveTextContent(/cannot delete a protected role/i);
    });

    // Role is still in the list — server truth was not changed
    expect(screen.getByText("moderator")).toBeInTheDocument();
  });

  // -------------------------------------------------------------------------
  // 5. Guard rails — MockedProvider leak detection
  // -------------------------------------------------------------------------

  it("does not produce 'No more mocked responses' warnings after one full CRUD cycle", async () => {
    const user = userEvent.setup();
    const initial = [ADMIN_ROLE];

    // 1 create + 1 update + 1 delete for the created role
    const mocks = [
      {
        request: { query: AdminCreateRoleDocument, variables: { name: "editor" } },
        result: vi.fn(() => ({
          data: { createRole: { __typename: "Role", id: "r-e", name: "editor" } },
        })),
      },
      {
        request: { query: AdminUpdateRoleDocument, variables: { id: "r-e", name: "writer" } },
        result: vi.fn(() => ({
          data: { updateRole: { __typename: "Role", id: "r-e", name: "writer" } },
        })),
      },
      {
        request: { query: AdminDeleteRoleDocument, variables: { id: "r-e" } },
        result: vi.fn(() => ({ data: { deleteRole: true } })),
      },
    ];

    renderClient(initial, mocks);

    // --- Create ---
    await user.type(screen.getByLabelText(/new role name/i), "editor");
    await user.click(screen.getByRole("button", { name: /add role/i }));
    await waitFor(() => expect(screen.getByText("editor")).toBeInTheDocument());

    // --- Update ---
    await user.click(screen.getByRole("button", { name: /edit editor/i }));
    const editInput = screen.getByTestId("admin-role-edit-input");
    await user.clear(editInput);
    await user.type(editInput, "writer");
    await user.click(screen.getByRole("button", { name: /save/i }));
    await waitFor(() => expect(screen.getByText("writer")).toBeInTheDocument());

    // --- Delete ---
    await user.click(screen.getByRole("button", { name: /delete writer/i }));
    await waitFor(() => expect(screen.queryByText("writer")).toBeNull());

    // No MockedProvider leak warnings for any of the role mutation operations
    const leakWarnings = consoleWarnSpy.mock.calls.filter((args: unknown[]) =>
      args.some(
        (arg: unknown) =>
          typeof arg === "string" &&
          (arg.includes("AdminCreateRole") ||
            arg.includes("AdminUpdateRole") ||
            arg.includes("AdminDeleteRole")),
      ),
    );
    expect(leakWarnings).toEqual([]);
  });
});

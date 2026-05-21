// @vitest-environment jsdom
/**
 * Behavioural tests for the AdminRolesClient list shell.
 *
 * Scope:
 *   - List rendering of editable + system roles
 *   - System-role guard: rows are non-clickable, delete button is disabled
 *   - Custom roles: row links to /admin/roles/:id/edit
 *   - Delete mutation: success removes from list, FORBIDDEN surfaces banner
 *   - "New role" CTA wires to /admin/roles/new
 *
 * NOT covered here:
 *   - Create form (owned by src/app/admin/roles/new/new-role-client.test.tsx)
 *   - Edit form (owned by src/app/admin/roles/[id]/edit/edit-role-client.test.tsx)
 *   - Page-level SSR seed (owned by admin-roles.test.tsx)
 */

import { MockedProvider } from "@apollo/client/testing/react";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { GraphQLError } from "graphql";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { AdminRolesClient } from "@/app/admin/roles/admin-roles-client";
import { AdminDeleteRoleDocument } from "@/generated/graphql";

// ---------------------------------------------------------------------------
// Module mocks
// ---------------------------------------------------------------------------

vi.mock("next/link", () => ({
  default: ({ href, children, ...rest }: { href: string; children: React.ReactNode }) => (
    <a href={href} {...rest}>
      {children}
    </a>
  ),
}));

// ---------------------------------------------------------------------------
// Types + console spies
// ---------------------------------------------------------------------------

type RoleItem = { id: string; name: string };

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
// Helpers
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
const GENERAL_ROLE: RoleItem = { id: "r-general", name: "general" };
const MOD_ROLE: RoleItem = { id: "r-mod", name: "moderator" };

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

describe("AdminRolesClient", () => {
  // -------------------------------------------------------------------------
  // 1. Display
  // -------------------------------------------------------------------------

  it("renders every role passed in initialRoles", () => {
    renderClient([ADMIN_ROLE, GENERAL_ROLE, MOD_ROLE], []);

    expect(screen.getByText("admin")).toBeInTheDocument();
    expect(screen.getByText("general")).toBeInTheDocument();
    expect(screen.getByText("moderator")).toBeInTheDocument();
  });

  it("links the 'New role' CTA to /admin/roles/new", () => {
    renderClient([], []);

    // Button uses asChild + <Link>: the testid is forwarded to the
    // rendered <a>, so the testid handle IS the anchor element itself.
    const cta = screen.getByTestId("admin-roles-new-btn");
    expect(cta.tagName).toBe("A");
    expect(cta).toHaveAttribute("href", "/admin/roles/new");
  });

  // -------------------------------------------------------------------------
  // 2. System-role guard (admin + general)
  // -------------------------------------------------------------------------

  it("renders system rows without an edit link and disables delete", () => {
    renderClient([ADMIN_ROLE, GENERAL_ROLE, MOD_ROLE], []);

    // Locate each system row by data-testid and assert no anchor inside.
    const adminRow = screen.getByTestId("admin-role-row-r-admin");
    const generalRow = screen.getByTestId("admin-role-row-r-general");
    expect(within(adminRow).queryByRole("link")).toBeNull();
    expect(within(generalRow).queryByRole("link")).toBeNull();

    // Delete buttons are disabled for both system roles.
    expect(within(adminRow).getByRole("button", { name: /delete admin/i })).toBeDisabled();
    expect(within(generalRow).getByRole("button", { name: /delete general/i })).toBeDisabled();

    // System rows render a "System role" caption.
    expect(within(adminRow).getByText("System role")).toBeInTheDocument();
    expect(within(generalRow).getByText("System role")).toBeInTheDocument();
  });

  it("renders custom rows as a link to the edit page with delete enabled", () => {
    renderClient([ADMIN_ROLE, MOD_ROLE], []);

    const modRow = screen.getByTestId("admin-role-row-r-mod");
    const link = within(modRow).getByRole("link", { name: /moderator/i });
    expect(link).toHaveAttribute("href", "/admin/roles/r-mod/edit");

    expect(within(modRow).getByRole("button", { name: /delete moderator/i })).not.toBeDisabled();
  });

  // -------------------------------------------------------------------------
  // 3. Delete
  // -------------------------------------------------------------------------

  it("removes the role from the list on successful delete", async () => {
    const user = userEvent.setup();
    const deleteMock = {
      request: { query: AdminDeleteRoleDocument, variables: { id: "r-mod" } },
      result: vi.fn(() => ({ data: { deleteRole: true } })),
    };

    renderClient([ADMIN_ROLE, MOD_ROLE], [deleteMock]);

    await user.click(screen.getByRole("button", { name: /delete moderator/i }));

    await waitFor(() => {
      expect(screen.queryByText("moderator")).toBeNull();
    });

    expect(deleteMock.result).toHaveBeenCalledTimes(1);
    // admin still present
    expect(screen.getByText("admin")).toBeInTheDocument();
  });

  it("shows an error banner when deleteRole returns FORBIDDEN", async () => {
    // Exercises the FORBIDDEN error path. Even though the UI disables delete
    // for system roles, a race (e.g. a concurrent role promotion) could cause
    // the server to return FORBIDDEN for a role that appeared deletable at
    // render time.
    const user = userEvent.setup();
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

    renderClient([ADMIN_ROLE, MOD_ROLE], [deleteMock]);

    await user.click(screen.getByRole("button", { name: /delete moderator/i }));

    await waitFor(() => {
      const alert = screen.getByRole("alert");
      expect(alert).toHaveTextContent(/cannot delete a protected role/i);
    });

    // Role is still in the list — server truth was not changed.
    expect(screen.getByText("moderator")).toBeInTheDocument();
  });
});

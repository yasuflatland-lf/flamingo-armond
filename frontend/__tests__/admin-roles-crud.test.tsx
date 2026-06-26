// @vitest-environment happy-dom
/**
 * Behavioural tests for the AdminRolesClient list shell.
 *
 * Scope:
 *   - List rendering of editable + system roles
 *   - System-role guard: rows are non-clickable, delete button is disabled
 *   - Custom roles: row opens ?edit=<id>
 *   - Delete mutation: success removes from list, FORBIDDEN surfaces banner
 *   - "New role" CTA opens ?new=true
 *
 * NOT covered here:
 *   - Create/edit sheet form flows (owned by src/app/admin/roles/admin-roles-client.test.tsx)
 *   - Page-level SSR seed (owned by admin-roles.test.tsx)
 */

import { MockedProvider } from "@apollo/client/testing/react";
import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { GraphQLError } from "graphql";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { AdminRolesClient } from "@/app/admin/roles/admin-roles-client";
import { AdminDeleteRoleDocument } from "@/generated/graphql";
import { UndoDeleteProvider } from "@/lib/undo-delete";
import { renderWithIntl } from "@/test/render-with-intl";

// ---------------------------------------------------------------------------
// Module mocks
// ---------------------------------------------------------------------------

// usePathname is used by UndoDeleteProvider for flush-on-navigation.
const mockPush = vi.fn();
const mockReplace = vi.fn();
const mockRefresh = vi.fn();

vi.mock("next/navigation", () => ({
  usePathname: () => "/admin/roles",
  useRouter: () => ({
    push: mockPush,
    refresh: mockRefresh,
    replace: mockReplace,
  }),
  useSearchParams: () => new URLSearchParams(""),
}));

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
  mockPush.mockReset();
  mockReplace.mockReset();
  mockRefresh.mockReset();
});

afterEach(() => {
  consoleWarnSpy.mockRestore();
  consoleErrorSpy.mockRestore();
});

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function renderClient(initialRoles: RoleItem[], mocks: object[]) {
  return renderWithIntl(
    <MockedProvider mocks={mocks as never}>
      <UndoDeleteProvider>
        <AdminRolesClient initialRoles={initialRoles} />
      </UndoDeleteProvider>
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

  it("opens the New role sheet query from the CTA", async () => {
    const user = userEvent.setup();
    renderClient([], []);

    const cta = screen.getByTestId("admin-roles-new-btn");
    expect(cta.tagName).toBe("BUTTON");

    await user.click(cta);

    expect(mockPush).toHaveBeenCalledWith("/admin/roles?new=true", { scroll: false });
  });

  // -------------------------------------------------------------------------
  // 2. System-role guard (admin + general)
  // -------------------------------------------------------------------------

  it("renders system rows without an edit link or delete button", () => {
    renderClient([ADMIN_ROLE, GENERAL_ROLE, MOD_ROLE], []);

    // Locate each system row by data-testid and assert no anchor inside.
    const adminRow = screen.getByTestId("admin-role-row-r-admin");
    const generalRow = screen.getByTestId("admin-role-row-r-general");
    expect(within(adminRow).queryByRole("link")).toBeNull();
    expect(within(generalRow).queryByRole("link")).toBeNull();

    // System roles have no delete button at all (isSystem branch is non-interactive).
    expect(within(adminRow).queryByRole("button", { name: /delete admin/i })).toBeNull();
    expect(within(generalRow).queryByRole("button", { name: /delete general/i })).toBeNull();

    // System rows render a "System role" caption.
    expect(within(adminRow).getByText("System role")).toBeInTheDocument();
    expect(within(generalRow).getByText("System role")).toBeInTheDocument();
  });

  it("renders custom rows with an edit button and delete enabled", async () => {
    const user = userEvent.setup();
    renderClient([ADMIN_ROLE, MOD_ROLE], []);

    const modRow = screen.getByTestId("admin-role-row-r-mod");
    await user.click(within(modRow).getByRole("button", { name: /edit role moderator/i }));

    expect(mockPush).toHaveBeenCalledWith("/admin/roles?edit=r-mod", { scroll: false });

    expect(within(modRow).getByRole("button", { name: /delete moderator/i })).not.toBeDisabled();
  });

  // -------------------------------------------------------------------------
  // 3. Delete
  // -------------------------------------------------------------------------

  describe("delete", () => {
    beforeEach(() => {
      vi.useFakeTimers({ shouldAdvanceTime: true });
    });

    afterEach(() => {
      vi.useRealTimers();
    });

    it("removes the role from the list on successful delete", async () => {
      const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime.bind(vi) });
      const deleteMock = {
        request: { query: AdminDeleteRoleDocument, variables: { id: "r-mod" } },
        result: vi.fn(() => ({ data: { deleteRole: true } })),
      };

      renderClient([ADMIN_ROLE, MOD_ROLE], [deleteMock]);

      await user.click(screen.getByRole("button", { name: /delete moderator/i }));

      // Optimistic removal hides the row immediately.
      await waitFor(() => {
        expect(screen.queryByTestId("admin-role-row-r-mod")).toBeNull();
      });

      // Advance past the 5-second undo window so the mutation is committed.
      vi.advanceTimersByTime(5100);
      vi.useRealTimers();
      await waitFor(() => {
        expect(deleteMock.result).toHaveBeenCalledTimes(1);
      });

      // admin still present
      expect(screen.getByText("admin")).toBeInTheDocument();
    });

    it("shows an error banner when deleteRole returns FORBIDDEN", async () => {
      // Exercises the FORBIDDEN error path. Even though the UI disables delete
      // for system roles, a race (e.g. a concurrent role promotion) could cause
      // the server to return FORBIDDEN for a role that appeared deletable at
      // render time.
      const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime.bind(vi) });
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

      // Optimistic removal hides the row immediately.
      await waitFor(() => {
        expect(screen.queryByTestId("admin-role-row-r-mod")).toBeNull();
      });

      // Advance past the 5-second undo window so the mutation fires and
      // FORBIDDEN triggers the rollback + error banner.
      vi.advanceTimersByTime(5100);
      vi.useRealTimers();

      await waitFor(() => {
        const alert = screen.getByRole("alert");
        expect(alert).toHaveTextContent(/cannot delete a protected role/i);
      });

      // Role is still in the list — server truth was not changed.
      expect(screen.getByText("moderator")).toBeInTheDocument();
    });
  });
});

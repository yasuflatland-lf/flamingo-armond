// @vitest-environment jsdom
/**
 * Broad page-level tests for /admin/roles (AdminRolesPage RSC).
 *
 * Scope: SSR-seed rendering, empty-seed path, gqlFetch error propagation, and
 * system-role presence in the rendered list.
 *
 * NOT covered here (owned by admin-roles-crud.test.tsx):
 *   - Create / update / delete mutation flows.
 *   - System-role Edit/Delete disabled state.
 *   - Mutation error banners and field-level errors.
 *
 * NOT covered here (owned by admin-layout.test.tsx):
 *   - Admin gate (Supabase auth, role check, redirect behaviour).
 */

// ---------------------------------------------------------------------------
// Module mocks — hoisted by Vitest before imports
// ---------------------------------------------------------------------------

vi.mock("@/lib/apollo/server", () => ({
  gqlFetch: vi.fn(),
}));

// AdminRolesClient uses Apollo mutations via useMutation; stub the provider so
// the client component mounts without a real Apollo client in scope.
vi.mock("@apollo/client/react", () => ({
  useMutation: vi.fn(() => [vi.fn(), { loading: false }]),
}));

// ---------------------------------------------------------------------------
// Imports — after vi.mock declarations
// ---------------------------------------------------------------------------

import { render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import AdminRolesPage from "@/app/admin/roles/page";
import { gqlFetch } from "@/lib/apollo/server";
import { resetMockSupabase } from "./utils/mock-supabase";

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

type RoleStub = { __typename: "Role"; id: string; name: string };

function makeRole(id: string, name: string): RoleStub {
  return { __typename: "Role", id, name };
}

function mockRoles(roles: RoleStub[]): void {
  vi.mocked(gqlFetch).mockResolvedValue({ roles } as never);
}

function mockGqlFetchError(err: Error): void {
  vi.mocked(gqlFetch).mockRejectedValue(err);
}

// ---------------------------------------------------------------------------
// Fixtures
// ---------------------------------------------------------------------------

const ADMIN_ROLE = makeRole("role-admin", "admin");
const GENERAL_ROLE = makeRole("role-general", "general");
const EDITOR_ROLE = makeRole("role-editor", "editor");

// ---------------------------------------------------------------------------
// Lifecycle
// ---------------------------------------------------------------------------

let consoleErrorSpy: ReturnType<typeof vi.spyOn>;

beforeEach(() => {
  resetMockSupabase();
  vi.clearAllMocks();
  consoleErrorSpy = vi.spyOn(console, "error").mockImplementation(() => {});
});

afterEach(() => {
  consoleErrorSpy.mockRestore();
  vi.restoreAllMocks();
});

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

describe("AdminRolesPage (page-level SSR seed)", () => {
  // -------------------------------------------------------------------------
  // 1. SSR seed: 3 roles → all shown in the rendered list
  // -------------------------------------------------------------------------

  it("renders all three role names seeded via gqlFetch", async () => {
    mockRoles([ADMIN_ROLE, GENERAL_ROLE, EDITOR_ROLE]);

    const tree = await AdminRolesPage();
    render(tree as React.ReactElement);

    const roleNames = screen.getAllByTestId("admin-role-name");
    const textContent = roleNames.map((el) => el.textContent);

    expect(textContent).toContain("admin");
    expect(textContent).toContain("general");
    expect(textContent).toContain("editor");

    // Each role gets its own list-item by id
    expect(screen.getByTestId("admin-role-row-role-admin")).toBeInTheDocument();
    expect(screen.getByTestId("admin-role-row-role-general")).toBeInTheDocument();
    expect(screen.getByTestId("admin-role-row-role-editor")).toBeInTheDocument();
  });

  // -------------------------------------------------------------------------
  // 2. Empty seed: zero roles — no role rows, but the New role CTA exists
  // -------------------------------------------------------------------------

  it("renders no role rows when gqlFetch returns an empty list", async () => {
    mockRoles([]);

    const tree = await AdminRolesPage();
    render(tree as React.ReactElement);

    // No role-name spans
    expect(screen.queryAllByTestId("admin-role-name")).toHaveLength(0);

    // The list container itself is still rendered.
    expect(screen.getByTestId("admin-roles-list")).toBeInTheDocument();

    // The top-right "New role" CTA is always present.
    expect(screen.getByTestId("admin-roles-new-btn")).toBeInTheDocument();
  });

  // -------------------------------------------------------------------------
  // 3. gqlFetch error: the RSC throws — error propagates to the caller
  // -------------------------------------------------------------------------

  it("propagates a gqlFetch error (page throws — error boundary handles it)", async () => {
    const networkErr = new Error("network failure");
    mockGqlFetchError(networkErr);

    await expect(AdminRolesPage()).rejects.toThrow("network failure");
  });

  // -------------------------------------------------------------------------
  // 4. System role surfaced in list: "admin" row is present after SSR seed
  // -------------------------------------------------------------------------

  it("includes the admin system role row in the rendered output", async () => {
    mockRoles([ADMIN_ROLE, GENERAL_ROLE]);

    const tree = await AdminRolesPage();
    render(tree as React.ReactElement);

    // The admin role name must be visible
    const adminRow = screen.getByTestId("admin-role-row-role-admin");
    expect(adminRow).toBeInTheDocument();
    expect(adminRow).toHaveTextContent("admin");
  });
});

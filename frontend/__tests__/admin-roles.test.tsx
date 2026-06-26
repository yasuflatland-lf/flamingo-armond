// @vitest-environment happy-dom
/**
 * Broad page-level tests for /admin/roles (AdminRolesPage RSC).
 *
 * Scope: SSR-seed rendering, empty-seed path, gqlFetch error branches
 * (auth-code redirect vs. redacted-log-and-rethrow), and system-role presence
 * in the rendered list.
 *
 * NOT covered here (owned by admin-roles-crud.test.tsx):
 *   - Create / update / delete mutation flows.
 *   - System-role Edit/Delete disabled state.
 *   - Mutation error banners and field-level errors.
 *
 * NOT covered here (owned by src/app/admin/layout.test.tsx):
 *   - Admin gate (x-auth-status header check, role check, redirect behaviour).
 */

// ---------------------------------------------------------------------------
// Module mocks — hoisted by Vitest before imports
// ---------------------------------------------------------------------------

// redirect() throws with this prefix so RSC tests can assert the target path,
// mirroring how Next.js's server runtime aborts the render on redirect.
const REDIRECT_PREFIX = "REDIRECT:";

vi.mock("@/lib/apollo/server", () => ({
  gqlFetch: vi.fn(),
}));

// AdminRolesClient uses Apollo mutations via useMutation; stub the provider so
// the client component mounts without a real Apollo client in scope.
vi.mock("@apollo/client/react", () => ({
  useLazyQuery: vi.fn(() => [vi.fn(), { data: null, error: null, loading: false }]),
  useMutation: vi.fn(() => [vi.fn(), { error: null, loading: false, reset: vi.fn() }]),
}));

// useUndoDelete requires <UndoDeleteProvider> in the tree; stub the hook so
// the RSC-rendered tree mounts without a real provider.
vi.mock("@/lib/undo-delete", () => ({
  useUndoDelete: vi.fn(() => ({ scheduleDelete: vi.fn() })),
  UndoDeleteProvider: ({ children }: { children: React.ReactNode }) => children,
}));

// redirect throws so the page's catch-block redirect aborts the render like
// Next.js does. usePathname / useRouter / useSearchParams are consumed by
// UndoDeleteProvider (flush-on-navigation) in the SSR-seed render tests.
vi.mock("next/navigation", () => ({
  redirect: vi.fn((path: string) => {
    throw new Error(`${REDIRECT_PREFIX}${path}`);
  }),
  usePathname: () => "/admin/roles",
  useRouter: () => ({ push: vi.fn(), refresh: vi.fn(), replace: vi.fn() }),
  useSearchParams: () => new URLSearchParams(""),
}));

// ---------------------------------------------------------------------------
// Imports — after vi.mock declarations
// ---------------------------------------------------------------------------

import { screen } from "@testing-library/react";
import { redirect } from "next/navigation";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import AdminRolesPage from "@/app/admin/roles/page";
import { gqlFetch } from "@/lib/apollo/server";
import { renderWithIntl } from "@/test/render-with-intl";

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
    renderWithIntl(tree as React.ReactElement);

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
    renderWithIntl(tree as React.ReactElement);

    // No role-name spans
    expect(screen.queryAllByTestId("admin-role-name")).toHaveLength(0);

    // The empty-state container is shown instead of the list.
    expect(screen.getByTestId("admin-roles-empty")).toBeInTheDocument();
    expect(screen.queryByTestId("admin-roles-list")).toBeNull();

    // The top-right "New role" CTA is always present.
    expect(screen.getByTestId("admin-roles-new-btn")).toBeInTheDocument();
  });

  // -------------------------------------------------------------------------
  // 3. gqlFetch error branches — auth codes redirect, others log + rethrow
  // -------------------------------------------------------------------------

  it("propagates a non-auth gqlFetch error and logs a redacted, scoped error", async () => {
    const networkErr = new Error("network failure");
    mockGqlFetchError(networkErr);

    await expect(AdminRolesPage()).rejects.toBe(networkErr);

    expect(redirect).not.toHaveBeenCalled();
    // PII redaction: only the error name is logged, never the message/object.
    expect(consoleErrorSpy).toHaveBeenCalledWith(expect.stringContaining("[admin/roles]"), {
      name: "Error",
    });
  });

  it("redirects to / on UNAUTHENTICATED without logging (admin pages → /, not /login)", async () => {
    mockGqlFetchError(
      new Error(`GraphQL errors: ${JSON.stringify([{ extensions: { code: "UNAUTHENTICATED" } }])}`),
    );

    await expect(AdminRolesPage()).rejects.toThrow(`${REDIRECT_PREFIX}/`);

    expect(redirect).toHaveBeenCalledWith("/");
    // The redirect path swallows the error cleanly — no error log.
    expect(consoleErrorSpy).not.toHaveBeenCalled();
  });

  it("redirects to / on FORBIDDEN without logging", async () => {
    mockGqlFetchError(
      new Error(`GraphQL errors: ${JSON.stringify([{ extensions: { code: "FORBIDDEN" } }])}`),
    );

    await expect(AdminRolesPage()).rejects.toThrow(`${REDIRECT_PREFIX}/`);

    expect(redirect).toHaveBeenCalledWith("/");
    expect(consoleErrorSpy).not.toHaveBeenCalled();
  });

  // -------------------------------------------------------------------------
  // 4. System role surfaced in list: "admin" row is present after SSR seed
  // -------------------------------------------------------------------------

  it("includes the admin system role row in the rendered output", async () => {
    mockRoles([ADMIN_ROLE, GENERAL_ROLE]);

    const tree = await AdminRolesPage();
    renderWithIntl(tree as React.ReactElement);

    // The admin role name must be visible
    const adminRow = screen.getByTestId("admin-role-row-role-admin");
    expect(adminRow).toBeInTheDocument();
    expect(adminRow).toHaveTextContent("admin");
  });
});

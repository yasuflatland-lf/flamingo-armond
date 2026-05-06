// @vitest-environment jsdom
/**
 * Broad page-level tests for /admin/users.
 *
 * Scope — things NOT covered by the narrower sibling tests:
 *   - admin-users-list.test.tsx  → DataTable shape, debounced search,
 *                                   in-flight guard, fetchMore error/Retry,
 *                                   FORBIDDEN banner, PII absence.
 *   - admin-users-roles.test.tsx → role checkbox assign / revoke / FORBIDDEN.
 *   - admin-layout.test.tsx      → layout gate (Supabase + me-query admin check).
 *
 * This file covers:
 *   1. AdminUsersPage RSC auth gate (Supabase no-user → redirect "/").
 *   2. AdminUsersPage RSC auth gate (Supabase error → rethrow).
 *   3. AdminUsersPage RSC renders AdminUsersClient for an authenticated user.
 *   4. User rows contain "Edit" links pointing at /admin/users/<id>/edit.
 *   5. Empty list: edges = [] → "No users." empty-state copy renders in DataTable.
 *   6. AdminUserEditPage RSC: no-user → redirect "/login".
 *   7. AdminUserEditPage RSC: Supabase auth error → rethrow.
 *   8. AdminUserEditPage RSC: user not found (adminUser null) → redirect "/admin/users".
 *   9. AdminUserEditPage RSC: renders edit form with seeded displayName / bio.
 *  10. AdminUserEditPage RSC: UNAUTHENTICATED gqlFetch error → redirect "/".
 *  11. AdminUserEditPage RSC: FORBIDDEN gqlFetch error → redirect "/".
 *  12. AdminUserEditPage RSC: non-auth error → rethrow.
 */

import { InMemoryCache } from "@apollo/client";
import { MockedProvider } from "@apollo/client/testing/react";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { AdminUsersClient } from "@/app/admin/users/AdminUsersClient";
import { ADMIN_USERS_DEFAULT_VARS, ADMIN_USERS_PAGE_SIZE } from "@/app/admin/users/queries";
import { AdminRolesDocument, AdminUsersDocument } from "@/generated/graphql";
import {
  adminRoleFixture,
  adminUserFixture,
  generalUserFixture,
  userWithoutRolesFixture,
} from "./fixtures/users";
import {
  type ApolloMockLeakSpyResult,
  installApolloMockLeakSpy,
} from "./utils/mock-apollo-paginated";
import {
  mockSupabaseServerClient,
  resetMockSupabase,
  setMockSupabaseUser,
  setMockSupabaseUserError,
} from "./utils/mock-supabase";

// ---------------------------------------------------------------------------
// Next.js stubs
// ---------------------------------------------------------------------------

const REDIRECT_PREFIX = "REDIRECT:";

const mockRouterReplace = vi.fn();

vi.mock("next/navigation", () => ({
  redirect: vi.fn((path: string) => {
    throw new Error(`${REDIRECT_PREFIX}${path}`);
  }),
  usePathname: vi.fn(() => "/admin/users"),
  useRouter: () => ({ replace: mockRouterReplace }),
  useSearchParams: () => new URLSearchParams(),
}));

vi.mock("next/link", () => ({
  default: ({
    href,
    children,
    ...rest
  }: {
    href: string;
    children: React.ReactNode;
    [key: string]: unknown;
  }) => (
    <a href={href} {...rest}>
      {children}
    </a>
  ),
}));

vi.mock("next/image", () => ({
  default: ({
    src,
    alt,
    width,
    height,
    ...rest
  }: {
    src: string;
    alt: string;
    width: number;
    height: number;
    [key: string]: unknown;
  }) => (
    // biome-ignore lint/performance/noImgElement: deliberate next/image stub for tests
    <img src={src} alt={alt} width={width} height={height} {...rest} />
  ),
}));

vi.mock("@/lib/supabase/server", () => ({
  createSupabaseServerClient: () => Promise.resolve(mockSupabaseServerClient()),
}));

vi.mock("@/lib/apollo/server", () => ({
  gqlFetch: vi.fn(),
}));

// ---------------------------------------------------------------------------
// Delayed imports (after vi.mock hoisting)
// ---------------------------------------------------------------------------

import { redirect } from "next/navigation";
import AdminUserEditPage from "@/app/admin/users/[id]/edit/page";
import AdminUsersPage from "@/app/admin/users/page";
import { gqlFetch } from "@/lib/apollo/server";

// No IntersectionObserver stub needed — the new AdminUsersClient uses
// discrete DataTable pagination, not infinite scroll.

// ---------------------------------------------------------------------------
// Fixtures helpers
// ---------------------------------------------------------------------------

type UserNode = {
  __typename: "User";
  id: string;
  displayName: string | null;
  bio: string | null;
  avatarUrl: string | null;
  lastActive: string | null;
  roles: Array<{ __typename: "Role"; id: string; name: string }>;
};

type UserEdge = {
  __typename: "UserEdge";
  cursor: string;
  node: UserNode;
};

function makeEdge(user: UserNode): UserEdge {
  return { __typename: "UserEdge", cursor: user.id, node: user };
}

function makeConnection(
  users: UserNode[],
  hasNextPage = false,
): {
  __typename: "UserConnection";
  edges: UserEdge[];
  pageInfo: {
    __typename: "PageInfo";
    hasNextPage: boolean;
    hasPreviousPage: boolean;
    startCursor: string | null;
    endCursor: string | null;
  };
  totalCount: number;
} {
  return {
    __typename: "UserConnection",
    edges: users.map(makeEdge),
    pageInfo: {
      __typename: "PageInfo",
      hasNextPage,
      hasPreviousPage: false,
      startCursor: users[0]?.id ?? null,
      endCursor: users[users.length - 1]?.id ?? null,
    },
    totalCount: users.length,
  };
}

// Cast shared fixtures to the internal UserNode shape used by Apollo mocks.
// The fixture type is a superset; the additional fields (like lastActive) are
// added with null below to satisfy the UserNode shape.
const adminUserNode: UserNode = { ...(adminUserFixture as unknown as UserNode), lastActive: null };
const generalUserNode: UserNode = {
  ...(generalUserFixture as unknown as UserNode),
  lastActive: null,
};
const noroleUserNode: UserNode = {
  ...(userWithoutRolesFixture as unknown as UserNode),
  lastActive: null,
};

/** Variables for page 0 default view — must match the exact shape AdminUsersClient uses. */
const ADMIN_USERS_VARIABLES = {
  ...ADMIN_USERS_DEFAULT_VARS,
  first: ADMIN_USERS_PAGE_SIZE,
  search: null,
  roleId: null,
  after: null,
};

/** Stub AdminRoles response (empty — keeps the toolbar dropdown simple). */
const EMPTY_ROLES_MOCK = {
  request: { query: AdminRolesDocument, variables: {} },
  result: { data: { roles: [] } },
};

/**
 * Build a `MockedProvider`-ready `{ cache, mocks }` pair pre-seeded with the
 * given users connection. Both halves are needed because AdminUsersClient
 * uses cache-first reads but MockedProvider must still match the request
 * variables exactly when the cache misses.
 */
function seedAdminUsersConnection(users: UserNode[], hasNextPage = false) {
  const connection = makeConnection(users, hasNextPage);
  const cache = new InMemoryCache();
  cache.writeQuery({
    query: AdminUsersDocument,
    variables: ADMIN_USERS_DEFAULT_VARS,
    data: { users: connection },
  });
  const mocks = [
    {
      request: { query: AdminUsersDocument, variables: ADMIN_USERS_VARIABLES },
      result: { data: { users: connection } },
    },
    EMPTY_ROLES_MOCK,
  ];
  return { cache, mocks };
}

// ---------------------------------------------------------------------------
// Console spy helpers
// ---------------------------------------------------------------------------

let leakSpy: ApolloMockLeakSpyResult;

beforeEach(() => {
  resetMockSupabase();
  mockRouterReplace.mockReset();
  leakSpy = installApolloMockLeakSpy({ operationNames: ["AdminUsers"] });
  vi.spyOn(console, "error").mockImplementation(() => {});
  vi.clearAllMocks();
});

afterEach(() => {
  leakSpy.assertNoLeaks();
  leakSpy.teardown();
  vi.restoreAllMocks();
});

// ---------------------------------------------------------------------------
// AdminUsersPage RSC — auth gate
// ---------------------------------------------------------------------------

describe("AdminUsersPage (RSC auth gate)", () => {
  it("redirects to '/' when no user is signed in", async () => {
    setMockSupabaseUser(null);

    await expect(AdminUsersPage()).rejects.toThrow(`${REDIRECT_PREFIX}/`);

    expect(redirect).toHaveBeenCalledWith("/");
  });

  it("rethrows when Supabase getUser returns an auth error", async () => {
    const boom = new Error("supabase transport failure");
    setMockSupabaseUserError(boom);

    await expect(AdminUsersPage()).rejects.toBe(boom);

    expect(redirect).not.toHaveBeenCalled();
  });

  it("renders AdminUsersClient (search box present) for an authenticated user", async () => {
    setMockSupabaseUser({ id: "u-1", email: "admin@test.test" });

    const { cache, mocks } = seedAdminUsersConnection([]);

    // AdminUsersPage returns the RSC tree; render inside MockedProvider so the
    // client component's useQuery has a provider.
    const tree = await AdminUsersPage();
    render(
      <MockedProvider mocks={mocks as never} cache={cache}>
        {tree as React.ReactElement}
      </MockedProvider>,
    );

    // The search box is the clearest unique affordance of AdminUsersClient.
    // aria-label="Filter users" matches type="search" in UsersToolbar.
    expect(screen.getByRole("searchbox", { name: /filter users/i })).toBeInTheDocument();

    // No redirect should have fired for an authenticated user.
    expect(redirect).not.toHaveBeenCalled();
  });
});

// ---------------------------------------------------------------------------
// AdminUsersClient — edit link targets and empty state
// (aspects not covered by admin-users-list.test.tsx)
// ---------------------------------------------------------------------------

describe("AdminUsersClient — edit links and empty state", () => {
  it("each user row has an Edit link in the actions dropdown pointing to /admin/users/<id>/edit", async () => {
    const ue = userEvent.setup({ delay: null });
    const users: UserNode[] = [adminUserNode, generalUserNode, noroleUserNode];
    const { cache, mocks } = seedAdminUsersConnection(users);

    render(
      <MockedProvider mocks={mocks as never} cache={cache}>
        <AdminUsersClient initialConnection={null} />
      </MockedProvider>,
    );

    // Wait for at least one display name to confirm the list has rendered.
    await screen.findByText(adminUserFixture.displayName as string);

    // Each row has an "Open user actions" trigger button. Open the first and
    // verify the Edit link target. The DataTable uses DropdownMenu — the edit
    // link only appears in the DOM once the trigger is clicked.
    for (const userNode of users) {
      // Open the actions dropdown for this user.
      const triggers = screen.getAllByRole("button", { name: /open user actions/i });
      // triggers appear in DOM order matching the table rows.
      const idx = users.indexOf(userNode);
      if (triggers[idx]) {
        await ue.click(triggers[idx]);
        // The Edit menuitem is now in the dropdown. Radix DropdownMenuItem asChild
        // sets role="menuitem" on the rendered <a>, overriding the default "link" role.
        const editLink = await screen.findByRole("menuitem", { name: /edit/i });
        expect(editLink).toHaveAttribute(
          "href",
          expect.stringContaining(`/admin/users/${userNode.id}/edit`),
        );
        // Close dropdown by pressing Escape.
        await ue.keyboard("{Escape}");
      }
    }
  });

  it("renders empty-state copy when the connection has no edges", async () => {
    const { cache, mocks } = seedAdminUsersConnection([]);

    render(
      <MockedProvider mocks={mocks as never} cache={cache}>
        <AdminUsersClient initialConnection={null} />
      </MockedProvider>,
    );

    // UsersTable renders "No users." in the empty TableCell when data is empty.
    const emptyCell = await screen.findByText("No users.");
    expect(emptyCell).toBeInTheDocument();
  });
});

// ---------------------------------------------------------------------------
// AdminUserEditPage RSC — auth gate, user-not-found, seeded form data
// ---------------------------------------------------------------------------

describe("AdminUserEditPage (RSC)", () => {
  const PARAMS = Promise.resolve({ id: adminUserFixture.id });

  it("redirects to '/login' when no user is signed in", async () => {
    setMockSupabaseUser(null);

    await expect(AdminUserEditPage({ params: PARAMS })).rejects.toThrow(`${REDIRECT_PREFIX}/login`);

    expect(redirect).toHaveBeenCalledWith("/login");
  });

  it("rethrows when Supabase getUser returns an auth error", async () => {
    const boom = new Error("supabase rsc error");
    setMockSupabaseUserError(boom);

    await expect(AdminUserEditPage({ params: PARAMS })).rejects.toBe(boom);

    expect(redirect).not.toHaveBeenCalled();
  });

  it("redirects to '/admin/users' when adminUser is null (user not found)", async () => {
    setMockSupabaseUser({ id: "u-1", email: "admin@test.test" });
    vi.mocked(gqlFetch)
      // First call → AdminUserQuery returns null user; second → AdminRolesQuery
      .mockResolvedValueOnce({ adminUser: null } as never)
      .mockResolvedValueOnce({ roles: [] } as never);

    await expect(AdminUserEditPage({ params: PARAMS })).rejects.toThrow(
      `${REDIRECT_PREFIX}/admin/users`,
    );

    expect(redirect).toHaveBeenCalledWith("/admin/users");
  });

  it("redirects to '/' on UNAUTHENTICATED gqlFetch error", async () => {
    setMockSupabaseUser({ id: "u-1", email: "admin@test.test" });
    vi.mocked(gqlFetch).mockRejectedValue(
      new Error('GraphQL errors: [{"message":"UNAUTHENTICATED: session expired"}]'),
    );

    await expect(AdminUserEditPage({ params: PARAMS })).rejects.toThrow(`${REDIRECT_PREFIX}/`);

    expect(redirect).toHaveBeenCalledWith("/");
  });

  it("redirects to '/' on FORBIDDEN gqlFetch error", async () => {
    setMockSupabaseUser({ id: "u-1", email: "admin@test.test" });
    vi.mocked(gqlFetch).mockRejectedValue(
      new Error('GraphQL errors: [{"message":"FORBIDDEN: admin only"}]'),
    );

    await expect(AdminUserEditPage({ params: PARAMS })).rejects.toThrow(`${REDIRECT_PREFIX}/`);

    expect(redirect).toHaveBeenCalledWith("/");
  });

  it("rethrows non-auth gqlFetch errors so the error boundary handles them", async () => {
    setMockSupabaseUser({ id: "u-1", email: "admin@test.test" });
    const networkErr = new Error("network down");
    vi.mocked(gqlFetch).mockRejectedValue(networkErr);

    await expect(AdminUserEditPage({ params: PARAMS })).rejects.toBe(networkErr);

    expect(redirect).not.toHaveBeenCalled();
  });

  it("renders the edit form pre-populated with the seeded user displayName and bio", async () => {
    setMockSupabaseUser({ id: "u-1", email: "admin@test.test" });

    const seededUser = {
      ...adminUserFixture,
      bio: "Seeded bio text",
      roles: [{ __typename: "Role", id: adminRoleFixture.id, name: adminRoleFixture.name }],
    };

    // gqlFetch is called twice in parallel: AdminUserQuery + AdminRolesQuery.
    vi.mocked(gqlFetch)
      .mockResolvedValueOnce({ adminUser: seededUser } as never)
      .mockResolvedValueOnce({ roles: [adminRoleFixture] } as never);

    const tree = await AdminUserEditPage({ params: PARAMS });

    render(<MockedProvider mocks={[]}>{tree as React.ReactElement}</MockedProvider>);

    // The display name field must be pre-populated.
    const displayNameInput = screen.getByLabelText<HTMLInputElement>(/display name/i);
    expect(displayNameInput.value).toBe(seededUser.displayName);

    // The bio textarea must be pre-populated.
    const bioTextarea = screen.getByLabelText<HTMLTextAreaElement>(/bio/i);
    expect(bioTextarea.value).toBe(seededUser.bio);

    // The roles section must show the admin role checkbox (checked state is
    // tested in admin-users-roles.test.tsx; here we verify it is rendered).
    expect(screen.getByRole("checkbox", { name: /admin/i })).toBeInTheDocument();

    // Back-to-users navigation link must be present.
    expect(screen.getByRole("link", { name: /back to users/i })).toBeInTheDocument();
  });
});

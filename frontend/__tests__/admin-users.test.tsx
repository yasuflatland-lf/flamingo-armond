// @vitest-environment jsdom
/**
 * Broad page-level tests for /admin/users.
 *
 * Scope — things NOT covered by the narrower sibling tests:
 *   - admin-users-list.test.tsx  → infinite-scroll IO, debounced search,
 *                                   in-flight guard, fetchMore error/Retry,
 *                                   FORBIDDEN banner, NetworkStatus indicator.
 *   - admin-users-roles.test.tsx → role checkbox assign / revoke / FORBIDDEN.
 *   - admin-layout.test.tsx      → layout gate (Supabase + me-query admin check).
 *
 * This file covers:
 *   1. AdminUsersPage RSC auth gate (Supabase no-user → redirect "/").
 *   2. AdminUsersPage RSC auth gate (Supabase error → rethrow).
 *   3. AdminUsersPage RSC renders AdminUsersClient for an authenticated user.
 *   4. User rows expose an Edit affordance that opens `?edit=<id>`.
 *   5. Empty list: edges = [] → "No users found." empty-state copy renders.
 */

import { InMemoryCache } from "@apollo/client";
import { MockedProvider } from "@apollo/client/testing/react";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { AdminUsersClient } from "@/app/admin/users/admin-users-client";
import { ADMIN_USERS_PAGE_SIZE } from "@/app/admin/users/queries";
import { AdminRolesDocument, AdminUsersDocument } from "@/generated/graphql";
import { adminUserFixture, generalUserFixture, userWithoutRolesFixture } from "./fixtures/users";
import {
  type ApolloMockLeakSpyResult,
  installApolloMockLeakSpy,
} from "./utils/mock-apollo-paginated";
import {
  mockCreateSupabaseServerClient,
  resetMockSupabase,
  setMockSupabaseUser,
  setMockSupabaseUserError,
} from "./utils/mock-supabase";

// ---------------------------------------------------------------------------
// Next.js stubs
// ---------------------------------------------------------------------------

const REDIRECT_PREFIX = "REDIRECT:";
const mockPush = vi.fn();
const mockReplace = vi.fn();
const mockRefresh = vi.fn();

vi.mock("next/navigation", () => ({
  redirect: vi.fn((path: string) => {
    throw new Error(`${REDIRECT_PREFIX}${path}`);
  }),
  usePathname: vi.fn(() => "/admin/users"),
  useRouter: () => ({
    push: mockPush,
    refresh: mockRefresh,
    replace: mockReplace,
  }),
  useSearchParams: () => new URLSearchParams(""),
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
  createSupabaseServerClient: mockCreateSupabaseServerClient,
}));

vi.mock("@/lib/apollo/server", () => ({
  gqlFetch: vi.fn(),
}));

// ---------------------------------------------------------------------------
// Delayed imports (after vi.mock hoisting)
// ---------------------------------------------------------------------------

import { redirect } from "next/navigation";
import AdminUsersPage from "@/app/admin/users/page";

// ---------------------------------------------------------------------------
// IntersectionObserver stub (required by AdminUsersClient)
// ---------------------------------------------------------------------------

class FakeIntersectionObserver {
  observe() {}
  unobserve() {}
  disconnect() {}
  takeRecords(): IntersectionObserverEntry[] {
    return [];
  }
}

// ---------------------------------------------------------------------------
// Fixtures helpers
// ---------------------------------------------------------------------------

type UserNode = {
  __typename: "User";
  id: string;
  displayName: string;
  bio: string | null;
  avatarUrl: string | null;
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
// The fixture type is a superset; the additional fields (like createdAt) are
// harmless for query mocks.
const adminUserNode = adminUserFixture as unknown as UserNode;
const generalUserNode = generalUserFixture as unknown as UserNode;
const noroleUserNode = userWithoutRolesFixture as unknown as UserNode;

const ADMIN_USERS_VARIABLES = { first: ADMIN_USERS_PAGE_SIZE, search: null };
const ADMIN_ROLES_MOCK = {
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
    variables: ADMIN_USERS_VARIABLES,
    data: { users: connection },
  });
  const mocks = [
    {
      request: { query: AdminUsersDocument, variables: ADMIN_USERS_VARIABLES },
      result: { data: { users: connection } },
    },
    ADMIN_ROLES_MOCK,
  ];
  return { cache, mocks };
}

// ---------------------------------------------------------------------------
// Console spy helpers
// ---------------------------------------------------------------------------

let leakSpy: ApolloMockLeakSpyResult;

beforeEach(() => {
  resetMockSupabase();
  vi.stubGlobal("IntersectionObserver", FakeIntersectionObserver);
  leakSpy = installApolloMockLeakSpy({ operationNames: ["AdminUsers", "AdminRoles"] });
  vi.spyOn(console, "error").mockImplementation(() => {});
  mockPush.mockReset();
  mockReplace.mockReset();
  mockRefresh.mockReset();
  vi.clearAllMocks();
});

afterEach(() => {
  leakSpy.assertNoLeaks();
  leakSpy.teardown();
  vi.unstubAllGlobals();
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
    expect(screen.getByRole("searchbox", { name: /search users/i })).toBeInTheDocument();

    // No redirect should have fired for an authenticated user.
    expect(redirect).not.toHaveBeenCalled();
  });
});

// ---------------------------------------------------------------------------
// AdminUsersClient — edit link targets and empty state
// (aspects not covered by admin-users-list.test.tsx)
// ---------------------------------------------------------------------------

describe("AdminUsersClient — edit affordance and empty state", () => {
  it("each user row has an Edit button that opens ?edit=<id>", async () => {
    const user = userEvent.setup();
    const users: UserNode[] = [adminUserNode, generalUserNode, noroleUserNode];
    const { cache, mocks } = seedAdminUsersConnection(users);

    render(
      <MockedProvider mocks={mocks as never} cache={cache}>
        <AdminUsersClient />
      </MockedProvider>,
    );

    // Wait for at least one display name to confirm the list has rendered.
    await screen.findByText(adminUserFixture.displayName as string);

    for (const userNode of users) {
      const row = screen.getByTestId(`admin-user-row-${userNode.id}`);
      await user.click(screen.getByRole("button", { name: `Edit ${userNode.displayName}` }));
      expect(row).toBeInTheDocument();
      expect(mockPush).toHaveBeenLastCalledWith(`/admin/users?edit=${userNode.id}`, {
        scroll: false,
      });
    }
  });

  it("renders empty-state copy when the connection has no edges", async () => {
    const { cache, mocks } = seedAdminUsersConnection([]);

    render(
      <MockedProvider mocks={mocks as never} cache={cache}>
        <AdminUsersClient />
      </MockedProvider>,
    );

    const empty = await screen.findByTestId("admin-users-empty");
    expect(empty).toHaveTextContent("No users found.");
    // The user list must not appear.
    expect(screen.queryByTestId("admin-users-list")).not.toBeInTheDocument();
  });
});

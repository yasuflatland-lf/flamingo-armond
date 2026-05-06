// @vitest-environment jsdom
/**
 * DataTable-shape tests for AdminUsersClient.
 *
 * Covers:
 *   T1  — initial render: column headers, avatar/initials, display name, role badges, relative time
 *   T2  — search debounce: exactly one query fires after the 300ms window
 *   T3  — role filter: query fires with roleId variable after selecting a role
 *   T4  — discrete pagination forward walk: next-page button fires fetchMore
 *   T5  — discrete pagination backward: prev-page uses cached cursor, no new request
 *   T6  — filter change resets pagination: role filter resets pageIndex to 0
 *   T7  — fetchMore error halts navigation + Retry re-issues (two MockedResponse entries)
 *   T8  — MockedProvider leak detection via installApolloMockLeakSpy / assertNoLeaks
 *   T9  — PII absence: fetchMore warn payload has no email / displayName / bio keys
 *   T10 — FORBIDDEN query error: permission-denied banner, no Retry button
 *   T11 — fetchMore FORBIDDEN: permission-denied banner, no Retry button
 *   T12 — fetchMore UNAUTHENTICATED: triggers router.replace("/")
 *   T13 — AdminRoles query failure: structured warn for operator triage
 *
 * Apollo discrete-pagination note: AdminUsersClient uses fetchMore + setPageIndex.
 * After fetchMore resolves, setPageIndex changes queryVariables (adding the new cursor),
 * which triggers a fresh useQuery network call for the new variables. MockedProvider
 * consumes each mock entry once, so each page-2 transition needs TWO mocks:
 *   Mock A — consumed by the fetchMore call
 *   Mock B — consumed by the subsequent useQuery call (for new variables after setPageIndex)
 */

import { MockedProvider } from "@apollo/client/testing/react";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { GraphQLError } from "graphql";
import { afterEach, beforeEach, describe, expect, test, vi } from "vitest";
import { AdminUsersClient } from "@/app/admin/users/AdminUsersClient";
import { ADMIN_USERS_DEFAULT_VARS, ADMIN_USERS_PAGE_SIZE } from "@/app/admin/users/queries";
import { AdminRolesDocument, AdminUsersDocument } from "@/generated/graphql";
import {
  type ApolloMockLeakSpyResult,
  installApolloMockLeakSpy,
} from "./utils/mock-apollo-paginated";

// ---------------------------------------------------------------------------
// Next.js stubs — AdminUsersClient uses useRouter, useSearchParams, redirect
// ---------------------------------------------------------------------------

const mockRouterReplace = vi.fn();

vi.mock("next/navigation", () => ({
  redirect: vi.fn(),
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

// ---------------------------------------------------------------------------
// Browser API stubs — cmdk (Popover/Command) uses ResizeObserver and
// scrollIntoView; jsdom does not implement either.
// ---------------------------------------------------------------------------

class FakeResizeObserver {
  observe() {}
  unobserve() {}
  disconnect() {}
}

// ---------------------------------------------------------------------------
// Data fixtures
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

function makeUser(i: number, overrides: Partial<UserNode> = {}): UserNode {
  return {
    __typename: "User",
    id: `user-${i}`,
    displayName: `User ${i}`,
    bio: null,
    avatarUrl: null,
    lastActive: null,
    roles: [{ __typename: "Role", id: "role-general", name: "general" }],
    ...overrides,
  };
}

function makeEdge(user: UserNode): UserEdge {
  return { __typename: "UserEdge", cursor: user.id, node: user };
}

function makeConnection(
  users: UserNode[],
  hasNextPage: boolean,
  totalCount?: number,
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
    totalCount: totalCount ?? users.length,
  };
}

/**
 * Variables that AdminUsersClient sends for the initial page-0 request.
 * The component always includes after: null in queryVariables even on page 0.
 */
const PAGE_0_VARS = {
  ...ADMIN_USERS_DEFAULT_VARS,
  first: ADMIN_USERS_PAGE_SIZE,
  search: null,
  roleId: null,
  after: null,
};

/** Stub AdminRoles response (no roles — keeps toolbar simple). */
const EMPTY_ROLES_MOCK = {
  request: { query: AdminRolesDocument, variables: {} },
  result: { data: { roles: [] } },
};

/** Stub AdminRoles with one "admin" role. */
const ADMIN_ROLE_MOCK = {
  request: { query: AdminRolesDocument, variables: {} },
  result: {
    data: {
      roles: [{ __typename: "Role", id: "role-admin", name: "admin" }],
    },
  },
};

// ---------------------------------------------------------------------------
// Leak spy — installed per-test via beforeEach/afterEach
// ---------------------------------------------------------------------------

let leakSpy: ApolloMockLeakSpyResult;

beforeEach(() => {
  mockRouterReplace.mockReset();
  vi.stubGlobal("ResizeObserver", FakeResizeObserver);
  // scrollIntoView is used by cmdk when the Command popover mounts.
  window.HTMLElement.prototype.scrollIntoView = vi.fn();
  leakSpy = installApolloMockLeakSpy({ operationNames: ["AdminUsers"] });
  vi.spyOn(console, "error").mockImplementation(() => {});
});

afterEach(() => {
  leakSpy.assertNoLeaks();
  leakSpy.teardown();
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
  vi.useRealTimers();
});

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

describe("AdminUsersClient — DataTable shape", () => {
  // T1 — Initial render: column headers, initials fallback, display name, role badge, relative time.
  test("T1: renders column headers, user rows with initials, display name, role badges, and relative time", async () => {
    const now = new Date().toISOString();
    const users: UserNode[] = [
      // User 1: admin role — badge uses "admin" variant
      makeUser(1, {
        roles: [{ __typename: "Role", id: "role-admin", name: "admin" }],
      }),
      // User 2: has lastActive = now → "Just now"
      makeUser(2, { lastActive: now }),
      // User 3: null displayName → initials "?" and "No name" span
      makeUser(3, { displayName: null }),
    ];
    const connection = makeConnection(users, false, 3);

    render(
      <MockedProvider
        mocks={[
          {
            request: { query: AdminUsersDocument, variables: PAGE_0_VARS },
            result: { data: { users: connection } },
          },
          EMPTY_ROLES_MOCK,
        ]}
      >
        <AdminUsersClient initialConnection={null} />
      </MockedProvider>,
    );

    // Column headers (Name, Roles, Last active, Actions sr-only)
    expect(await screen.findByText("Name")).toBeInTheDocument();
    expect(screen.getByText("Roles")).toBeInTheDocument();
    expect(screen.getByText("Last active")).toBeInTheDocument();
    // Actions column header has sr-only text
    const actionHeader = document.querySelector("th span.sr-only");
    expect(actionHeader).not.toBeNull();

    // Display names
    expect(await screen.findByText("User 1")).toBeInTheDocument();
    expect(screen.getByText("User 2")).toBeInTheDocument();

    // Null displayName → "No name" italic span
    expect(screen.getByText("No name")).toBeInTheDocument();

    // Admin role badge for User 1 (admin variant)
    expect(screen.getByText("admin")).toBeInTheDocument();

    // General role badge(s)
    const generalBadges = screen.getAllByText("general");
    expect(generalBadges.length).toBeGreaterThanOrEqual(1);

    // User 2 lastActive = now → "Just now"
    expect(screen.getByText("Just now")).toBeInTheDocument();

    // Users 1 and 3 have null lastActive → "Never" span
    const neverSpans = screen.getAllByText("Never");
    expect(neverSpans.length).toBeGreaterThanOrEqual(1);

    // User 3 null displayName → initials fallback "?"
    expect(screen.getByText("?")).toBeInTheDocument();
  });

  // T2 — Search debounce: exactly one AdminUsers query fires after 300ms.
  test("T2: debounced search fires exactly one AdminUsers query after 300ms", async () => {
    const ue = userEvent.setup({ delay: null });
    vi.useFakeTimers({ shouldAdvanceTime: true });

    const initialUsers = [makeUser(1), makeUser(2)];
    const searchUsers = [makeUser(99, { displayName: "Alice", id: "user-alice" })];

    let searchQueryCalls = 0;
    const searchResult = vi.fn(() => {
      searchQueryCalls += 1;
      return { data: { users: makeConnection(searchUsers, false) } };
    });

    const searchVars = {
      ...ADMIN_USERS_DEFAULT_VARS,
      first: ADMIN_USERS_PAGE_SIZE,
      search: "ali",
      roleId: null,
      after: null,
    };

    render(
      <MockedProvider
        mocks={[
          {
            request: { query: AdminUsersDocument, variables: PAGE_0_VARS },
            result: { data: { users: makeConnection(initialUsers, false) } },
          },
          {
            request: { query: AdminUsersDocument, variables: searchVars },
            result: searchResult,
          },
          EMPTY_ROLES_MOCK,
        ]}
      >
        <AdminUsersClient initialConnection={null} />
      </MockedProvider>,
    );

    // Wait for initial render.
    expect(await screen.findByText("User 1")).toBeInTheDocument();

    // aria-label="Filter users" on type="search" input in UsersToolbar.
    const searchInput = screen.getByRole("searchbox", { name: /filter users/i });
    await ue.type(searchInput, "ali");

    // Advance past the 300ms debounce window.
    vi.advanceTimersByTime(350);
    vi.useRealTimers();

    await waitFor(() => {
      expect(screen.getByText("Alice")).toBeInTheDocument();
    });

    // Exactly one query for "ali" — not once per keystroke.
    expect(searchQueryCalls).toBe(1);
  });

  // T3 — Role filter: clicking a role emits a query with roleId.
  test("T3: role filter fires AdminUsers with roleId after selecting a role from the popover", async () => {
    const ue = userEvent.setup({ delay: null });

    const initialUsers = [makeUser(1)];
    const filteredUsers = [makeUser(1, { displayName: "Admin Alice", id: "user-admin-alice" })];

    let roleQueryCalls = 0;
    const roleResult = vi.fn(() => {
      roleQueryCalls += 1;
      return { data: { users: makeConnection(filteredUsers, false) } };
    });

    const roleVars = {
      ...ADMIN_USERS_DEFAULT_VARS,
      first: ADMIN_USERS_PAGE_SIZE,
      search: null,
      roleId: "role-admin",
      after: null,
    };

    render(
      <MockedProvider
        mocks={[
          {
            request: { query: AdminUsersDocument, variables: PAGE_0_VARS },
            result: { data: { users: makeConnection(initialUsers, false) } },
          },
          {
            request: { query: AdminUsersDocument, variables: roleVars },
            result: roleResult,
          },
          // AdminRoles query — needed for the role popover content.
          ADMIN_ROLE_MOCK,
        ]}
      >
        <AdminUsersClient initialConnection={null} />
      </MockedProvider>,
    );

    // Wait for initial users.
    expect(await screen.findByText("User 1")).toBeInTheDocument();

    // Open the Role filter popover.
    const roleButton = screen.getByRole("button", { name: /role/i });
    await ue.click(roleButton);

    // The "admin" role item appears in the command palette.
    const adminItem = await screen.findByText("admin");
    await ue.click(adminItem);

    await waitFor(() => {
      expect(roleQueryCalls).toBeGreaterThanOrEqual(1);
    });

    // After filtering, "Admin Alice" should appear.
    expect(await screen.findByText("Admin Alice")).toBeInTheDocument();
  });

  // T4 — Discrete pagination forward: next-page button fires fetchMore.
  //
  // Apollo discrete-pagination note: after fetchMore resolves, setPageIndex changes
  // queryVariables (adding the cursor), triggering a second useQuery for the new
  // variables. MockedProvider consumes each mock once, so we need two mocks for
  // the fetchMore variables: one for the fetchMore call, one for the subsequent useQuery.
  test("T4: clicking next-page button fires fetchMore with the page-1 endCursor", async () => {
    const ue = userEvent.setup({ delay: null });

    const page1Users = Array.from({ length: ADMIN_USERS_PAGE_SIZE }, (_, i) => makeUser(i + 1));
    const page2Users = Array.from({ length: 5 }, (_, i) => makeUser(i + 101));
    const endCursor = `user-${ADMIN_USERS_PAGE_SIZE}`;

    const page1Connection = makeConnection(page1Users, true, ADMIN_USERS_PAGE_SIZE + 5);
    const page2Connection = makeConnection(page2Users, false, ADMIN_USERS_PAGE_SIZE + 5);

    let nextPageCalls = 0;
    const nextPageResult = vi.fn(() => {
      nextPageCalls += 1;
      return { data: { users: page2Connection } };
    });

    const fetchMoreVars = {
      ...ADMIN_USERS_DEFAULT_VARS,
      first: ADMIN_USERS_PAGE_SIZE,
      search: null,
      roleId: null,
      after: endCursor,
    };

    render(
      <MockedProvider
        mocks={[
          // Mock 1: initial page-0 useQuery
          {
            request: { query: AdminUsersDocument, variables: PAGE_0_VARS },
            result: { data: { users: page1Connection } },
          },
          // Mock 2: fetchMore call (consumed by fetchMore)
          {
            request: { query: AdminUsersDocument, variables: fetchMoreVars },
            result: nextPageResult,
          },
          // Mock 3: useQuery re-fires with page-1 variables after setPageIndex(1)
          {
            request: { query: AdminUsersDocument, variables: fetchMoreVars },
            result: nextPageResult,
          },
          EMPTY_ROLES_MOCK,
        ]}
      >
        <AdminUsersClient initialConnection={null} />
      </MockedProvider>,
    );

    // Page 1 renders.
    expect(await screen.findByText("User 1")).toBeInTheDocument();
    expect(screen.getByText(`User ${ADMIN_USERS_PAGE_SIZE}`)).toBeInTheDocument();

    // DataTablePagination has one "Go to next page" button.
    const nextButtons = screen.getAllByRole("button", { name: /go to next page/i });
    const nextButton = nextButtons[0] as HTMLElement;
    expect(nextButton).not.toBeDisabled();
    await ue.click(nextButton);

    // Page 2 renders after fetchMore resolves.
    await waitFor(() => {
      expect(screen.getByText("User 101")).toBeInTheDocument();
    });

    // nextPageCalls includes both fetchMore AND the subsequent useQuery.
    expect(nextPageCalls).toBeGreaterThanOrEqual(1);
  });

  // T5 — Discrete pagination backward: prev-page uses cached page-0 cursor (null),
  // so NO new fetchMore fires. The existing mock pair from forward nav serves useQuery.
  test("T5: going back to page 0 after advancing uses cached cursor and fires no new request", async () => {
    const ue = userEvent.setup({ delay: null });

    const page1Users = Array.from({ length: ADMIN_USERS_PAGE_SIZE }, (_, i) => makeUser(i + 1));
    const page2Users = Array.from({ length: 5 }, (_, i) => makeUser(i + 101));
    const endCursor = `user-${ADMIN_USERS_PAGE_SIZE}`;

    const page1Connection = makeConnection(page1Users, true, ADMIN_USERS_PAGE_SIZE + 5);
    const page2Connection = makeConnection(page2Users, false, ADMIN_USERS_PAGE_SIZE + 5);

    let nextPageCalls = 0;
    const nextPageResult = vi.fn(() => {
      nextPageCalls += 1;
      return { data: { users: page2Connection } };
    });

    const fetchMoreVars = {
      ...ADMIN_USERS_DEFAULT_VARS,
      first: ADMIN_USERS_PAGE_SIZE,
      search: null,
      roleId: null,
      after: endCursor,
    };

    render(
      <MockedProvider
        mocks={[
          // Mock 1: initial page-0 useQuery
          {
            request: { query: AdminUsersDocument, variables: PAGE_0_VARS },
            result: { data: { users: page1Connection } },
          },
          // Mock 2: fetchMore call
          {
            request: { query: AdminUsersDocument, variables: fetchMoreVars },
            result: nextPageResult,
          },
          // Mock 3: useQuery re-fires after setPageIndex(1)
          {
            request: { query: AdminUsersDocument, variables: fetchMoreVars },
            result: nextPageResult,
          },
          // Mock 4: page-0 useQuery again after going back (cache-first: may be cache hit,
          // but provide a mock in case of a cache miss to avoid a leak).
          {
            request: { query: AdminUsersDocument, variables: PAGE_0_VARS },
            result: { data: { users: page1Connection } },
          },
          EMPTY_ROLES_MOCK,
        ]}
      >
        <AdminUsersClient initialConnection={null} />
      </MockedProvider>,
    );

    // Page 1.
    expect(await screen.findByText("User 1")).toBeInTheDocument();

    // Navigate to page 2.
    const nextButtons = screen.getAllByRole("button", { name: /go to next page/i });
    await ue.click(nextButtons[0] as HTMLElement);

    await waitFor(() => {
      expect(screen.getByText("User 101")).toBeInTheDocument();
    });
    const callsAfterForward = nextPageCalls;
    expect(callsAfterForward).toBeGreaterThanOrEqual(1);

    // Navigate back to page 1 using the "Go to previous page" button.
    // handlePageChange checks cursorByPage.has(0) → true → just setPageIndex(0).
    // No new fetchMore fires. useQuery with PAGE_0_VARS may be a cache hit.
    const prevButtons = screen.getAllByRole("button", { name: /go to previous page/i });
    await ue.click(prevButtons[0] as HTMLElement);

    // After navigating back, nextPageCalls must NOT have increased from a new fetchMore.
    await waitFor(() => {
      expect(nextPageCalls).toBe(callsAfterForward);
    });
  });

  // T6 — Filter change resets pagination: advancing to page 2 then applying a role
  // filter resets pageIndex to 0 and fires a fresh query (not a fetchMore).
  test("T6: applying a role filter after advancing to page 2 resets pageIndex to 0", async () => {
    const ue = userEvent.setup({ delay: null });

    const page1Users = Array.from({ length: ADMIN_USERS_PAGE_SIZE }, (_, i) => makeUser(i + 1));
    const page2Users = Array.from({ length: 5 }, (_, i) => makeUser(i + 101));
    const filteredUsers = [makeUser(200, { displayName: "Filtered User" })];
    const endCursor = `user-${ADMIN_USERS_PAGE_SIZE}`;

    const page1Connection = makeConnection(page1Users, true, ADMIN_USERS_PAGE_SIZE + 5);
    const page2Connection = makeConnection(page2Users, false, ADMIN_USERS_PAGE_SIZE + 5);

    let filteredQueryCalls = 0;
    const filteredResult = vi.fn(() => {
      filteredQueryCalls += 1;
      return { data: { users: makeConnection(filteredUsers, false, 1) } };
    });

    const fetchMoreVars = {
      ...ADMIN_USERS_DEFAULT_VARS,
      first: ADMIN_USERS_PAGE_SIZE,
      search: null,
      roleId: null,
      after: endCursor,
    };

    const roleFilterVars = {
      ...ADMIN_USERS_DEFAULT_VARS,
      first: ADMIN_USERS_PAGE_SIZE,
      search: null,
      roleId: "role-admin",
      after: null,
    };

    // Stale intermediate variables: React re-renders with the new roleFilter but
    // the OLD cursorByPage (still containing user-20 for page 1) before the
    // filter-reset useEffect fires and resets cursorByPage to {0: null}. Apollo
    // fires a useQuery for this stale shape — it needs a mock to avoid a leak.
    const staleRoleVars = {
      ...ADMIN_USERS_DEFAULT_VARS,
      first: ADMIN_USERS_PAGE_SIZE,
      search: null,
      roleId: "role-admin",
      after: endCursor, // old cursor from page 1
    };

    render(
      <MockedProvider
        mocks={[
          // Mock 1: initial page-0 useQuery
          {
            request: { query: AdminUsersDocument, variables: PAGE_0_VARS },
            result: { data: { users: page1Connection } },
          },
          // Mock 2: fetchMore to page 2
          {
            request: { query: AdminUsersDocument, variables: fetchMoreVars },
            result: { data: { users: page2Connection } },
          },
          // Mock 3: useQuery re-fires after setPageIndex(1)
          {
            request: { query: AdminUsersDocument, variables: fetchMoreVars },
            result: { data: { users: page2Connection } },
          },
          // Mock 4 (stale): React re-renders with new roleId but old cursor before
          // the filter-reset useEffect resets cursorByPage. Apollo fires for this
          // stale shape; it resolves to page2Connection and is immediately overwritten
          // once the effect resets state to page 0.
          {
            request: { query: AdminUsersDocument, variables: staleRoleVars },
            result: { data: { users: page2Connection } },
          },
          // Mock 5: filtered query fires after role filter applied (pageIndex reset to 0)
          {
            request: { query: AdminUsersDocument, variables: roleFilterVars },
            result: filteredResult,
          },
          ADMIN_ROLE_MOCK,
        ]}
      >
        <AdminUsersClient initialConnection={null} />
      </MockedProvider>,
    );

    // Page 1.
    expect(await screen.findByText("User 1")).toBeInTheDocument();

    // Navigate to page 2.
    const nextButtons = screen.getAllByRole("button", { name: /go to next page/i });
    await ue.click(nextButtons[0] as HTMLElement);
    await waitFor(() => {
      expect(screen.getByText("User 101")).toBeInTheDocument();
    });

    // Confirm page indicator shows "Page 2 of ...".
    expect(screen.getByText(/page 2 of/i)).toBeInTheDocument();

    // Apply role filter — this must reset pageIndex to 0.
    const roleButton = screen.getByRole("button", { name: /role/i });
    await ue.click(roleButton);
    const adminItem = await screen.findByText("admin");
    await ue.click(adminItem);

    await waitFor(() => {
      expect(filteredQueryCalls).toBeGreaterThanOrEqual(1);
    });

    // Page indicator must reset to "Page 1 of ...".
    await waitFor(() => {
      expect(screen.getByText(/page 1 of/i)).toBeInTheDocument();
    });
  });

  // T7 — fetchMore error: halts navigation, shows Retry. Two MockedResponse entries
  // for the same cursor variables: first errors, second succeeds.
  // See .claude/rules/pagination.md § "Provide two MockedResponse entries to test a
  // Retry-after-error path".
  test("T7: fetchMore error shows banner, and Retry re-issues the request successfully", async () => {
    const ue = userEvent.setup({ delay: null });

    const page1Users = Array.from({ length: ADMIN_USERS_PAGE_SIZE }, (_, i) => makeUser(i + 1));
    const page2Users = Array.from({ length: 5 }, (_, i) => makeUser(i + 101));
    const endCursor = `user-${ADMIN_USERS_PAGE_SIZE}`;

    const page1Connection = makeConnection(page1Users, true, ADMIN_USERS_PAGE_SIZE + 5);
    const page2Connection = makeConnection(page2Users, false, ADMIN_USERS_PAGE_SIZE + 5);

    let fetchMoreCallCount = 0;

    const fetchMoreVars = {
      ...ADMIN_USERS_DEFAULT_VARS,
      first: ADMIN_USERS_PAGE_SIZE,
      search: null,
      roleId: null,
      after: endCursor,
    };

    render(
      <MockedProvider
        mocks={[
          // Mock 1: initial page-0 useQuery
          {
            request: { query: AdminUsersDocument, variables: PAGE_0_VARS },
            result: { data: { users: page1Connection } },
          },
          // Mock 2: first fetchMore — FAILS (consumed by the next-page click)
          {
            request: { query: AdminUsersDocument, variables: fetchMoreVars },
            result: vi.fn(() => {
              fetchMoreCallCount += 1;
              return {
                errors: [{ message: "Could not load page. Please try again." }],
              };
            }),
          },
          // Mock 3: Retry fetchMore — SUCCEEDS
          {
            request: { query: AdminUsersDocument, variables: fetchMoreVars },
            result: vi.fn(() => {
              fetchMoreCallCount += 1;
              return { data: { users: page2Connection } };
            }),
          },
          // Mock 4: useQuery re-fires after setPageIndex(1) on successful Retry
          {
            request: { query: AdminUsersDocument, variables: fetchMoreVars },
            result: { data: { users: page2Connection } },
          },
          EMPTY_ROLES_MOCK,
        ]}
      >
        <AdminUsersClient initialConnection={null} />
      </MockedProvider>,
    );

    // Page 1 renders.
    expect(await screen.findByText("User 1")).toBeInTheDocument();

    // Click next — fetchMore fails.
    const nextButtons = screen.getAllByRole("button", { name: /go to next page/i });
    await ue.click(nextButtons[0] as HTMLElement);

    // Error banner with Retry appears.
    const errorBanner = await screen.findByTestId("admin-users-fetch-more-error");
    expect(errorBanner).toBeInTheDocument();
    // fetchMoreCallCount = 1 (one failed fetchMore)
    expect(fetchMoreCallCount).toBe(1);

    // Page 1 data still visible.
    expect(screen.getByText("User 1")).toBeInTheDocument();

    // Retry button present.
    const retryButton = screen.getByRole("button", { name: /retry/i });
    expect(retryButton).toBeInTheDocument();

    // Click Retry — clears banner and fires the second (success) fetchMore.
    await ue.click(retryButton);

    await waitFor(() => {
      expect(screen.getByText("User 101")).toBeInTheDocument();
    });

    expect(screen.queryByTestId("admin-users-fetch-more-error")).not.toBeInTheDocument();
    // fetchMoreCallCount is now 2 (1 fail + 1 success via Retry).
    expect(fetchMoreCallCount).toBe(2);
  });

  // T8 — MockedProvider leak detection: the in-flight guard prevents a double-fetch.
  // Stage exactly TWO mocks for next-page variables (one for fetchMore, one for the
  // subsequent useQuery). A guard bypass would fire an EXTRA fetchMore, consuming both
  // and leaving the useQuery with no mock → assertNoLeaks catches it.
  test("T8: in-flight guard prevents double fetchMore — leak spy catches any leak", async () => {
    const ue = userEvent.setup({ delay: null });

    const page1Users = Array.from({ length: ADMIN_USERS_PAGE_SIZE }, (_, i) => makeUser(i + 1));
    const page2Users = Array.from({ length: 5 }, (_, i) => makeUser(i + 101));
    const endCursor = `user-${ADMIN_USERS_PAGE_SIZE}`;

    const page1Connection = makeConnection(page1Users, true, ADMIN_USERS_PAGE_SIZE + 5);
    const page2Connection = makeConnection(page2Users, false, ADMIN_USERS_PAGE_SIZE + 5);

    let nextPageCalls = 0;
    const nextPageResult = vi.fn(() => {
      nextPageCalls += 1;
      return { data: { users: page2Connection } };
    });

    const fetchMoreVars = {
      ...ADMIN_USERS_DEFAULT_VARS,
      first: ADMIN_USERS_PAGE_SIZE,
      search: null,
      roleId: null,
      after: endCursor,
    };

    render(
      <MockedProvider
        mocks={[
          // Mock 1: initial page-0 useQuery
          {
            request: { query: AdminUsersDocument, variables: PAGE_0_VARS },
            result: { data: { users: page1Connection } },
          },
          // Mock 2: ONE fetchMore mock. If the guard is broken and a second fetchMore
          // fires, it consumes this mock and the useQuery mock has no entry →
          // MockedProvider warns → assertNoLeaks fails the test.
          {
            request: { query: AdminUsersDocument, variables: fetchMoreVars },
            result: nextPageResult,
          },
          // Mock 3: useQuery after setPageIndex(1) — correctly consumed when guard works.
          {
            request: { query: AdminUsersDocument, variables: fetchMoreVars },
            result: nextPageResult,
          },
          EMPTY_ROLES_MOCK,
        ]}
      >
        <AdminUsersClient initialConnection={null} />
      </MockedProvider>,
    );

    expect(await screen.findByText("User 1")).toBeInTheDocument();

    const nextButtons = screen.getAllByRole("button", { name: /go to next page/i });

    // Click next twice back-to-back — in-flight guard must absorb the second.
    await ue.click(nextButtons[0] as HTMLElement);
    await ue.click(nextButtons[0] as HTMLElement);

    await waitFor(() => {
      expect(screen.getByText("User 101")).toBeInTheDocument();
    });

    // nextPageCalls should be exactly 2 (one fetchMore + one useQuery re-fire).
    // assertNoLeaks() in afterEach closes the loop for any extra fetches.
    expect(nextPageCalls).toBe(2);
  });

  // T9 — PII absence: the console.warn emitted on fetchMore failure MUST NOT
  // contain email, displayName, or bio keys. It MUST contain pageIndex and name.
  // See .claude/rules/error-wrapping.md § "Assert PII *absence*".
  test("T9: fetchMore warn payload has pageIndex/name discriminators and no PII fields", async () => {
    const ue = userEvent.setup({ delay: null });

    const page1Users = Array.from({ length: ADMIN_USERS_PAGE_SIZE }, (_, i) => makeUser(i + 1));
    const endCursor = `user-${ADMIN_USERS_PAGE_SIZE}`;

    const page1Connection = makeConnection(page1Users, true, ADMIN_USERS_PAGE_SIZE + 5);

    const fetchMoreVars = {
      ...ADMIN_USERS_DEFAULT_VARS,
      first: ADMIN_USERS_PAGE_SIZE,
      search: null,
      roleId: null,
      after: endCursor,
    };

    // Install an outer warn spy. Per .claude/rules/pagination.md § "Spy stacking":
    // the outer spy MUST NOT call mockImplementation(() => {}) — that would swallow
    // leak warnings from the inner leak spy (installed in beforeEach).
    // No mockImplementation here lets calls flow through to the leak spy's mock.
    const outerWarnSpy = vi.spyOn(console, "warn");

    render(
      <MockedProvider
        mocks={[
          // Mock 1: initial page-0 useQuery
          {
            request: { query: AdminUsersDocument, variables: PAGE_0_VARS },
            result: { data: { users: page1Connection } },
          },
          // Mock 2: fetchMore fails.
          {
            request: { query: AdminUsersDocument, variables: fetchMoreVars },
            result: {
              errors: [{ message: "network error" }],
            },
          },
          EMPTY_ROLES_MOCK,
        ]}
      >
        <AdminUsersClient initialConnection={null} />
      </MockedProvider>,
    );

    expect(await screen.findByText("User 1")).toBeInTheDocument();

    const nextButtons = screen.getAllByRole("button", { name: /go to next page/i });
    await ue.click(nextButtons[0] as HTMLElement);

    // Wait for the error banner.
    await screen.findByTestId("admin-users-fetch-more-error");

    // Find the specific [admin-users] fetchMore failed warn call.
    const warnCalls = outerWarnSpy.mock.calls.filter(
      (args) =>
        args.length >= 2 &&
        typeof args[0] === "string" &&
        args[0].includes("[admin-users] fetchMore failed"),
    );
    expect(warnCalls.length).toBeGreaterThanOrEqual(1);

    const payload = warnCalls[0]?.[1];
    expect(typeof payload).toBe("object");
    expect(payload).not.toBeNull();

    // Must have discriminating keys (non-Error.prototype) per
    // .claude/rules/frontend-typescript-conventions.md § "expect.objectContaining".
    expect(payload).toHaveProperty("pageIndex");
    expect(payload).toHaveProperty("name");

    // Must NOT contain PII fields.
    expect(payload).not.toHaveProperty("email");
    expect(payload).not.toHaveProperty("displayName");
    expect(payload).not.toHaveProperty("bio");

    // Teardown LIFO: outer spy first, then leak spy in afterEach.
    outerWarnSpy.mockRestore();
  });

  // T11 — fetchMore FORBIDDEN: permission-denied banner with no Retry.
  // A FORBIDDEN response on fetchMore must surface the same permission copy as
  // the initial-load FORBIDDEN path. Retry is suppressed because re-issuing the
  // same request would fail again on the same revoked role.
  test("T11: fetchMore FORBIDDEN renders permission-denied banner without Retry", async () => {
    const ue = userEvent.setup({ delay: null });

    const page1Users = Array.from({ length: ADMIN_USERS_PAGE_SIZE }, (_, i) => makeUser(i + 1));
    const endCursor = `user-${ADMIN_USERS_PAGE_SIZE}`;
    const page1Connection = makeConnection(page1Users, true, ADMIN_USERS_PAGE_SIZE + 5);

    const fetchMoreVars = {
      ...ADMIN_USERS_DEFAULT_VARS,
      first: ADMIN_USERS_PAGE_SIZE,
      search: null,
      roleId: null,
      after: endCursor,
    };

    render(
      <MockedProvider
        mocks={[
          // Mock 1: initial page-0 useQuery
          {
            request: { query: AdminUsersDocument, variables: PAGE_0_VARS },
            result: { data: { users: page1Connection } },
          },
          // Mock 2: fetchMore returns FORBIDDEN
          {
            request: { query: AdminUsersDocument, variables: fetchMoreVars },
            result: {
              errors: [
                new GraphQLError("admin role required", {
                  extensions: { code: "FORBIDDEN" },
                }),
              ],
            },
          },
          EMPTY_ROLES_MOCK,
        ]}
      >
        <AdminUsersClient initialConnection={null} />
      </MockedProvider>,
    );

    expect(await screen.findByText("User 1")).toBeInTheDocument();

    const nextButtons = screen.getAllByRole("button", { name: /go to next page/i });
    await ue.click(nextButtons[0] as HTMLElement);

    // FORBIDDEN banner appears with the permission-loss copy.
    const banner = await screen.findByTestId("admin-users-fetch-more-error");
    expect(banner).toHaveTextContent("You no longer have permission to load more users.");

    // Retry button MUST be absent on the fetchMore banner — Retry is suppressed
    // for FORBIDDEN because re-issuing would fail again. Scope the query to the
    // banner so a stray Retry elsewhere does not satisfy this assertion.
    expect(within(banner).queryByRole("button", { name: /retry/i })).not.toBeInTheDocument();
  });

  // T12 — fetchMore UNAUTHENTICATED: triggers router.replace("/").
  // redirect() inside .catch() does NOT navigate (its NEXT_REDIRECT throw is
  // captured as a promise rejection); router.replace is fire-and-forget and is
  // the correct primitive in this branch.
  test("T12: fetchMore UNAUTHENTICATED triggers router.replace('/')", async () => {
    const ue = userEvent.setup({ delay: null });

    const page1Users = Array.from({ length: ADMIN_USERS_PAGE_SIZE }, (_, i) => makeUser(i + 1));
    const endCursor = `user-${ADMIN_USERS_PAGE_SIZE}`;
    const page1Connection = makeConnection(page1Users, true, ADMIN_USERS_PAGE_SIZE + 5);

    const fetchMoreVars = {
      ...ADMIN_USERS_DEFAULT_VARS,
      first: ADMIN_USERS_PAGE_SIZE,
      search: null,
      roleId: null,
      after: endCursor,
    };

    render(
      <MockedProvider
        mocks={[
          // Mock 1: initial page-0 useQuery
          {
            request: { query: AdminUsersDocument, variables: PAGE_0_VARS },
            result: { data: { users: page1Connection } },
          },
          // Mock 2: fetchMore returns UNAUTHENTICATED
          {
            request: { query: AdminUsersDocument, variables: fetchMoreVars },
            result: {
              errors: [
                new GraphQLError("session expired", {
                  extensions: { code: "UNAUTHENTICATED" },
                }),
              ],
            },
          },
          EMPTY_ROLES_MOCK,
        ]}
      >
        <AdminUsersClient initialConnection={null} />
      </MockedProvider>,
    );

    expect(await screen.findByText("User 1")).toBeInTheDocument();

    const nextButtons = screen.getAllByRole("button", { name: /go to next page/i });
    await ue.click(nextButtons[0] as HTMLElement);

    // router.replace must be called with "/" so the user lands at the app's
    // re-auth entry point.
    await waitFor(() => {
      expect(mockRouterReplace).toHaveBeenCalledWith("/");
    });
  });

  // T13 — AdminRoles query failure: structured warn for operator triage.
  // The dropdown silently degrades to empty (non-fatal) but the failure must be
  // observable in logs. The discriminator key here is `name` — Error.prototype
  // also exposes `name`, so this is a known weakness of the assertion (per
  // .claude/rules/frontend-typescript-conventions.md § "expect.objectContaining
  // ({ message }) is not enough"). The structured payload `{ name }` is what
  // production emits; a stronger discriminator would require adding a fixed
  // string field (e.g. `where: "AdminRoles"`) to the production code, which is
  // out of scope for this iteration.
  test("T13: AdminRoles query failure logs structured warn with name", async () => {
    // Outer warn spy — see .claude/rules/pagination.md § "Spy stacking". No
    // mockImplementation so calls flow through to the inner leak spy.
    const outerWarnSpy = vi.spyOn(console, "warn");

    const page1Users = [makeUser(1)];
    const page1Connection = makeConnection(page1Users, false, 1);

    render(
      <MockedProvider
        mocks={[
          {
            request: { query: AdminUsersDocument, variables: PAGE_0_VARS },
            result: { data: { users: page1Connection } },
          },
          // AdminRoles query fails. The Error reaches useQuery.error, which the
          // useEffect in AdminUsersClient surfaces via console.warn.
          {
            request: { query: AdminRolesDocument, variables: {} },
            error: new Error("network down"),
          },
        ]}
      >
        <AdminUsersClient initialConnection={null} />
      </MockedProvider>,
    );

    expect(await screen.findByText("User 1")).toBeInTheDocument();

    // The structured warn fires inside the rolesResult.error useEffect.
    await waitFor(() => {
      const adminRolesWarns = outerWarnSpy.mock.calls.filter(
        (args) =>
          args.length >= 2 &&
          typeof args[0] === "string" &&
          args[0].includes("[admin-users] AdminRoles query failed"),
      );
      expect(adminRolesWarns.length).toBeGreaterThanOrEqual(1);
    });

    const adminRolesWarns = outerWarnSpy.mock.calls.filter(
      (args) =>
        args.length >= 2 &&
        typeof args[0] === "string" &&
        args[0].includes("[admin-users] AdminRoles query failed"),
    );
    expect(adminRolesWarns[0]?.[1]).toEqual(
      expect.objectContaining({ name: expect.any(String) }),
    );

    // Teardown LIFO: outer spy first, then leak spy in afterEach.
    outerWarnSpy.mockRestore();
  });

  // T10 — FORBIDDEN query error: permission-denied banner appears, no Retry button.
  test("T10: FORBIDDEN query error renders permission-denied banner without Retry", async () => {
    render(
      <MockedProvider
        mocks={[
          {
            request: { query: AdminUsersDocument, variables: PAGE_0_VARS },
            result: {
              errors: [
                new GraphQLError("admin role required", {
                  extensions: { code: "FORBIDDEN" },
                }),
              ],
            },
          },
          EMPTY_ROLES_MOCK,
        ]}
      >
        <AdminUsersClient initialConnection={null} />
      </MockedProvider>,
    );

    const banner = await screen.findByTestId("admin-users-query-error");
    expect(banner).toBeInTheDocument();
    expect(banner).toHaveTextContent("You do not have permission to view this page.");

    // No Retry — re-issuing the FORBIDDEN query would fail again.
    expect(screen.queryByRole("button", { name: /retry/i })).not.toBeInTheDocument();
  });
});

// @vitest-environment jsdom
import { InMemoryCache, NetworkStatus } from "@apollo/client";
import { MockedProvider } from "@apollo/client/testing/react";
import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { GraphQLError } from "graphql";
import { afterEach, beforeEach, describe, expect, test, vi } from "vitest";
import { AdminUsersClient } from "@/app/admin/users/admin-users-client";
import { ADMIN_USERS_PAGE_SIZE } from "@/app/admin/users/queries";
import { AdminRolesDocument, AdminUsersDocument } from "@/generated/graphql";
import { renderWithIntl } from "@/test/render-with-intl";
import { adminUserFixture, generalUserFixture, userWithoutRolesFixture } from "./fixtures/users";

// ---------------------------------------------------------------------------
// Next.js stubs
// ---------------------------------------------------------------------------

const mockPush = vi.fn();

vi.mock("next/navigation", () => ({
  redirect: vi.fn(),
  usePathname: () => "/admin/users",
  useRouter: () => ({ push: mockPush, refresh: vi.fn(), replace: vi.fn() }),
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

// ---------------------------------------------------------------------------
// Data fixtures
// ---------------------------------------------------------------------------

type UserNode = {
  __typename: "User";
  id: string;
  displayName: string;
  bio: null;
  avatarUrl: null;
  lastSignInAt: null;
  roles: Array<{ __typename: "Role"; id: string; name: string }>;
};

type UserEdge = {
  __typename: "UserEdge";
  cursor: string;
  node: UserNode;
};

const ADMIN_ROLES_MOCK = {
  request: { query: AdminRolesDocument, variables: {} },
  result: {
    data: {
      roles: [{ __typename: "Role" as const, id: "role-general", name: "general" }],
    },
  },
};

function makeUser(i: number): UserNode {
  return {
    __typename: "User",
    id: `user-${i}`,
    displayName: `User ${i}`,
    bio: null,
    avatarUrl: null,
    lastSignInAt: null,
    roles: [{ __typename: "Role", id: `role-general`, name: "general" }],
  };
}

function makeEdge(user: UserNode): UserEdge {
  return {
    __typename: "UserEdge",
    cursor: user.id,
    node: user,
  };
}

function makeConnection(
  users: UserNode[],
  hasNextPage: boolean,
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

// ---------------------------------------------------------------------------
// IntersectionObserver mock (mirrors cards-pagination.test.tsx verbatim)
// ---------------------------------------------------------------------------

let ioCallbacks: IntersectionObserverCallback[] = [];

class FakeIntersectionObserver {
  callback: IntersectionObserverCallback;
  constructor(cb: IntersectionObserverCallback) {
    this.callback = cb;
    ioCallbacks.push(cb);
  }
  observe() {}
  unobserve() {}
  disconnect() {
    ioCallbacks = ioCallbacks.filter((cb) => cb !== this.callback);
  }
  takeRecords(): IntersectionObserverEntry[] {
    return [];
  }
}

function fireIntersect() {
  const cb = ioCallbacks[ioCallbacks.length - 1];
  if (!cb) return;
  cb([{ isIntersecting: true } as IntersectionObserverEntry], {} as IntersectionObserver);
}

// ---------------------------------------------------------------------------
// console spy helpers
// ---------------------------------------------------------------------------

let consoleWarnSpy: ReturnType<typeof vi.spyOn>;

beforeEach(() => {
  ioCallbacks = [];
  mockPush.mockReset();
  vi.stubGlobal("IntersectionObserver", FakeIntersectionObserver);
  consoleWarnSpy = vi.spyOn(console, "warn").mockImplementation(() => {});
  vi.spyOn(console, "error").mockImplementation(() => {});
});

afterEach(() => {
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
  vi.useRealTimers();
});

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

describe("AdminUsersClient", () => {
  // T1: Initial render — edges, display names, edit affordances, and totalCount shown.
  test("renders edges with display name and edit affordances and shows totalCount", async () => {
    const users = Array.from({ length: 3 }, (_, i) => makeUser(i + 1));
    const connection = makeConnection(users, false);

    const mocks = [
      ADMIN_ROLES_MOCK,
      {
        request: {
          query: AdminUsersDocument,
          variables: { first: ADMIN_USERS_PAGE_SIZE, search: null },
        },
        result: { data: { users: connection } },
      },
    ];

    const cache = new InMemoryCache();
    cache.writeQuery({
      query: AdminUsersDocument,
      variables: { first: ADMIN_USERS_PAGE_SIZE, search: null },
      data: { users: connection },
    });

    renderWithIntl(
      <MockedProvider mocks={mocks as never} cache={cache}>
        <AdminUsersClient />
      </MockedProvider>,
    );

    // Display names visible.
    expect(await screen.findByText("User 1")).toBeInTheDocument();
    expect(screen.getByText("User 2")).toBeInTheDocument();
    expect(screen.getByText("User 3")).toBeInTheDocument();

    // Inline role toggles moved into the edit sheet.
    expect(screen.queryByRole("checkbox", { name: "general" })).not.toBeInTheDocument();
    expect(screen.getAllByRole("button", { name: /edit user/i })).toHaveLength(3);

    // totalCount shown as the "N total" pill label.
    expect(screen.getByText("3 total")).toBeInTheDocument();
  });

  // T2: Debounced search — typing "ali" issues ONE query with search:"ali" after 300ms.
  test('debounced search issues AdminUsersDocument exactly once with search "ali" after 300ms', async () => {
    const user = userEvent.setup({ delay: null });
    vi.useFakeTimers({ shouldAdvanceTime: true });

    const initialUsers = Array.from({ length: 2 }, (_, i) => makeUser(i + 1));
    const searchUsers = [
      {
        __typename: "User" as const,
        id: "user-ali",
        displayName: "Alice",
        bio: null,
        avatarUrl: null,
        lastSignInAt: null,
        roles: [{ __typename: "Role" as const, id: "role-general", name: "general" }],
      },
    ];

    let searchQueryCalls = 0;
    const searchResult = vi.fn(() => {
      searchQueryCalls += 1;
      return { data: { users: makeConnection(searchUsers, false) } };
    });

    const mocks = [
      ADMIN_ROLES_MOCK,
      {
        request: {
          query: AdminUsersDocument,
          variables: { first: ADMIN_USERS_PAGE_SIZE, search: null },
        },
        result: { data: { users: makeConnection(initialUsers, false) } },
      },
      {
        request: {
          query: AdminUsersDocument,
          variables: { first: ADMIN_USERS_PAGE_SIZE, search: "ali" },
        },
        result: searchResult,
      },
    ];

    const cache = new InMemoryCache();
    cache.writeQuery({
      query: AdminUsersDocument,
      variables: { first: ADMIN_USERS_PAGE_SIZE, search: null },
      data: { users: makeConnection(initialUsers, false) },
    });

    renderWithIntl(
      <MockedProvider mocks={mocks as never} cache={cache}>
        <AdminUsersClient />
      </MockedProvider>,
    );

    // Initial render present.
    expect(await screen.findByText("User 1")).toBeInTheDocument();

    const searchInput = screen.getByRole("searchbox", { name: /search users/i });

    // Type "ali" — three keystrokes, each resets the debounce timer.
    await user.type(searchInput, "ali");

    // Advance past the debounce window.
    vi.advanceTimersByTime(300);

    // Restore real timers before waiting for DOM updates.
    vi.useRealTimers();

    await waitFor(() => {
      expect(screen.getByText("Alice")).toBeInTheDocument();
    });

    // The query must have fired exactly once — not once per keystroke.
    expect(searchQueryCalls).toBe(1);
  });

  // T3: In-flight guard — two observer firings in the same frame produce one fetchMore.
  test("IntersectionObserver fires fetchMore exactly once when triggered twice in-flight", async () => {
    const firstBatch = Array.from({ length: 20 }, (_, i) => makeUser(i + 1));
    const secondBatch = Array.from({ length: 20 }, (_, i) => makeUser(i + 21));

    let nextPageCalls = 0;
    const nextPageResult = vi.fn(() => {
      nextPageCalls += 1;
      return { data: { users: makeConnection(secondBatch, false) } };
    });

    const mocks = [
      ADMIN_ROLES_MOCK,
      {
        request: {
          query: AdminUsersDocument,
          variables: { first: ADMIN_USERS_PAGE_SIZE, search: null },
        },
        result: { data: { users: makeConnection(firstBatch, true) } },
      },
      {
        request: {
          query: AdminUsersDocument,
          variables: {
            first: ADMIN_USERS_PAGE_SIZE,
            after: `user-${firstBatch.length}`,
            search: null,
          },
        },
        delay: 50,
        result: nextPageResult,
      },
    ];

    const cache = new InMemoryCache();
    cache.writeQuery({
      query: AdminUsersDocument,
      variables: { first: ADMIN_USERS_PAGE_SIZE, search: null },
      data: { users: makeConnection(firstBatch, true) },
    });

    renderWithIntl(
      <MockedProvider mocks={mocks as never} cache={cache}>
        <AdminUsersClient />
      </MockedProvider>,
    );

    expect(await screen.findByText("User 1")).toBeInTheDocument();

    // Synchronously fire the observer twice — only the first must reach fetchMore.
    fireIntersect();
    fireIntersect();

    await waitFor(() => {
      expect(screen.getByText("User 21")).toBeInTheDocument();
    });

    expect(nextPageCalls).toBe(1);

    // If the ref-guard were replaced with useState, a second fetchMore would leak
    // into MockedProvider and emit a "No more mocked responses" console.warn.
    // Assert that no such warning fired for the AdminUsers query.
    const adminUsersWarn = consoleWarnSpy.mock.calls.flatMap((args: unknown[]) =>
      args.filter(
        (a): a is string =>
          typeof a === "string" &&
          a.includes("No more mocked responses") &&
          a.includes("AdminUsers"),
      ),
    );
    expect(adminUsersWarn).toEqual([]);
  });

  // T4: fetchMoreError halts the IO loop; Retry clears the banner and re-issues.
  test("fetchMoreError stops the IO loop and Retry re-issues the request", async () => {
    const user = userEvent.setup({ delay: null });
    const firstBatch = Array.from({ length: 20 }, (_, i) => makeUser(i + 1));
    const secondBatch = Array.from({ length: 20 }, (_, i) => makeUser(i + 21));
    const endCursor = `user-${firstBatch.length}`;

    let fetchMoreCallCount = 0;

    const failResult = vi.fn(() => {
      fetchMoreCallCount += 1;
      return {
        errors: [{ message: "Could not load more users. Please try again." }],
      };
    });

    const successResult = vi.fn(() => {
      fetchMoreCallCount += 1;
      return { data: { users: makeConnection(secondBatch, false) } };
    });

    const fetchMoreVars = {
      first: ADMIN_USERS_PAGE_SIZE,
      after: endCursor,
      search: null,
    };

    const mocks = [
      ADMIN_ROLES_MOCK,
      {
        request: {
          query: AdminUsersDocument,
          variables: { first: ADMIN_USERS_PAGE_SIZE, search: null },
        },
        result: { data: { users: makeConnection(firstBatch, true) } },
      },
      // First fetchMore fails.
      {
        request: {
          query: AdminUsersDocument,
          variables: fetchMoreVars,
        },
        result: failResult,
      },
      // After Retry, the next fetchMore succeeds.
      {
        request: {
          query: AdminUsersDocument,
          variables: fetchMoreVars,
        },
        result: successResult,
      },
    ];

    const cache = new InMemoryCache();
    cache.writeQuery({
      query: AdminUsersDocument,
      variables: { first: ADMIN_USERS_PAGE_SIZE, search: null },
      data: { users: makeConnection(firstBatch, true) },
    });

    renderWithIntl(
      <MockedProvider mocks={mocks as never} cache={cache}>
        <AdminUsersClient />
      </MockedProvider>,
    );

    expect(await screen.findByText("User 1")).toBeInTheDocument();

    // Trigger the observer — first fetchMore fails.
    fireIntersect();

    const banner = await screen.findByTestId("admin-users-fetch-more-error");
    expect(banner).toBeInTheDocument();
    expect(fetchMoreCallCount).toBe(1);

    // While the error banner is present, firing the observer again must NOT issue fetchMore.
    fireIntersect();
    fireIntersect();
    await new Promise((resolve) => setTimeout(resolve, 0));
    expect(fetchMoreCallCount).toBe(1);

    // Click Retry — the banner clears and the second request (success) fires.
    await user.click(screen.getByRole("button", { name: /retry/i }));

    await waitFor(() => {
      expect(screen.getByText("User 21")).toBeInTheDocument();
    });

    expect(screen.queryByTestId("admin-users-fetch-more-error")).not.toBeInTheDocument();
    // fetchMoreCallCount is now 2 (1 fail + 1 success via Retry).
    expect(fetchMoreCallCount).toBe(2);
  });

  // T5: MockedProvider leak detection — no "No more mocked responses" warning fires.
  test("no MockedProvider leak warning fires for AdminUsers after one fetchMore", async () => {
    const firstBatch = Array.from({ length: 20 }, (_, i) => makeUser(i + 1));
    const secondBatch = Array.from({ length: 20 }, (_, i) => makeUser(i + 21));
    const endCursor = `user-${firstBatch.length}`;

    const mocks = [
      ADMIN_ROLES_MOCK,
      {
        request: {
          query: AdminUsersDocument,
          variables: { first: ADMIN_USERS_PAGE_SIZE, search: null },
        },
        result: { data: { users: makeConnection(firstBatch, true) } },
      },
      {
        request: {
          query: AdminUsersDocument,
          variables: {
            first: ADMIN_USERS_PAGE_SIZE,
            after: endCursor,
            search: null,
          },
        },
        result: { data: { users: makeConnection(secondBatch, false) } },
      },
    ];

    const cache = new InMemoryCache();
    cache.writeQuery({
      query: AdminUsersDocument,
      variables: { first: ADMIN_USERS_PAGE_SIZE, search: null },
      data: { users: makeConnection(firstBatch, true) },
    });

    renderWithIntl(
      <MockedProvider mocks={mocks as never} cache={cache}>
        <AdminUsersClient />
      </MockedProvider>,
    );

    expect(await screen.findByText("User 1")).toBeInTheDocument();

    fireIntersect();

    await waitFor(() => {
      expect(screen.getByText("User 21")).toBeInTheDocument();
    });

    // No "No more mocked responses for the query AdminUsers" warning should have fired.
    const adminUsersLeakWarnings = consoleWarnSpy.mock.calls.filter((args: unknown[]) =>
      args.some((arg: unknown) => typeof arg === "string" && arg.includes("AdminUsers")),
    );
    expect(adminUsersLeakWarnings).toEqual([]);
  });

  // T6: NetworkStatus.fetchMore indicator — "Loading more" shown while in-flight, cleared on resolve.
  // Uses the named NetworkStatus.fetchMore enum (value 3) rather than a magic number.
  test("shows loading-more indicator while fetchMore is in flight using named NetworkStatus.fetchMore", async () => {
    const firstBatch = Array.from({ length: 20 }, (_, i) => makeUser(i + 1));
    const secondBatch = Array.from({ length: 20 }, (_, i) => makeUser(i + 21));
    const endCursor = `user-${firstBatch.length}`;

    // Explicit assertion that the named enum matches the expected value —
    // guards against magic-number drift without coupling to the number itself.
    expect(NetworkStatus.fetchMore).toBe(3);

    const mocks = [
      ADMIN_ROLES_MOCK,
      {
        request: {
          query: AdminUsersDocument,
          variables: { first: ADMIN_USERS_PAGE_SIZE, search: null },
        },
        result: { data: { users: makeConnection(firstBatch, true) } },
      },
      // A 200ms delay keeps the fetchMore in-flight long enough for the indicator check.
      {
        request: {
          query: AdminUsersDocument,
          variables: {
            first: ADMIN_USERS_PAGE_SIZE,
            after: endCursor,
            search: null,
          },
        },
        delay: 200,
        result: { data: { users: makeConnection(secondBatch, false) } },
      },
    ];

    const cache = new InMemoryCache();
    cache.writeQuery({
      query: AdminUsersDocument,
      variables: { first: ADMIN_USERS_PAGE_SIZE, search: null },
      data: { users: makeConnection(firstBatch, true) },
    });

    renderWithIntl(
      <MockedProvider mocks={mocks as never} cache={cache}>
        <AdminUsersClient />
      </MockedProvider>,
    );

    expect(await screen.findByText("User 1")).toBeInTheDocument();

    // Trigger the observer — fetchMore is now in-flight (delayed 200ms).
    fireIntersect();

    // The "Loading more users..." indicator must appear while the request is in-flight.
    const indicator = await screen.findByTestId("admin-users-loading-more");
    expect(indicator).toBeInTheDocument();

    // After the delay resolves, the indicator must disappear and the second batch renders.
    await waitFor(
      () => {
        expect(screen.queryByTestId("admin-users-loading-more")).not.toBeInTheDocument();
      },
      { timeout: 1000 },
    );

    expect(screen.getByText("User 21")).toBeInTheDocument();
  });

  // T7: FORBIDDEN query error — non-retry permission banner; no Retry button.
  test("renders permission-denied banner without Retry when AdminUsers returns FORBIDDEN", async () => {
    const mocks = [
      ADMIN_ROLES_MOCK,
      {
        request: {
          query: AdminUsersDocument,
          variables: { first: ADMIN_USERS_PAGE_SIZE, search: null },
        },
        result: {
          errors: [
            new GraphQLError("admin role required", {
              extensions: { code: "FORBIDDEN" },
            }),
          ],
        },
      },
    ];

    renderWithIntl(
      <MockedProvider mocks={mocks as never}>
        <AdminUsersClient />
      </MockedProvider>,
    );

    // The permission-denied banner must appear.
    const banner = await screen.findByTestId("admin-users-query-error");
    expect(banner).toBeInTheDocument();
    expect(banner).toHaveTextContent("You do not have permission to view this page.");

    // No Retry button — re-issuing the same query would fail again.
    expect(screen.queryByRole("button", { name: /retry/i })).not.toBeInTheDocument();
  });

  // T8: Each user row exposes an Edit button that pushes ?edit=<id> via the router, implementing URL-backed sheet state.
  test("each user row has an Edit button that pushes ?edit=<id>", async () => {
    const user = userEvent.setup({ delay: null });

    // Cast shared fixtures to the internal UserNode shape (superset is safe).
    const adminUserNode = adminUserFixture as unknown as UserNode;
    const generalUserNode = generalUserFixture as unknown as UserNode;
    const noroleUserNode = userWithoutRolesFixture as unknown as UserNode;
    const users: UserNode[] = [adminUserNode, generalUserNode, noroleUserNode];

    const connection = makeConnection(users, false);
    const cache = new InMemoryCache();
    cache.writeQuery({
      query: AdminUsersDocument,
      variables: { first: ADMIN_USERS_PAGE_SIZE, search: null },
      data: { users: connection },
    });
    const mocks = [
      {
        request: {
          query: AdminUsersDocument,
          variables: { first: ADMIN_USERS_PAGE_SIZE, search: null },
        },
        result: { data: { users: connection } },
      },
      ADMIN_ROLES_MOCK,
    ];

    renderWithIntl(
      <MockedProvider mocks={mocks as never} cache={cache}>
        <AdminUsersClient />
      </MockedProvider>,
    );

    // Wait for the first display name to confirm the list rendered.
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

  // T9: An empty connection (edges = []) renders the "No users found." empty-state copy and hides the user list container.
  test("renders empty-state copy when the connection has no edges", async () => {
    const connection = makeConnection([], false);
    const cache = new InMemoryCache();
    cache.writeQuery({
      query: AdminUsersDocument,
      variables: { first: ADMIN_USERS_PAGE_SIZE, search: null },
      data: { users: connection },
    });
    const mocks = [
      {
        request: {
          query: AdminUsersDocument,
          variables: { first: ADMIN_USERS_PAGE_SIZE, search: null },
        },
        result: { data: { users: connection } },
      },
      ADMIN_ROLES_MOCK,
    ];

    renderWithIntl(
      <MockedProvider mocks={mocks as never} cache={cache}>
        <AdminUsersClient />
      </MockedProvider>,
    );

    const empty = await screen.findByTestId("admin-users-empty");
    expect(empty).toHaveTextContent("No users found.");
    expect(screen.queryByTestId("admin-users-list")).not.toBeInTheDocument();
  });
});

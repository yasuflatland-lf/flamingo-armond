// @vitest-environment jsdom

import { readFileSync } from "node:fs";
import { join } from "node:path";
import { InMemoryCache } from "@apollo/client";
import { MockedProvider } from "@apollo/client/testing/react";
import { act, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { GraphQLError } from "graphql";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  AdminDeleteUserDocument,
  AdminEditUserDocument,
  AdminRolesDocument,
  AdminUserDocument,
  AdminUsersDocument,
} from "@/generated/graphql";
import { renderWithIntl } from "@/test/render-with-intl";
import {
  type ApolloMockLeakSpyResult,
  installApolloMockLeakSpy,
} from "../../../../__tests__/utils/mock-apollo-paginated";
import { AdminUsersClient } from "./admin-users-client";
import { ADMIN_USERS_PAGE_SIZE } from "./queries";

const mockPush = vi.fn();
const mockReplace = vi.fn();
const mockRefresh = vi.fn();

let mockPathname = "/admin/users";
let mockSearchParamsValue = "";

vi.mock("next/navigation", () => ({
  usePathname: () => mockPathname,
  useRouter: () => ({
    push: mockPush,
    refresh: mockRefresh,
    replace: mockReplace,
  }),
  useSearchParams: () => new URLSearchParams(mockSearchParamsValue),
}));

const toastSuccessMock = vi.fn();
vi.mock("sonner", () => ({
  toast: { success: (...args: unknown[]) => toastSuccessMock(...args) },
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
  default: (props: { src: string; alt: string; width: number; height: number }) => (
    // biome-ignore lint/performance/noImgElement: jsdom-friendly stand-in for next/image
    // biome-ignore lint/a11y/useAltText: alt is forwarded from props
    <img {...props} />
  ),
}));

const MOD_ROLE = {
  __typename: "Role" as const,
  id: "r-mod",
  name: "moderator",
};

const USER_1 = {
  __typename: "User" as const,
  id: "u-1",
  version: 41,
  displayName: "Alice",
  bio: null,
  avatarUrl: null,
  lastSignInAt: null,
  roles: [],
};

const USER_2 = {
  __typename: "User" as const,
  id: "u-2",
  version: 7,
  displayName: "Bob",
  bio: null,
  avatarUrl: null,
  lastSignInAt: null,
  roles: [],
};

// The backend emits opaque "v1:..." cursors (cursor.Encode(id)), never the raw
// user id — fixtures mirror the encoder so a cursor-based match cannot pass on a
// raw id (.claude/rules/pagination.md "Resolve an edge by node.id").
function userEdge(user: typeof USER_1) {
  return {
    __typename: "UserEdge" as const,
    cursor: `v1:${btoa(user.id)}`,
    node: user,
  };
}

function makeConnection(items: (typeof USER_1)[], hasNextPage = false, totalCount?: number) {
  const first = items[0];
  const last = items[items.length - 1];
  return {
    __typename: "UserConnection" as const,
    edges: items.map(userEdge),
    pageInfo: {
      __typename: "PageInfo" as const,
      hasNextPage,
      hasPreviousPage: false,
      startCursor: first ? `v1:${btoa(first.id)}` : null,
      endCursor: last ? `v1:${btoa(last.id)}` : null,
    },
    totalCount: totalCount ?? items.length,
  };
}

function makeUsersMock(items = [USER_1], hasNextPage = false) {
  const variables = { first: ADMIN_USERS_PAGE_SIZE, search: null };
  return {
    request: { query: AdminUsersDocument, variables },
    result: { data: { users: makeConnection(items, hasNextPage) } },
  };
}

function makeRolesMock() {
  return {
    request: { query: AdminRolesDocument, variables: {} },
    result: { data: { roles: [MOD_ROLE] } },
  };
}

let ioCallbacks: IntersectionObserverCallback[] = [];

class FakeIntersectionObserver {
  constructor(cb: IntersectionObserverCallback) {
    ioCallbacks.push(cb);
  }
  observe() {}
  unobserve() {}
  disconnect() {}
  takeRecords() {
    return [];
  }
}

function fireIntersect() {
  const cb = ioCallbacks[ioCallbacks.length - 1];
  if (!cb) return;
  cb([{ isIntersecting: true } as IntersectionObserverEntry], {} as IntersectionObserver);
}

let leakSpy: ApolloMockLeakSpyResult;

beforeEach(() => {
  leakSpy = installApolloMockLeakSpy({
    operationNames: ["AdminUsers", "AdminRoles"],
  });
  ioCallbacks = [];
  mockPush.mockReset();
  mockReplace.mockReset();
  mockRefresh.mockReset();
  toastSuccessMock.mockReset();
  mockPathname = "/admin/users";
  mockSearchParamsValue = "";
  vi.stubGlobal("IntersectionObserver", FakeIntersectionObserver);
});

afterEach(() => {
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
  leakSpy.assertNoLeaks();
  leakSpy.teardown();
});

describe("<AdminUsersClient> initial loading", () => {
  it("renders the skeleton immediately on first render then hides it once data arrives", async () => {
    renderWithIntl(
      <MockedProvider mocks={[makeUsersMock(), makeRolesMock()]}>
        <AdminUsersClient />
      </MockedProvider>,
    );

    // Synchronous assertion: Apollo starts with networkStatus === loading (1) and
    // no data, so initialLoading is true and the skeleton must be in the DOM
    // before any microtask resolves the mock response.
    expect(screen.getByTestId("admin-users-skeleton")).toBeInTheDocument();

    // Wait for the mock to resolve and the list to appear.
    expect(await screen.findByTestId("admin-users-list")).toBeInTheDocument();

    // Once data has arrived the skeleton must be gone.
    expect(screen.queryByTestId("admin-users-skeleton")).not.toBeInTheDocument();
  });
});

describe("<AdminUsersClient> sheet", () => {
  it("pushes ?edit=<id> when the row Edit affordance is clicked", async () => {
    const user = userEvent.setup();

    renderWithIntl(
      <MockedProvider mocks={[makeUsersMock(), makeRolesMock()]}>
        <AdminUsersClient />
      </MockedProvider>,
    );

    expect(await screen.findByText("Alice")).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: /edit alice/i }));

    expect(mockPush).toHaveBeenCalledWith("/admin/users?edit=u-1", { scroll: false });
  });

  it("does not render inline role checkboxes in the user list", async () => {
    renderWithIntl(
      <MockedProvider mocks={[makeUsersMock(), makeRolesMock()]}>
        <AdminUsersClient />
      </MockedProvider>,
    );

    expect(await screen.findByText("Alice")).toBeInTheDocument();
    expect(screen.queryByRole("checkbox", { name: /moderator/i })).not.toBeInTheDocument();
  });
});

describe("<AdminUsersClient> edit-sheet lazy query", () => {
  function makeAdminUserMock(
    id: string,
    overrides: { error?: Error; result?: { data: { adminUser: typeof USER_1 | null } } } = {},
  ) {
    const base = {
      request: { query: AdminUserDocument, variables: { id } },
    };
    if (overrides.error) return { ...base, error: overrides.error };
    return {
      ...base,
      result: overrides.result ?? { data: { adminUser: { ...USER_1, id } } },
    };
  }

  it("passes all roles to the edit sheet", async () => {
    mockSearchParamsValue = "edit=u-1";
    leakSpy.teardown();
    leakSpy = installApolloMockLeakSpy({
      operationNames: ["AdminUsers", "AdminRoles", "AdminUser"],
    });

    const userMock = makeAdminUserMock("u-1");

    renderWithIntl(
      <MockedProvider mocks={[makeUsersMock(), makeRolesMock(), userMock]}>
        <AdminUsersClient />
      </MockedProvider>,
    );

    expect(await screen.findByDisplayValue("Alice")).toBeInTheDocument();
    // findByRole (not getByRole): the roles query may settle a tick after the
    // user query, so await the checkbox rather than asserting synchronously.
    expect(await screen.findByRole("checkbox", { name: /moderator/i })).toBeInTheDocument();
  });

  it("FORBIDDEN on the admin-user lazy query renders the permission banner inside the sheet", async () => {
    mockSearchParamsValue = "edit=u-1";
    leakSpy.teardown();
    leakSpy = installApolloMockLeakSpy({
      operationNames: ["AdminUsers", "AdminRoles", "AdminUser"],
    });

    const forbiddenError = new GraphQLError("forbidden", {
      extensions: { code: "FORBIDDEN" },
    });
    const userMock = {
      request: { query: AdminUserDocument, variables: { id: "u-1" } },
      result: { errors: [forbiddenError] },
    };

    renderWithIntl(
      <MockedProvider mocks={[makeUsersMock(), makeRolesMock(), userMock]}>
        <AdminUsersClient />
      </MockedProvider>,
    );

    await waitFor(() => {
      expect(screen.getByText(/do not have permission to edit this user/i)).toBeInTheDocument();
    });
    expect(screen.queryByLabelText(/display name/i)).not.toBeInTheDocument();
  });

  it("UNAUTHENTICATED on the admin-user lazy query renders the session-expired banner", async () => {
    mockSearchParamsValue = "edit=u-1";
    leakSpy.teardown();
    leakSpy = installApolloMockLeakSpy({
      operationNames: ["AdminUsers", "AdminRoles", "AdminUser"],
    });

    const unauthError = new GraphQLError("expired", {
      extensions: { code: "UNAUTHENTICATED" },
    });
    const userMock = {
      request: { query: AdminUserDocument, variables: { id: "u-1" } },
      result: { errors: [unauthError] },
    };

    renderWithIntl(
      <MockedProvider mocks={[makeUsersMock(), makeRolesMock(), userMock]}>
        <AdminUsersClient />
      </MockedProvider>,
    );

    await waitFor(() => {
      expect(screen.getByText(/session has expired/i)).toBeInTheDocument();
    });
  });

  it("ConcurrentUpdateError save reloads the list and the sheet, preserving the banner", async () => {
    const user = userEvent.setup();
    mockSearchParamsValue = "edit=u-1";
    leakSpy.teardown();
    leakSpy = installApolloMockLeakSpy({
      operationNames: ["AdminUsers", "AdminRoles", "AdminUser"],
    });

    const reloadedUser = { ...USER_1, version: 50, displayName: "Reloaded Alice" };
    // Keep the list empty so the shared User:u-1 cache entity is written only by
    // the detail query — otherwise list cache broadcasts race the typed input.
    const mocks = [
      makeUsersMock([]), // initial list query (empty)
      makeUsersMock([]), // refetch fired by reloadEditedUser
      makeRolesMock(),
      makeAdminUserMock("u-1"), // mount lazy load (v41 "Alice")
      {
        // Save with no edits — profileDirty is false, so the mutation omits the
        // profile fields. This keeps the form untouched and avoids racing the
        // typed input against cache re-broadcasts during the test.
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
              __typename: "ConcurrentUpdateError" as const,
              message: "user has changed",
            },
          },
        },
      },
      makeAdminUserMock("u-1", { result: { data: { adminUser: reloadedUser } } }), // reload
    ];

    renderWithIntl(
      <MockedProvider mocks={mocks}>
        <AdminUsersClient />
      </MockedProvider>,
    );

    await screen.findByDisplayValue("Alice");

    await user.click(screen.getByRole("button", { name: /save changes/i }));

    // reloadEditedUser must re-issue the detail query (proving the reload wiring
    // fires); the latest server value swaps into the form and the conflict
    // banner survives the swap.
    await waitFor(() => {
      expect(screen.getByDisplayValue("Reloaded Alice")).toBeInTheDocument();
    });
    expect(screen.getByRole("alert")).toHaveTextContent(
      "This user was changed by someone else. Reload and try again.",
    );
  });

  it("race-guard: stale lazy-query response for the previously-open user is dropped", async () => {
    const user = userEvent.setup();
    leakSpy.teardown();
    leakSpy = installApolloMockLeakSpy({
      operationNames: ["AdminUsers", "AdminRoles", "AdminUser"],
    });

    // Initially open edit for u-1
    mockSearchParamsValue = "edit=u-1";

    // Provide a fast resolution for u-1 (the stale one) and a different shape
    // for u-2, so we can prove the sheet shows u-2's data, not u-1's, after switching.
    const u1Mock = makeAdminUserMock("u-1", {
      result: { data: { adminUser: { ...USER_1, id: "u-1", displayName: "Stale Alice" } } },
    });
    const u2Mock = makeAdminUserMock("u-2", {
      result: { data: { adminUser: { ...USER_2, id: "u-2", displayName: "Fresh Bob" } } },
    });

    const { rerender } = renderWithIntl(
      <MockedProvider mocks={[makeUsersMock([USER_1, USER_2]), makeRolesMock(), u1Mock, u2Mock]}>
        <AdminUsersClient />
      </MockedProvider>,
    );

    await screen.findByDisplayValue("Stale Alice");

    // Simulate a navigation/search-param change to edit=u-2 (user clicks Edit on Bob)
    mockSearchParamsValue = "edit=u-2";
    rerender(
      <MockedProvider mocks={[makeUsersMock([USER_1, USER_2]), makeRolesMock(), u1Mock, u2Mock]}>
        <AdminUsersClient />
      </MockedProvider>,
    );

    // The sheet must show u-2's data once the new query settles, never u-1's stale data
    await waitFor(() => {
      expect(screen.getByDisplayValue("Fresh Bob")).toBeInTheDocument();
    });
    expect(screen.queryByDisplayValue("Stale Alice")).not.toBeInTheDocument();
    // Silence the unused-binding warning from setup helpers
    void user;
  });
});

describe("<AdminUsersClient> fetchMore catch", () => {
  // PII redaction contract — docs/frontend/rsc-error-handling/redact-err-message-from-console-payloads.md.
  it("logs structured payload without err.message when fetchMore fails", async () => {
    const cache = new InMemoryCache();
    const page1Conn = makeConnection([USER_1, USER_2], true, 3);
    const initialVars = { first: ADMIN_USERS_PAGE_SIZE, search: null };
    cache.writeQuery({
      query: AdminUsersDocument,
      variables: initialVars,
      data: { users: page1Conn },
    });

    const initialMock = {
      request: { query: AdminUsersDocument, variables: initialVars },
      result: { data: { users: page1Conn } },
    };

    const fetchMoreVars = {
      first: ADMIN_USERS_PAGE_SIZE,
      // The client echoes the opaque endCursor (encoded) back as `after`.
      after: `v1:${btoa(USER_2.id)}`,
      search: null,
    };

    const errorMock = {
      request: { query: AdminUsersDocument, variables: fetchMoreVars },
      result: {
        errors: [new GraphQLError("boom", { extensions: { code: "INTERNAL" } })],
      },
    };

    // Forwarding spy: do NOT call mockImplementation here — see
    // docs/pagination/capture-mockedprovider-warn-leaks.md § "Spy stacking".
    const consoleWarnSpy = vi.spyOn(console, "warn");

    renderWithIntl(
      <MockedProvider mocks={[initialMock, makeRolesMock(), errorMock]} cache={cache}>
        <AdminUsersClient />
      </MockedProvider>,
    );

    expect(await screen.findByText("Alice")).toBeInTheDocument();

    // Trigger fetchMore — it will fail and emit the structured warn.
    fireIntersect();

    await waitFor(() => {
      expect(consoleWarnSpy).toHaveBeenCalledWith(
        "[admin-users] fetchMore failed",
        expect.objectContaining({
          name: expect.any(String),
          endCursor: expect.any(String),
        }),
      );
    });

    const warnCall = consoleWarnSpy.mock.calls.find(
      (call) => call[0] === "[admin-users] fetchMore failed",
    );
    expect(warnCall?.[1]).not.toHaveProperty("message");

    consoleWarnSpy.mockRestore();
  });
});

describe("<AdminUsersClient> delete", () => {
  it("deletes a user, drops the row from the cached connection, and toasts", async () => {
    const user = userEvent.setup();
    mockSearchParamsValue = "edit=u-1";
    leakSpy.teardown();
    leakSpy = installApolloMockLeakSpy({
      operationNames: ["AdminUsers", "AdminRoles", "AdminUser", "AdminDeleteUser"],
    });

    const cache = new InMemoryCache();
    const initialVars = { first: ADMIN_USERS_PAGE_SIZE, search: null };
    const conn = makeConnection([USER_1, USER_2], false, 2);
    cache.writeQuery({
      query: AdminUsersDocument,
      variables: initialVars,
      data: { users: conn },
    });

    const mocks = [
      {
        request: { query: AdminUsersDocument, variables: initialVars },
        result: { data: { users: conn } },
      },
      makeRolesMock(),
      {
        request: { query: AdminUserDocument, variables: { id: "u-1" } },
        result: { data: { adminUser: USER_1 } },
      },
      {
        request: { query: AdminDeleteUserDocument, variables: { id: "u-1" } },
        result: { data: { adminDeleteUser: true } },
      },
      // Evicting User:u-1 makes the still-open sheet's network-only lazy query
      // re-observe and re-fetch (a test artifact — in production sheet.close()
      // drops ?edit so editUserId becomes null and the query unmounts). The user
      // is gone, so this absorbing response returns null.
      {
        request: { query: AdminUserDocument, variables: { id: "u-1" } },
        result: { data: { adminUser: null } },
      },
    ];

    renderWithIntl(
      <MockedProvider mocks={mocks} cache={cache}>
        <AdminUsersClient />
      </MockedProvider>,
    );

    // The sheet opens for u-1, exposing its danger-zone delete trigger.
    await screen.findByTestId("admin-delete-user-trigger");
    expect(screen.getByTestId("admin-user-row-u-1")).toBeInTheDocument();
    expect(screen.getByTestId("admin-user-row-u-2")).toBeInTheDocument();

    await user.click(screen.getByTestId("admin-delete-user-trigger"));
    await user.click(screen.getByTestId("admin-delete-user-confirm"));

    // cache.modify removes the u-1 edge from the live connection; the list
    // re-renders without that row while u-2 stays.
    await waitFor(() => {
      expect(screen.queryByTestId("admin-user-row-u-1")).not.toBeInTheDocument();
    });
    expect(screen.getByTestId("admin-user-row-u-2")).toBeInTheDocument();
    expect(toastSuccessMock).toHaveBeenCalledWith("User deleted.");

    // The cached connection reflects the removal and the decremented totalCount,
    // and the normalized User entity was evicted.
    const after = cache.readQuery<{ users: typeof conn }>({
      query: AdminUsersDocument,
      variables: initialVars,
    });
    expect(after?.users.edges.map((edge) => edge.node.id)).toEqual(["u-2"]);
    expect(after?.users.totalCount).toBe(1);
    expect(cache.extract()["User:u-1"]).toBeUndefined();
  });

  it("does not configure optimisticResponse for adminDeleteUser", () => {
    const source = readFileSync(
      join(process.cwd(), "src/app/admin/users/use-admin-user-mutations.ts"),
      "utf8",
    );
    const start = source.indexOf("runDeleteUser({");
    const end = source.indexOf("});", start);

    expect(start).toBeGreaterThanOrEqual(0);
    expect(source.slice(start, end)).not.toContain("optimisticResponse");
  });
});

// ---------------------------------------------------------------------------
// Mobile search takeover — flamingo:open-search opens the bar; the desktop
// input is gated mobile-off so the two do not double up on mobile.
// ---------------------------------------------------------------------------

describe("<AdminUsersClient> mobile search takeover", () => {
  it("opens the takeover on flamingo:open-search and renders the input", async () => {
    renderWithIntl(
      <MockedProvider mocks={[makeUsersMock(), makeRolesMock()]}>
        <AdminUsersClient />
      </MockedProvider>,
    );
    await screen.findByTestId("admin-users-list");

    expect(screen.queryByTestId("search-takeover")).not.toBeInTheDocument();

    act(() => {
      window.dispatchEvent(new CustomEvent("flamingo:open-search"));
    });

    expect(screen.getByTestId("search-takeover")).toBeInTheDocument();
    expect(screen.getByTestId("search-takeover-input")).toBeInTheDocument();
  });

  it("gates the desktop search input mobile-off (hidden md:block)", async () => {
    renderWithIntl(
      <MockedProvider mocks={[makeUsersMock(), makeRolesMock()]}>
        <AdminUsersClient />
      </MockedProvider>,
    );
    await screen.findByTestId("admin-users-list");

    const desktop = screen.getByRole("searchbox", { name: /search users/i });
    expect(desktop.parentElement).toHaveClass("hidden", "md:block");
  });
});

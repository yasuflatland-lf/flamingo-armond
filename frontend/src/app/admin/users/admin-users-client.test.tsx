// @vitest-environment jsdom

import { InMemoryCache } from "@apollo/client";
import { MockedProvider } from "@apollo/client/testing/react";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { GraphQLError } from "graphql";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  AdminEditUserDocument,
  AdminRolesDocument,
  AdminUserDocument,
  AdminUsersDocument,
} from "@/generated/graphql";
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
  roles: [],
};

const USER_2 = {
  __typename: "User" as const,
  id: "u-2",
  version: 7,
  displayName: "Bob",
  bio: null,
  avatarUrl: null,
  roles: [],
};

function userEdge(user: typeof USER_1) {
  return {
    __typename: "UserEdge" as const,
    cursor: user.id,
    node: user,
  };
}

function makeConnection(items: (typeof USER_1)[], hasNextPage = false, totalCount?: number) {
  return {
    __typename: "UserConnection" as const,
    edges: items.map(userEdge),
    pageInfo: {
      __typename: "PageInfo" as const,
      hasNextPage,
      hasPreviousPage: false,
      startCursor: items[0]?.id ?? null,
      endCursor: items[items.length - 1]?.id ?? null,
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
    render(
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

    render(
      <MockedProvider mocks={[makeUsersMock(), makeRolesMock()]}>
        <AdminUsersClient />
      </MockedProvider>,
    );

    expect(await screen.findByText("Alice")).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: /edit alice/i }));

    expect(mockPush).toHaveBeenCalledWith("/admin/users?edit=u-1", { scroll: false });
  });

  it("does not render inline role checkboxes in the user list", async () => {
    render(
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

    render(
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

    render(
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

    render(
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

    render(
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

    const { rerender } = render(
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
      after: USER_2.id,
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

    render(
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

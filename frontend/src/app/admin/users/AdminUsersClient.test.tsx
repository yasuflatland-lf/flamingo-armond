// @vitest-environment jsdom

import { InMemoryCache } from "@apollo/client";
import { MockedProvider } from "@apollo/client/testing/react";
import { render, screen, waitFor } from "@testing-library/react";
import { GraphQLError } from "graphql";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { AdminUsersDocument } from "@/generated/graphql";
import {
  type ApolloMockLeakSpyResult,
  installApolloMockLeakSpy,
} from "../../../../__tests__/utils/mock-apollo-paginated";
import { AdminUsersClient } from "./AdminUsersClient";
import { ADMIN_USERS_PAGE_SIZE } from "./queries";

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

const USER_1 = {
  __typename: "User" as const,
  id: "u-1",
  displayName: "Alice",
  bio: null,
  avatarUrl: null,
  roles: [],
};

const USER_2 = {
  __typename: "User" as const,
  id: "u-2",
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
  leakSpy = installApolloMockLeakSpy({ operationNames: ["AdminUsers"] });
  ioCallbacks = [];
  vi.stubGlobal("IntersectionObserver", FakeIntersectionObserver);
});

afterEach(() => {
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
  leakSpy.assertNoLeaks();
  leakSpy.teardown();
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
      <MockedProvider mocks={[initialMock, errorMock]} cache={cache}>
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

// @vitest-environment jsdom

import { InMemoryCache } from "@apollo/client";
import type { MockedResponse } from "@apollo/client/testing";
import { MockedProvider } from "@apollo/client/testing/react";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { GraphQLError } from "graphql";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { CARDGROUPS_DEFAULT_VARS } from "@/app/cardgroups/queries";
import { CreateCardgroupDocument, MyCardgroupsConnectionDocument } from "@/generated/graphql";
import { sanitizeReturnTo } from "@/lib/sanitize-return-to";
import { createSupabaseServerClient } from "@/lib/supabase/server";
import { NewCardgroupClient } from "./new-cardgroup-client";
import NewCardgroupPage from "./page";

// Stub next/navigation so the client component can render outside Next.js.
const mockPush = vi.fn();
const mockRefresh = vi.fn();
const mockRedirect = vi.fn((path: string) => {
  throw new Error(`REDIRECT:${path}`);
});
vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: mockPush, refresh: mockRefresh }),
  redirect: (path: string) => mockRedirect(path),
}));

// Stub next/link so it renders an anchor without Next.js router context.
vi.mock("next/link", () => ({
  default: ({ href, children, ...rest }: { href: string; children: React.ReactNode }) => (
    <a href={href} {...rest}>
      {children}
    </a>
  ),
}));

// Stub the Supabase server client — required because page.tsx imports it at
// module-evaluation time (the import itself triggers the module graph).
// Using vi.fn() so individual tests can override with mockResolvedValueOnce.
vi.mock("@/lib/supabase/server", () => ({
  createSupabaseServerClient: vi.fn(() =>
    Promise.resolve({
      auth: {
        getUser: () => Promise.resolve({ data: { user: { id: "u-1" } }, error: null }),
      },
    }),
  ),
}));

const CREATED_CARDGROUP = {
  __typename: "Cardgroup" as const,
  id: "cg-42",
  name: "My New Group",
  updatedAt: "2026-04-30T00:00:00Z",
};

function makeCreateMock(name: string, onCalled?: () => void) {
  return {
    request: {
      query: CreateCardgroupDocument,
      variables: { input: { name } },
    },
    result: () => {
      onCalled?.();
      return {
        data: {
          createCardgroup: {
            __typename: "CreateCardgroupPayload" as const,
            cardgroup: { ...CREATED_CARDGROUP, name },
          },
        },
      };
    },
  };
}

function renderPage(
  mocks: MockedResponse[] = [],
  errorPolicy?: "all" | "none" | "ignore",
  cache?: InMemoryCache,
  returnTo: string | null = null,
) {
  const defaultOptions = errorPolicy ? { mutate: { errorPolicy } } : undefined;
  render(
    <MockedProvider mocks={mocks} defaultOptions={defaultOptions} cache={cache}>
      <NewCardgroupClient returnTo={returnTo} />
    </MockedProvider>,
  );
}

describe("<NewCardgroupPage> (client)", () => {
  it("renders form with empty name by default", () => {
    renderPage();
    expect(screen.getByRole("textbox")).toHaveValue("");
    expect(screen.getByRole("button", { name: /create/i })).toBeInTheDocument();
  });

  it("renders page header and back link", () => {
    renderPage();
    expect(screen.getByRole("heading", { name: /new cardgroup/i })).toBeInTheDocument();
    const backLink = screen.getByRole("link");
    expect(backLink).toHaveAttribute("href", "/cardgroups");
  });

  it("welcome mode renders the welcome H1 and hides Back / 'New cardgroup'", () => {
    render(
      <MockedProvider mocks={[]}>
        <NewCardgroupClient showWelcome returnTo={null} />
      </MockedProvider>,
    );
    expect(
      screen.getByRole("heading", { level: 1, name: /welcome.*first cardgroup/i }),
    ).toBeInTheDocument();
    expect(screen.queryByRole("link", { name: /back/i })).not.toBeInTheDocument();
    expect(screen.queryByRole("heading", { name: /^new cardgroup$/i })).not.toBeInTheDocument();
  });

  it("submitting a valid name triggers the CreateCardgroup mutation", async () => {
    const user = userEvent.setup();
    const mutationCalled = vi.fn();

    renderPage([makeCreateMock("My New Group", mutationCalled)]);

    await user.type(screen.getByRole("textbox"), "My New Group");
    await user.click(screen.getByRole("button", { name: /create/i }));

    await waitFor(() => {
      expect(mutationCalled).toHaveBeenCalledOnce();
    });
  });

  it("on success calls router.push with returned id and router.refresh", async () => {
    const user = userEvent.setup();
    mockPush.mockClear();
    mockRefresh.mockClear();

    renderPage([makeCreateMock("My New Group")]);

    await user.type(screen.getByRole("textbox"), "My New Group");
    await user.click(screen.getByRole("button", { name: /create/i }));

    await waitFor(() => {
      expect(mockPush).toHaveBeenCalledWith(`/cardgroups/${CREATED_CARDGROUP.id}`);
    });
    expect(mockRefresh).toHaveBeenCalledOnce();
  });

  it("seeds the new cardgroup into a cold MyCardgroupsConnection cache via production update callback", async () => {
    // This test exercises the cold-cache `else` branch in
    // new-cardgroup-client.tsx's useMutation update() callback.
    // The cache has NO pre-seeded MyCardgroupsConnection entry — MockedProvider
    // calls the production update callback, which builds a minimal connection.
    // Per docs/backend/library-gotchas/inline-copy-of-production-logic-in-tests.md,
    // we do NOT re-implement the update logic here; the mutation callback in
    // new-cardgroup-client.tsx is the sole implementation.
    const user = userEvent.setup();
    mockPush.mockClear();

    // Cold cache: deliberately no pre-seeded MyCardgroupsConnectionDocument —
    // this exercises the cold-cache else-branch.
    const cache = new InMemoryCache();

    renderPage([makeCreateMock("Cold Cache Group")], undefined, cache);

    await user.type(screen.getByRole("textbox"), "Cold Cache Group");
    await user.click(screen.getByRole("button", { name: /create/i }));

    // Wait for navigation (signals mutation + update callback completed).
    await waitFor(() => {
      expect(mockPush).toHaveBeenCalledWith(`/cardgroups/${CREATED_CARDGROUP.id}`);
    });

    // The production else-branch in update() must have written a minimal
    // MyCardgroupsConnection into the cache.
    const result = cache.readQuery({
      query: MyCardgroupsConnectionDocument,
      variables: CARDGROUPS_DEFAULT_VARS,
    });
    expect(result?.myCardgroupsConnection.edges).toHaveLength(1);
    // makeCreateMock echoes the submitted name back in the result, so the node
    // name is "Cold Cache Group", not CREATED_CARDGROUP.name.
    expect(result?.myCardgroupsConnection.edges[0]).toMatchObject({
      cursor: CREATED_CARDGROUP.id,
      node: { id: CREATED_CARDGROUP.id, name: "Cold Cache Group" },
    });
    expect(result?.myCardgroupsConnection.pageInfo.hasNextPage).toBe(false);
    expect(result?.myCardgroupsConnection.totalCount).toBe(1);
  });

  it("writes the new cardgroup into the existing MyCardgroupsConnection cache", async () => {
    // This test exercises the warm-cache branch in new-cardgroup-client.tsx's
    // useMutation update() callback: when readQuery returns an existing
    // Connection the callback prepends the new edge and increments totalCount.
    // The production update() callback is the sole implementation — we do NOT
    // re-inline the update logic here.
    const user = userEvent.setup();
    mockPush.mockClear();

    const EXISTING_CARDGROUP = {
      __typename: "Cardgroup" as const,
      id: "cg-existing-1",
      name: "Existing Group",
      updatedAt: "2026-01-01T00:00:00Z",
    };

    // Warm cache: pre-seed the MyCardgroupsConnection with one existing entry
    // so the update() callback takes the warm-cache branch.
    const cache = new InMemoryCache();
    cache.writeQuery({
      query: MyCardgroupsConnectionDocument,
      variables: CARDGROUPS_DEFAULT_VARS,
      data: {
        myCardgroupsConnection: {
          __typename: "CardgroupConnection" as const,
          edges: [
            {
              __typename: "CardgroupEdge" as const,
              cursor: EXISTING_CARDGROUP.id,
              node: EXISTING_CARDGROUP,
            },
          ],
          pageInfo: {
            __typename: "PageInfo" as const,
            hasNextPage: false,
            hasPreviousPage: false,
            startCursor: EXISTING_CARDGROUP.id,
            endCursor: EXISTING_CARDGROUP.id,
          },
          totalCount: 1,
        },
      },
    });

    renderPage([makeCreateMock("Warm Cache Group")], undefined, cache);

    await user.type(screen.getByRole("textbox"), "Warm Cache Group");
    await user.click(screen.getByRole("button", { name: /create/i }));

    // Wait for navigation — signals the mutation + update() callback completed.
    await waitFor(() => {
      expect(mockPush).toHaveBeenCalledWith(`/cardgroups/${CREATED_CARDGROUP.id}`);
    });

    // The production warm-cache branch in update() must have prepended the new
    // edge and bumped totalCount.
    const result = cache.readQuery({
      query: MyCardgroupsConnectionDocument,
      variables: CARDGROUPS_DEFAULT_VARS,
    });
    expect(result?.myCardgroupsConnection.edges).toHaveLength(2);
    // Newest entry is prepended at index 0.
    expect(result?.myCardgroupsConnection.edges[0]).toMatchObject({
      cursor: CREATED_CARDGROUP.id,
      node: { id: CREATED_CARDGROUP.id, name: "Warm Cache Group" },
    });
    // Pre-seeded entry is shifted to index 1.
    expect(result?.myCardgroupsConnection.edges[1]).toMatchObject({
      cursor: EXISTING_CARDGROUP.id,
      node: { id: EXISTING_CARDGROUP.id },
    });
    expect(result?.myCardgroupsConnection.totalCount).toBe(2);
  });

  it("BAD_USER_INPUT on field 'name' shows inline error", async () => {
    const user = userEvent.setup();

    const mocks = [
      {
        request: {
          query: CreateCardgroupDocument,
          variables: { input: { name: "Bad Name" } },
        },
        result: {
          errors: [
            new GraphQLError("name already exists", {
              extensions: { code: "BAD_USER_INPUT", field: "name" },
            }),
          ],
        },
      },
    ];

    renderPage(mocks, "all");

    await user.type(screen.getByRole("textbox"), "Bad Name");
    await user.click(screen.getByRole("button", { name: /create/i }));

    const errorEl = await screen.findByText("name already exists");
    expect(errorEl).toBeInTheDocument();
    expect(errorEl.className).toMatch(/text-destructive/);
  });

  it("UNAUTHENTICATED error shows session-expired banner", async () => {
    const user = userEvent.setup();

    const mocks = [
      {
        request: {
          query: CreateCardgroupDocument,
          variables: { input: { name: "My Group" } },
        },
        result: {
          errors: [
            new GraphQLError("Unauthenticated", {
              extensions: { code: "UNAUTHENTICATED" },
            }),
          ],
        },
      },
    ];

    renderPage(mocks, "all");

    await user.type(screen.getByRole("textbox"), "My Group");
    await user.click(screen.getByRole("button", { name: /create/i }));

    await waitFor(() => {
      expect(screen.getByText("Your session expired. Please sign in again.")).toBeInTheDocument();
    });
    expect(screen.getByRole("alert")).toBeInTheDocument();
  });

  it("on success with returnTo navigates to returnTo?cardgroup=<id> and does not refresh", async () => {
    const user = userEvent.setup();
    mockPush.mockClear();
    mockRefresh.mockClear();

    renderPage([makeCreateMock("My New Group")], undefined, undefined, "/cards/new");

    await user.type(screen.getByRole("textbox"), "My New Group");
    await user.click(screen.getByRole("button", { name: /create/i }));

    await waitFor(() => {
      expect(mockPush).toHaveBeenCalledWith(`/cards/new?cardgroup=${CREATED_CARDGROUP.id}`);
    });
    expect(mockRefresh).not.toHaveBeenCalled();
  });

  it("on success with returnTo containing query string uses & separator", async () => {
    const user = userEvent.setup();
    mockPush.mockClear();

    renderPage([makeCreateMock("My New Group")], undefined, undefined, "/foo?bar=1");

    await user.type(screen.getByRole("textbox"), "My New Group");
    await user.click(screen.getByRole("button", { name: /create/i }));

    await waitFor(() => {
      expect(mockPush).toHaveBeenCalledWith(`/foo?bar=1&cardgroup=${CREATED_CARDGROUP.id}`);
    });
  });

  it("on success with null returnTo falls back to /cardgroups/<id>", async () => {
    const user = userEvent.setup();
    mockPush.mockClear();

    renderPage([makeCreateMock("My New Group")], undefined, undefined, null);

    await user.type(screen.getByRole("textbox"), "My New Group");
    await user.click(screen.getByRole("button", { name: /create/i }));

    await waitFor(() => {
      expect(mockPush).toHaveBeenCalledWith(`/cardgroups/${CREATED_CARDGROUP.id}`);
    });
  });
});

describe("authentication boundary", () => {
  let consoleErrorSpy: ReturnType<typeof vi.spyOn>;

  beforeEach(() => {
    consoleErrorSpy = vi.spyOn(console, "error").mockImplementation(() => {});
    mockRedirect.mockClear();
  });

  afterEach(() => {
    consoleErrorSpy.mockRestore();
  });

  it("redirects to /login when getUser returns no user (AuthSessionMissingError)", async () => {
    vi.mocked(createSupabaseServerClient).mockResolvedValueOnce({
      auth: {
        getUser: () =>
          Promise.resolve({
            data: { user: null },
            error: { name: "AuthSessionMissingError", message: "Auth session missing!" },
          }),
      },
    } as Awaited<ReturnType<typeof createSupabaseServerClient>>);

    await expect(NewCardgroupPage({ searchParams: Promise.resolve({}) })).rejects.toThrow(
      "REDIRECT:/login",
    );

    expect(mockRedirect).toHaveBeenCalledWith("/login");
  });

  it("console.errors and rethrows on a non-AuthSessionMissingError getUser failure", async () => {
    vi.mocked(createSupabaseServerClient).mockResolvedValueOnce({
      auth: {
        getUser: () =>
          Promise.resolve({
            data: { user: null },
            error: { name: "SomeOtherError", message: "boom" },
          }),
      },
    } as Awaited<ReturnType<typeof createSupabaseServerClient>>);

    await expect(NewCardgroupPage({ searchParams: Promise.resolve({}) })).rejects.toMatchObject({
      name: "SomeOtherError",
      message: "boom",
    });

    expect(consoleErrorSpy).toHaveBeenCalledWith(
      "[cardgroups-new] getUser() failed:",
      "SomeOtherError",
      "boom",
    );
  });
});

describe("sanitizeReturnTo", () => {
  it("allows internal paths starting with /", () => {
    expect(sanitizeReturnTo("/cards/new")).toBe("/cards/new");
  });

  it("allows internal paths with query string", () => {
    expect(sanitizeReturnTo("/cards/new?foo=1")).toBe("/cards/new?foo=1");
  });

  it("rejects protocol-relative URLs starting with //", () => {
    expect(sanitizeReturnTo("//evil.com")).toBeNull();
  });

  it("rejects https:// URLs", () => {
    expect(sanitizeReturnTo("https://evil.com")).toBeNull();
  });

  it("rejects bare hostnames without leading slash", () => {
    expect(sanitizeReturnTo("evil.com")).toBeNull();
  });

  it("rejects undefined", () => {
    expect(sanitizeReturnTo(undefined)).toBeNull();
  });

  it("rejects empty string", () => {
    expect(sanitizeReturnTo("")).toBeNull();
  });

  it("rejects backslash-bypass /\\evil.com", () => {
    expect(sanitizeReturnTo("/\\evil.com")).toBeNull();
  });

  it("rejects backslash-bypass /\\\\evil.com", () => {
    // double-escaped to land "/\\evil.com" as the runtime string -- verify the helper
    // when called with the raw form a browser may emit
    expect(sanitizeReturnTo("/\\\\evil.com")).toBeNull();
  });
});

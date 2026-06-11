// @vitest-environment jsdom

import { InMemoryCache } from "@apollo/client";
import { CombinedGraphQLErrors } from "@apollo/client/errors";
import type { MockedResponse } from "@apollo/client/testing";
import { MockedProvider } from "@apollo/client/testing/react";
import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { headers } from "next/headers";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { CARDGROUPS_DEFAULT_VARS } from "@/app/cardgroups/queries";
import { CreateCardgroupDocument, MyCardgroupsConnectionDocument } from "@/generated/graphql";
import { sanitizeReturnTo } from "@/lib/sanitize-return-to";
import { renderWithIntl } from "@/test/render-with-intl";
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

// Stub next/headers so the page can read the middleware-forwarded auth status.
// Default: authenticated. Individual tests can override with mockResolvedValueOnce.
vi.mock("next/headers", () => ({
  headers: vi.fn(async () => new Headers({ "x-auth-status": "authenticated" })),
}));

const CREATED_CARDGROUP = {
  __typename: "Cardgroup" as const,
  id: "cg-42",
  name: "My New Group",
  updatedAt: "2026-04-30T00:00:00Z",
};

/**
 * Build a CombinedGraphQLErrors carrying a single extension code. Mirrors the
 * runtime shape Apollo Client v4 surfaces to useMutation's catch path — this
 * is the shape liftGraphQLCodes narrows via CombinedGraphQLErrors.is.
 */
function makeCodedError(code: string): CombinedGraphQLErrors {
  return new CombinedGraphQLErrors({
    errors: [{ message: "transport", extensions: { code } }],
  });
}

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
            __typename: "CreateCardgroupSuccess" as const,
            cardgroup: { ...CREATED_CARDGROUP, name },
          },
        },
      };
    },
  };
}

function renderPage(
  mocks: MockedResponse[] = [],
  cache?: InMemoryCache,
  returnTo: string | null = null,
) {
  renderWithIntl(
    <MockedProvider mocks={mocks} cache={cache}>
      <NewCardgroupClient returnTo={returnTo} />
    </MockedProvider>,
  );
}

describe("<NewCardgroupPage> (client)", () => {
  afterEach(() => {
    mockPush.mockClear();
    mockRefresh.mockClear();
  });

  it("renders form with empty name by default", () => {
    renderPage();
    expect(screen.getByRole("textbox")).toHaveValue("");
    expect(screen.getByRole("button", { name: /create/i })).toBeInTheDocument();
  });

  it("renders page header without a back link", () => {
    renderPage();
    expect(screen.getByRole("heading", { name: /new cardgroup/i })).toBeInTheDocument();
    // The "← Back" link was removed; the drawer-based create flow and the app
    // shell nav provide navigation instead.
    expect(screen.queryByRole("link", { name: /back/i })).not.toBeInTheDocument();
  });

  it("welcome mode renders the welcome H1 and hides Back / 'New cardgroup'", () => {
    renderWithIntl(
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

  it("CreateCardgroupSuccess — submits valid name, navigates to cardgroup detail, writes cache", async () => {
    const user = userEvent.setup();
    const mutationCalled = vi.fn();

    renderPage([makeCreateMock("My New Group", mutationCalled)]);

    await user.type(screen.getByRole("textbox"), "My New Group");
    await user.click(screen.getByRole("button", { name: /create/i }));

    await waitFor(() => {
      expect(mutationCalled).toHaveBeenCalledOnce();
    });
    expect(mockPush).toHaveBeenCalledWith(`/cardgroups/${CREATED_CARDGROUP.id}`);
    expect(mockRefresh).toHaveBeenCalledOnce();
  });

  it("InputValidationError — shows inline field error, no navigation", async () => {
    const user = userEvent.setup();

    const mocks = [
      {
        request: {
          query: CreateCardgroupDocument,
          variables: { input: { name: "Bad Name" } },
        },
        result: () => ({
          data: {
            createCardgroup: {
              __typename: "InputValidationError" as const,
              field: "name",
              message: "name already exists",
            },
          },
        }),
      },
    ];

    renderPage(mocks);

    await user.type(screen.getByRole("textbox"), "Bad Name");
    await user.click(screen.getByRole("button", { name: /create/i }));

    await waitFor(() => {
      expect(screen.getByTestId("cardgroup-new-validation-error")).toBeInTheDocument();
    });
    expect(screen.getByTestId("cardgroup-new-validation-error")).toHaveTextContent(
      "name already exists",
    );
    // No navigation: the error variant is data, not a success.
    expect(mockPush).not.toHaveBeenCalled();
    expect(mockRefresh).not.toHaveBeenCalled();
  });

  it("InputValidationError — does not write to the MyCardgroupsConnection cache (regression guard)", async () => {
    // When the server returns InputValidationError, the useMutation update()
    // callback guards on __typename !== "CreateCardgroupSuccess" and returns
    // early without calling cache.writeQuery. This test asserts that the cache
    // is not mutated — the guard is load-bearing.
    const user = userEvent.setup();

    const cache = new InMemoryCache();

    const mocks = [
      {
        request: {
          query: CreateCardgroupDocument,
          variables: { input: { name: "Bad Name" } },
        },
        result: () => ({
          data: {
            createCardgroup: {
              __typename: "InputValidationError" as const,
              field: "name",
              message: "name already exists",
            },
          },
        }),
      },
    ];

    renderWithIntl(
      <MockedProvider mocks={mocks} cache={cache}>
        <NewCardgroupClient returnTo={null} />
      </MockedProvider>,
    );

    await user.type(screen.getByRole("textbox"), "Bad Name");
    await user.click(screen.getByRole("button", { name: /create/i }));

    // Wait for the validation error banner to appear (mutation completed).
    await waitFor(() => {
      expect(screen.getByTestId("cardgroup-new-validation-error")).toBeInTheDocument();
    });

    // The MyCardgroupsConnection must NOT have been written to the cache —
    // the update() early-return guard must have fired.
    const result = cache.readQuery({
      query: MyCardgroupsConnectionDocument,
      variables: CARDGROUPS_DEFAULT_VARS,
    });
    expect(result).toBeNull();
  });

  it("UNAUTHENTICATED transport rejection — shows session-expired banner with login link, no navigation", async () => {
    const user = userEvent.setup();

    const mocks = [
      {
        request: {
          query: CreateCardgroupDocument,
          variables: { input: { name: "My Group" } },
        },
        error: makeCodedError("UNAUTHENTICATED"),
      },
    ];

    const warnSpy = vi.spyOn(console, "warn").mockImplementation(() => {});
    try {
      renderPage(mocks);

      await user.type(screen.getByRole("textbox"), "My Group");
      await user.click(screen.getByRole("button", { name: /create/i }));

      await waitFor(() => {
        expect(screen.getByTestId("cardgroup-new-auth-error")).toBeInTheDocument();
      });
      const banner = screen.getByTestId("cardgroup-new-auth-error");
      expect(banner).toHaveTextContent(/session has expired/i);
      const signIn = within(banner).getByRole("link", { name: /sign in again/i });
      expect(signIn).toHaveAttribute("href", "/login");

      expect(mockPush).not.toHaveBeenCalled();
      expect(mockRefresh).not.toHaveBeenCalled();
      // Auth branch returns early — generic warn must NOT fire.
      expect(warnSpy).not.toHaveBeenCalled();
    } finally {
      warnSpy.mockRestore();
    }
  });

  it("FORBIDDEN transport rejection — shows permission banner with login link, no navigation", async () => {
    const user = userEvent.setup();

    const mocks = [
      {
        request: {
          query: CreateCardgroupDocument,
          variables: { input: { name: "My Group" } },
        },
        error: makeCodedError("FORBIDDEN"),
      },
    ];

    const warnSpy = vi.spyOn(console, "warn").mockImplementation(() => {});
    try {
      renderPage(mocks);

      await user.type(screen.getByRole("textbox"), "My Group");
      await user.click(screen.getByRole("button", { name: /create/i }));

      await waitFor(() => {
        expect(screen.getByTestId("cardgroup-new-auth-error")).toBeInTheDocument();
      });
      expect(screen.getByTestId("cardgroup-new-auth-error")).toHaveTextContent(
        /do not have permission/i,
      );

      expect(mockPush).not.toHaveBeenCalled();
      expect(mockRefresh).not.toHaveBeenCalled();
      expect(warnSpy).not.toHaveBeenCalled();
    } finally {
      warnSpy.mockRestore();
    }
  });

  it("generic transport rejection — warns and shows no auth/validation banner", async () => {
    const user = userEvent.setup();

    const networkError = new Error("network down");
    const mocks = [
      {
        request: {
          query: CreateCardgroupDocument,
          variables: { input: { name: "My Group" } },
        },
        error: networkError,
      },
    ];

    const warnSpy = vi.spyOn(console, "warn").mockImplementation(() => {});
    try {
      renderPage(mocks);

      await user.type(screen.getByRole("textbox"), "My Group");
      await user.click(screen.getByRole("button", { name: /create/i }));

      await waitFor(() => expect(warnSpy).toHaveBeenCalled());

      expect(mockPush).not.toHaveBeenCalled();
      expect(mockRefresh).not.toHaveBeenCalled();
      expect(screen.queryByTestId("cardgroup-new-auth-error")).not.toBeInTheDocument();
      expect(screen.queryByTestId("cardgroup-new-validation-error")).not.toBeInTheDocument();
      expect(
        screen.queryByTestId("cardgroup-new-unexpected-payload-error"),
      ).not.toBeInTheDocument();

      // Warn payload MUST NOT include err.message — backend messages may echo user input.
      expect(warnSpy).not.toHaveBeenCalledWith(
        expect.anything(),
        expect.objectContaining({ message: expect.anything() }),
      );
      expect(warnSpy).toHaveBeenCalledWith(
        expect.stringContaining("createCardgroup rejected"),
        expect.objectContaining({ name: "Error", codes: [] }),
      );
    } finally {
      warnSpy.mockRestore();
    }
  });

  it("unexpected __typename — warns and shows degraded banner, no navigation", async () => {
    const user = userEvent.setup();

    const mocks = [
      {
        request: {
          query: CreateCardgroupDocument,
          variables: { input: { name: "My Group" } },
        },
        result: () => ({
          data: {
            createCardgroup: {
              __typename: "FutureVariantClientDidNotKnowAbout",
            } as never,
          },
        }),
      },
    ];

    const warnSpy = vi.spyOn(console, "warn").mockImplementation(() => {});
    try {
      renderPage(mocks);

      await user.type(screen.getByRole("textbox"), "My Group");
      await user.click(screen.getByRole("button", { name: /create/i }));

      await waitFor(() => expect(warnSpy).toHaveBeenCalled());

      expect(screen.getByTestId("cardgroup-new-unexpected-payload-error")).toBeInTheDocument();
      expect(screen.getByTestId("cardgroup-new-unexpected-payload-error")).toHaveTextContent(
        /something went wrong/i,
      );
      expect(screen.queryByTestId("cardgroup-new-validation-error")).not.toBeInTheDocument();

      expect(mockPush).not.toHaveBeenCalled();
      expect(mockRefresh).not.toHaveBeenCalled();

      expect(warnSpy).toHaveBeenCalledWith(
        expect.stringContaining("unexpected createCardgroup payload"),
        expect.objectContaining({ typename: "FutureVariantClientDidNotKnowAbout" }),
      );
    } finally {
      warnSpy.mockRestore();
    }
  });

  it("null createCardgroup payload (partial-response null bubble) — warns and shows degraded banner", async () => {
    const user = userEvent.setup();

    const mocks = [
      {
        request: {
          query: CreateCardgroupDocument,
          variables: { input: { name: "My Group" } },
        },
        result: () => ({
          data: { createCardgroup: null as never },
        }),
      },
    ];

    const warnSpy = vi.spyOn(console, "warn").mockImplementation(() => {});
    try {
      renderPage(mocks);

      await user.type(screen.getByRole("textbox"), "My Group");
      await user.click(screen.getByRole("button", { name: /create/i }));

      await waitFor(() => expect(warnSpy).toHaveBeenCalled());

      expect(screen.getByTestId("cardgroup-new-unexpected-payload-error")).toBeInTheDocument();
      expect(screen.getByTestId("cardgroup-new-unexpected-payload-error")).toHaveTextContent(
        /something went wrong/i,
      );

      expect(mockPush).not.toHaveBeenCalled();
      expect(mockRefresh).not.toHaveBeenCalled();

      expect(warnSpy).toHaveBeenCalledWith(
        expect.stringContaining("unexpected createCardgroup payload"),
        expect.objectContaining({ typename: null }),
      );
    } finally {
      warnSpy.mockRestore();
    }
  });

  it("cold-cache — seeds a minimal MyCardgroupsConnection via the update callback", async () => {
    // This test exercises the cold-cache `else` branch in
    // new-cardgroup-client.tsx's useMutation update() callback.
    // The cache has NO pre-seeded MyCardgroupsConnection entry — MockedProvider
    // calls the production update callback, which builds a minimal connection.
    const user = userEvent.setup();

    // Cold cache: deliberately no pre-seeded MyCardgroupsConnectionDocument —
    // this exercises the cold-cache else-branch.
    const cache = new InMemoryCache();

    renderPage([makeCreateMock("Cold Cache Group")], cache);

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

  it("warm-cache — prepends the new cardgroup edge and increments totalCount", async () => {
    // This test exercises the warm-cache branch in new-cardgroup-client.tsx's
    // useMutation update() callback: when readQuery returns an existing
    // Connection the callback prepends the new edge and increments totalCount.
    const user = userEvent.setup();

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

    renderPage([makeCreateMock("Warm Cache Group")], cache);

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

  it("on success with returnTo navigates to returnTo?cardgroup=<id> and does not refresh", async () => {
    const user = userEvent.setup();

    renderPage([makeCreateMock("My New Group")], undefined, "/cards/new");

    await user.type(screen.getByRole("textbox"), "My New Group");
    await user.click(screen.getByRole("button", { name: /create/i }));

    await waitFor(() => {
      expect(mockPush).toHaveBeenCalledWith(`/cards/new?cardgroup=${CREATED_CARDGROUP.id}`);
    });
    expect(mockRefresh).not.toHaveBeenCalled();
  });

  it("on success with returnTo containing query string uses & separator", async () => {
    const user = userEvent.setup();

    renderPage([makeCreateMock("My New Group")], undefined, "/foo?bar=1");

    await user.type(screen.getByRole("textbox"), "My New Group");
    await user.click(screen.getByRole("button", { name: /create/i }));

    await waitFor(() => {
      expect(mockPush).toHaveBeenCalledWith(`/foo?bar=1&cardgroup=${CREATED_CARDGROUP.id}`);
    });
  });

  it("on success with null returnTo falls back to /cardgroups/<id>", async () => {
    const user = userEvent.setup();

    renderPage([makeCreateMock("My New Group")], undefined, null);

    await user.type(screen.getByRole("textbox"), "My New Group");
    await user.click(screen.getByRole("button", { name: /create/i }));

    await waitFor(() => {
      expect(mockPush).toHaveBeenCalledWith(`/cardgroups/${CREATED_CARDGROUP.id}`);
    });
  });
});

describe("authentication boundary", () => {
  beforeEach(() => {
    mockRedirect.mockClear();
  });

  it("redirects to /login when x-auth-status is anonymous", async () => {
    vi.mocked(headers).mockResolvedValueOnce(new Headers({ "x-auth-status": "anonymous" }));

    await expect(NewCardgroupPage({ searchParams: Promise.resolve({}) })).rejects.toThrow(
      "REDIRECT:/login",
    );

    expect(mockRedirect).toHaveBeenCalledWith("/login");
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

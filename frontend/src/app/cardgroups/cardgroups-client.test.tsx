// @vitest-environment jsdom
import { InMemoryCache } from "@apollo/client";
import { MockedProvider } from "@apollo/client/testing/react";
import { act, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { GraphQLError } from "graphql";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { DeleteCardgroupDocument, MyCardgroupsConnectionDocument } from "@/generated/graphql";
import { UndoDeleteProvider } from "@/lib/undo-delete";
import {
  type ApolloMockLeakSpyResult,
  installApolloMockLeakSpy,
} from "../../../__tests__/utils/mock-apollo-paginated";
import CardgroupsClient from "./cardgroups-client";
import { CARDGROUPS_DEFAULT_VARS, CARDGROUPS_PAGE_SIZE } from "./queries";

// ---------------------------------------------------------------------------
// sonner mock — capture Undo action callback for programmatic invocation.
// ---------------------------------------------------------------------------
let lastToastLabel: string | undefined;
let lastUndoAction: (() => void) | undefined;
vi.mock("sonner", () => ({
  toast: vi.fn((label: string, opts?: { action?: { onClick?: () => void } }) => {
    lastToastLabel = label;
    lastUndoAction = opts?.action?.onClick;
    return "toast-id";
  }),
  Toaster: () => null,
}));

// ---------------------------------------------------------------------------
// Stub next/navigation and next/link
// ---------------------------------------------------------------------------

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: vi.fn(), refresh: vi.fn() }),
  redirect: vi.fn((path: string) => {
    throw new Error(`REDIRECT:${path}`);
  }),
  usePathname: () => "/cardgroups",
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
// Fixtures
// ---------------------------------------------------------------------------

const CG_1 = {
  __typename: "Cardgroup" as const,
  id: "cg-1",
  name: "Spanish Vocab",
  updatedAt: "2024-06-15T10:00:00.000Z",
};

const CG_2 = {
  __typename: "Cardgroup" as const,
  id: "cg-2",
  name: "Math Formulas",
  updatedAt: "2024-05-20T08:00:00.000Z",
};

const CG_3 = {
  __typename: "Cardgroup" as const,
  id: "cg-3",
  name: "Biology Notes",
  updatedAt: "2024-04-10T06:00:00.000Z",
};

function cgEdge(cg: typeof CG_1) {
  return {
    __typename: "CardgroupEdge" as const,
    cursor: cg.id,
    node: cg,
  };
}

function makeConnection(items: (typeof CG_1)[], hasNextPage = false, totalCount?: number) {
  return {
    __typename: "CardgroupConnection" as const,
    edges: items.map(cgEdge),
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

// ---------------------------------------------------------------------------
// IntersectionObserver stub
// ---------------------------------------------------------------------------

// Holds the callback for the most recently created observer so tests can
// trigger intersection events.
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

// ---------------------------------------------------------------------------
// Lifecycle — leak spy + stub IntersectionObserver
// ---------------------------------------------------------------------------

let leakSpy: ApolloMockLeakSpyResult;

beforeEach(() => {
  leakSpy = installApolloMockLeakSpy({
    operationNames: ["MyCardgroupsConnection", "DeleteCardgroup"],
  });
  ioCallbacks = [];
  lastToastLabel = undefined;
  lastUndoAction = undefined;
  vi.stubGlobal("IntersectionObserver", FakeIntersectionObserver);
});

afterEach(() => {
  // Spy teardown: unstub globals first, then assert + tear down the leak spy last.
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
  vi.useRealTimers();
  leakSpy.assertNoLeaks();
  leakSpy.teardown();
});

// ---------------------------------------------------------------------------
// Render helper
// ---------------------------------------------------------------------------

function renderClient(
  mocks: unknown[],
  initialConnection: ReturnType<typeof makeConnection> | null = null,
  cache?: InMemoryCache,
) {
  render(
    <MockedProvider mocks={mocks as never} cache={cache}>
      <UndoDeleteProvider>
        <CardgroupsClient initialConnection={initialConnection} />
      </UndoDeleteProvider>
    </MockedProvider>,
  );
}

// ---------------------------------------------------------------------------
// Scenarios
// ---------------------------------------------------------------------------

describe("<CardgroupsClient>", () => {
  // The desktop "New cardgroup" header button is hidden below md; on mobile the
  // nav-header "+" dispatches flamingo:add-cardgroup and the empty-state CTA
  // calls openAddSheet directly.
  it("hides the 'New cardgroup' header button below md", async () => {
    const cache = new InMemoryCache();
    const emptyConn = makeConnection([]);
    cache.writeQuery({
      query: MyCardgroupsConnectionDocument,
      variables: { first: CARDGROUPS_PAGE_SIZE, search: null },
      data: { myCardgroupsConnection: emptyConn },
    });

    const initialMock = {
      request: {
        query: MyCardgroupsConnectionDocument,
        variables: { first: CARDGROUPS_PAGE_SIZE, search: null },
      },
      result: { data: { myCardgroupsConnection: emptyConn } },
    };

    renderClient([initialMock], null, cache);

    const button = await screen.findByTestId("cardgroups-header-new-btn");
    expect(button.className).toContain("hidden");
    expect(button.className).toContain("md:inline-flex");
  });

  // The desktop "New cardgroup" button opens the in-list create drawer instead
  // of navigating to /cardgroups/new.
  it("opens the create-cardgroup drawer when the desktop 'New cardgroup' button is clicked", async () => {
    const user = userEvent.setup();
    const cache = new InMemoryCache();
    const emptyConn = makeConnection([]);
    cache.writeQuery({
      query: MyCardgroupsConnectionDocument,
      variables: { first: CARDGROUPS_PAGE_SIZE, search: null },
      data: { myCardgroupsConnection: emptyConn },
    });
    const initialMock = {
      request: {
        query: MyCardgroupsConnectionDocument,
        variables: { first: CARDGROUPS_PAGE_SIZE, search: null },
      },
      result: { data: { myCardgroupsConnection: emptyConn } },
    };

    renderClient([initialMock], null, cache);

    await user.click(await screen.findByTestId("cardgroups-header-new-btn"));

    expect(await screen.findByRole("textbox", { name: /name/i })).toBeInTheDocument();
  });

  it("opens the create-cardgroup drawer on the flamingo:add-cardgroup event (nav-header +)", async () => {
    const cache = new InMemoryCache();
    const emptyConn = makeConnection([]);
    cache.writeQuery({
      query: MyCardgroupsConnectionDocument,
      variables: { first: CARDGROUPS_PAGE_SIZE, search: null },
      data: { myCardgroupsConnection: emptyConn },
    });
    const initialMock = {
      request: {
        query: MyCardgroupsConnectionDocument,
        variables: { first: CARDGROUPS_PAGE_SIZE, search: null },
      },
      result: { data: { myCardgroupsConnection: emptyConn } },
    };

    renderClient([initialMock], null, cache);
    // Let the initial query settle before dispatching the open event.
    await screen.findByTestId("cardgroups-header-new-btn");

    const event = new CustomEvent("flamingo:add-cardgroup", { cancelable: true });
    act(() => {
      window.dispatchEvent(event);
    });
    // The listener cancels any default navigation.
    expect(event.defaultPrevented).toBe(true);
    expect(await screen.findByRole("textbox", { name: /name/i })).toBeInTheDocument();
  });

  // S1: empty state — no cardgroups, no active search
  it("renders empty state when no cardgroups and no search", async () => {
    const cache = new InMemoryCache();
    const emptyConn = makeConnection([]);
    cache.writeQuery({
      query: MyCardgroupsConnectionDocument,
      variables: { first: CARDGROUPS_PAGE_SIZE, search: null },
      data: { myCardgroupsConnection: emptyConn },
    });

    const initialMock = {
      request: {
        query: MyCardgroupsConnectionDocument,
        variables: { first: CARDGROUPS_PAGE_SIZE, search: null },
      },
      result: { data: { myCardgroupsConnection: emptyConn } },
    };

    renderClient([initialMock], null, cache);

    expect(await screen.findByTestId("cardgroups-empty")).toBeInTheDocument();
    expect(screen.getByText("No cardgroups yet")).toBeInTheDocument();
    expect(screen.queryByTestId("cardgroups-list")).not.toBeInTheDocument();
  });

  // S4a: empty-state CTA opens the create drawer (replaces the removed FAB for mobile discoverability)
  it("opens the create-cardgroup drawer when the empty-state CTA button is clicked", async () => {
    const user = userEvent.setup();
    const cache = new InMemoryCache();
    const emptyConn = makeConnection([]);
    cache.writeQuery({
      query: MyCardgroupsConnectionDocument,
      variables: { first: CARDGROUPS_PAGE_SIZE, search: null },
      data: { myCardgroupsConnection: emptyConn },
    });

    const initialMock = {
      request: {
        query: MyCardgroupsConnectionDocument,
        variables: { first: CARDGROUPS_PAGE_SIZE, search: null },
      },
      result: { data: { myCardgroupsConnection: emptyConn } },
    };

    renderClient([initialMock], null, cache);

    // Wait for the empty state to render.
    await screen.findByTestId("cardgroups-empty");

    // The CTA inside the empty state must be present.
    const ctaButton = screen.getByTestId("cardgroups-empty-cta");
    expect(ctaButton).toBeInTheDocument();

    // Clicking the CTA opens the create drawer.
    await user.click(ctaButton);

    expect(await screen.findByRole("textbox", { name: /name/i })).toBeInTheDocument();
  });

  // S4: renders list when query returns edges
  it("renders cardgroup list when query returns edges", async () => {
    const cache = new InMemoryCache();
    const conn = makeConnection([CG_1, CG_2]);
    cache.writeQuery({
      query: MyCardgroupsConnectionDocument,
      variables: { first: CARDGROUPS_PAGE_SIZE, search: null },
      data: { myCardgroupsConnection: conn },
    });

    const listMock = {
      request: {
        query: MyCardgroupsConnectionDocument,
        variables: { first: CARDGROUPS_PAGE_SIZE, search: null },
      },
      result: { data: { myCardgroupsConnection: conn } },
    };

    renderClient([listMock], null, cache);

    expect(await screen.findByTestId("cardgroups-list")).toBeInTheDocument();
    expect(screen.getByText("Spanish Vocab")).toBeInTheDocument();
    expect(screen.getByText("Math Formulas")).toBeInTheDocument();
  });

  // S2: no-hits state when search yields zero matches
  it("renders no-hits state when search yields zero matches", async () => {
    const user = userEvent.setup({ delay: null });
    vi.useFakeTimers({ shouldAdvanceTime: true });

    const cache = new InMemoryCache();
    const emptyConn = makeConnection([]);
    cache.writeQuery({
      query: MyCardgroupsConnectionDocument,
      variables: { first: CARDGROUPS_PAGE_SIZE, search: null },
      data: { myCardgroupsConnection: emptyConn },
    });

    const initialMock = {
      request: {
        query: MyCardgroupsConnectionDocument,
        variables: { first: CARDGROUPS_PAGE_SIZE, search: null },
      },
      result: { data: { myCardgroupsConnection: emptyConn } },
    };
    const searchMock = {
      request: {
        query: MyCardgroupsConnectionDocument,
        variables: { first: CARDGROUPS_PAGE_SIZE, search: "nonexistent" },
      },
      result: { data: { myCardgroupsConnection: makeConnection([]) } },
    };

    renderClient([initialMock, searchMock], null, cache);

    // Wait for initial empty state
    expect(await screen.findByTestId("cardgroups-empty")).toBeInTheDocument();

    const searchInput = screen.getByRole("searchbox");
    await user.type(searchInput, "nonexistent");

    // Advance past the 300ms debounce
    vi.advanceTimersByTime(300);
    vi.useRealTimers();

    await waitFor(() => {
      expect(screen.getByTestId("cardgroups-empty-search")).toBeInTheDocument();
    });
    expect(screen.getByText(/No cardgroups match "nonexistent"/)).toBeInTheDocument();
  });

  // S3: debounces search input — query fires only after 300ms
  it("debounces search input by 300ms", async () => {
    const user = userEvent.setup({ delay: null });
    vi.useFakeTimers({ shouldAdvanceTime: true });

    const cache = new InMemoryCache();
    const emptyConn = makeConnection([]);
    cache.writeQuery({
      query: MyCardgroupsConnectionDocument,
      variables: { first: CARDGROUPS_PAGE_SIZE, search: null },
      data: { myCardgroupsConnection: emptyConn },
    });

    const initialMock = {
      request: {
        query: MyCardgroupsConnectionDocument,
        variables: { first: CARDGROUPS_PAGE_SIZE, search: null },
      },
      result: { data: { myCardgroupsConnection: emptyConn } },
    };
    let queryCalled = false;
    const searchMock = {
      request: {
        query: MyCardgroupsConnectionDocument,
        variables: { first: CARDGROUPS_PAGE_SIZE, search: "vocab" },
      },
      result: () => {
        queryCalled = true;
        return { data: { myCardgroupsConnection: makeConnection([CG_1]) } };
      },
    };

    renderClient([initialMock, searchMock], null, cache);

    // Wait for initial render
    expect(await screen.findByTestId("cardgroups-empty")).toBeInTheDocument();

    const searchInput = screen.getByRole("searchbox");
    await user.type(searchInput, "vocab");

    // Should not have fired yet — still within debounce window
    expect(queryCalled).toBe(false);

    // Advance past the 300ms debounce
    vi.advanceTimersByTime(300);
    vi.useRealTimers();

    await waitFor(() => {
      expect(queryCalled).toBe(true);
    });
  });

  // S5: fetches next page when sentinel intersects (forward pagination)
  it("fetches next page when sentinel intersects", async () => {
    const cache = new InMemoryCache();
    const page1Conn = makeConnection([CG_1, CG_2], true, 3);
    cache.writeQuery({
      query: MyCardgroupsConnectionDocument,
      variables: { first: CARDGROUPS_PAGE_SIZE, search: null },
      data: { myCardgroupsConnection: page1Conn },
    });

    // Initial query mock (may be served from cache, kept for completeness)
    const initialMock = {
      request: {
        query: MyCardgroupsConnectionDocument,
        variables: { first: CARDGROUPS_PAGE_SIZE, search: null },
      },
      result: { data: { myCardgroupsConnection: page1Conn } },
    };

    // fetchMore mock with after cursor
    const page2Mock = {
      request: {
        query: MyCardgroupsConnectionDocument,
        variables: {
          first: CARDGROUPS_PAGE_SIZE,
          after: CG_2.id,
          search: null,
        },
      },
      result: {
        data: {
          myCardgroupsConnection: makeConnection([CG_3], false, 3),
        },
      },
    };

    renderClient([initialMock, page2Mock], null, cache);

    // Wait for page 1 to load
    expect(await screen.findByText("Spanish Vocab")).toBeInTheDocument();
    expect(screen.getByText("Math Formulas")).toBeInTheDocument();

    // Simulate the sentinel entering the viewport
    fireIntersect();

    // Page 2 item should appear after fetchMore resolves
    await waitFor(() => {
      expect(screen.getByText("Biology Notes")).toBeInTheDocument();
    });
  });

  // S5b: useRef<boolean> in-flight guard swallows the second intersection call.
  // A leaked second fetchMore request would surface as an unmatched mock in afterEach.
  it("does not fire fetchMore twice when sentinel intersects in the same animation frame", async () => {
    const cache = new InMemoryCache();
    const page1Conn = makeConnection([CG_1, CG_2], true, 3);
    cache.writeQuery({
      query: MyCardgroupsConnectionDocument,
      variables: { first: CARDGROUPS_PAGE_SIZE, search: null },
      data: { myCardgroupsConnection: page1Conn },
    });

    const initialMock = {
      request: {
        query: MyCardgroupsConnectionDocument,
        variables: { first: CARDGROUPS_PAGE_SIZE, search: null },
      },
      result: { data: { myCardgroupsConnection: page1Conn } },
    };

    let nextPageCalls = 0;
    // Only ONE nextPageMock entry — a second fetchMore would be an unmatched
    // request and the LeakSpy in afterEach converts the console.warn into a
    // hard test failure.
    const nextPageMock = {
      request: {
        query: MyCardgroupsConnectionDocument,
        variables: {
          first: CARDGROUPS_PAGE_SIZE,
          after: CG_2.id,
          search: null,
        },
      },
      result: () => {
        nextPageCalls += 1;
        return { data: { myCardgroupsConnection: makeConnection([CG_3], false, 3) } };
      },
    };

    renderClient([initialMock, nextPageMock], null, cache);

    // Wait for page 1 to render.
    expect(await screen.findByText("Spanish Vocab")).toBeInTheDocument();

    // Fire the sentinel intersection **twice synchronously in the same tick**.
    // The useRef<boolean> in-flight guard must swallow the second call before it
    // reaches fetchMore. If it does not, the LeakSpy in afterEach will catch the
    // unmatched second request and fail the test.
    fireIntersect();
    fireIntersect();

    // Wait for the single page-2 fetch to complete.
    await waitFor(() => {
      expect(screen.getByText("Biology Notes")).toBeInTheDocument();
    });

    // The production fetchMore must have been called exactly once.
    expect(nextPageCalls).toBe(1);
  });

  // S6: fetchMore error halts the IO loop; Retry clears the error and retries.
  // Two mocked responses: first errors, second succeeds on retry.
  it("halts IO loop on fetchMore error and shows retry banner, succeeds after retry", async () => {
    // Forwarding spy: do NOT use mockImplementation(() => {}) — this outer spy
    // must forward calls so the file-wide leak spy still records MockedProvider leaks.
    const consoleWarnSpy = vi.spyOn(console, "warn");

    const cache = new InMemoryCache();
    const page1Conn = makeConnection([CG_1, CG_2], true, 3);
    cache.writeQuery({
      query: MyCardgroupsConnectionDocument,
      variables: { first: CARDGROUPS_PAGE_SIZE, search: null },
      data: { myCardgroupsConnection: page1Conn },
    });

    const initialMock = {
      request: {
        query: MyCardgroupsConnectionDocument,
        variables: { first: CARDGROUPS_PAGE_SIZE, search: null },
      },
      result: { data: { myCardgroupsConnection: page1Conn } },
    };

    const fetchMoreVars = {
      first: CARDGROUPS_PAGE_SIZE,
      after: CG_2.id,
      search: null,
    };

    // First attempt fails
    const errorMock = {
      request: {
        query: MyCardgroupsConnectionDocument,
        variables: fetchMoreVars,
      },
      result: {
        errors: [
          new GraphQLError("Could not load more", {
            extensions: { code: "INTERNAL" },
          }),
        ],
      },
    };
    // Retry succeeds
    const retryMock = {
      request: {
        query: MyCardgroupsConnectionDocument,
        variables: fetchMoreVars,
      },
      result: {
        data: {
          myCardgroupsConnection: makeConnection([CG_3], false, 3),
        },
      },
    };

    renderClient([initialMock, errorMock, retryMock], null, cache);

    // Wait for page 1 to load
    expect(await screen.findByText("Spanish Vocab")).toBeInTheDocument();

    // Trigger the first fetchMore — it will fail
    fireIntersect();

    // Error banner should appear
    await waitFor(() => {
      expect(screen.getByTestId("cardgroups-fetch-more-error")).toBeInTheDocument();
    });

    // err.message is omitted from the warn payload to avoid leaking user-authored content.
    const warnCall = consoleWarnSpy.mock.calls.find(
      (call) => call[0] === "[cardgroups] fetchMore failed",
    );
    expect(warnCall).toBeDefined();
    const payload = warnCall?.[1];
    expect(payload).toMatchObject({
      name: expect.any(String),
      endCursor: expect.any(String),
    });
    expect(payload).not.toHaveProperty("message");

    // The user clicks Retry to resume
    const retryBtn = screen.getByRole("button", { name: /retry/i });
    await userEvent.click(retryBtn);

    // Success: page 2 item appears
    await waitFor(() => {
      expect(screen.getByText("Biology Notes")).toBeInTheDocument();
    });

    // Error banner should be gone
    expect(screen.queryByTestId("cardgroups-fetch-more-error")).not.toBeInTheDocument();
  });
});

// ---------------------------------------------------------------------------
// S-delete: optimistic remove + scheduleDelete invocation
//
// handleDelete reads the active-query cache entry (queryVariables), filters the
// edge out, decrements totalCount, then delegates to scheduleDelete for the
// undo window and eventual commit. The rollback restores the same cache entry.
// ---------------------------------------------------------------------------

describe("<CardgroupsClient> delete — optimistic cache update and scheduleDelete", () => {
  it("optimistically removes the edge from the cache and decrements totalCount on delete", async () => {
    const user = userEvent.setup();
    vi.useFakeTimers({ shouldAdvanceTime: true });

    const cache = new InMemoryCache();
    const conn = makeConnection([CG_1, CG_2], false, 2);
    cache.writeQuery({
      query: MyCardgroupsConnectionDocument,
      variables: CARDGROUPS_DEFAULT_VARS,
      data: { myCardgroupsConnection: conn },
    });

    // The DeleteCardgroup mutation mock — consumed when the undo window elapses.
    const deleteMock = {
      request: {
        query: DeleteCardgroupDocument,
        variables: { id: CG_1.id },
      },
      result: { data: { deleteCardgroup: true } },
    };

    const initialMock = {
      request: {
        query: MyCardgroupsConnectionDocument,
        variables: CARDGROUPS_DEFAULT_VARS,
      },
      result: { data: { myCardgroupsConnection: conn } },
    };

    renderClient([initialMock, deleteMock], null, cache);

    // Wait for the list to render.
    expect(await screen.findByText("Spanish Vocab")).toBeInTheDocument();

    // Click the delete button for CG_1.
    const deleteBtn = screen.getByRole("button", {
      name: new RegExp(`delete cardgroup ${CG_1.name}`, "i"),
    });
    await user.click(deleteBtn);

    // Edge is removed optimistically: CG_1 is gone, CG_2 still visible.
    await waitFor(() => {
      expect(screen.queryByText("Spanish Vocab")).not.toBeInTheDocument();
    });
    expect(screen.getByText("Math Formulas")).toBeInTheDocument();

    // Verify the cache reflects the optimistic removal.
    const afterDelete = cache.readQuery({
      query: MyCardgroupsConnectionDocument,
      variables: CARDGROUPS_DEFAULT_VARS,
    });
    expect(afterDelete?.myCardgroupsConnection.edges).toHaveLength(1);
    expect(afterDelete?.myCardgroupsConnection.totalCount).toBe(1);

    // Advance past the 5-second undo window so the commit mock is consumed.
    vi.advanceTimersByTime(5100);
    vi.useRealTimers();

    await waitFor(() => {
      // After commit: the remaining edge is still CG_2 only.
      const afterCommit = cache.readQuery({
        query: MyCardgroupsConnectionDocument,
        variables: CARDGROUPS_DEFAULT_VARS,
      });
      expect(afterCommit?.myCardgroupsConnection.edges.map((e) => e.node.id)).not.toContain(
        CG_1.id,
      );
    });
  });

  it("calls scheduleDelete (shows toast) when delete is triggered", async () => {
    const user = userEvent.setup();

    const cache = new InMemoryCache();
    const conn = makeConnection([CG_1]);
    cache.writeQuery({
      query: MyCardgroupsConnectionDocument,
      variables: CARDGROUPS_DEFAULT_VARS,
      data: { myCardgroupsConnection: conn },
    });

    const initialMock = {
      request: {
        query: MyCardgroupsConnectionDocument,
        variables: CARDGROUPS_DEFAULT_VARS,
      },
      result: { data: { myCardgroupsConnection: conn } },
    };

    // The DeleteCardgroup mutation mock — keep available so the leak spy does not fail.
    const deleteMock = {
      request: {
        query: DeleteCardgroupDocument,
        variables: { id: CG_1.id },
      },
      result: { data: { deleteCardgroup: true } },
    };

    vi.useFakeTimers({ shouldAdvanceTime: true });

    renderClient([initialMock, deleteMock], null, cache);

    expect(await screen.findByText("Spanish Vocab")).toBeInTheDocument();

    const deleteBtn = screen.getByRole("button", {
      name: new RegExp(`delete cardgroup ${CG_1.name}`, "i"),
    });
    await user.click(deleteBtn);

    // scheduleDelete calls sonner toast — the mock captures the label.
    expect(lastToastLabel).toBe(`Cardgroup "${CG_1.name}" deleted`);
    // The Undo callback must be present.
    expect(lastUndoAction).toBeInstanceOf(Function);

    // Advance to consume the pending timer so no leak is recorded.
    vi.advanceTimersByTime(5100);
    vi.useRealTimers();
    await waitFor(() => {});
  });

  it("shows delete-error banner and restores row when commit fails with FORBIDDEN", async () => {
    const user = userEvent.setup();
    vi.useFakeTimers({ shouldAdvanceTime: true });

    const cache = new InMemoryCache();
    const conn = makeConnection([CG_1, CG_2], false, 2);
    cache.writeQuery({
      query: MyCardgroupsConnectionDocument,
      variables: CARDGROUPS_DEFAULT_VARS,
      data: { myCardgroupsConnection: conn },
    });

    const initialMock = {
      request: {
        query: MyCardgroupsConnectionDocument,
        variables: CARDGROUPS_DEFAULT_VARS,
      },
      result: { data: { myCardgroupsConnection: conn } },
    };

    const forbiddenMock = {
      request: {
        query: DeleteCardgroupDocument,
        variables: { id: CG_1.id },
      },
      result: {
        errors: [
          new GraphQLError("Forbidden", {
            extensions: { code: "FORBIDDEN" },
          }),
        ],
      },
    };

    renderClient([initialMock, forbiddenMock], null, cache);

    expect(await screen.findByText("Spanish Vocab")).toBeInTheDocument();

    const deleteBtn = screen.getByRole("button", {
      name: new RegExp(`delete cardgroup ${CG_1.name}`, "i"),
    });
    await user.click(deleteBtn);

    // Row is removed optimistically
    await waitFor(() => {
      expect(screen.queryByText("Spanish Vocab")).not.toBeInTheDocument();
    });

    // Advance past the 5-second undo window to trigger commitDelete
    act(() => vi.advanceTimersByTime(5100));
    vi.useRealTimers();

    // Row reappears after rollback triggered by FORBIDDEN failure
    await waitFor(() => {
      expect(screen.getByText("Spanish Vocab")).toBeInTheDocument();
    });

    // Error banner is visible
    expect(screen.getByTestId("cardgroups-delete-error")).toBeInTheDocument();
  });

  it("restores the edge in the cache when Undo is invoked within the window", async () => {
    const user = userEvent.setup();
    vi.useFakeTimers({ shouldAdvanceTime: true });

    const cache = new InMemoryCache();
    const conn = makeConnection([CG_1, CG_2], false, 2);
    cache.writeQuery({
      query: MyCardgroupsConnectionDocument,
      variables: CARDGROUPS_DEFAULT_VARS,
      data: { myCardgroupsConnection: conn },
    });

    const initialMock = {
      request: {
        query: MyCardgroupsConnectionDocument,
        variables: CARDGROUPS_DEFAULT_VARS,
      },
      result: { data: { myCardgroupsConnection: conn } },
    };

    // No DeleteCardgroup mock: Undo cancels the commit so the mutation must NOT fire.
    renderClient([initialMock], null, cache);

    expect(await screen.findByText("Spanish Vocab")).toBeInTheDocument();

    const deleteBtn = screen.getByRole("button", {
      name: new RegExp(`delete cardgroup ${CG_1.name}`, "i"),
    });
    await user.click(deleteBtn);

    // Row is gone optimistically.
    await waitFor(() => {
      expect(screen.queryByText("Spanish Vocab")).not.toBeInTheDocument();
    });

    // Invoke Undo within the 5-second window.
    expect(lastUndoAction).toBeInstanceOf(Function);
    lastUndoAction?.();

    // The optimistic rollback restores the snapshot in the cache.
    await waitFor(() => {
      const afterUndo = cache.readQuery({
        query: MyCardgroupsConnectionDocument,
        variables: CARDGROUPS_DEFAULT_VARS,
      });
      expect(afterUndo?.myCardgroupsConnection.edges).toHaveLength(2);
      expect(afterUndo?.myCardgroupsConnection.totalCount).toBe(2);
    });

    vi.useRealTimers();
  });

  // S-delete-search: handleDelete uses active search variables, not default vars.
  // A delete while a search is active must read/write the search-keyed cache entry.
  it("optimistically removes the edge from the search-keyed cache entry when a search is active", async () => {
    const user = userEvent.setup({ delay: null });
    vi.useFakeTimers({ shouldAdvanceTime: true });

    const cache = new InMemoryCache();
    const searchVars = { ...CARDGROUPS_DEFAULT_VARS, search: "Spanish" };

    // Seed the search-keyed cache entry (mirrors what queryVariables resolves to
    // after the 300ms debounce when "Spanish" is the active search).
    const searchConn = makeConnection([CG_1], false, 1);
    cache.writeQuery({
      query: MyCardgroupsConnectionDocument,
      variables: searchVars,
      data: { myCardgroupsConnection: searchConn },
    });

    // Also seed default vars so the SSR path does not warn.
    const defaultConn = makeConnection([CG_1, CG_2], false, 2);
    cache.writeQuery({
      query: MyCardgroupsConnectionDocument,
      variables: CARDGROUPS_DEFAULT_VARS,
      data: { myCardgroupsConnection: defaultConn },
    });

    const initialMock = {
      request: { query: MyCardgroupsConnectionDocument, variables: CARDGROUPS_DEFAULT_VARS },
      result: { data: { myCardgroupsConnection: defaultConn } },
    };
    const searchMock = {
      request: { query: MyCardgroupsConnectionDocument, variables: searchVars },
      result: { data: { myCardgroupsConnection: searchConn } },
    };
    const deleteMock = {
      request: { query: DeleteCardgroupDocument, variables: { id: CG_1.id } },
      result: { data: { deleteCardgroup: true } },
    };

    renderClient([initialMock, searchMock, deleteMock], null, cache);

    // Advance through the 300ms debounce after typing the search term.
    const searchInput = screen.getByRole("searchbox");
    await user.type(searchInput, "Spanish");
    vi.advanceTimersByTime(300);
    vi.useRealTimers();

    // Wait for the search result to render.
    await waitFor(() => {
      expect(screen.getByText("Spanish Vocab")).toBeInTheDocument();
    });

    // Delete CG_1 while the search is active.
    const deleteBtn = screen.getByRole("button", {
      name: new RegExp(`delete cardgroup ${CG_1.name}`, "i"),
    });
    await user.click(deleteBtn);

    // Optimistic removal: the edge is gone from the UI immediately.
    await waitFor(() => {
      expect(screen.queryByText("Spanish Vocab")).not.toBeInTheDocument();
    });

    // The search-keyed cache entry must be updated, not the default-vars entry.
    const searchAfter = cache.readQuery({
      query: MyCardgroupsConnectionDocument,
      variables: searchVars,
    });
    expect(searchAfter?.myCardgroupsConnection.edges).toHaveLength(0);
    expect(searchAfter?.myCardgroupsConnection.totalCount).toBe(0);

    // Default-vars entry must be untouched (still has both CG_1 and CG_2).
    const defaultAfter = cache.readQuery({
      query: MyCardgroupsConnectionDocument,
      variables: CARDGROUPS_DEFAULT_VARS,
    });
    expect(defaultAfter?.myCardgroupsConnection.edges).toHaveLength(2);

    // Advance past the undo window to consume the delete mock.
    vi.useFakeTimers({ shouldAdvanceTime: true });
    vi.advanceTimersByTime(5100);
    vi.useRealTimers();
    await waitFor(() => {});
  });
});

// ---------------------------------------------------------------------------
// S7: connection cache update — readQuery + writeQuery (not cache.modify)
// ---------------------------------------------------------------------------
//
// cache.modify skips non-existent fields on a cold cache; readQuery + writeQuery
// handles both the warm-cache and cold-cache paths correctly.

describe("<CardgroupsClient> connection cache update (readQuery + writeQuery)", () => {
  it("prepends new cardgroup into MyCardgroupsConnection cache using readQuery + writeQuery", () => {
    const cache = new InMemoryCache();

    // CARDGROUPS_DEFAULT_VARS keeps the cache key identical to the SSR seed and
    // the client useQuery — any mismatch produces a silent cache miss.
    cache.writeQuery({
      query: MyCardgroupsConnectionDocument,
      variables: CARDGROUPS_DEFAULT_VARS,
      data: { myCardgroupsConnection: makeConnection([CG_1, CG_2]) },
    });

    const NEW_CG = {
      __typename: "Cardgroup" as const,
      id: "cg-new",
      name: "New Cardgroup",
      updatedAt: "2026-05-01T00:00:00.000Z",
    };

    // Warm-cache path: readQuery returns data, writeQuery prepends
    const existingConnection = cache.readQuery({
      query: MyCardgroupsConnectionDocument,
      variables: CARDGROUPS_DEFAULT_VARS,
    });

    // Verify readQuery returned non-null (warm cache)
    expect(existingConnection).not.toBeNull();

    if (existingConnection) {
      cache.writeQuery({
        query: MyCardgroupsConnectionDocument,
        variables: CARDGROUPS_DEFAULT_VARS,
        data: {
          myCardgroupsConnection: {
            ...existingConnection.myCardgroupsConnection,
            edges: [
              { __typename: "CardgroupEdge" as const, cursor: NEW_CG.id, node: NEW_CG },
              ...existingConnection.myCardgroupsConnection.edges,
            ],
            totalCount: existingConnection.myCardgroupsConnection.totalCount + 1,
          },
        },
      });
    }

    const result = cache.readQuery({
      query: MyCardgroupsConnectionDocument,
      variables: CARDGROUPS_DEFAULT_VARS,
    });

    expect(result?.myCardgroupsConnection.edges).toHaveLength(3);
    expect(result?.myCardgroupsConnection.edges[0]).toMatchObject({
      cursor: NEW_CG.id,
      node: { id: NEW_CG.id, name: NEW_CG.name },
    });
    expect(result?.myCardgroupsConnection.totalCount).toBe(3);
  });

  it("builds minimal connection on cold cache (readQuery returns null)", () => {
    const cache = new InMemoryCache();
    // Do NOT seed — cold-cache scenario

    const NEW_CG = {
      __typename: "Cardgroup" as const,
      id: "cg-new",
      name: "New Cardgroup",
      updatedAt: "2026-05-01T00:00:00.000Z",
    };

    const existingConnection = cache.readQuery({
      query: MyCardgroupsConnectionDocument,
      variables: CARDGROUPS_DEFAULT_VARS,
    });

    // Cold cache: readQuery must return null
    expect(existingConnection).toBeNull();

    // Build a minimal connection (as new-cardgroup-client.tsx does in the else branch)
    cache.writeQuery({
      query: MyCardgroupsConnectionDocument,
      variables: CARDGROUPS_DEFAULT_VARS,
      data: {
        myCardgroupsConnection: {
          __typename: "CardgroupConnection" as const,
          edges: [{ __typename: "CardgroupEdge" as const, cursor: NEW_CG.id, node: NEW_CG }],
          pageInfo: {
            __typename: "PageInfo" as const,
            hasNextPage: false,
            hasPreviousPage: false,
            startCursor: NEW_CG.id,
            endCursor: NEW_CG.id,
          },
          totalCount: 1,
        },
      },
    });

    const result = cache.readQuery({
      query: MyCardgroupsConnectionDocument,
      variables: CARDGROUPS_DEFAULT_VARS,
    });

    expect(result?.myCardgroupsConnection.edges).toHaveLength(1);
    expect(result?.myCardgroupsConnection.edges[0]).toMatchObject({
      cursor: NEW_CG.id,
      node: { id: NEW_CG.id, name: NEW_CG.name },
    });
    expect(result?.myCardgroupsConnection.pageInfo.hasNextPage).toBe(false);
    expect(result?.myCardgroupsConnection.totalCount).toBe(1);
  });
});

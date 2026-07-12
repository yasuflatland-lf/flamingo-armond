// @vitest-environment happy-dom
import { screen } from "@testing-library/react";
import { createRef } from "react";
import { describe, expect, it, vi } from "vitest";
import type { UseHeaderTakeoverSearchResult } from "@/hooks/use-header-takeover-search";
import { renderWithIntl } from "@/test/render-with-intl";
import { PaginatedPublicListScreen } from "./paginated-public-list-screen";

// A minimal useHeaderTakeoverSearch stand-in — the shell only reads searchOpen /
// input and the three callbacks off it, so the fake covers the full shape.
function makeSearch(
  overrides: Partial<UseHeaderTakeoverSearchResult> = {},
): UseHeaderTakeoverSearchResult {
  return {
    input: "",
    query: null,
    setInput: vi.fn(),
    clear: vi.fn(),
    searchOpen: false,
    closeSearch: vi.fn(),
    ...overrides,
  };
}

type ShellProps = Parameters<typeof PaginatedPublicListScreen>[0];

function renderShell(overrides: Partial<ShellProps> = {}) {
  const props: ShellProps = {
    search: {
      search: makeSearch(overrides.search?.search),
      placeholder: "Search decks",
      ariaLabel: "Search decks",
    },
    desktopSearch: <div data-testid="demo-desktop-search">desktop search</div>,
    title: "Demo title",
    count: 7,
    countLabel: "7 total",
    initialLoading: false,
    loadingLabel: "Loading…",
    isEmpty: false,
    hasSearch: false,
    emptyState: <p data-testid="demo-empty">No items yet</p>,
    emptySearchState: <p data-testid="demo-empty-search">No matches</p>,
    footer: {
      sentinelRef: createRef<HTMLDivElement>(),
      fetchMoreError: null,
      onRetry: vi.fn(),
      fetchingMore: false,
      hasNextPage: true,
      retryLabel: "Retry",
      loadingMoreLabel: "Loading more…",
    },
    testIdPrefix: "demo",
    children: <ul data-testid="demo-list">list</ul>,
    ...overrides,
  };
  renderWithIntl(<PaginatedPublicListScreen {...props} />);
  return props;
}

describe("<PaginatedPublicListScreen>", () => {
  it("renders the header title, count pill, and desktop search toolbar", () => {
    renderShell();

    expect(screen.getByText("Demo title")).toBeInTheDocument();
    expect(screen.getByText("7 total")).toBeInTheDocument();
    expect(screen.getByTestId("demo-desktop-search")).toBeInTheDocument();
  });

  it("shows the first-load line and suppresses the empty branches while loading", () => {
    renderShell({ initialLoading: true, isEmpty: true });

    const loading = screen.getByTestId("demo-loading");
    expect(loading).toBeInTheDocument();
    expect(loading).toHaveTextContent("Loading…");
    expect(screen.queryByTestId("demo-empty")).not.toBeInTheDocument();
    expect(screen.queryByTestId("demo-empty-search")).not.toBeInTheDocument();
  });

  it("shows the empty state when isEmpty and no active search", () => {
    renderShell({ isEmpty: true, hasSearch: false });

    expect(screen.getByTestId("demo-empty")).toBeInTheDocument();
    expect(screen.queryByTestId("demo-empty-search")).not.toBeInTheDocument();
    expect(screen.queryByTestId("demo-loading")).not.toBeInTheDocument();
  });

  it("shows the no-match state when isEmpty and a search is active", () => {
    renderShell({ isEmpty: true, hasSearch: true });

    expect(screen.getByTestId("demo-empty-search")).toBeInTheDocument();
    expect(screen.queryByTestId("demo-empty")).not.toBeInTheDocument();
    expect(screen.queryByTestId("demo-loading")).not.toBeInTheDocument();
  });

  it("renders children (the list) and no trio branches when not empty", () => {
    renderShell();

    expect(screen.getByTestId("demo-list")).toBeInTheDocument();
    expect(screen.queryByTestId("demo-empty")).not.toBeInTheDocument();
    expect(screen.queryByTestId("demo-empty-search")).not.toBeInTheDocument();
    expect(screen.queryByTestId("demo-loading")).not.toBeInTheDocument();
  });

  it("wires the footer with the prefixed sentinel and attaches the ref", () => {
    const sentinelRef = createRef<HTMLDivElement>();
    renderShell({
      footer: {
        sentinelRef,
        fetchMoreError: "boom",
        onRetry: vi.fn(),
        fetchingMore: false,
        hasNextPage: true,
        retryLabel: "Retry",
        loadingMoreLabel: "Loading more…",
      },
    });

    const sentinel = screen.getByTestId("demo-sentinel");
    expect(sentinel).toBeInTheDocument();
    expect(sentinelRef.current).toBe(sentinel);
    // fetchMoreError drives the prefixed error banner.
    expect(screen.getByTestId("demo-fetch-more-error")).toHaveTextContent("boom");
  });

  it("keeps the mobile takeover bar closed while search.searchOpen is false", () => {
    renderShell({
      search: {
        search: makeSearch({ searchOpen: false }),
        placeholder: "Search decks",
        ariaLabel: "Search decks",
      },
    });
    expect(screen.queryByTestId("search-takeover")).not.toBeInTheDocument();
  });

  it("renders the mobile takeover bar when search.searchOpen is true", () => {
    renderShell({
      search: {
        search: makeSearch({ searchOpen: true }),
        placeholder: "Search decks",
        ariaLabel: "Search decks",
      },
    });

    expect(screen.getByTestId("search-takeover")).toBeInTheDocument();
    expect(screen.getByTestId("search-takeover-input")).toBeInTheDocument();
  });
});

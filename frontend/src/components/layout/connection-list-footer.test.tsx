// @vitest-environment happy-dom
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { createRef } from "react";
import { describe, expect, it, vi } from "vitest";
import { ConnectionListFooter } from "./connection-list-footer";

function renderFooter(overrides: Partial<Parameters<typeof ConnectionListFooter>[0]> = {}) {
  const props = {
    sentinelRef: createRef<HTMLDivElement>(),
    fetchMoreError: null as string | null,
    onRetry: vi.fn(),
    fetchingMore: false,
    hasNextPage: true,
    retryLabel: "Retry",
    loadingMoreLabel: "Loading more…",
    testIdPrefix: "demo",
    ...overrides,
  };
  return { props, ...render(<ConnectionListFooter {...props} />) };
}

describe("<ConnectionListFooter>", () => {
  it("renders the sentinel with the prefixed testid and the ref attached", () => {
    const sentinelRef = createRef<HTMLDivElement>();
    renderFooter({ sentinelRef });

    const sentinel = screen.getByTestId("demo-sentinel");
    expect(sentinel).toBeInTheDocument();
    expect(sentinel).toHaveAttribute("aria-hidden", "true");
    // The observer attaches via the ref, so the ref must resolve to the node.
    expect(sentinelRef.current).toBe(sentinel);
  });

  it("reproduces each screen's testid namespace from testIdPrefix", () => {
    renderFooter({
      testIdPrefix: "cardgroups",
      fetchMoreError: "boom",
      fetchingMore: false,
    });
    expect(screen.getByTestId("cardgroups-sentinel")).toBeInTheDocument();
    expect(screen.getByTestId("cardgroups-fetch-more-error")).toBeInTheDocument();
  });

  it("hides the error banner and loading line in the healthy idle state", () => {
    renderFooter({ fetchMoreError: null, fetchingMore: false });
    expect(screen.queryByTestId("demo-fetch-more-error")).not.toBeInTheDocument();
    expect(screen.queryByTestId("demo-loading-more")).not.toBeInTheDocument();
  });

  it("renders the error banner with the localized message and a canonical outline Retry button", () => {
    renderFooter({ fetchMoreError: "Could not load more" });

    const banner = screen.getByTestId("demo-fetch-more-error");
    expect(banner).toHaveTextContent("Could not load more");
    const retry = screen.getByRole("button", { name: "Retry" });
    // The unified Retry uses the shadcn outline Button variant.
    expect(retry).toHaveAttribute("type", "button");
    expect(retry.className).toContain("border");
  });

  it("calls onRetry when the Retry button is clicked", async () => {
    const user = userEvent.setup();
    const { props } = renderFooter({ fetchMoreError: "Could not load more" });

    await user.click(screen.getByRole("button", { name: "Retry" }));
    expect(props.onRetry).toHaveBeenCalledTimes(1);
  });

  it("renders the loading-more line only while fetching with more pages and no error", () => {
    renderFooter({ fetchMoreError: null, fetchingMore: true, hasNextPage: true });
    expect(screen.getByTestId("demo-loading-more")).toHaveTextContent("Loading more…");
  });

  it("suppresses the loading-more line when an error is showing", () => {
    renderFooter({ fetchMoreError: "boom", fetchingMore: true, hasNextPage: true });
    expect(screen.queryByTestId("demo-loading-more")).not.toBeInTheDocument();
    expect(screen.getByTestId("demo-fetch-more-error")).toBeInTheDocument();
  });

  it("suppresses the loading-more line when no further pages remain", () => {
    renderFooter({ fetchMoreError: null, fetchingMore: true, hasNextPage: false });
    expect(screen.queryByTestId("demo-loading-more")).not.toBeInTheDocument();
  });
});

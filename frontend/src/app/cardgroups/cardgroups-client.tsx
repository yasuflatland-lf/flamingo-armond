"use client";

import { NetworkStatus } from "@apollo/client";
import { useApolloClient, useQuery } from "@apollo/client/react";
import Link from "next/link";
import { useCallback, useEffect, useRef, useState } from "react";
import { CardgroupListItem } from "@/components/cardgroups/cardgroup-list-item";
import { CardgroupsToolbar } from "@/components/cardgroups/cardgroups-toolbar";
import { ListingPageShell } from "@/components/layout/listing-page-shell";
import { Button } from "@/components/ui/button";
import {
  MyCardgroupsConnectionDocument,
  type MyCardgroupsConnectionQuery,
} from "@/generated/graphql";
import { getBackendErrorBanner } from "@/lib/apollo/errors";
import { CARDGROUPS_DEFAULT_VARS } from "./queries";

type Connection = MyCardgroupsConnectionQuery["myCardgroupsConnection"];

interface CardgroupsClientProps {
  initialConnection: Connection | null;
}

/**
 * Client component for the /cardgroups listing page.
 *
 * Wires:
 *  - Debounced search (300ms; searchInput → searchQuery), passed as the
 *    `search` variable on MyCardgroupsConnectionQuery.
 *  - Infinite scroll via IntersectionObserver, with an in-flight guard via
 *    useRef<boolean> (per .claude/rules/pagination.md).
 *  - fetchMoreError halt gate — observer short-circuits while an error banner
 *    is showing; user must click Retry to resume.
 *  - SSR seed: writes initialConnection into the cache once at mount via
 *    cache.writeQuery so useQuery (cache-first) renders immediately without
 *    a network round-trip.
 */
export default function CardgroupsClient({ initialConnection }: CardgroupsClientProps) {
  const [searchInput, setSearchInput] = useState("");
  const [searchQuery, setSearchQuery] = useState<string | null>(null);
  const [fetchMoreError, setFetchMoreError] = useState<string | null>(null);

  // Debounce: update searchQuery 300ms after the last keystroke.
  useEffect(() => {
    const timer = setTimeout(() => {
      setSearchQuery(searchInput.trim() || null);
    }, 300);
    return () => clearTimeout(timer);
  }, [searchInput]);

  // When the active search query changes, any in-flight fetchMore from the
  // previous search holds a stale cursor. Reset the IO guard and error state
  // immediately so the new query starts from a clean slate.
  // See .claude/rules/pagination.md § "IntersectionObserver in-flight guard".
  // biome-ignore lint/correctness/useExhaustiveDependencies: searchQuery is an intentional trigger dependency; it is not referenced in the body because the effect resets derived IO state, not searchQuery itself.
  useEffect(() => {
    fetchingRef.current = false;
    setFetchMoreError(null);
  }, [searchQuery]);

  // Seed the cache once with the SSR initialConnection so the first useQuery
  // pass (cache-first) renders immediately. Only seed when searchQuery is the
  // unfiltered null state, since that is the variable shape the SSR page used.
  // CARDGROUPS_DEFAULT_VARS keeps the cache key identical to the SSR seed and
  // the client useQuery — any mismatch silently splits the cache.
  // useRef<boolean> ensures Strict Mode's double-mount does not write twice.
  const apollo = useApolloClient();
  const seededRef = useRef(false);
  useEffect(() => {
    if (seededRef.current) return;
    if (initialConnection == null) return;
    seededRef.current = true;
    apollo.writeQuery({
      query: MyCardgroupsConnectionDocument,
      variables: CARDGROUPS_DEFAULT_VARS,
      data: { myCardgroupsConnection: initialConnection },
    });
  }, [apollo, initialConnection]);

  // When searchQuery is null we use CARDGROUPS_DEFAULT_VARS verbatim so the
  // cache key matches the SSR seed exactly. For non-null searches we spread and
  // override `search`, keeping `first` in sync with the default.
  const queryVariables =
    searchQuery === null
      ? CARDGROUPS_DEFAULT_VARS
      : { ...CARDGROUPS_DEFAULT_VARS, search: searchQuery };

  const { data, fetchMore, loading, networkStatus } = useQuery(MyCardgroupsConnectionDocument, {
    variables: queryVariables,
    fetchPolicy: "cache-first",
    notifyOnNetworkStatusChange: true,
  });

  const connection = data?.myCardgroupsConnection;
  const edges = connection?.edges ?? [];
  const hasNextPage = connection?.pageInfo.hasNextPage ?? false;
  const endCursor = connection?.pageInfo.endCursor ?? null;

  const sentinelRef = useRef<HTMLDivElement | null>(null);
  // pagination.md: in-flight guard MUST be useRef<boolean>, not useState.
  const fetchingRef = useRef(false);

  const requestNextPage = useCallback(() => {
    if (fetchingRef.current) return;
    if (!hasNextPage) return;

    fetchingRef.current = true;
    fetchMore({
      variables: {
        ...CARDGROUPS_DEFAULT_VARS,
        after: endCursor,
        search: searchQuery,
      },
      updateQuery: (prev, { fetchMoreResult }) => {
        if (!fetchMoreResult) return prev;
        return {
          myCardgroupsConnection: {
            ...fetchMoreResult.myCardgroupsConnection,
            edges: [
              ...prev.myCardgroupsConnection.edges,
              ...fetchMoreResult.myCardgroupsConnection.edges,
            ],
          },
        };
      },
    })
      .then(() => {
        // Clear any previous fetchMore error on success so the observer can resume.
        setFetchMoreError(null);
      })
      .catch((err) => {
        const banner =
          getBackendErrorBanner(err) ?? "Could not load more cardgroups. Please try again.";
        // Structured warn for operator triage: name + request context only.
        // err.message is omitted — backend messages may carry user-authored content.
        // See .claude/rules/frontend-typescript-conventions.md § "expect.objectContaining".
        console.warn("[cardgroups] fetchMore failed", {
          name: err instanceof Error ? err.name : "unknown",
          searchQuery,
          endCursor: data?.myCardgroupsConnection.pageInfo.endCursor ?? null,
        });
        setFetchMoreError(banner);
      })
      .finally(() => {
        fetchingRef.current = false;
      });
  }, [fetchMore, endCursor, hasNextPage, searchQuery, data]);

  useEffect(() => {
    if (!hasNextPage) return;
    // Halt the observer loop while a previous fetch failed; user must click Retry to resume.
    if (fetchMoreError != null) return;
    const node = sentinelRef.current;
    if (!node) return;

    const observer = new IntersectionObserver((entries) => {
      const entry = entries[0];
      if (!entry?.isIntersecting) return;
      if (fetchingRef.current) return;
      if (!hasNextPage) return;
      requestNextPage();
    });

    observer.observe(node);
    return () => observer.disconnect();
  }, [hasNextPage, fetchMoreError, requestNextPage]);

  const fetchingMore = networkStatus === NetworkStatus.fetchMore || (loading && edges.length > 0);
  const initialLoading = loading && edges.length === 0 && networkStatus !== NetworkStatus.fetchMore;

  const hasSearch = searchQuery !== null && searchQuery !== "";

  return (
    <ListingPageShell
      title="My cardgroups"
      description="Browse and manage the cardgroups you have created."
      primaryActions={
        <Button asChild variant="brand">
          <Link href="/cardgroups/new">+ New cardgroup</Link>
        </Button>
      }
      toolbar={<CardgroupsToolbar searchInput={searchInput} onSearchInputChange={setSearchInput} />}
    >
      {initialLoading && (
        <p className="text-sm text-muted-foreground" data-testid="cardgroups-loading">
          Loading...
        </p>
      )}

      {!initialLoading && edges.length === 0 && !hasSearch && (
        <p className="text-sm text-muted-foreground" data-testid="cardgroups-empty">
          No cardgroups yet
        </p>
      )}

      {!initialLoading && edges.length === 0 && hasSearch && (
        <p className="text-sm text-muted-foreground" data-testid="cardgroups-empty-search">
          No cardgroups match "{searchQuery}"
        </p>
      )}

      {edges.length > 0 && (
        <ul className="space-y-3" data-testid="cardgroups-list">
          {edges.map((edge) => (
            <CardgroupListItem
              key={edge.node.id}
              id={edge.node.id}
              name={edge.node.name}
              updatedAt={edge.node.updatedAt as string}
            />
          ))}
        </ul>
      )}

      <div ref={sentinelRef} aria-hidden="true" data-testid="cardgroups-sentinel" />

      {fetchMoreError && (
        <div
          className="mt-3 flex flex-col items-center gap-2 rounded-md bg-destructive/10 p-3 text-sm text-destructive"
          role="alert"
          data-testid="cardgroups-fetch-more-error"
        >
          <span>{fetchMoreError}</span>
          <Button
            type="button"
            variant="outline"
            size="sm"
            onClick={() => {
              setFetchMoreError(null);
              requestNextPage();
            }}
          >
            Retry
          </Button>
        </div>
      )}

      {!fetchMoreError && fetchingMore && hasNextPage && (
        <p
          className="mt-3 text-center text-xs text-muted-foreground"
          data-testid="cardgroups-loading-more"
        >
          Loading more cardgroups...
        </p>
      )}
    </ListingPageShell>
  );
}

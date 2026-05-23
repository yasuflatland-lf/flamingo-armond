"use client";

import { NetworkStatus } from "@apollo/client";
import { useApolloClient, useMutation, useQuery } from "@apollo/client/react";
import { Plus } from "lucide-react";
import Link from "next/link";
import { useCallback, useEffect, useEffectEvent, useRef, useState } from "react";
import { CardgroupListItem } from "@/components/cardgroups/cardgroup-list-item";
import { CardgroupsToolbar } from "@/components/cardgroups/cardgroups-toolbar";
import { ListingPageShell } from "@/components/layout/listing-page-shell";
import { Button } from "@/components/ui/button";
import {
  MyCardgroupsConnectionDocument,
  type MyCardgroupsConnectionQuery,
} from "@/generated/graphql";
import { getBackendErrorBanner } from "@/lib/apollo/errors";
import type { FetchNextPageInput } from "@/lib/pagination/types";
import { useUndoDelete } from "@/lib/undo-delete";
import { CARDGROUPS_DEFAULT_VARS, DeleteCardgroupMutation } from "./queries";

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
 *    useRef<boolean> (per docs/pagination/intersection-observer-in-flight-guard.md).
 *  - fetchMoreError halt gate — observer short-circuits while an error banner
 *    is showing; user must click Retry to resume.
 *  - SSR seed: writes initialConnection into the cache once at mount via
 *    cache.writeQuery so useQuery (cache-first) renders immediately without
 *    a network round-trip.
 */
export default function CardgroupsClient({ initialConnection }: CardgroupsClientProps) {
  const apollo = useApolloClient();
  const [searchInput, setSearchInput] = useState("");
  const [searchQuery, setSearchQuery] = useState<string | null>(null);
  const [fetchMoreError, setFetchMoreError] = useState<string | null>(null);
  const [deleteCommitError, setDeleteCommitError] = useState<string | null>(null);

  const { scheduleDelete } = useUndoDelete();
  const [deleteCardgroup] = useMutation(DeleteCardgroupMutation);

  const sentinelRef = useRef<HTMLDivElement | null>(null);
  // In-flight guard MUST be useRef<boolean>, not useState — see
  // docs/pagination/intersection-observer-in-flight-guard.md.
  const fetchingRef = useRef(false);
  // Strict Mode double-mount safety: only write the SSR seed into the cache once.
  const seededRef = useRef(false);

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
  // See docs/pagination/intersection-observer-in-flight-guard.md.
  // biome-ignore lint/correctness/useExhaustiveDependencies: searchQuery is an intentional trigger dependency; it is not referenced in the body because the effect resets derived IO state, not searchQuery itself.
  useEffect(() => {
    fetchingRef.current = false;
    setFetchMoreError(null);
  }, [searchQuery]);

  // Seed the cache synchronously during render (before useQuery runs) with the
  // SSR initialConnection so the first useQuery pass (cache-first) finds the
  // data already in the cache and renders without a network round-trip. Doing
  // this in a useEffect would create a window between first paint and the
  // post-render write where useQuery sees an empty cache.
  // CARDGROUPS_DEFAULT_VARS keeps the cache key identical to the SSR seed and
  // the client useQuery — any mismatch silently splits the cache.
  // The seededRef guard is synchronous, so it survives Strict Mode's
  // double-invoke without producing a second write.
  if (!seededRef.current && initialConnection === null) {
    console.warn(
      "[cardgroups-client] initialConnection is null — SSR seed skipped; useQuery will fetch fresh",
    );
  }
  if (!seededRef.current && initialConnection != null) {
    seededRef.current = true;
    apollo.writeQuery({
      query: MyCardgroupsConnectionDocument,
      variables: CARDGROUPS_DEFAULT_VARS,
      data: { myCardgroupsConnection: initialConnection },
    });
  }

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

  function handleDelete(id: string, name: string) {
    const snapshot = apollo.cache.readQuery({
      query: MyCardgroupsConnectionDocument,
      variables: CARDGROUPS_DEFAULT_VARS,
    });
    if (!snapshot) {
      console.warn("[cardgroups] handleDelete: cache miss on snapshot read", { id });
      setDeleteCommitError("Could not delete cardgroup. Please reload and try again.");
      return;
    }

    apollo.cache.writeQuery({
      query: MyCardgroupsConnectionDocument,
      variables: CARDGROUPS_DEFAULT_VARS,
      data: {
        myCardgroupsConnection: {
          ...snapshot.myCardgroupsConnection,
          edges: snapshot.myCardgroupsConnection.edges.filter((e) => e.node.id !== id),
          totalCount: Math.max(0, snapshot.myCardgroupsConnection.totalCount - 1),
        },
      },
    });

    scheduleDelete({
      id,
      label: `Cardgroup "${name}" deleted`,
      optimisticRollback: () => {
        apollo.cache.writeQuery({
          query: MyCardgroupsConnectionDocument,
          variables: CARDGROUPS_DEFAULT_VARS,
          data: snapshot,
        });
      },
      commitDelete: async () => {
        await deleteCardgroup({ variables: { id } });
        apollo.cache.evict({ id: apollo.cache.identify({ __typename: "Cardgroup", id }) });
        apollo.cache.gc();
      },
      onCommitFailed: (err) => {
        setDeleteCommitError(
          getBackendErrorBanner(err) ?? "Could not delete cardgroup. Please try again.",
        );
      },
    });
  }

  const fetchNextPage = useCallback(
    ({ hasNextPage, endCursor, searchQuery }: FetchNextPageInput) => {
      if (fetchingRef.current || !hasNextPage) return;

      fetchingRef.current = true;
      fetchMore({
        variables: { ...CARDGROUPS_DEFAULT_VARS, after: endCursor, search: searchQuery },
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
          // Structured warn for operator triage: name + request context only.
          // err.message is omitted — backend messages may carry user-authored content.
          // See docs/frontend/typescript-conventions.md § "expect.objectContaining".
          console.warn("[cardgroups] fetchMore failed", {
            name: err instanceof Error ? err.name : "unknown",
            searchQuery,
            endCursor,
          });
          setFetchMoreError(
            getBackendErrorBanner(err) ?? "Could not load more cardgroups. Please try again.",
          );
        })
        .finally(() => {
          fetchingRef.current = false;
        });
    },
    [fetchMore],
  );

  const requestNextPageFromObserver = useEffectEvent(() => {
    fetchNextPage({ hasNextPage, endCursor, searchQuery });
  });

  useEffect(() => {
    // Halt the observer loop while a previous fetch failed; user must click Retry to resume.
    if (!hasNextPage || fetchMoreError != null) return;
    const node = sentinelRef.current;
    if (!node) return;

    const observer = new IntersectionObserver((entries) => {
      if (!entries[0]?.isIntersecting || fetchingRef.current) return;
      requestNextPageFromObserver();
    });

    observer.observe(node);
    return () => observer.disconnect();
  }, [hasNextPage, fetchMoreError]);

  const fetchingMore = networkStatus === NetworkStatus.fetchMore || (loading && edges.length > 0);
  const initialLoading = loading && edges.length === 0 && networkStatus !== NetworkStatus.fetchMore;
  const hasSearch = searchQuery !== null && searchQuery !== "";

  return (
    <ListingPageShell
      title="My cardgroups"
      description="Browse and manage the cardgroups you have created."
      primaryActions={
        <Button asChild variant="brand" className="hidden md:inline-flex">
          <Link href="/cardgroups/new">
            <span>New cardgroup</span>
            <Plus aria-hidden="true" />
          </Link>
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
              onDelete={handleDelete}
            />
          ))}
        </ul>
      )}

      {deleteCommitError && (
        <div
          className="mt-3 rounded-md bg-destructive/10 p-3 text-sm text-destructive"
          role="alert"
          data-testid="cardgroups-delete-error"
        >
          {deleteCommitError}
        </div>
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
              fetchNextPage({ hasNextPage, endCursor, searchQuery });
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

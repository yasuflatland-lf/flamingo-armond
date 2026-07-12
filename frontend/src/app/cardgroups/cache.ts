import type { ApolloCache } from "@apollo/client";
import type {
  MyCardgroupsConnectionQuery,
  MyCardgroupsConnectionQueryVariables,
} from "@/generated/graphql";
import { MyCardgroupsConnectionDocument } from "@/generated/graphql";
import { prependConnectionEdge } from "@/lib/apollo/connection-cache";
import { CARDGROUPS_DEFAULT_VARS } from "./queries";

/**
 * The `node` projection carried by an edge of the `myCardgroupsConnection`
 * query — `{ __typename: "Cardgroup", id, name, updatedAt }`. Extracted from the
 * generated query type so callers pass a complete node and the helper stays in
 * sync with the document's selection set automatically.
 */
type MyCardgroupNode =
  MyCardgroupsConnectionQuery["myCardgroupsConnection"]["edges"][number]["node"];

/**
 * Prepend a freshly-created cardgroup as a new `{ cursor, node }` edge to the
 * cached `myCardgroupsConnection` and bump `totalCount`, so the new deck appears
 * on `/cardgroups` without a refetch.
 *
 * Shared by `useCreateCardgroup` (the `/cardgroups` feature), `useImportMaster`
 * (the `/catalog` feature, which seeds the cardgroups connection across features),
 * and `useSeedDefaultStarters` (onboarding). Each calls this from inside its
 * `__typename`-narrowed `update` callback after pulling the complete node off the
 * success payload.
 *
 * Delegates the warm-prepend / cold-build / dedup mechanics to the generic
 * `prependConnectionEdge`. The cold-cache build is supplied so a user landing on
 * `/cardgroups` or `/catalog` without an SSR seed still sees the new edge.
 * `CARDGROUPS_DEFAULT_VARS` keeps the cache key in sync with the `/cardgroups` SSR
 * seed and client `useQuery`; any mismatch makes this write invisible. See
 * .claude/rules/pagination.md.
 */
export function prependMyCardgroupEdge(cache: ApolloCache, node: MyCardgroupNode): void {
  prependConnectionEdge(cache, {
    document: MyCardgroupsConnectionDocument,
    variables: CARDGROUPS_DEFAULT_VARS,
    connectionField: "myCardgroupsConnection",
    edgeTypename: "CardgroupEdge",
    node,
    buildColdConnection: () => ({
      __typename: "CardgroupConnection" as const,
      edges: [{ __typename: "CardgroupEdge" as const, cursor: node.id, node }],
      pageInfo: {
        __typename: "PageInfo" as const,
        hasNextPage: false,
        hasPreviousPage: false,
        startCursor: node.id,
        endCursor: node.id,
      },
      totalCount: 1,
    }),
  });
}

/**
 * Optimistically remove a cardgroup edge from the cached `myCardgroupsConnection`
 * and decrement `totalCount` (clamped at 0 — the deleted item may live on a page
 * that was never fetched into `edges`), returning the pre-removal snapshot so the
 * caller can restore it via {@link restoreMyCardgroupSnapshot} if the user undoes
 * the delete.
 *
 * Deliberately does NOT evict the normalized `Cardgroup:<id>` entity: the
 * delayed-DELETE undo-toast flow keeps the entity live during the undo window and
 * evicts + gcs only after the `deleteCardgroup` mutation commits, so an early
 * evict would change the observable cache timing. Returns `null` on a cold-cache
 * miss so the caller can surface a reload error instead of writing an empty
 * connection.
 *
 * `variables` is the live query variables passed by reference (never re-spelled)
 * so the cache key matches the `useQuery`-keyed entry under any active search
 * filter — see `docs/pagination/optimistic-rollback-cache-key.md`. Resolves the
 * edge by `node.id`, never `edge.cursor` (per `.claude/rules/pagination.md`).
 */
export function removeMyCardgroupEdge(
  cache: ApolloCache,
  id: string,
  variables: MyCardgroupsConnectionQueryVariables,
): MyCardgroupsConnectionQuery | null {
  const snapshot = cache.readQuery({
    query: MyCardgroupsConnectionDocument,
    variables,
  });
  if (!snapshot) return null;

  cache.writeQuery({
    query: MyCardgroupsConnectionDocument,
    variables,
    data: {
      myCardgroupsConnection: {
        ...snapshot.myCardgroupsConnection,
        edges: snapshot.myCardgroupsConnection.edges.filter((edge) => edge.node.id !== id),
        totalCount: Math.max(0, snapshot.myCardgroupsConnection.totalCount - 1),
      },
    },
  });

  return snapshot;
}

/**
 * Restore a previously-captured `myCardgroupsConnection` snapshot, re-inserting
 * the optimistically-removed edge when the user undoes a delete. The snapshot
 * carries each node's full projection, so `writeQuery` re-normalizes the entity
 * on its own.
 *
 * `variables` MUST match the object passed to the {@link removeMyCardgroupEdge}
 * call that produced the snapshot so the write targets the same cache key.
 */
export function restoreMyCardgroupSnapshot(
  cache: ApolloCache,
  snapshot: MyCardgroupsConnectionQuery,
  variables: MyCardgroupsConnectionQueryVariables,
): void {
  cache.writeQuery({
    query: MyCardgroupsConnectionDocument,
    variables,
    data: snapshot,
  });
}

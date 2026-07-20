import type { ApolloCache, OperationVariables, Reference } from "@apollo/client";
import type { TypedDocumentNode } from "@graphql-typed-document-node/core";

/**
 * Generic Relay-Connection cache helpers.
 *
 * The "prepend edge + bump totalCount" and "filter edge by node.id + clamp
 * totalCount at 0 + evict + gc" blocks were copy-pasted across every paginated
 * mutation hook. The cache invariants they enforce — resolve an edge by
 * `node.id` (never `edge.cursor`), clamp `totalCount` at `Math.max(0, …)`, use
 * `readQuery` + `writeQuery` (never `cache.modify`, which skips non-existent
 * fields) — are documented in `.claude/rules/pagination.md`. Centralizing the
 * mechanics here means a fix to any invariant lands once instead of being
 * hand-propagated to N call sites.
 *
 * These helpers are behavior-preserving factor-outs. They take the live
 * `variables` object by reference (never re-spell it) so the cache key matches
 * the `useQuery`-keyed entry under any active search filter — see
 * `docs/pagination/optimistic-rollback-cache-key.md`.
 */

/** A single `{ __typename, cursor, node }` edge of a Relay connection. */
interface ConnectionEdge<TTypename extends string, TNode extends { id: string }> {
  __typename: TTypename;
  cursor: string;
  node: TNode;
}

/** The minimal `<Type>Connection` shape these helpers read and write. */
interface CacheConnectionShape<TEdge> {
  __typename: string;
  edges: TEdge[];
  pageInfo: {
    __typename: "PageInfo";
    hasNextPage: boolean;
    hasPreviousPage: boolean;
    startCursor: string | null;
    endCursor: string | null;
  };
  totalCount: number;
}

interface PrependConnectionEdgeInput<
  TData,
  TVariables extends OperationVariables,
  K extends keyof TData,
  TNode extends { id: string },
> {
  /** The connection query document (provides `TData` / `TVariables`). */
  document: TypedDocumentNode<TData, TVariables>;
  /**
   * The live query variables, passed by reference so the cache key matches the
   * `useQuery`-keyed entry. Never re-spell this object.
   */
  variables: TVariables;
  /** The connection field key on `TData` (e.g. `"cardsByCardgroupConnection"`). */
  connectionField: K;
  /** The `__typename` of an edge (e.g. `"CardEdge"`). */
  edgeTypename: string;
  /** The complete node to prepend as a new edge. */
  node: TNode;
  /**
   * Build a fresh connection when the cache is cold (no SSR seed). Omit to
   * no-op on a cold cache — the card hooks rely on the page's own `useQuery`
   * to populate the connection, so a cold-cache write would be discarded.
   */
  buildColdConnection?: () => TData[K];
}

interface RemoveConnectionEdgesInput<
  TData,
  TVariables extends OperationVariables,
  K extends keyof TData,
> {
  /** The connection query document (provides `TData` / `TVariables`). */
  document: TypedDocumentNode<TData, TVariables>;
  /**
   * The live query variables, passed by reference so the cache key matches the
   * `useQuery`-keyed entry. Never re-spell this object.
   */
  variables: TVariables;
  /** The connection field key on `TData` (e.g. `"cardsByCardgroupConnection"`). */
  connectionField: K;
  /** The `__typename` of an entity, used to evict the normalized entry. */
  entityTypename: string;
  /** The ids of the nodes to remove (bulk delete passes many, per-row passes one). */
  ids: string[];
  /**
   * The number to decrement `totalCount` by, clamped at 0 — the deleted item may
   * live on a page that was never fetched into `edges`, so the array filter alone
   * underestimates the removal.
   */
  deletedCount: number;
}

/**
 * Prepend a freshly-created complete node as a new `{ cursor, node }` edge to
 * the cached connection and bump `totalCount`.
 *
 * Cold-cache behavior is caller-controlled:
 * - Omit `buildColdConnection` to no-op when `readQuery` returns `null` (the
 *   card hooks: the page's own `useQuery` owns connection population).
 * - Provide `buildColdConnection` to seed a fresh connection (create-from-anywhere
 *   cases that must surface the new edge on a route the user may land on directly).
 *
 * The dedup guard skips the write if an edge with the same `node.id` already
 * exists. No `cache.writeFragment` is needed: the prepended node carries its full
 * projection, so `writeQuery` normalizes it into the standalone `<Type>:<id>`
 * entry on its own.
 */
export function prependConnectionEdge<
  TData,
  TVariables extends OperationVariables,
  K extends keyof TData,
  TNode extends { id: string },
>(cache: ApolloCache, input: PrependConnectionEdgeInput<TData, TVariables, K, TNode>): void {
  const { document, variables, connectionField, edgeTypename, node, buildColdConnection } = input;

  const existing = cache.readQuery({ query: document, variables });

  const newEdge: ConnectionEdge<string, { id: string }> = {
    __typename: edgeTypename,
    cursor: node.id,
    node,
  };

  if (!existing) {
    if (!buildColdConnection) return;
    cache.writeQuery({
      query: document,
      variables,
      data: { [connectionField]: buildColdConnection() } as TData,
    });
    return;
  }

  const current = existing[connectionField] as CacheConnectionShape<
    ConnectionEdge<string, { id: string }>
  >;
  if (current.edges.some((edge) => edge.node.id === node.id)) return;

  const nextConnection = {
    ...current,
    edges: [newEdge, ...current.edges],
    totalCount: current.totalCount + 1,
  };

  cache.writeQuery({
    query: document,
    variables,
    data: { [connectionField]: nextConnection } as unknown as TData,
  });
}

/**
 * Remove edges whose `node.id` is in `ids` from the cached connection, decrement
 * `totalCount` (clamped at 0), then evict + gc the normalized entities.
 *
 * Covers both bulk delete (`ids.length > 1`) and per-row delete (`ids.length === 1`).
 * No-ops on a cold cache for the edge filter, but always evicts + gcs so the
 * normalized entries are dropped regardless of whether the connection was seeded.
 */
export function removeConnectionEdges<
  TData,
  TVariables extends OperationVariables,
  K extends keyof TData,
>(cache: ApolloCache, input: RemoveConnectionEdgesInput<TData, TVariables, K>): void {
  const { document, variables, connectionField, entityTypename, ids, deletedCount } = input;

  const existing = cache.readQuery({ query: document, variables });
  if (existing) {
    const current = existing[connectionField] as CacheConnectionShape<
      ConnectionEdge<string, { id: string }>
    >;
    const nextConnection = {
      ...current,
      edges: current.edges.filter((edge) => !ids.includes(edge.node.id)),
      totalCount: Math.max(0, current.totalCount - deletedCount),
    };
    cache.writeQuery({
      query: document,
      variables,
      data: { [connectionField]: nextConnection } as unknown as TData,
    });
  }

  for (const id of ids) {
    cache.evict({ id: cache.identify({ __typename: entityTypename, id }) });
  }
  cache.gc();
}

interface RemoveConnectionEdgeAcrossVariantsInput {
  /** The connection field key on the root query (e.g. `"adminMasters"`, `"users"`). */
  connectionField: string;
  /** The `__typename` of the entity, used to evict the normalized entry. */
  entityTypename: string;
  /** The id of the node to remove from every cached variant of the connection. */
  id: string;
}

/**
 * Remove the edge whose `node.id` matches `id` from EVERY cached variant of the
 * connection field (across all `search`/filter argument keys at once), decrement
 * each variant's `totalCount` (clamped at 0), then evict + gc the normalized entity.
 *
 * This uses `cache.modify` deliberately — unlike `removeConnectionEdges`, which
 * targets a single `variables` key via `readQuery`/`writeQuery`, `cache.modify`
 * visits every cached instance of the field, so a delete drops the entity from
 * all active search-filter variants in one pass. The filter resolves the edge by
 * the normalized `node.id` (never `edge.cursor`, which is an opaque "v1:..." or
 * "v2:..." envelope that never equals the raw id) and short-circuits when nothing matched so an
 * unaffected variant keeps its cached reference. See `.claude/rules/pagination.md`.
 */
export function removeConnectionEdgeAcrossVariants(
  cache: ApolloCache,
  input: RemoveConnectionEdgeAcrossVariantsInput,
): void {
  const { connectionField, entityTypename, id } = input;

  cache.modify({
    fields: {
      [connectionField](existing, { readField }) {
        const conn = existing as {
          edges?: ReadonlyArray<{ node: Reference }>;
          totalCount?: number;
        };
        if (!conn.edges) return existing;
        // Filter by the normalized node id, not by `cursor` (the edge cursor is
        // an opaque "v1:..." / "v2:..." envelope that never equals the raw id).
        const next = conn.edges.filter((edge) => readField<string>("id", edge.node) !== id);
        if (next.length === conn.edges.length) return existing;
        return { ...conn, edges: next, totalCount: Math.max(0, (conn.totalCount ?? 0) - 1) };
      },
    },
  });

  const cacheId = cache.identify({ __typename: entityTypename, id });
  if (cacheId) {
    cache.evict({ id: cacheId });
    cache.gc();
  }
}

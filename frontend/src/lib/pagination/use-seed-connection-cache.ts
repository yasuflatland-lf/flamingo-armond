import type { DocumentNode, OperationVariables } from "@apollo/client";
import { useApolloClient } from "@apollo/client/react";
import { useRef } from "react";

export interface UseSeedConnectionCacheInput {
  /** The typed connection query document the SSR seed is keyed on. */
  document: DocumentNode;
  /**
   * The cache key for the seed. MUST be the same `<TYPE>_DEFAULT_VARS` object the
   * client `useQuery` reads with — any mismatch silently splits the cache and the
   * seed becomes invisible to the first cache-first read.
   */
  variables: OperationVariables;
  /**
   * The full `{ <field>: connection }` object passed straight to `writeQuery`.
   * The seed is skipped when the connection payload is missing — i.e. `data` is
   * null/undefined or its single field value is null/undefined (the SSR prop was
   * absent). When `warnScope` is set, that skip emits a triage warning.
   */
  data: unknown;
  /**
   * Optional log scope. When set, a missing connection payload logs
   * `[<warnScope>] initialConnection is null — SSR seed skipped; useQuery will
   * fetch fresh`. Screens whose initial connection prop is required omit it.
   */
  warnScope?: string;
}

/**
 * Returns the connection payload carried by a `{ <field>: connection }` wrapper,
 * or `undefined` when it is absent. SSR-seed wrappers carry exactly one field
 * (the connection), so the single value is the payload; any other shape falls
 * back to the object itself so a present payload is never dropped.
 */
function connectionPayload(data: unknown): unknown {
  if (data == null || typeof data !== "object") return data ?? undefined;
  const values = Object.values(data as Record<string, unknown>);
  return values.length === 1 ? values[0] : data;
}

/**
 * Seeds an SSR-rendered connection into the Apollo cache exactly once,
 * synchronously during render — before the client `useQuery` runs — so the
 * first cache-first read finds the data already present and renders without a
 * network round-trip.
 *
 * The seed is gated by a synchronous `seededRef` boolean so React Strict Mode's
 * double-invoke still produces exactly one write. Seeding in a `useEffect` would
 * create a window between first paint and the post-render write where `useQuery`
 * sees an empty cache. See `docs/pagination/synchronous-ssr-cache-seed.md`.
 *
 * `writeQuery` is the rare write-during-render that is safe here: it does not
 * synchronously re-render the caller, and writing the same data to the same
 * cache key twice is idempotent.
 */
export function useSeedConnectionCache({
  document,
  variables,
  data,
  warnScope,
}: UseSeedConnectionCacheInput): void {
  const apollo = useApolloClient();
  const seededRef = useRef(false);

  if (seededRef.current) return;

  if (connectionPayload(data) == null) {
    if (warnScope) {
      console.warn(
        `[${warnScope}] initialConnection is null — SSR seed skipped; useQuery will fetch fresh`,
      );
    }
    return;
  }

  seededRef.current = true;
  apollo.writeQuery({ query: document, variables, data });
}

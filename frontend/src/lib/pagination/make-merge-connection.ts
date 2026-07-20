/**
 * Minimal structural view of the Relay connection slice this reducer touches.
 * Only `edges` is read; `pageInfo` / `totalCount` ride through the `...moreConn`
 * spread untouched, so they are deliberately not modelled here.
 */
interface EdgesSlice {
  edges: readonly unknown[];
}

/**
 * Builds the `mergeConnection` reducer `useConnectionPagination` takes: it
 * concatenates the next page's edges onto the cached connection stored under
 * `connectionField`, keeping the incoming page's `pageInfo` / `totalCount`.
 *
 * This is the single implementation of that reducer in the tree. Every
 * cursor-paginated screen builds its instance from here rather than hand-rolling
 * the three-line concat, so a future change to the merge (deduplication, cursor
 * handling, a page-info fix) lands in one place.
 *
 * The result object spreads `...more` at the top level so sibling fields of the
 * query result are carried through, then overrides `connectionField` with the
 * concatenated slice.
 *
 * The returned function is a fresh identity per call, and
 * `useConnectionPagination` feeds `mergeConnection` into a `useCallback`
 * dependency array — so callers MUST keep the instance referentially stable:
 * build it once at module scope, or memoize it on `connectionField`.
 */
export function makeMergeConnection<TData>(
  connectionField: keyof TData,
): (prev: TData, more: TData) => TData {
  return (prev, more) => {
    const prevConn = prev[connectionField] as EdgesSlice;
    const moreConn = more[connectionField] as EdgesSlice;
    return {
      ...more,
      [connectionField]: {
        ...moreConn,
        edges: [...prevConn.edges, ...moreConn.edges],
      },
    } as TData;
  };
}

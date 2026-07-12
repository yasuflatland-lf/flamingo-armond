"use client";

/**
 * Loading gate for a lazily-fetched edit-sheet target row.
 *
 * When a listing page opens its edit sheet via `?edit=<id>`, the target row is
 * fetched by a `useLazyQuery` fired inside a `useEffect`. Between the URL change
 * and that effect committing, the lazy query reports `loading === false`,
 * `called === false`, and no data — so a naive `data ? <Form/> : <NotFound/>`
 * body flashes its not-found branch for one frame. This helper collapses the
 * two "still resolving the target row" states into a single `loading` flag:
 *
 *   1. the firing effect has not run yet (`!called`), and
 *   2. the settled result belongs to a previously-open id, not the current one
 *      (`!matchesSheet`) — the race-guard for switching `?edit=<id>` mid-flight.
 *
 * `matchesSheet` is `true` once the lazy query's own `variables.id` equals the
 * currently-open sheet id, i.e. the settled result belongs to the open sheet.
 *
 * Shared by `admin-users-client.tsx` and `admin-roles-client.tsx`. See
 * `docs/frontend/url-backed-sheet-state.md` for the full rationale.
 */
export type SheetTargetQueryState = {
  /** `useLazyQuery`'s `called` flag — false until the firing effect runs. */
  called: boolean;
  /**
   * `useLazyQuery`'s current `variables` — carries the last-fired `id`. The
   * generated type widens `id` to optional (`Partial<Exact<…>>`) before the
   * query has ever fired, so `id` may be `undefined`.
   */
  variables: { id?: string | number } | undefined;
  /** `useLazyQuery`'s `loading` flag. */
  loading: boolean;
};

export type SheetTargetLoading = {
  /** The settled result's `variables.id` matches the currently-open sheet id. */
  matchesSheet: boolean;
  /** True while the target row is still resolving; drives the sheet's loading branch. */
  loading: boolean;
};

/**
 * @param id      The currently-open sheet's edit id, or `null` when no edit
 *                sheet is open.
 * @param query   The lazy query's `called` / `variables` / `loading` fields.
 */
export function useSheetTargetLoading(
  id: string | null,
  query: SheetTargetQueryState,
): SheetTargetLoading {
  const matchesSheet = id !== null && query.variables?.id === id;
  const loading = query.loading || (id !== null && (!query.called || !matchesSheet));
  return { matchesSheet, loading };
}

import type { PageInfo } from "@/generated/base-types";

/**
 * Empty `PageInfo` render-fallback shared by the cursor-paginated list clients.
 *
 * Each `useConnectionPagination` consumer needs an `initial` connection slice to
 * render before the first query resolves; this object is its empty `pageInfo`.
 * It is typed against the canonical generated `PageInfo` (the base schema object
 * type), so it is structurally assignable to every operation's nominally-distinct
 * inline `pageInfo` type without coupling this constant to one operation.
 */
export const EMPTY_PAGE_INFO: PageInfo = {
  __typename: "PageInfo",
  hasNextPage: false,
  hasPreviousPage: false,
  startCursor: null,
  endCursor: null,
};

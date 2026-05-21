/** Input to a parameterised `fetchNextPage` helper used by IntersectionObserver-driven pagination components. */
export interface FetchNextPageInput {
  hasNextPage: boolean;
  endCursor: string | null;
  searchQuery: string | null;
}

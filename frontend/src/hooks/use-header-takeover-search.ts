import { useCallback, useEffect, useState } from "react";
import { type UseDebouncedSearchResult, useDebouncedSearch } from "./use-debounced-search";

export interface UseHeaderTakeoverSearchResult extends UseDebouncedSearchResult {
  /** Whether the mobile takeover bar is open. */
  searchOpen: boolean;
  /** Closes the takeover bar (back button / Escape). */
  closeSearch: () => void;
}

/**
 * Bundles useDebouncedSearch with the mobile header-takeover event-bus wiring:
 * listens for `flamingo:open-search` (header magnifier -> open bar), dispatches
 * `flamingo:search-state` (bar visibility + filter-active -> header trigger), and
 * resets the trigger on unmount. The page renders <SearchTakeoverBar> with the
 * returned { searchOpen, input, setInput, clear, closeSearch }.
 */
export function useHeaderTakeoverSearch(opts?: {
  delayMs?: number;
}): UseHeaderTakeoverSearchResult {
  const search = useDebouncedSearch(opts);
  const searchQuery = search.query;
  const [searchOpen, setSearchOpen] = useState(false);

  // Header magnifier -> open the takeover in place.
  useEffect(() => {
    function handleOpenSearch() {
      setSearchOpen(true);
    }
    window.addEventListener("flamingo:open-search", handleOpenSearch);
    return () => window.removeEventListener("flamingo:open-search", handleOpenSearch);
  }, []);

  // Report filter state back to the header trigger (active dot + aria-expanded).
  useEffect(() => {
    window.dispatchEvent(
      new CustomEvent("flamingo:search-state", {
        detail: { active: searchQuery !== null && searchQuery !== "", visible: searchOpen },
      }),
    );
  }, [searchQuery, searchOpen]);

  // Reset the header trigger when the page unmounts (route leave).
  useEffect(() => {
    return () => {
      window.dispatchEvent(
        new CustomEvent("flamingo:search-state", {
          detail: { active: false, visible: false },
        }),
      );
    };
  }, []);

  const closeSearch = useCallback(() => setSearchOpen(false), []);
  return { ...search, searchOpen, closeSearch };
}

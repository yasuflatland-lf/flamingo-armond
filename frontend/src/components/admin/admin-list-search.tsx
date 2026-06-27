"use client";

import { SearchTakeoverBar } from "@/components/search/search-takeover-bar";
import { Input } from "@/components/ui/input";
import type { useHeaderTakeoverSearch } from "@/hooks/use-header-takeover-search";

interface AdminListSearchProps {
  /** The header-takeover search instance owned by the consuming client. */
  search: ReturnType<typeof useHeaderTakeoverSearch>;
  /** Field placeholder — caller-specific copy. */
  placeholder: string;
  /** Field aria-label — caller-specific copy. */
  ariaLabel: string;
}

/**
 * Search controls shared by the paginated admin list screens (users, masters).
 * Renders the mobile header-takeover bar plus the desktop search box, both wired
 * to the caller's `useHeaderTakeoverSearch` instance. The client still owns the
 * hook; this component only renders its value/onChange/open state. The desktop
 * box uses the design-system `<Input>` so the two screens stay consistent.
 */
export function AdminListSearch({ search, placeholder, ariaLabel }: AdminListSearchProps) {
  return (
    <>
      <SearchTakeoverBar
        open={search.searchOpen}
        value={search.input}
        onChange={search.setInput}
        onClear={search.clear}
        onClose={search.closeSearch}
        placeholder={placeholder}
        ariaLabel={ariaLabel}
      />
      {/* Desktop-only search input; mobile uses the header takeover above. */}
      <div className="hidden md:block">
        <Input
          type="search"
          placeholder={placeholder}
          value={search.input}
          onChange={(e) => search.setInput(e.target.value)}
          aria-label={ariaLabel}
        />
      </div>
    </>
  );
}

"use client";

import { useTranslations } from "next-intl";
import { SearchInput } from "@/components/search/search-input";

interface CardgroupsToolbarProps {
  searchInput: string;
  onSearchInputChange: (value: string) => void;
}

/**
 * Toolbar for the /cardgroups listing page.
 * Renders a search input that filters cardgroups by name (case-insensitive).
 */
export function CardgroupsToolbar({ searchInput, onSearchInputChange }: CardgroupsToolbarProps) {
  const t = useTranslations("Cardgroups");

  return (
    <div className="mb-6 hidden md:block">
      <SearchInput
        placeholder={t("filterPlaceholder")}
        value={searchInput}
        onChange={(e) => onSearchInputChange(e.target.value)}
        aria-label={t("filterAriaLabel")}
      />
    </div>
  );
}

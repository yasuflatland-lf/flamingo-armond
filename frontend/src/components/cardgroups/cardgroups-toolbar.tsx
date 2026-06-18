"use client";

import { useTranslations } from "next-intl";

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
      <input
        type="search"
        placeholder={t("filterPlaceholder")}
        value={searchInput}
        onChange={(e) => onSearchInputChange(e.target.value)}
        className="w-full rounded-md border border-input bg-background px-3 py-2 text-sm focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
        aria-label={t("filterAriaLabel")}
      />
    </div>
  );
}

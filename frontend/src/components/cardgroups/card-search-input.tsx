"use client";

import { useTranslations } from "next-intl";
import { SearchInput } from "@/components/search/search-input";

export function CardSearchInput({
  value,
  onChange,
}: {
  value: string;
  onChange: (next: string) => void;
}) {
  const t = useTranslations("Cards");
  return (
    // Desktop only: on mobile the search moves into the header-takeover bar.
    <div className="mb-3 hidden md:block">
      <SearchInput
        icon
        placeholder={t("searchPlaceholder")}
        value={value}
        onChange={(e) => onChange(e.target.value)}
        aria-label={t("searchAriaLabel")}
        data-testid="cards-search-input"
      />
    </div>
  );
}

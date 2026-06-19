"use client";

import { Search } from "lucide-react";
import { useTranslations } from "next-intl";

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
      <div className="relative">
        <Search
          aria-hidden="true"
          className="pointer-events-none absolute left-2.5 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground"
        />
        <input
          type="search"
          placeholder={t("searchPlaceholder")}
          value={value}
          onChange={(e) => onChange(e.target.value)}
          className="w-full rounded-md border border-input bg-background pl-8 pr-3 py-2 text-sm focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
          aria-label={t("searchAriaLabel")}
          data-testid="cards-search-input"
        />
      </div>
    </div>
  );
}

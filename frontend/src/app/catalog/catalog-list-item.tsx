"use client";

import Link from "next/link";
import { useTranslations } from "next-intl";
import { CatalogCardFieldsFragment } from "@/app/catalog/queries";
import { Badge } from "@/components/ui/badge";
import { type FragmentType, useFragment } from "@/generated/fragment-masking";

export type CatalogListItemProps = {
  /** A masked `CatalogCardFields` ref — unmasked once via `useFragment` below. */
  node: FragmentType<typeof CatalogCardFieldsFragment>;
};

/**
 * List row for one published master cardgroup on /catalog. The entire row is a
 * single link to the deck-detail page (`/catalog/[id]`); Import has moved to that
 * detail page, so the row carries no nested interactive element. Information
 * hierarchy: Tier 1 = deck name (lead) + card-count stat (subordinate figure);
 * Tier 2 = language / level / category badges (always visible); Tier 3 = the
 * description, revealed on hover / keyboard focus via CSS — a desktop progressive
 * enhancement, never the only path to the info (it also lives on the detail page).
 * The locale-independent `data-testid` (`catalog-row-{id}`) lets e2e — which runs
 * in ja-JP — target the row without depending on translated copy.
 */
export function CatalogListItem({ node }: CatalogListItemProps) {
  const t = useTranslations("Catalog");
  const card = useFragment(CatalogCardFieldsFragment, node);

  return (
    <li>
      <Link
        href={`/catalog/${card.id}`}
        data-testid={`catalog-row-${card.id}`}
        aria-label={t("viewDeckAriaLabel", { name: card.name })}
        className="group block rounded-md border border-border p-4 transition-[box-shadow,transform,background-color] duration-150 ease-out hover:-translate-y-0.5 hover:bg-accent hover:shadow-md motion-reduce:transition-none motion-reduce:hover:translate-y-0 sm:py-3"
      >
        {/* Tier 1: name (lead) + card-count stat (subordinate figure, right). */}
        <div className="flex items-baseline gap-3">
          <h3 className="min-w-0 flex-1 truncate text-[15px] font-semibold leading-[1.3] tracking-[-0.01em] text-foreground">
            {card.name}
          </h3>
          <span className="shrink-0 whitespace-nowrap">
            <span className="text-sm font-semibold tabular-nums text-foreground">
              {t("cardCountStat", { count: card.cardCount })}
            </span>{" "}
            <span className="text-[10px] text-muted-foreground">
              {t("unitCards", { count: card.cardCount })}
            </span>
          </span>
        </div>

        {/* Tier 2: badges (always visible) + chevron navigability affordance. */}
        <div className="mt-1.5 flex items-center gap-2">
          <div className="flex min-w-0 flex-1 flex-wrap items-center gap-1.5">
            {card.language && (
              <Badge variant="secondary" className="shrink-0">
                {card.language}
              </Badge>
            )}
            {card.level && (
              <Badge variant="outline" className="shrink-0">
                {t("level", { level: card.level })}
              </Badge>
            )}
            {card.category && (
              <Badge variant="outline" className="shrink-0">
                {card.category}
              </Badge>
            )}
          </div>
          <span
            aria-hidden="true"
            className="shrink-0 text-border transition-transform duration-150 ease-out group-hover:translate-x-0.5 motion-reduce:transition-none motion-reduce:group-hover:translate-x-0"
          >
            ›
          </span>
        </div>

        {/* Tier 3: description — collapsed by default, revealed on hover / focus
            (desktop progressive enhancement) via a grid-rows 0fr→1fr transition.
            Always in the DOM for screen readers; visually hidden on touch (no
            hover), where the deck's detail page carries the description instead. */}
        {card.description && (
          <div className="grid grid-rows-[0fr] opacity-0 transition-[grid-template-rows,opacity] duration-200 ease-out group-hover:grid-rows-[1fr] group-hover:opacity-100 group-focus-within:grid-rows-[1fr] group-focus-within:opacity-100 motion-reduce:transition-none">
            <div className="overflow-hidden">
              <p
                data-testid={`catalog-row-desc-${card.id}`}
                className="mt-1.5 line-clamp-1 text-[11px] leading-[1.5] text-muted-foreground"
              >
                {card.description}
              </p>
            </div>
          </div>
        )}
      </Link>
    </li>
  );
}

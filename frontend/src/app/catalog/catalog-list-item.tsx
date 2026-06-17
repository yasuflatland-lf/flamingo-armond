"use client";

import { Check, Download } from "lucide-react";
import { useTranslations } from "next-intl";
import { CatalogCardFieldsFragment } from "@/app/catalog/queries";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { type FragmentType, useFragment } from "@/generated/fragment-masking";

export type CatalogListItemProps = {
  /** A masked `CatalogCardFields` ref — unmasked once via `useFragment` below. */
  node: FragmentType<typeof CatalogCardFieldsFragment>;
  /** True while this cardgroup's import mutation is in flight. */
  importing: boolean;
  /** True once this cardgroup has been imported in the current session. */
  imported: boolean;
  onImport: (id: string) => void;
  /**
   * Optional button label overrides. Defaults reproduce the `/catalog` copy
   * (`Catalog` namespace) so existing call sites are unaffected.
   */
  labels?: { action: string; inProgress: string; done: string };
  /** Optional `data-testid` prefix on the Import button. Defaults to `"catalog-import"`. */
  testIdPrefix?: string;
};

/**
 * List row for one published master cardgroup. Mirrors AdminMasterRow's density
 * (name + metadata + right-aligned action; mobile two-tier, desktop single inline
 * row) so the public catalog and the admin masters list speak the same perceptual
 * language for the same decks. The description is intentionally not shown — the
 * row optimises for scanning many decks. The Import button keeps the
 * locale-independent `data-testid` (`catalog-import-{id}`) so e2e — which runs in
 * the ja-JP locale — can target it without depending on translated copy.
 */
export function CatalogListItem({
  node,
  importing,
  imported,
  onImport,
  labels,
  testIdPrefix,
}: CatalogListItemProps) {
  const t = useTranslations("Catalog");
  const card = useFragment(CatalogCardFieldsFragment, node);

  const actionLabel = labels?.action ?? t("import");
  const inProgressLabel = labels?.inProgress ?? t("importing");
  const doneLabel = labels?.done ?? t("imported");
  const idPrefix = testIdPrefix ?? "catalog-import";

  return (
    <li
      className="rounded-md border border-border transition-colors hover:bg-accent"
      data-testid={`catalog-row-${card.id}`}
    >
      {/*
       * Mobile (default): two tiers — the name as a full-width title line, then a
       * meta line carrying the badges + count on the left and Import on the right.
       * Desktop (sm+): a single inline row [name][badges][count][Import]. One DOM
       * serves both: `w-full` + `order-*` force the mobile line break, and
       * `sm:contents` dissolves the meta wrapper on desktop so the badges and count
       * rejoin the inline row in their own order. Spacing rhythm 16 / 12 / 8.
       */}
      <div className="flex flex-wrap items-center gap-x-3 gap-y-3 p-4 sm:py-3">
        <h3
          className="order-1 w-full min-w-0 truncate text-sm font-medium tracking-tight sm:w-auto sm:flex-1"
          title={card.name}
        >
          {card.name}
        </h3>

        <div className="order-2 flex min-w-0 flex-1 flex-wrap items-center gap-2 sm:contents">
          {card.language && (
            <Badge variant="secondary" className="shrink-0 sm:order-2">
              {card.language}
            </Badge>
          )}
          {card.level && (
            <Badge variant="outline" className="shrink-0 sm:order-2">
              {t("level", { level: card.level })}
            </Badge>
          )}
          {card.category && (
            <Badge variant="outline" className="shrink-0 sm:order-2">
              {card.category}
            </Badge>
          )}
          <span className="shrink-0 text-xs tabular-nums text-muted-foreground sm:order-3">
            {t("cardCount", { count: card.cardCount })}
          </span>
        </div>

        <Button
          type="button"
          variant={imported ? "outline" : "brand"}
          onClick={() => onImport(card.id)}
          disabled={importing || imported}
          data-testid={`${idPrefix}-${card.id}`}
          className="order-3 shrink-0 sm:order-4"
        >
          {imported ? (
            <>
              <Check aria-hidden="true" className="h-4 w-4" />
              <span className="break-keep">{doneLabel}</span>
            </>
          ) : (
            <>
              <Download aria-hidden="true" className="h-4 w-4" />
              <span className="break-keep">{importing ? inProgressLabel : actionLabel}</span>
            </>
          )}
        </Button>
      </div>
    </li>
  );
}

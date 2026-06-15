"use client";

import { Check, Download } from "lucide-react";
import { useTranslations } from "next-intl";
import type { CSSProperties } from "react";
import { CatalogCardFieldsFragment } from "@/app/catalog/queries";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { type FragmentType, useFragment } from "@/generated/fragment-masking";
import { cn } from "@/lib/utils";

export type CatalogCardProps = {
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
  /** Optional `data-testid` prefix. Defaults to `"catalog-import"`. */
  testIdPrefix?: string;
  /** Optional class passthrough on the root `<li>` (e.g. fixed width, entrance animation). */
  className?: string;
  /** Optional inline style passthrough on the root `<li>` (e.g. staggered animation-delay). */
  style?: CSSProperties;
};

/**
 * Presentational tile for one published master cardgroup. The card count sits in
 * the metadata row and the CTA spans the card's full width, so the bottom row
 * never collides in a narrow column. The Import button carries a
 * locale-independent `data-testid` (`catalog-import-{id}`) so e2e — which runs in
 * the ja-JP locale — can target it without depending on translated copy.
 */
export function CatalogCard({
  node,
  importing,
  imported,
  onImport,
  labels,
  testIdPrefix,
  className,
  style,
}: CatalogCardProps) {
  const t = useTranslations("Catalog");
  const card = useFragment(CatalogCardFieldsFragment, node);

  const actionLabel = labels?.action ?? t("import");
  const inProgressLabel = labels?.inProgress ?? t("importing");
  const doneLabel = labels?.done ?? t("imported");
  const idPrefix = testIdPrefix ?? "catalog-import";

  return (
    <li
      className={cn(
        "flex flex-col gap-3 rounded-xl border border-border bg-card p-5 shadow-sm transition-[box-shadow,transform] duration-150 ease-out hover:-translate-y-0.5 hover:shadow-md motion-reduce:transition-none motion-reduce:hover:translate-y-0",
        className,
      )}
      style={style}
    >
      <div className="flex flex-col gap-1">
        <h3 className="truncate font-semibold leading-[1.3] tracking-[-0.011em] text-foreground">
          {card.name}
        </h3>
        {card.description && (
          <p className="line-clamp-2 text-sm leading-[1.55] text-muted-foreground">
            {card.description}
          </p>
        )}
      </div>

      <div className="flex flex-wrap items-center gap-1.5">
        {card.language && <Badge variant="secondary">{card.language}</Badge>}
        {card.level && <Badge variant="outline">{t("level", { level: card.level })}</Badge>}
        {card.category && <Badge variant="outline">{card.category}</Badge>}
        <span className="text-[13px] tabular-nums text-muted-foreground">
          {t("cardCount", { count: card.cardCount })}
        </span>
      </div>

      <Button
        type="button"
        variant={imported ? "outline" : "brand"}
        onClick={() => onImport(card.id)}
        disabled={importing || imported}
        data-testid={`${idPrefix}-${card.id}`}
        className="mt-auto w-full"
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
    </li>
  );
}

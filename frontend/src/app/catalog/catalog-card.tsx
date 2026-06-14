"use client";

import { Check, Download } from "lucide-react";
import { useTranslations } from "next-intl";
import { CatalogCardFieldsFragment } from "@/app/catalog/queries";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { type FragmentType, useFragment } from "@/generated/fragment-masking";

export type CatalogCardProps = {
  /** A masked `CatalogCardFields` ref — unmasked once via `useFragment` below. */
  node: FragmentType<typeof CatalogCardFieldsFragment>;
  /** True while this deck's import mutation is in flight. */
  importing: boolean;
  /** True once this deck has been imported in the current session. */
  imported: boolean;
  onImport: (id: string) => void;
  /**
   * Optional button label overrides. Defaults reproduce the `/catalog` copy
   * (`Catalog` namespace) so existing call sites are unaffected.
   */
  labels?: { action: string; inProgress: string; done: string };
  /** Optional `data-testid` prefix. Defaults to `"catalog-import"`. */
  testIdPrefix?: string;
};

/**
 * Presentational catalog tile for one published master deck. The Import button
 * carries a locale-independent `data-testid` (`catalog-import-{id}`) so e2e —
 * which runs in the ja-JP locale — can target it without depending on
 * translated copy. See project memory "E2E runs in Japanese locale".
 */
export function CatalogCard({
  node,
  importing,
  imported,
  onImport,
  labels,
  testIdPrefix,
}: CatalogCardProps) {
  const t = useTranslations("Catalog");
  const card = useFragment(CatalogCardFieldsFragment, node);

  const actionLabel = labels?.action ?? t("import");
  const inProgressLabel = labels?.inProgress ?? t("importing");
  const doneLabel = labels?.done ?? t("imported");
  const idPrefix = testIdPrefix ?? "catalog-import";

  return (
    <li className="flex flex-col gap-3 rounded-lg border border-border bg-background p-4">
      <div className="flex flex-col gap-1">
        <h3 className="truncate font-medium text-foreground">{card.name}</h3>
        {card.description && (
          <p className="line-clamp-2 text-sm text-muted-foreground">{card.description}</p>
        )}
      </div>

      {(card.language || card.level || card.category) && (
        <div className="flex flex-wrap gap-1.5">
          {card.language && <Badge variant="secondary">{card.language}</Badge>}
          {card.level && <Badge variant="outline">{t("level", { level: card.level })}</Badge>}
          {card.category && <Badge variant="outline">{card.category}</Badge>}
        </div>
      )}

      <div className="mt-auto flex items-center justify-between gap-2">
        <span className="text-sm text-muted-foreground">
          {t("cardCount", { count: card.cardCount })}
        </span>
        <Button
          type="button"
          variant={imported ? "outline" : "brand"}
          size="sm"
          onClick={() => onImport(card.id)}
          disabled={importing || imported}
          data-testid={`${idPrefix}-${card.id}`}
        >
          {imported ? (
            <>
              <Check aria-hidden="true" className="h-4 w-4" />
              <span>{doneLabel}</span>
            </>
          ) : (
            <>
              <Download aria-hidden="true" className="h-4 w-4" />
              <span>{importing ? inProgressLabel : actionLabel}</span>
            </>
          )}
        </Button>
      </div>
    </li>
  );
}

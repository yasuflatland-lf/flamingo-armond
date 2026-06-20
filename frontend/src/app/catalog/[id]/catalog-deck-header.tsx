"use client";

import { useTranslations } from "next-intl";
import { CatalogImportButton } from "@/app/catalog/_components/catalog-import-button";
import { DetailPageHeader } from "@/components/nav/detail-page-header";
import { Badge } from "@/components/ui/badge";
import type { CatalogMasterDeckQuery } from "@/generated/graphql";

/** The published master deck the header renders. Non-null projection of `masterCardgroup`. */
export type CatalogDeck = NonNullable<CatalogMasterDeckQuery["masterCardgroup"]>;

type CatalogDeckHeaderProps = {
  deck: CatalogDeck;
  /** Live card count, sourced from the connection's `totalCount`. */
  cardCount: number;
  /** True while this deck's import mutation is in flight. */
  importing: boolean;
  /** True once this deck has been imported in the current session. */
  imported: boolean;
  onImport: (id: string) => void;
};

/**
 * Detail-page header for the public catalog deck view. Composes the shared
 * {@link DetailPageHeader} (inline back to `/catalog`, centered card count,
 * trailing "Import this deck" CTA) with the deck's catalog metadata — the
 * language / level / category badges (mirroring `CatalogListItem`) and the
 * description paragraph — rendered in the title-row slot below the app bar.
 *
 * Import state (`importing` / `imported` / `onImport`) is owned by the client and
 * threaded in via props; this component holds no mutation state of its own.
 */
export function CatalogDeckHeader({
  deck,
  cardCount,
  importing,
  imported,
  onImport,
}: CatalogDeckHeaderProps) {
  const t = useTranslations("Catalog");

  return (
    <DetailPageHeader
      backHref="/catalog"
      backLabel={t("backToCatalog")}
      title={deck.name}
      meta={
        <span className="text-sm text-muted-foreground">
          {t("cardCount", { count: cardCount })}
        </span>
      }
      actions={
        <CatalogImportButton
          card={{ id: deck.id }}
          importing={importing}
          imported={imported}
          onImport={onImport}
          testIdPrefix="catalog-deck-import"
        />
      }
    >
      <div className="mt-3 flex flex-col items-center gap-2">
        {(deck.language || deck.level || deck.category) && (
          <div className="flex flex-wrap items-center justify-center gap-2">
            {deck.language && <Badge variant="secondary">{deck.language}</Badge>}
            {deck.level && <Badge variant="outline">{t("level", { level: deck.level })}</Badge>}
            {deck.category && <Badge variant="outline">{deck.category}</Badge>}
          </div>
        )}
        {deck.description && (
          <p
            className="max-w-2xl text-center text-sm text-muted-foreground"
            data-testid="catalog-deck-description"
          >
            {deck.description}
          </p>
        )}
      </div>
    </DetailPageHeader>
  );
}

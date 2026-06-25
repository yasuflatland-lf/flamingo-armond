"use client";

import { useTranslations } from "next-intl";
import { CatalogImportButton } from "@/app/catalog/_components/catalog-import-button";
import { DetailPageHeader } from "@/components/nav/detail-page-header";
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
 * {@link DetailPageHeader} (inline back to `/catalog`, centered card count) with
 * the deck's description paragraph and a full-width "Import this deck" CTA, all
 * rendered in the title-row slot below the app bar. The CTA sits below the
 * description rather than in the app-bar action cluster so it reads as the page's
 * primary call to action.
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
    >
      {deck.description && (
        <div className="mt-2 flex flex-col items-center gap-2">
          <p
            className="max-w-2xl text-center text-sm text-muted-foreground"
            data-testid="catalog-deck-description"
          >
            {deck.description}
          </p>
        </div>
      )}
      <div className="mt-2">
        <CatalogImportButton
          card={{ id: deck.id }}
          importing={importing}
          imported={imported}
          onImport={onImport}
          testIdPrefix="catalog-deck-import"
          className="w-full"
        />
      </div>
    </DetailPageHeader>
  );
}

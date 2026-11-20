"use client";

import { Check, Download } from "lucide-react";
import { useTranslations } from "next-intl";
import { Button } from "@/components/ui/button";

export type CatalogImportButtonProps = {
  /** The deck to import — only `id` is read (the click-handler arg and the `data-testid` suffix). */
  deck: { id: string };
  /** True while this cardgroup's import mutation is in flight. */
  importing: boolean;
  /** True once this cardgroup has been imported in the current session. */
  imported: boolean;
  onImport: (id: string) => void;
  /**
   * Optional button label overrides. When omitted, the labels default to the
   * `/catalog` copy (`Catalog` namespace); no production caller overrides them today.
   * All three keys must be supplied together — partial override is not supported.
   */
  labels?: { action: string; inProgress: string; done: string };
  /** Optional `data-testid` prefix on the Import button. Defaults to `"catalog-import"`. */
  testIdPrefix?: string;
  /**
   * Layout class passthrough merged onto the rendered `Button` (e.g. `w-full` on the
   * deck-detail header). `Button` merges it with its variant classes via `cn`.
   */
  className?: string;
};

/**
 * The Import CTA on the /catalog deck-detail header (`CatalogDeckHeader`): imported / in-flight /
 * idle rendering. `data-testid` is `{testIdPrefix}-{id}` (default prefix `catalog-import`; the
 * header passes `catalog-deck-import`) so tests select it without depending on translated copy.
 * Invariant: `importing` and `imported` are never both true; if they are, imported (done) wins.
 */
export function CatalogImportButton({
  deck,
  importing,
  imported,
  onImport,
  labels,
  testIdPrefix,
  className,
}: CatalogImportButtonProps) {
  const t = useTranslations("Catalog");

  const actionLabel = labels?.action ?? t("import");
  const inProgressLabel = labels?.inProgress ?? t("importing");
  const doneLabel = labels?.done ?? t("imported");
  const idPrefix = testIdPrefix ?? "catalog-import";

  return (
    <Button
      type="button"
      variant={imported ? "outline" : "brand"}
      onClick={() => onImport(deck.id)}
      disabled={importing || imported}
      data-testid={`${idPrefix}-${deck.id}`}
      className={className}
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
  );
}

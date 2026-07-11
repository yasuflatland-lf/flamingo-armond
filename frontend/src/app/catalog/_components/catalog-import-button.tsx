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
   * `/catalog` copy (`Catalog` namespace); pass an explicit object to reuse this
   * button in another context (e.g. an onboarding chooser). All three keys must
   * be supplied together — partial override is not supported.
   */
  labels?: { action: string; inProgress: string; done: string };
  /** Optional `data-testid` prefix on the Import button. Defaults to `"catalog-import"`. */
  testIdPrefix?: string;
  /**
   * Layout class passthrough merged onto the rendered `Button` (e.g. `mt-auto w-full`
   * for the card tile, `order-3 shrink-0 sm:order-4` for the list row). `Button`
   * merges it with its variant classes via `cn`.
   */
  className?: string;
};

/**
 * The Import CTA shared by `CatalogDeckTile` (full-width tile button) and
 * `CatalogListItem` (right-aligned inline button). Owns the three-state label
 * derivation and the imported / in-flight / idle rendering so the two layouts
 * cannot drift. The locale-independent `data-testid` (`catalog-import-{id}`) lets
 * e2e — which runs in the ja-JP locale — target the button without depending on
 * translated copy. The only per-layout difference is `className`.
 *
 * Invariant: `importing` and `imported` are never both true; if they are, the
 * imported (done) state wins and the in-flight label is not shown.
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

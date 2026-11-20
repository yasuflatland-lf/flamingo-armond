"use client";

import { Download } from "lucide-react";
import { useTranslations } from "next-intl";
import type { CSSProperties } from "react";
import { CatalogDeckFieldsFragment } from "@/app/catalog/queries";
import { Button } from "@/components/ui/button";
import { type FragmentType, useFragment } from "@/generated/fragment-masking";
import { cn } from "@/lib/utils";

export type OnboardingDeckTileProps = {
  /** A masked `CatalogDeckFields` ref — unmasked once via `useFragment` below. */
  node: FragmentType<typeof CatalogDeckFieldsFragment>;
  /** True while this cardgroup's import mutation is in flight. */
  importing: boolean;
  onImport: (id: string) => void;
  /** Optional class passthrough on the root `<li>` (e.g. fixed width, entrance animation). */
  className?: string;
  /** Optional inline style passthrough on the root `<li>` (e.g. staggered animation-delay). */
  style?: CSSProperties;
};

/**
 * Preset-deck tile for the /onboarding/start chooser. Unmasks `CatalogDeckFields` (the same
 * fragment the /catalog list spreads) and renders the name, description, card count and a
 * full-width Start CTA. The CTA carries a locale-independent `data-testid`
 * (`onboarding-deck-{id}`) so tests select it without depending on translated copy.
 */
export function OnboardingDeckTile({
  node,
  importing,
  onImport,
  className,
  style,
}: OnboardingDeckTileProps) {
  const t = useTranslations("Catalog");
  const tStart = useTranslations("OnboardingStart");
  const deck = useFragment(CatalogDeckFieldsFragment, node);

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
          {deck.name}
        </h3>
        {deck.description && (
          <p className="line-clamp-2 text-sm leading-[1.55] text-muted-foreground">
            {deck.description}
          </p>
        )}
      </div>

      <div className="flex flex-wrap items-center gap-1.5">
        <span className="text-[13px] tabular-nums text-muted-foreground">
          {t("cardCount", { count: deck.cardCount })}
        </span>
      </div>

      <Button
        type="button"
        variant="brand"
        onClick={() => onImport(deck.id)}
        disabled={importing}
        data-testid={`onboarding-deck-${deck.id}`}
        className="mt-auto w-full"
      >
        <Download aria-hidden="true" className="h-4 w-4" />
        <span className="break-keep">
          {importing ? tStart("starting") : tStart("startWithDeck")}
        </span>
      </Button>
    </li>
  );
}

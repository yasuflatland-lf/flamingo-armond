"use client";

import { CircleCheck, FilePlus2, Sprout, TrendingUp } from "lucide-react";
import Link from "next/link";
import { useTranslations } from "next-intl";
import { buttonVariants } from "@/components/ui/button";
import { EmptyState } from "@/components/ui/empty-state";

/**
 * The four `/stats` empty states. They share the dashed-border empty-state look
 * via the `EmptyState` primitive (`components/ui/empty-state.tsx`) but are kept
 * as distinct components because the CTA composition differs per state: each
 * carries at most one filled `brand` CTA (the emphasis ladder), passed through
 * the primitive's `actions` slot, plus a muted lucide icon (matching the app's
 * icon language — no emoji). Copy flows through `useTranslations("Stats")`.
 */

/**
 * Truly-new user who owns no cardgroup at all, and nothing studied. Two CTAs:
 * browse the catalog (primary) or create a deck (secondary). Distinct from
 * `EmptyDeckEmpty`, which is the "owns a deck but it has no cards yet" case.
 */
export function WelcomeEmpty() {
  const t = useTranslations("Stats");
  return (
    <EmptyState
      icon={<Sprout aria-hidden="true" className="h-8 w-8 text-muted-foreground" />}
      heading={t("emptyNewUser")}
      body={t("emptyNewUserBody")}
      bodyClassName="mx-auto max-w-md"
      actions={
        <div className="flex flex-col items-center justify-center gap-3 sm:flex-row">
          <Link href="/catalog" className={buttonVariants({ variant: "brand" })}>
            {t("emptyBrowseCatalog")}
          </Link>
          <Link href="/cardgroups/new" className={buttonVariants({ variant: "outline" })}>
            {t("emptyCreateDeck")}
          </Link>
        </div>
      }
    />
  );
}

/**
 * Owns at least one cardgroup but every deck is empty (the backend omits
 * zero-card decks from `decks`), and nothing studied. One CTA: go add cards.
 */
export function EmptyDeckEmpty() {
  const t = useTranslations("Stats");
  return (
    <EmptyState
      icon={<FilePlus2 aria-hidden="true" className="h-8 w-8 text-muted-foreground" />}
      heading={t("emptyDeck")}
      body={t("emptyDeckBody")}
      bodyClassName="mx-auto max-w-md"
      actions={
        <Link href="/cardgroups" className={buttonVariants({ variant: "brand" })}>
          {t("emptyDeckCta")}
        </Link>
      }
    />
  );
}

/**
 * Has decks but no reviews yet. One CTA: start studying.
 */
export function NotStudiedEmpty() {
  const t = useTranslations("Stats");
  return (
    <EmptyState
      icon={<TrendingUp aria-hidden="true" className="h-8 w-8 text-muted-foreground" />}
      heading={t("emptyNotStudied")}
      body={t("emptyNotStudiedBody")}
      bodyClassName="mx-auto max-w-md"
      actions={
        <Link href="/cardgroups" className={buttonVariants({ variant: "brand" })}>
          {t("emptyStartStudying")}
        </Link>
      }
    />
  );
}

/**
 * No repeatedly-forgotten cards. Calm, no CTA — a reassuring `CircleCheck` rather
 * than a celebratory emoji (coral-minimal restraint). Kept intentionally small
 * (`text-sm` heading, `text-xs` body) so it stays quiet inside the struggling slot.
 */
export function StrugglingEmpty() {
  const t = useTranslations("Stats");
  return (
    <EmptyState
      className="justify-center"
      icon={<CircleCheck aria-hidden="true" className="h-8 w-8 text-muted-foreground" />}
      heading={t("strugglingEmpty")}
      headingClassName="text-sm"
      body={t("strugglingEmptyBody")}
      bodyClassName="mt-1 mx-auto max-w-xs text-xs"
    />
  );
}

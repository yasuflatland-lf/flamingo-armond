"use client";

import { CircleCheck, FilePlus2, Sprout, TrendingUp } from "lucide-react";
import Link from "next/link";
import { useTranslations } from "next-intl";
import { buttonVariants } from "@/components/ui/button";

/**
 * The three `/stats` empty states. They share the dashed-border empty-state look
 * of `AllCaughtUp` (`components/learn/all-caught-up.tsx`) but are reimplemented
 * locally because the CTA composition differs per state. Each carries at most one
 * filled `brand` CTA (the emphasis ladder), and a muted lucide icon (matching the
 * app's icon language — no emoji). Copy flows through `useTranslations("Stats")`.
 */

/**
 * Truly-new user who owns no cardgroup at all, and nothing studied. Two CTAs:
 * browse the catalog (primary) or create a deck (secondary). Distinct from
 * `EmptyDeckEmpty`, which is the "owns a deck but it has no cards yet" case.
 */
export function WelcomeEmpty() {
  const t = useTranslations("Stats");
  return (
    <div className="flex flex-col items-center rounded-lg border border-dashed border-border p-8 text-center">
      <Sprout aria-hidden="true" className="h-8 w-8 text-muted-foreground" />
      <h2 className="mt-2 text-xl font-semibold">{t("emptyNewUser")}</h2>
      <p className="mx-auto mt-2 max-w-md text-sm text-muted-foreground">{t("emptyNewUserBody")}</p>
      <div className="mt-6 flex flex-col items-center justify-center gap-3 sm:flex-row">
        <Link href="/catalog" className={buttonVariants({ variant: "brand" })}>
          {t("emptyBrowseCatalog")}
        </Link>
        <Link href="/cardgroups/new" className={buttonVariants({ variant: "outline" })}>
          {t("emptyCreateDeck")}
        </Link>
      </div>
    </div>
  );
}

/**
 * Owns at least one cardgroup but every deck is empty (the backend omits
 * zero-card decks from `decks`), and nothing studied. One CTA: go add cards.
 */
export function EmptyDeckEmpty() {
  const t = useTranslations("Stats");
  return (
    <div className="flex flex-col items-center rounded-lg border border-dashed border-border p-8 text-center">
      <FilePlus2 aria-hidden="true" className="h-8 w-8 text-muted-foreground" />
      <h2 className="mt-2 text-xl font-semibold">{t("emptyDeck")}</h2>
      <p className="mx-auto mt-2 max-w-md text-sm text-muted-foreground">{t("emptyDeckBody")}</p>
      <div className="mt-6 flex justify-center">
        <Link href="/cardgroups" className={buttonVariants({ variant: "brand" })}>
          {t("emptyDeckCta")}
        </Link>
      </div>
    </div>
  );
}

/**
 * Has decks but no reviews yet. One CTA: start studying.
 */
export function NotStudiedEmpty() {
  const t = useTranslations("Stats");
  return (
    <div className="flex flex-col items-center rounded-lg border border-dashed border-border p-8 text-center">
      <TrendingUp aria-hidden="true" className="h-8 w-8 text-muted-foreground" />
      <h2 className="mt-2 text-xl font-semibold">{t("emptyNotStudied")}</h2>
      <p className="mx-auto mt-2 max-w-md text-sm text-muted-foreground">
        {t("emptyNotStudiedBody")}
      </p>
      <div className="mt-6 flex justify-center">
        <Link href="/cardgroups" className={buttonVariants({ variant: "brand" })}>
          {t("emptyStartStudying")}
        </Link>
      </div>
    </div>
  );
}

/**
 * No repeatedly-forgotten cards. Calm, no CTA — a reassuring `CircleCheck` rather
 * than a celebratory emoji (coral-minimal restraint).
 */
export function StrugglingEmpty() {
  const t = useTranslations("Stats");
  return (
    <div className="flex flex-col items-center justify-center rounded-lg border border-dashed border-border p-8 text-center">
      <CircleCheck aria-hidden="true" className="h-8 w-8 text-muted-foreground" />
      <h2 className="mt-2 text-sm font-semibold">{t("strugglingEmpty")}</h2>
      <p className="mx-auto mt-1 max-w-xs text-xs text-muted-foreground">
        {t("strugglingEmptyBody")}
      </p>
    </div>
  );
}

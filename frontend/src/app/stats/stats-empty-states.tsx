"use client";

import { CircleCheck } from "lucide-react";
import Link from "next/link";
import { useTranslations } from "next-intl";
import { buttonVariants } from "@/components/ui/button";

/**
 * The three `/stats` empty states, all in the dashed-border `AllCaughtUp` idiom.
 * Each carries at most one filled `brand` CTA (the emphasis ladder). Copy flows
 * through `useTranslations("Stats")`.
 */

/**
 * Brand-new user (no decks, nothing studied). Two CTAs: browse the catalog
 * (primary) or create a deck (secondary).
 */
export function WelcomeEmpty() {
  const t = useTranslations("Stats");
  return (
    <div className="rounded-lg border border-dashed border-border p-8 text-center">
      <p aria-hidden="true" className="text-4xl">
        🌱
      </p>
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
 * Has decks but no reviews yet. One CTA: start studying.
 */
export function NotStudiedEmpty() {
  const t = useTranslations("Stats");
  return (
    <div className="rounded-lg border border-dashed border-border p-8 text-center">
      <p aria-hidden="true" className="text-4xl">
        📈
      </p>
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

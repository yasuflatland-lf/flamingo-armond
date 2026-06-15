"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useTranslations } from "next-intl";
import { type CSSProperties, useCallback, useState } from "react";
import { CatalogCard } from "@/app/catalog/catalog-card";
import type { CatalogCardFieldsFragment } from "@/app/catalog/queries";
import { useImportMaster } from "@/app/catalog/use-import-master";
import { FlamingoMark } from "@/components/brand/flamingo-mark";
import { Button } from "@/components/ui/button";
import { ErrorBanner } from "@/components/ui/error-banner";
import type { FragmentType } from "@/generated/fragment-masking";

// `id` is read at this level (React keys, per-cardgroup `importing` state); the
// rest of the fields travel as a masked `CatalogCardFields` ref that `CatalogCard`
// unmasks — the same fragment the /catalog gallery feeds it.
type MasterDeckNode = { id: string } & FragmentType<typeof CatalogCardFieldsFragment>;

interface OnboardingStartClientProps {
  decks: MasterDeckNode[];
}

/**
 * First-run chooser for a deckless, just-onboarded user. Centered hero: the real
 * FlamingoMark, the heading + subline, then a content-hugging brand-tint panel of
 * preset cardgroups (the hero path), and a subtle "create your own" link. Single
 * selection — each card imports that preset via `useImportMaster` and, on success,
 * navigates to `/learn/{id}`. Imports are serialized to one at a time via
 * `importingId`.
 */
export function OnboardingStartClient({ decks }: OnboardingStartClientProps) {
  const t = useTranslations("OnboardingStart");
  const router = useRouter();
  const { importMasterCardgroup } = useImportMaster();

  const [importingId, setImportingId] = useState<string | null>(null);
  const [importError, setImportError] = useState<string | null>(null);
  const [importAuthError, setImportAuthError] = useState<"unauthenticated" | "forbidden" | null>(
    null,
  );

  const handleStart = useCallback(
    async (id: string) => {
      // Serialize: ignore a second click while another import is in flight.
      if (importingId !== null) return;
      setImportError(null);
      setImportAuthError(null);
      setImportingId(id);

      const outcome = await importMasterCardgroup(id);

      switch (outcome.status) {
        case "success":
          // Leave importingId set so the buttons stay disabled through the
          // navigation that unmounts this component.
          router.push(`/learn/${outcome.cardgroupId}`);
          return;
        case "not_found":
          setImportError(t("importNotFound"));
          setImportingId(null);
          return;
        case "auth":
          setImportAuthError(outcome.kind);
          setImportingId(null);
          return;
        case "rejected":
          setImportError(t("importError"));
          setImportingId(null);
          return;
      }
    },
    [importingId, importMasterCardgroup, router, t],
  );

  const hasBanner = importAuthError !== null || importError !== null;

  return (
    <div className="mx-auto max-w-2xl px-6 py-12 text-center sm:py-16">
      <FlamingoMark aria-hidden="true" className="mx-auto size-12" />
      <h1 className="mt-4 text-3xl font-semibold leading-[1.35] tracking-normal sm:text-4xl">
        {t("heading")}
      </h1>
      <p className="mt-3 text-base leading-[1.7] tracking-[0.01em] text-muted-foreground sm:text-[17px]">
        {t("subline")}
      </p>

      {hasBanner && (
        <div className="mt-6 space-y-6 text-left">
          {importAuthError ? (
            <ErrorBanner data-testid="onboarding-import-auth-error">
              <span>{t("sessionExpired")}</span>
              <Link href="/login" className="underline">
                {t("signInAgain")}
              </Link>
            </ErrorBanner>
          ) : null}
          {importError ? (
            <ErrorBanner data-testid="onboarding-import-error">{importError}</ErrorBanner>
          ) : null}
        </div>
      )}

      <section className="mt-11" aria-labelledby="onboarding-preset-title">
        <div className="mx-auto w-fit max-w-full rounded-2xl border border-brand-tint-border bg-brand-tint p-6 text-left">
          <h2
            id="onboarding-preset-title"
            className="text-xs font-semibold uppercase tracking-[0.08em] text-brand-tint-foreground"
          >
            {t("presetLabel")}
          </h2>
          <ul
            className="mt-4 flex flex-wrap justify-center gap-4 sm:gap-5"
            data-testid="onboarding-deck-list"
          >
            {decks.map((node, index) => (
              <CatalogCard
                key={node.id}
                node={node}
                importing={importingId === node.id}
                imported={false}
                onImport={handleStart}
                labels={{
                  action: t("startWithDeck"),
                  inProgress: t("starting"),
                  done: t("imported"),
                }}
                testIdPrefix="onboarding-deck"
                className="w-[17rem] motion-safe:animate-in motion-safe:fade-in motion-safe:slide-in-from-bottom-2 motion-safe:duration-500"
                style={{ animationDelay: `${Math.min(index, 5) * 70}ms` } satisfies CSSProperties}
              />
            ))}
          </ul>
        </div>
      </section>

      <p className="mt-8 text-sm text-muted-foreground">
        {t("or")}{" "}
        <Button asChild variant="link" className="h-auto p-0 align-baseline text-sm font-medium">
          <Link href="/cardgroups/new?welcome=1" data-testid="onboarding-create-link">
            {t("createCta")} →
          </Link>
        </Button>
      </p>
    </div>
  );
}

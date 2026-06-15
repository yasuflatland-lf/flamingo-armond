"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useTranslations } from "next-intl";
import { useCallback, useState } from "react";
import { CatalogCard } from "@/app/catalog/catalog-card";
import type { CatalogCardFieldsFragment } from "@/app/catalog/queries";
import { useImportMaster } from "@/app/catalog/use-import-master";
import { BrandSplash } from "@/components/pwa/brand-splash";
import { ErrorBanner } from "@/components/ui/error-banner";
import type { FragmentType } from "@/generated/fragment-masking";

// `id` is read at this level (React keys, per-deck `importing` state); the rest
// of the card's fields travel as a masked `CatalogCardFields` ref that
// `CatalogCard` unmasks — the same fragment the /catalog gallery feeds it.
type MasterDeckNode = { id: string } & FragmentType<typeof CatalogCardFieldsFragment>;

interface OnboardingStartClientProps {
  decks: MasterDeckNode[];
}

/**
 * Onboarding chooser for a deckless, just-onboarded user. Presents two
 * first-class paths: import a ready-made master deck (primary) or create an
 * empty cardgroup (secondary). Single selection — each card's button imports
 * that deck via `useImportMaster` and, on success, navigates straight to
 * `/learn/{id}`. Imports are serialized to one at a time via `importingId`.
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

  return (
    <>
      {/* Full-viewport coral splash held from the moment a deck import starts
          until this component unmounts on the success navigation to /learn/{id}.
          On any failure outcome, handleStart resets importingId, tearing the
          splash down so the error banner below becomes visible. Rendered outside
          the layout container so it never participates in the `space-y-8` flow. */}
      {importingId !== null ? (
        <BrandSplash label={t("starting")}>
          <p className="text-sm opacity-80">{t("starting")}</p>
        </BrandSplash>
      ) : null}

      <div className="mx-auto max-w-3xl space-y-8">
        <h1 className="text-2xl font-semibold">{t("heading")}</h1>

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

        <section className="space-y-3" aria-labelledby="onboarding-catalog-title">
          <h2 id="onboarding-catalog-title" className="text-lg font-medium">
            {t("catalogTitle")}
          </h2>
          <p className="text-sm text-muted-foreground">{t("catalogDescription")}</p>
          <ul
            className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3"
            data-testid="onboarding-deck-list"
          >
            {decks.map((node) => (
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
              />
            ))}
          </ul>
        </section>

        <div
          className="flex items-center gap-3 text-xs uppercase text-muted-foreground"
          aria-hidden="true"
        >
          <span className="h-px flex-1 bg-border" />
          {t("or")}
          <span className="h-px flex-1 bg-border" />
        </div>

        <section
          className="space-y-3 rounded-lg border border-border p-4"
          aria-labelledby="onboarding-create-title"
        >
          <h2 id="onboarding-create-title" className="text-lg font-medium">
            {t("createTitle")}
          </h2>
          <p className="text-sm text-muted-foreground">{t("createDescription")}</p>
          <Link
            href="/cardgroups/new?welcome=1"
            className="inline-flex items-center rounded-md bg-primary px-4 py-2 text-sm font-medium text-primary-foreground"
            data-testid="onboarding-create-link"
          >
            {t("createCta")}
          </Link>
        </section>
      </div>
    </>
  );
}

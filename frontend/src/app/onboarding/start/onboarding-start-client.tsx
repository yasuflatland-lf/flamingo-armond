"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useTranslations } from "next-intl";
import { type CSSProperties, useCallback, useState } from "react";
import { CatalogCard } from "@/app/catalog/catalog-card";
import type { CatalogCardFieldsFragment } from "@/app/catalog/queries";
import { useImportMaster } from "@/app/catalog/use-import-master";
import { OnboardingShell } from "@/components/onboarding/onboarding-shell";
import { BrandSplash } from "@/components/pwa/brand-splash";
import { Button } from "@/components/ui/button";
import { ErrorBanner } from "@/components/ui/error-banner";
import type { FragmentType } from "@/generated/fragment-masking";
import { useSeedDefaultStarters } from "./use-seed-default-starters";

// `id` is read at this level (React keys, per-cardgroup `importing` state); the
// rest of the fields travel as a masked `CatalogCardFields` ref that `CatalogCard`
// unmasks — the same fragment the /catalog gallery feeds it.
type MasterCardgroupNode = { id: string } & FragmentType<typeof CatalogCardFieldsFragment>;

interface OnboardingStartClientProps {
  cardgroups: MasterCardgroupNode[];
}

/**
 * First-run chooser for a just-onboarded user with no cardgroup yet. Centered hero: the real
 * FlamingoMark, the heading + subline, then a content-hugging brand-tint panel of
 * preset cardgroups (the hero path), and a subtle "create your own" link. Single
 * selection — each card imports that preset via `useImportMaster` and, on success,
 * navigates to `/learn/{id}`. Imports are serialized to one at a time via
 * `importingId`.
 */
export function OnboardingStartClient({ cardgroups }: OnboardingStartClientProps) {
  const t = useTranslations("OnboardingStart");
  const router = useRouter();
  const { importMasterCardgroup } = useImportMaster();
  const { seedDefaultStarters } = useSeedDefaultStarters();

  const [importingId, setImportingId] = useState<string | null>(null);
  const [seeding, setSeeding] = useState(false);
  const [importError, setImportError] = useState<string | null>(null);
  const [importAuthError, setImportAuthError] = useState<"unauthenticated" | "forbidden" | null>(
    null,
  );

  const handleStart = useCallback(
    async (id: string) => {
      // Serialize: ignore a second click while another import or a seed is in flight.
      if (importingId !== null || seeding) return;
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
    [importingId, seeding, importMasterCardgroup, router, t],
  );

  const handleStartWithDefaults = useCallback(async () => {
    // Serialize: ignore if a preset import or another seed is in flight.
    if (importingId !== null || seeding) return;
    setImportError(null);
    setImportAuthError(null);
    setSeeding(true);

    const outcome = await seedDefaultStarters();

    switch (outcome.status) {
      case "success":
        // Leave seeding true — the navigation will unmount this component.
        router.push("/cardgroups");
        return;
      case "auth":
        setImportAuthError(outcome.kind);
        setSeeding(false);
        return;
      case "rejected":
        setImportError(t("seedError"));
        setSeeding(false);
        return;
    }
  }, [importingId, seeding, seedDefaultStarters, router, t]);

  const hasBanner = importAuthError !== null || importError !== null;

  return (
    <>
      {/* Full-viewport coral splash held from the moment a cardgroup import or
          default-deck seed starts until this component unmounts on the success
          navigation. On any failure outcome, the handler resets importingId /
          seeding, tearing the splash down so the error banner becomes visible.
          Rendered outside the hero container so the fixed overlay never
          participates in its flow. */}
      {importingId !== null || seeding ? (
        <BrandSplash label={t("starting")}>
          <p className="text-sm opacity-80">{t("starting")}</p>
        </BrandSplash>
      ) : null}

      <OnboardingShell heading={t("heading")} subline={t("subline")}>
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
              {cardgroups.map((node, index) => (
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
          <Button
            variant="link"
            className="h-auto p-0 align-baseline text-sm font-medium"
            onClick={handleStartWithDefaults}
            disabled={importingId !== null || seeding}
            data-testid="onboarding-start-defaults"
          >
            {t("startWithDefaults")} →
          </Button>
        </p>
      </OnboardingShell>
    </>
  );
}

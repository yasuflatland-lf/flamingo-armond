"use client";

import { useMutation } from "@apollo/client/react";
import { useTranslations } from "next-intl";
import { useState } from "react";
import { UpdateLearnDisplayModeMutation } from "@/app/learn/queries";
import { ErrorBanner } from "@/components/ui/error-banner";
import type { LearnDisplayMode } from "@/generated/graphql";
import { mutationAuthBanner } from "@/lib/apollo/errors";
import { liftGraphQLCodes } from "@/lib/apollo/graphql-errors";
import { cn } from "@/lib/utils";

const MODES: readonly LearnDisplayMode[] = ["FLIP_TO_REVEAL", "ALWAYS_VISIBLE"];

/**
 * Learn display-mode toggle on `/profile`: lets the signed-in user choose
 * whether the back of a flashcard is hidden until tapped ("Flip to reveal",
 * the default) or shown alongside the front from the start ("Always visible").
 *
 * Each option writes immediately via `updateLearnDisplayMode`; the chosen value
 * is reflected optimistically in local state so the segmented control updates
 * without waiting for the round-trip. No Apollo `optimisticResponse` is used:
 * the mutation can return UNAUTHENTICATED and Apollo v3 does not reliably roll
 * back optimistic writes on typed GraphQL errors. Apollo normalizes the
 * returned `User` by `id`, so no manual cache update is needed.
 */
export function DisplayModeSection({ initialMode }: { initialMode: LearnDisplayMode }) {
  const t = useTranslations("Profile");
  const tCommon = useTranslations("Common");
  const [mode, setMode] = useState<LearnDisplayMode>(initialMode);
  // Surfaced when a save fails, mirroring ProfileForm's banner approach on this
  // page: UNAUTHENTICATED maps to the session-expired copy, FORBIDDEN to the
  // permission-denied copy, anything else to the generic "something went wrong"
  // message. Cleared at the start of each new selection.
  const [saveError, setSaveError] = useState<string | null>(null);
  const [updateMode, { loading }] = useMutation(UpdateLearnDisplayModeMutation);

  const label: Record<LearnDisplayMode, string> = {
    FLIP_TO_REVEAL: t("flipToReveal"),
    ALWAYS_VISIBLE: t("alwaysVisible"),
  };

  async function handleSelect(next: LearnDisplayMode) {
    if (next === mode || loading) return;
    const previous = mode;
    setSaveError(null);
    setMode(next);
    try {
      await updateMode({ variables: { mode: next } });
    } catch (err) {
      // Roll the segmented control back to the previously committed mode so the
      // UI never shows a value the server rejected.
      setMode(previous);
      setSaveError(
        mutationAuthBanner(err, {
          forbidden: tCommon("forbidden"),
          unauthenticated: t("sessionExpired"),
          fallback: tCommon("somethingWentWrong"),
        }),
      );
      console.warn("[profile] updateLearnDisplayMode rejected", { codes: liftGraphQLCodes(err) });
    }
  }

  return (
    <fieldset className="flex flex-col gap-2 border-0 p-0">
      <legend className="mb-2 text-sm font-medium leading-none">{t("displayMode")}</legend>
      <div className="inline-flex w-full max-w-xs rounded-md border p-1 sm:w-auto">
        {MODES.map((option) => {
          const active = option === mode;
          return (
            <button
              key={option}
              type="button"
              data-testid={`display-mode-${option === "FLIP_TO_REVEAL" ? "flip-to-reveal" : "always-visible"}`}
              aria-pressed={active}
              disabled={loading}
              onClick={() => handleSelect(option)}
              className={cn(
                "flex-1 rounded-sm px-3 py-1.5 text-sm font-medium transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring disabled:opacity-50",
                active
                  ? "bg-brand-primary text-brand-primary-foreground"
                  : "text-muted-foreground hover:bg-muted",
              )}
            >
              {label[option]}
            </button>
          );
        })}
      </div>
      {saveError ? <ErrorBanner data-testid="display-mode-error">{saveError}</ErrorBanner> : null}
    </fieldset>
  );
}

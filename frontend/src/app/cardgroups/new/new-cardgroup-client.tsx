"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useTranslations } from "next-intl";
import { useCallback, useState } from "react";
import { useCreateCardgroup } from "@/app/cardgroups/use-create-cardgroup";
import { FlamingoMark } from "@/components/brand/flamingo-mark";
import { CardgroupForm } from "@/components/cardgroups/cardgroup-form";
import { ErrorBanner } from "@/components/ui/error-banner";
import { useSheetForm } from "@/lib/forms/use-sheet-form";

interface NewCardgroupClientProps {
  showWelcome?: boolean;
  /** Sanitized internal path to return to after creation, or null for the default redirect. */
  returnTo: string | null;
}

export function NewCardgroupClient({ showWelcome = false, returnTo }: NewCardgroupClientProps) {
  const router = useRouter();
  const t = useTranslations("Cardgroups");
  const tCommon = useTranslations("Common");

  // Semantically distinct from validationError: this surfaces a degraded
  // "Something went wrong" banner when the server returns a __typename the
  // client was not regenerated against, or null payload from a partial-
  // response null bubble.
  const [unexpectedPayloadError, setUnexpectedPayloadError] = useState<string | null>(null);

  // Server-enforced cardgroup limit reached. Rendered as a banner so the
  // user knows creation is blocked without a field-level error indicator.
  const [limitError, setLimitError] = useState<string | null>(null);

  // Mid-session auth failures. Cleared on each new submission attempt so a
  // retry after re-login does not show a stale banner.
  // Per .claude/rules/frontend-rsc-error-handling.md § "Mid-session
  // UNAUTHENTICATED in a client component: degraded banner with
  // <Link href="/login">, not redirect()".
  const [authError, setAuthError] = useState<"unauthenticated" | "forbidden" | null>(null);

  const { create, loading } = useCreateCardgroup();

  // Full-page create: success navigates away (parameterized by the returned
  // cardgroup id), so the `success` arm stays in the switch below. `useSheetForm`
  // owns only the field-level `validationError` + its clear-on-submit reset;
  // there is no sheet to close, so `onSuccess` is a no-op.
  const onSuccess = useCallback(() => undefined, []);
  const { validationError, run } = useSheetForm(onSuccess);

  async function handleSubmit(values: { name: string }) {
    setUnexpectedPayloadError(null);
    setAuthError(null);
    setLimitError(null);

    const outcome = await run(() => create(values.name));

    switch (outcome.status) {
      case "validation":
        // `run` cleared and stored the field-level error.
        return;
      case "auth":
        setAuthError(outcome.kind);
        return;
      case "limit":
        setLimitError(t("limitReached", { limit: outcome.limit, current: outcome.current }));
        return;
      case "unexpected":
        setUnexpectedPayloadError(tCommon("somethingWentWrong"));
        return;
      case "rejected":
        // Transport/network failure: the hook already warned. Stay silent (no
        // banner) to preserve the established full-page behavior.
        return;
      case "success":
        if (returnTo) {
          const sep = returnTo.includes("?") ? "&" : "?";
          router.push(`${returnTo}${sep}cardgroup=${outcome.cardgroupId}`);
          return;
        }
        router.push(`/cardgroups/${outcome.cardgroupId}`);
        router.refresh();
        return;
    }
  }

  return (
    <main className="p-8">
      {showWelcome ? (
        <section
          aria-labelledby="welcome-heading"
          className="mb-8 rounded-2xl border border-brand-tint-border bg-brand-tint p-6 sm:p-8"
        >
          <div className="flex items-start gap-4">
            <FlamingoMark aria-hidden="true" className="size-10 shrink-0" />
            <div>
              <h1 id="welcome-heading" className="text-2xl font-semibold tracking-tight">
                {t("welcomeHeading")}
              </h1>
              <p className="mt-2 text-sm text-muted-foreground">{t("welcomeDesc")}</p>
            </div>
          </div>
        </section>
      ) : (
        <h1 className="mb-6 text-2xl font-semibold" data-testid="new-cardgroup-page-heading">
          {t("newCardgroup")}
        </h1>
      )}

      {authError ? (
        <ErrorBanner className="mb-4" data-testid="cardgroup-new-auth-error">
          <span>{authError === "unauthenticated" ? t("sessionExpired") : t("noPermission")}</span>
          <Link href="/login" className="underline">
            {t("signInAgain")}
          </Link>
          .
        </ErrorBanner>
      ) : null}

      {validationError ? (
        <ErrorBanner className="mb-4" data-testid="cardgroup-new-validation-error">
          {validationError.message}
        </ErrorBanner>
      ) : null}

      {limitError ? (
        <ErrorBanner className="mb-4" data-testid="cardgroup-new-limit-error">
          {limitError}
        </ErrorBanner>
      ) : null}

      {unexpectedPayloadError ? (
        <ErrorBanner className="mb-4" data-testid="cardgroup-new-unexpected-payload-error">
          {unexpectedPayloadError}
        </ErrorBanner>
      ) : null}

      <CardgroupForm
        mode="create"
        defaultValues={{ name: "" }}
        submit={handleSubmit}
        submitting={loading}
        validationError={validationError}
      />
    </main>
  );
}

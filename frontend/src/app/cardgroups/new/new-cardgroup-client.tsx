"use client";

import { Sparkles } from "lucide-react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useTranslations } from "next-intl";
import { useState } from "react";
import { useCreateCardgroup } from "@/app/cardgroups/use-create-cardgroup";
import { CardgroupForm } from "@/components/cardgroups/cardgroup-form";

interface NewCardgroupClientProps {
  showWelcome?: boolean;
  /** Sanitized internal path to return to after creation, or null for the default redirect. */
  returnTo: string | null;
}

export function NewCardgroupClient({ showWelcome = false, returnTo }: NewCardgroupClientProps) {
  const router = useRouter();
  const t = useTranslations("Cardgroups");
  const tCommon = useTranslations("Common");

  // Typed InputValidationError variant — field-level validation failure
  // surfaced by the server. Cleared on each new submission attempt.
  const [validationError, setValidationError] = useState<{
    field: string;
    message: string;
  } | null>(null);

  // Semantically distinct from validationError: this surfaces a degraded
  // "Something went wrong" banner when the server returns a __typename the
  // client was not regenerated against, or null payload from a partial-
  // response null bubble.
  const [unexpectedPayloadError, setUnexpectedPayloadError] = useState<string | null>(null);

  // Mid-session auth failures. Cleared on each new submission attempt so a
  // retry after re-login does not show a stale banner.
  // Per .claude/rules/frontend-rsc-error-handling.md § "Mid-session
  // UNAUTHENTICATED in a client component: degraded banner with
  // <Link href="/login">, not redirect()".
  const [authError, setAuthError] = useState<"unauthenticated" | "forbidden" | null>(null);

  const { create, loading } = useCreateCardgroup();

  async function handleSubmit(values: { name: string }) {
    setValidationError(null);
    setUnexpectedPayloadError(null);
    setAuthError(null);

    const outcome = await create(values.name);

    switch (outcome.status) {
      case "validation":
        setValidationError({ field: outcome.field, message: outcome.message });
        return;
      case "auth":
        setAuthError(outcome.kind);
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
            <Sparkles aria-hidden className="mt-1 h-6 w-6 shrink-0 text-brand-primary" />
            <div>
              <h1 id="welcome-heading" className="text-lg font-semibold tracking-tight">
                {t("welcomeHeading")}
              </h1>
              <p className="mt-1 text-sm text-muted-foreground">{t("welcomeDesc")}</p>
            </div>
          </div>
        </section>
      ) : (
        <h1 className="mb-6 text-2xl font-semibold">{t("newCardgroup")}</h1>
      )}

      {authError ? (
        <div
          role="alert"
          data-testid="cardgroup-new-auth-error"
          className="mb-4 rounded-md bg-destructive/10 p-3 text-sm text-destructive"
        >
          <span>{authError === "unauthenticated" ? t("sessionExpired") : t("noPermission")}</span>
          <Link href="/login" className="underline">
            {t("signInAgain")}
          </Link>
          .
        </div>
      ) : null}

      {validationError ? (
        <div
          role="alert"
          data-testid="cardgroup-new-validation-error"
          className="mb-4 rounded-md bg-destructive/10 p-3 text-sm text-destructive"
        >
          {validationError.message}
        </div>
      ) : null}

      {unexpectedPayloadError ? (
        <div
          role="alert"
          data-testid="cardgroup-new-unexpected-payload-error"
          className="mb-4 rounded-md bg-destructive/10 p-3 text-sm text-destructive"
        >
          {unexpectedPayloadError}
        </div>
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

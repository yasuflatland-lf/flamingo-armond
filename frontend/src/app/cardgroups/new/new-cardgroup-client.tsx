"use client";

import { useRouter } from "next/navigation";
import { useTranslations } from "next-intl";
import { useCreateCardgroupForm } from "@/app/cardgroups/use-create-cardgroup-form";
import { FlamingoMark } from "@/components/brand/flamingo-mark";
import { CardgroupForm } from "@/components/cardgroups/cardgroup-form";
import { AuthErrorBanner } from "@/components/ui/auth-error-banner";
import { ErrorBanner } from "@/components/ui/error-banner";

interface NewCardgroupClientProps {
  showWelcome?: boolean;
  /** Sanitized internal path to return to after creation, or null for the default redirect. */
  returnTo: string | null;
}

export function NewCardgroupClient({ showWelcome = false, returnTo }: NewCardgroupClientProps) {
  const router = useRouter();
  const t = useTranslations("Cardgroups");
  const tCommon = useTranslations("Common");

  // The hook owns the create mutation, outcome routing, and the structured
  // error state (validation / auth / limit / unexpected). Auth failures are
  // surfaced as a degraded banner with a <Link href="/login"> rather than a
  // redirect — see .claude/rules/frontend-rsc-error-handling.md § "Mid-session
  // UNAUTHENTICATED in a client component".
  const { submit, loading, validationError, authError, limitError, unexpectedError } =
    useCreateCardgroupForm();

  // Server-enforced cardgroup limit reached. Rendered as a banner so the user
  // knows creation is blocked without a field-level error indicator.
  const formattedLimitError = limitError
    ? t("limitReached", { limit: limitError.limit, current: limitError.current })
    : null;

  // Degraded "Something went wrong" banner when the server returns a __typename
  // the client was not regenerated against, or a null payload from a partial-
  // response null bubble. A transport rejection ("rejected") stays silent — the
  // IO hook already warned — to preserve the established full-page behavior.
  const unexpectedPayloadError =
    unexpectedError === "unexpected" ? tCommon("somethingWentWrong") : null;

  async function handleSubmit(values: { name: string }) {
    const outcome = await submit(values);

    if (outcome.status === "success") {
      if (returnTo) {
        const sep = returnTo.includes("?") ? "&" : "?";
        router.push(`${returnTo}${sep}cardgroup=${outcome.cardgroupId}`);
        return;
      }
      router.push(`/cardgroups/${outcome.cardgroupId}`);
      router.refresh();
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
        <AuthErrorBanner
          className="mb-4"
          testId="cardgroup-new-auth-error"
          message={authError === "unauthenticated" ? t("sessionExpired") : t("noPermission")}
          signInLabel={t("signInAgain")}
        />
      ) : null}

      {validationError ? (
        <ErrorBanner className="mb-4" data-testid="cardgroup-new-validation-error">
          {validationError.message}
        </ErrorBanner>
      ) : null}

      {formattedLimitError ? (
        <ErrorBanner className="mb-4" data-testid="cardgroup-new-limit-error">
          {formattedLimitError}
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

"use client";

import { useForm } from "@tanstack/react-form";
import { useRouter } from "next/navigation";
import { useTranslations } from "next-intl";
import { useState } from "react";
import { useUpdateProfile } from "@/app/profile/use-update-profile";
import { OnboardingShell } from "@/components/onboarding/onboarding-shell";
import { Button } from "@/components/ui/button";
import { ErrorBanner } from "@/components/ui/error-banner";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { FieldError } from "@/lib/forms/field-error";
import { submitFormHandler } from "@/lib/forms/submit-handler";
import { updateProfileSchema } from "@/schemas/profile";

export function OnboardingForm() {
  const router = useRouter();
  const t = useTranslations("Onboarding");
  const tCommon = useTranslations("Common");

  // Typed InputValidationError variant — field-level validation failure
  // surfaced by the server via the outcome union. Cleared on each new submission.
  const [validationError, setValidationError] = useState<{
    field: string;
    message: string;
  } | null>(null);

  // Mid-session auth failures or unexpected payloads. Cleared on each submission.
  const [bannerMessage, setBannerMessage] = useState<string | null>(null);

  // Held true from a successful save until this component unmounts on the
  // `router.push("/onboarding/start")` navigation. Apollo's `loading` flips back
  // to false the instant the mutation resolves — before the App Router
  // navigation (RSC fetch + transition) completes — so gating the button on
  // `loading` alone makes it flicker from "Saving…" back to an enabled
  // "Continue" while the form is still mounted. Mirrors how OnboardingStartClient
  // holds `importingId` through its unmounting navigation. Set only on the
  // success path; every failure branch leaves it false so the user can resubmit.
  const [navigating, setNavigating] = useState(false);

  const { submit, loading } = useUpdateProfile();

  const displayNameSchema = updateProfileSchema.shape.displayName;

  const form = useForm({
    defaultValues: {
      displayName: "",
    },
    onSubmit: async ({ value }) => {
      setValidationError(null);
      setBannerMessage(null);

      const outcome = await submit({
        displayName: value.displayName,
      });

      switch (outcome.status) {
        case "success":
          // Keep the button in its "Saving…"/disabled state through the
          // navigation that unmounts this component, so it does not flicker back
          // to "Continue" once Apollo's `loading` clears.
          setNavigating(true);
          router.push("/onboarding/start");
          return;
        case "validation":
          setValidationError({ field: outcome.field, message: outcome.message });
          return;
        case "unauthenticated":
          setBannerMessage(t("sessionExpired"));
          return;
        case "unexpected":
          setBannerMessage(tCommon("somethingWentWrong"));
          return;
        case "rejected":
          setBannerMessage(outcome.banner ?? tCommon("somethingWentWrong"));
          // Re-throw so TanStack Form keeps formState.isSubmitSuccessful=false;
          // submitFormHandler (the outer onSubmit) swallows the re-thrown rejection.
          throw new Error("[OnboardingForm] update profile rejected");
      }
    },
  });

  // Derive per-field backend errors from the validationError state (outcome union path).
  const fieldErrors: Record<string, string | undefined> = validationError
    ? { [validationError.field]: validationError.message }
    : {};

  return (
    <OnboardingShell heading={t("welcome")} subline={t("subtitle")}>
      <form
        onSubmit={submitFormHandler(form)}
        // Centered card inside the shell's centered column. Internals stay
        // text-left so the label / input / hint read as a normal form.
        className="mx-auto mt-8 w-full max-w-[420px] space-y-4 rounded-2xl border border-border/70 bg-card p-6 text-left shadow-[0_1px_2px_rgba(0,0,0,0.04),0_12px_32px_-12px_rgba(0,0,0,0.12)] sm:p-8"
      >
        {bannerMessage ? <ErrorBanner>{bannerMessage}</ErrorBanner> : null}

        <form.Field
          name="displayName"
          validators={{ onChange: displayNameSchema, onBlur: displayNameSchema }}
        >
          {(field) => (
            <div className="space-y-2">
              <Label htmlFor={field.name}>{t("displayName")}</Label>
              <Input
                id={field.name}
                name={field.name}
                value={field.state.value}
                onBlur={field.handleBlur}
                onChange={(e) => field.handleChange(e.target.value)}
              />
              <p className="text-sm text-muted-foreground">{t("displayNameHint")}</p>
              <FieldError
                zodErrors={field.state.meta.errors}
                backendError={fieldErrors.displayName}
              />
            </div>
          )}
        </form.Field>

        <Button
          type="submit"
          variant="brand"
          disabled={loading || navigating}
          data-testid="onboarding-submit"
          className="w-full"
        >
          {loading || navigating ? tCommon("saving") : t("continue")}
        </Button>

        {/* Hidden sentinel used by tests to observe formState.isSubmitSuccessful */}
        <form.Subscribe selector={(state) => state.isSubmitSuccessful}>
          {(isSubmitSuccessful) => (
            <span
              data-testid="is-submit-successful"
              data-value={String(isSubmitSuccessful)}
              hidden
            />
          )}
        </form.Subscribe>
      </form>
    </OnboardingShell>
  );
}

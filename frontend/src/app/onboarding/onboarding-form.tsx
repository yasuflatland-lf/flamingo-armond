"use client";

import { useForm } from "@tanstack/react-form";
import { useRouter } from "next/navigation";
import { useTranslations } from "next-intl";
import { useState } from "react";
import { useUpdateProfile } from "@/app/profile/use-update-profile";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { FieldError } from "@/lib/forms/field-error";
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
          // Re-throw so TanStack Form keeps formState.isSubmitSuccessful=false; the
          // outer form.handleSubmit().catch() at the JSX call site swallows it.
          throw new Error("[OnboardingForm] update profile rejected");
      }
    },
  });

  // Derive per-field backend errors from the validationError state (outcome union path).
  const fieldErrors: Record<string, string | undefined> = validationError
    ? { [validationError.field]: validationError.message }
    : {};

  return (
    <form
      onSubmit={(e) => {
        e.preventDefault();
        e.stopPropagation();
        form.handleSubmit().catch(() => {
          // The inner submit handler's .catch already logged; swallow here so the
          // re-thrown rejection (which keeps formState.isSubmitSuccessful=false correct)
          // does not surface as an unhandled browser promise rejection.
        });
      }}
      className="space-y-4"
    >
      <h1 className="mb-6 text-2xl font-semibold">{t("welcome")}</h1>

      {bannerMessage ? (
        <div className="rounded-md bg-destructive/10 p-3 text-sm text-destructive" role="alert">
          {bannerMessage}
        </div>
      ) : null}

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

      <Button type="submit" variant="brand" disabled={loading} data-testid="onboarding-submit">
        {loading ? tCommon("saving") : t("continue")}
      </Button>

      {/* Hidden sentinel used by tests to observe formState.isSubmitSuccessful */}
      <form.Subscribe selector={(state) => state.isSubmitSuccessful}>
        {(isSubmitSuccessful) => (
          <span data-testid="is-submit-successful" data-value={String(isSubmitSuccessful)} hidden />
        )}
      </form.Subscribe>
    </form>
  );
}

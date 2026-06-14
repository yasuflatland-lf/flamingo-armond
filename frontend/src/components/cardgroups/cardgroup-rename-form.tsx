"use client";

import { useMutation } from "@apollo/client/react";
import { useRouter } from "next/navigation";
import { useTranslations } from "next-intl";
import { useEffect, useState } from "react";
import { UpdateCardgroupMutation } from "@/app/cardgroups/queries";
import { CardgroupForm } from "@/components/cardgroups/cardgroup-form";
import { getBackendErrorBanner } from "@/lib/apollo/errors";
import { liftGraphQLCodes } from "@/lib/apollo/graphql-errors";

type Props = {
  cardgroup: { id: string; name: string };
  onSaved: () => void;
  onSubmittingChange?: (submitting: boolean) => void;
};

export function CardgroupRenameForm({ cardgroup, onSaved, onSubmittingChange }: Props) {
  const router = useRouter();
  const t = useTranslations("Cardgroups");
  const tCommon = useTranslations("Common");

  // Typed InputValidationError variant — field-level validation failure
  // surfaced by the server via the outcome union. Cleared on each new submission.
  const [validationError, setValidationError] = useState<{
    field: string;
    message: string;
  } | null>(null);

  // Network / auth / unexpected-payload banner. Cleared on each new submission.
  const [bannerMessage, setBannerMessage] = useState<string | null>(null);

  const [updateCardgroup, { loading: updating }] = useMutation(UpdateCardgroupMutation);

  useEffect(() => {
    onSubmittingChange?.(updating);
  }, [onSubmittingChange, updating]);

  useEffect(
    () => () => {
      onSubmittingChange?.(false);
    },
    [onSubmittingChange],
  );

  async function handleSave(values: { name: string }) {
    setValidationError(null);
    setBannerMessage(null);

    const result = await updateCardgroup({
      variables: { id: cardgroup.id, input: { name: values.name } },
    }).catch((err) => {
      console.error("[CardgroupRenameForm] update rejection", {
        name: err instanceof Error ? err.name : "unknown",
        cardgroupId: cardgroup.id,
      });
      const codes = liftGraphQLCodes(err);
      if (codes.includes("UNAUTHENTICATED")) {
        setBannerMessage(t("sessionExpired"));
        return null;
      }
      const banner = getBackendErrorBanner(err) ?? tCommon("somethingWentWrong");
      setBannerMessage(banner);
      return null;
    });

    if (!result) return;

    const payload = result.data?.updateCardgroup;

    if (payload?.__typename === "InputValidationError") {
      setValidationError({ field: payload.field, message: payload.message });
      return;
    }

    if (payload?.__typename === "UpdateCardgroupSuccess") {
      onSaved();
      // Refresh the RSC tree so the h1 and Badge reflect the new name immediately.
      router.refresh();
      return;
    }

    // Unknown variant: null payload or a future union variant the client was not
    // regenerated against.
    const unknownPayload = payload as unknown as { __typename?: string } | null | undefined;
    console.warn("[CardgroupRenameForm] unexpected updateCardgroup payload", {
      typename: unknownPayload?.__typename ?? null,
    });
    setBannerMessage(tCommon("somethingWentWrong"));
  }

  return (
    <div className="space-y-4">
      {bannerMessage ? (
        <div className="rounded-md bg-destructive/10 p-3 text-sm text-destructive" role="alert">
          {bannerMessage}
        </div>
      ) : null}
      <CardgroupForm
        mode="edit"
        defaultValues={{ name: cardgroup.name }}
        submit={handleSave}
        submitting={updating}
        validationError={validationError}
      />
    </div>
  );
}

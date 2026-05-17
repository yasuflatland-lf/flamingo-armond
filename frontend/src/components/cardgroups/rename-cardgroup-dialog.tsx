"use client";

import { useMutation } from "@apollo/client/react";
import { useRouter } from "next/navigation";
import { useState } from "react";
import { UpdateCardgroupMutation } from "@/app/cardgroups/queries";
import { CardgroupForm } from "@/components/cardgroups/cardgroup-form";
import { Dialog, DialogContent, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { getBackendErrorBanner } from "@/lib/apollo/errors";
import { liftGraphQLCodes } from "@/lib/apollo/graphql-errors";

type Props = {
  cardgroup: { id: string; name: string };
  open: boolean;
  onOpenChange: (open: boolean) => void;
};

export function RenameCardgroupDialog({ cardgroup, open, onOpenChange }: Props) {
  const router = useRouter();

  // Typed InputValidationError variant — field-level validation failure
  // surfaced by the server via the outcome union. Cleared on each new submission.
  const [validationError, setValidationError] = useState<{
    field: string;
    message: string;
  } | null>(null);

  // Network / auth / unexpected-payload banner. Cleared on each new submission.
  const [bannerMessage, setBannerMessage] = useState<string | null>(null);

  const [updateCardgroup, { loading: updating }] = useMutation(UpdateCardgroupMutation);

  async function handleSave(values: { name: string }) {
    setValidationError(null);
    setBannerMessage(null);

    const result = await updateCardgroup({
      variables: { id: cardgroup.id, input: { name: values.name } },
    }).catch((err) => {
      console.error("[RenameCardgroupDialog] update rejection", err);
      const codes = liftGraphQLCodes(err);
      if (codes.includes("UNAUTHENTICATED")) {
        setBannerMessage("Your session expired. Please sign in again.");
        return null;
      }
      const banner = getBackendErrorBanner(err) ?? "Something went wrong. Please try again.";
      setBannerMessage(banner);
      return null;
    });

    if (!result) return;

    const payload = result.data?.updateCardgroup;
    const typename = payload?.__typename ?? null;

    if (typename === "InputValidationError" && payload?.__typename === "InputValidationError") {
      setValidationError({ field: payload.field, message: payload.message });
      return;
    }

    if (typename === "UpdateCardgroupSuccess") {
      onOpenChange(false);
      // Refresh the RSC tree so the h1 and Badge reflect the new name immediately.
      router.refresh();
      return;
    }

    // Unknown variant: null payload or a future union variant the client was not
    // regenerated against.
    console.warn("[RenameCardgroupDialog] unexpected updateCardgroup payload", { typename });
    setBannerMessage("Something went wrong. Please try again.");
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Rename cardgroup</DialogTitle>
        </DialogHeader>
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
      </DialogContent>
    </Dialog>
  );
}

"use client";

import { useMutation } from "@apollo/client/react";
import { useRouter } from "next/navigation";
import { UpdateCardgroupMutation } from "@/app/cardgroups/queries";
import { CardgroupForm } from "@/components/cardgroups/cardgroup-form";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";

type Props = {
  cardgroup: { id: string; name: string };
  open: boolean;
  onOpenChange: (open: boolean) => void;
};

export function RenameCardgroupDialog({ cardgroup, open, onOpenChange }: Props) {
  const router = useRouter();

  const [updateCardgroup, { loading: updating, error: updateError }] =
    useMutation(UpdateCardgroupMutation);

  async function handleSave(values: { name: string }) {
    const result = await updateCardgroup({
      variables: { id: cardgroup.id, input: { name: values.name } },
    }).catch((err) => {
      console.error("[RenameCardgroupDialog] update rejection", err);
      return null;
    });

    if (result?.data?.updateCardgroup?.cardgroup) {
      onOpenChange(false);
      // Refresh the RSC tree so the h1 and Badge reflect the new name immediately.
      router.refresh();
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Rename cardgroup</DialogTitle>
        </DialogHeader>
        <CardgroupForm
          mode="edit"
          defaultValues={{ name: cardgroup.name }}
          submit={handleSave}
          submitting={updating}
          error={updateError}
        />
      </DialogContent>
    </Dialog>
  );
}

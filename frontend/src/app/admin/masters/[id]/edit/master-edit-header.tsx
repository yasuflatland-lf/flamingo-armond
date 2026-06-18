"use client";

import { MoreHorizontal } from "lucide-react";
import { useRouter } from "next/navigation";
import { useTranslations } from "next-intl";
import { useCallback, useState } from "react";
import { toast } from "sonner";
import { AdminMasterForm, type MasterFormValues } from "@/app/admin/masters/admin-master-form";
import { type AuthKind, useMasterMutations } from "@/app/admin/masters/use-master-mutations";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog";
import { Badge } from "@/components/ui/badge";
import { Button, buttonVariants } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { FormSheet } from "@/components/ui/form-sheet";
import type { AdminMasterDeck } from "./queries";

type Props = { master: AdminMasterDeck; cardCount: number };

export function MasterEditHeader({ master, cardCount }: Props) {
  const t = useTranslations("AdminMasters");
  const tCommon = useTranslations("Common");
  const router = useRouter();

  const [settingsOpen, setSettingsOpen] = useState(false);
  const [settingsDirty, setSettingsDirty] = useState(false);
  const [validationError, setValidationError] = useState<{ field: string; message: string } | null>(
    null,
  );
  const [deleteOpen, setDeleteOpen] = useState(false);
  const [deleting, setDeleting] = useState(false);
  const [publishing, setPublishing] = useState(false);

  const { updateMaster, deleteMaster, publishMaster, unpublishMaster, updating } =
    useMasterMutations();

  const published = master.status === "PUBLISHED";
  const emptyDraft = master.status === "DRAFT" && cardCount === 0;

  const authToast = useCallback(
    (kind: AuthKind) => {
      toast.error(kind === "forbidden" ? t("forbidden") : t("unauthenticated"));
    },
    [t],
  );

  const handleUpdate = useCallback(
    async (values: MasterFormValues) => {
      setValidationError(null);
      const outcome = await updateMaster(master.id, values);
      switch (outcome.status) {
        case "validation":
          setValidationError({ field: outcome.field, message: outcome.message });
          return;
        case "success":
          toast.success(t("updateSuccess"));
          setSettingsDirty(false);
          setSettingsOpen(false);
          router.refresh();
          return;
        case "auth":
          authToast(outcome.kind);
          return;
        default:
          toast.error(t("unexpectedError"));
      }
    },
    [master.id, updateMaster, t, authToast, router],
  );

  const handlePublishToggle = useCallback(async () => {
    setPublishing(true);
    try {
      if (published) {
        const outcome = await unpublishMaster(master.id);
        switch (outcome.status) {
          case "success":
            toast.success(t("unpublishSuccess"));
            router.refresh();
            return;
          case "auth":
            authToast(outcome.kind);
            return;
          default:
            toast.error(t("unexpectedError"));
            return;
        }
      }
      const outcome = await publishMaster(master.id);
      switch (outcome.status) {
        case "success":
          toast.success(t("publishSuccess"));
          router.refresh();
          return;
        case "empty":
          toast.error(t("publishEmptyToast"));
          return;
        case "auth":
          authToast(outcome.kind);
          return;
        default:
          toast.error(t("unexpectedError"));
      }
    } finally {
      setPublishing(false);
    }
  }, [published, publishMaster, unpublishMaster, master.id, t, authToast, router]);

  const handleConfirmDelete = useCallback(async () => {
    setDeleting(true);
    try {
      const outcome = await deleteMaster(master.id);
      switch (outcome.status) {
        case "success":
          toast.success(t("deleteMasterSuccess"));
          setDeleteOpen(false);
          router.push("/admin/masters");
          return;
        case "auth":
          authToast(outcome.kind);
          return;
        default:
          toast.error(t("deleteMasterFailed"));
      }
    } finally {
      setDeleting(false);
    }
  }, [deleteMaster, master.id, t, authToast, router]);

  return (
    <>
      <div className="mb-6 flex flex-wrap items-start justify-between gap-3">
        <div className="min-w-0 flex-1">
          <h1 className="text-2xl font-semibold leading-tight break-words">{master.name}</h1>
          <div className="mt-1 flex items-center gap-2">
            <Badge
              variant={published ? "default" : "secondary"}
              role="status"
              data-testid="master-edit-status-badge"
            >
              {published ? t("statusPublished") : t("statusDraft")}
            </Badge>
            <span className="text-sm text-muted-foreground">
              {t("cardCount", { count: cardCount })}
            </span>
          </div>
          {emptyDraft ? (
            <p className="mt-1 text-xs text-muted-foreground" data-testid="master-edit-empty-hint">
              {t("publishEmptyHint")}
            </p>
          ) : null}
        </div>

        <div className="flex shrink-0 items-center gap-2">
          <Button
            type="button"
            variant={published ? "outline" : "brand"}
            data-testid="master-edit-publish"
            aria-pressed={published}
            disabled={publishing || emptyDraft}
            onClick={handlePublishToggle}
          >
            {publishing ? tCommon("loading") : published ? t("unpublish") : t("publish")}
          </Button>

          <Button
            type="button"
            variant="outline"
            className="hidden sm:inline-flex"
            data-testid="master-edit-deck-settings"
            onClick={() => setSettingsOpen(true)}
          >
            {t("deckSettingsButton")}
          </Button>

          <Button
            type="button"
            variant="destructive"
            className="hidden sm:inline-flex"
            data-testid="master-edit-delete"
            onClick={() => setDeleteOpen(true)}
          >
            {t("deleteMasterButton")}
          </Button>

          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <Button
                type="button"
                variant="ghost"
                size="icon"
                className="sm:hidden"
                data-testid="master-edit-overflow"
                aria-label={t("masterOptions")}
              >
                <MoreHorizontal className="h-4 w-4" />
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end">
              <DropdownMenuItem onSelect={() => setSettingsOpen(true)}>
                {t("deckSettingsButton")}
              </DropdownMenuItem>
              <DropdownMenuItem
                onSelect={() => setDeleteOpen(true)}
                className="text-destructive focus:text-destructive"
              >
                {t("deleteMasterButton")}
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
        </div>
      </div>

      <FormSheet
        title={t("deckSettingsTitle")}
        open={settingsOpen}
        onOpenChange={(next) => {
          if (!next) {
            setValidationError(null);
            setSettingsDirty(false);
            setSettingsOpen(false);
          }
        }}
        submitting={updating}
        dirty={settingsDirty}
        confirmOnDismiss
      >
        {settingsOpen ? (
          <AdminMasterForm
            mode="edit"
            master={master}
            submitting={updating}
            submit={handleUpdate}
            validationError={validationError}
            onDirtyChange={setSettingsDirty}
          />
        ) : null}
      </FormSheet>

      <AlertDialog open={deleteOpen} onOpenChange={setDeleteOpen}>
        <AlertDialogContent data-testid="master-delete-dialog">
          <AlertDialogHeader>
            <AlertDialogTitle>{t("deleteMasterDialogTitle")}</AlertDialogTitle>
            <AlertDialogDescription>{t("deleteMasterDialogDescription")}</AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel data-testid="master-delete-dialog-cancel" disabled={deleting}>
              {tCommon("cancel")}
            </AlertDialogCancel>
            <AlertDialogAction
              className={buttonVariants({ variant: "destructive" })}
              data-testid="master-delete-dialog-confirm"
              disabled={deleting}
              onClick={(e) => {
                e.preventDefault();
                void handleConfirmDelete();
              }}
            >
              {t("deleteMasterConfirmButton")}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  );
}

"use client";

import { Eye, EyeOff, MoreHorizontal, Settings, Trash2 } from "lucide-react";
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
import { SplitButtonMenu } from "@/components/ui/split-button-menu";
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

  const publishIcon = published ? (
    <EyeOff aria-hidden="true" className="h-4 w-4" />
  ) : (
    <Eye aria-hidden="true" className="h-4 w-4" />
  );
  const publishLabel = published ? t("unpublish") : t("publish");

  return (
    <>
      {/* Mobile: the title takes the full line width and the actions collapse
          into the meta row (kebab) / move below. Desktop: title left, the
          publish split button on the right. */}
      <div className="mb-6 flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
        <div className="min-w-0 sm:flex-1">
          <h1 className="text-2xl font-semibold leading-tight break-words">{master.name}</h1>
          <div className="mt-1 flex items-center justify-between gap-2">
            <div className="flex items-center gap-2">
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

            {/* Mobile management menu: publish toggle + deck settings + delete. */}
            <DropdownMenu>
              <DropdownMenuTrigger asChild>
                <Button
                  type="button"
                  variant="ghost"
                  size="icon"
                  className="shrink-0 sm:hidden"
                  data-testid="master-edit-overflow"
                  aria-label={t("masterOptions")}
                >
                  <MoreHorizontal aria-hidden="true" className="h-4 w-4" />
                </Button>
              </DropdownMenuTrigger>
              <DropdownMenuContent align="end">
                <DropdownMenuItem
                  onSelect={handlePublishToggle}
                  disabled={publishing || emptyDraft}
                  data-testid="master-edit-publish-mobile"
                  className="gap-2"
                >
                  {publishIcon}
                  {publishLabel}
                </DropdownMenuItem>
                <DropdownMenuItem
                  onSelect={() => setSettingsOpen(true)}
                  data-testid="master-edit-deck-settings-mobile"
                  className="gap-2"
                >
                  <Settings aria-hidden="true" className="h-4 w-4" />
                  {t("deckSettingsButton")}
                </DropdownMenuItem>
                <DropdownMenuItem
                  onSelect={() => setDeleteOpen(true)}
                  data-testid="master-edit-delete-mobile"
                  className="gap-2 text-destructive focus:text-destructive"
                >
                  <Trash2 aria-hidden="true" className="h-4 w-4" />
                  {t("deleteMasterButton")}
                </DropdownMenuItem>
              </DropdownMenuContent>
            </DropdownMenu>
          </div>
          {emptyDraft ? (
            <p className="mt-1 text-xs text-muted-foreground" data-testid="master-edit-empty-hint">
              {t("publishEmptyHint")}
            </p>
          ) : null}
        </div>

        {/* Desktop split button: publish primary + dropdown (deck settings, delete). */}
        <div className="hidden shrink-0 items-center sm:flex">
          <Button
            type="button"
            variant={published ? "outline" : "brand"}
            className="rounded-r-none"
            data-testid="master-edit-publish"
            aria-pressed={published}
            disabled={publishing || emptyDraft}
            onClick={handlePublishToggle}
          >
            {publishIcon}
            {publishing ? tCommon("loading") : publishLabel}
          </Button>
          <SplitButtonMenu
            triggerLabel={t("masterOptions")}
            variant={published ? "outline" : "brand"}
            size="default"
            data-testid="master-edit-more-options"
            items={[
              {
                key: "settings",
                icon: <Settings aria-hidden="true" className="h-4 w-4" />,
                label: t("deckSettingsButton"),
                onSelect: () => setSettingsOpen(true),
                "data-testid": "master-edit-deck-settings",
              },
              {
                key: "delete",
                icon: <Trash2 aria-hidden="true" className="h-4 w-4" />,
                label: t("deleteMasterButton"),
                onSelect: () => setDeleteOpen(true),
                destructive: true,
                "data-testid": "master-edit-delete",
              },
            ]}
          />
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

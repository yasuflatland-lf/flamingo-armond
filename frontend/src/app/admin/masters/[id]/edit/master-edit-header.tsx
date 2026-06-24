"use client";

import { ChevronDown, Eye, EyeOff, Import, MoreHorizontal, Settings, Trash2 } from "lucide-react";
import { useRouter } from "next/navigation";
import { useTranslations } from "next-intl";
import { useCallback, useState } from "react";
import { toast } from "sonner";
import { AdminMasterForm, type MasterFormValues } from "@/app/admin/masters/admin-master-form";
import { MasterStatusBadge } from "@/app/admin/masters/master-status-badge";
import { type AuthKind, useMasterMutations } from "@/app/admin/masters/use-master-mutations";
import { DetailPageHeader } from "@/components/nav/detail-page-header";
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
import { Button, buttonVariants } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { FormSheet } from "@/components/ui/form-sheet";
import { SplitButtonMenu } from "@/components/ui/split-button-menu";
import { cn } from "@/lib/utils";
import type { AdminMasterDeck } from "./queries";

type Props = {
  master: AdminMasterDeck;
  cardCount: number;
  /**
   * Opens the batch-import sheet (owned by `MasterCardsClient`). Surfaced here
   * so the mobile overflow menu can host batch import, where the standalone
   * toolbar button was removed. Required so the wire from `MasterCardsClient`
   * is enforced at compile time rather than silently defaulting to a no-op.
   */
  onBatchImport: () => void;
};

export function MasterEditHeader({ master, cardCount, onBatchImport }: Props) {
  const t = useTranslations("AdminMasters");
  const tCommon = useTranslations("Common");
  const tCardgroups = useTranslations("Cardgroups");
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
  const statusLabel = published ? t("statusPublished") : t("statusDraft");

  // App-bar meta: the muted card count only. The publish-status indicator moved
  // out to the title-row `status` slot below, so the count is breakpoint-agnostic
  // and renders once.
  const meta = (
    <span className="text-sm text-muted-foreground">{t("cardCount", { count: cardCount })}</span>
  );

  // Status indicator centered directly below the deck title, reading as an
  // attribute of it. Mobile gets the interactive chip (the only publish
  // affordance there); desktop gets the read-only badge, since the publish
  // action lives in the app-bar split button.
  const status = (
    <>
      {/* Mobile: interactive status chip doubles as the publish trigger. */}
      <div className="md:hidden">
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <button
              type="button"
              data-testid="master-edit-status-chip"
              aria-label={t("changePublishState", { state: statusLabel })}
              className="inline-flex min-h-9 items-center gap-1.5 rounded-full border px-3 py-1 text-sm font-medium hover:bg-accent focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2"
            >
              <span
                aria-hidden="true"
                className={cn(
                  "h-2 w-2 rounded-full",
                  published ? "bg-success" : "bg-muted-foreground",
                )}
              />
              {statusLabel}
              <ChevronDown aria-hidden="true" className="h-3.5 w-3.5 text-muted-foreground" />
            </button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="center">
            <DropdownMenuItem
              onSelect={handlePublishToggle}
              disabled={publishing || emptyDraft}
              data-testid="master-edit-publish-mobile"
              className="gap-2"
            >
              {publishIcon}
              {publishLabel}
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      </div>
      {/* Desktop: read-only display badge; the publish action lives in the
          app-bar split button. */}
      <MasterStatusBadge
        published={published}
        data-testid="master-edit-status-badge"
        className="hidden md:inline-flex"
      />
    </>
  );

  const actions = (
    <>
      {/* Mobile overflow: deck-lifecycle only (settings + delete). */}
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <Button
            type="button"
            variant="ghost"
            size="icon"
            className="shrink-0 md:hidden"
            data-testid="master-edit-overflow"
            aria-label={t("masterOptions")}
          >
            <MoreHorizontal aria-hidden="true" className="h-4 w-4" />
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="end">
          {/* Batch import lives here on mobile after the standalone toolbar
              button was removed. The trigger is md:hidden, so this menu only
              opens on mobile; desktop keeps batch import in the cards toolbar's
              split-button menu. */}
          <DropdownMenuItem
            onSelect={onBatchImport}
            data-testid="master-edit-import-mobile"
            className="gap-2"
          >
            <Import aria-hidden="true" className="h-4 w-4" />
            {tCardgroups("batchImport")}
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

      {/* Desktop split button: publish primary + settings/delete dropdown. */}
      <div className="hidden shrink-0 items-center md:flex">
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
    </>
  );

  return (
    <>
      <DetailPageHeader
        backHref="/admin/masters"
        backLabel={t("backToList")}
        title={master.name}
        meta={meta}
        actions={actions}
        status={status}
      >
        {emptyDraft ? (
          <p
            className="mt-1 text-center text-xs text-muted-foreground"
            data-testid="master-edit-empty-hint"
          >
            {t("publishEmptyHint")}
          </p>
        ) : null}
      </DetailPageHeader>

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

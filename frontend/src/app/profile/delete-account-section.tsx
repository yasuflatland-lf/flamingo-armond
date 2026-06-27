"use client";

import { useMutation } from "@apollo/client/react";
import { Trash2 } from "lucide-react";
import { useRouter } from "next/navigation";
import { useTranslations } from "next-intl";
import { useState } from "react";
import {
  AlertDialog,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogTrigger,
} from "@/components/ui/alert-dialog";
import { Button } from "@/components/ui/button";
import { ErrorBanner } from "@/components/ui/error-banner";
import { mutationAuthBanner } from "@/lib/apollo/errors";
import { liftGraphQLCodes } from "@/lib/apollo/graphql-errors";
import { createSupabaseBrowserClient } from "@/lib/supabase/client";
import { DeleteMyAccountMutation } from "./mutations";

/**
 * Danger zone on `/profile`: lets the signed-in user permanently delete their
 * own account. A type-to-confirm input gates the irreversible action. On
 * success the local Supabase session is cleared and the user is redirected to
 * `/login`.
 */
export function DeleteAccountSection() {
  const t = useTranslations("Profile");
  const tCommon = useTranslations("Common");
  const router = useRouter();
  const [open, setOpen] = useState(false);
  const [confirmInput, setConfirmInput] = useState("");
  const [deleting, setDeleting] = useState(false);
  const [deleteError, setDeleteError] = useState("");
  const [runDelete] = useMutation(DeleteMyAccountMutation);

  const confirmPhrase = t("deleteAccountConfirmPhrase");
  const confirmed = confirmInput.trim() === confirmPhrase;

  function handleOpenChange(next: boolean) {
    setOpen(next);
    if (!next) {
      setConfirmInput("");
      setDeleteError("");
    }
  }

  async function handleConfirmDelete() {
    if (!confirmed || deleting) return;
    setDeleteError("");
    setDeleting(true);
    try {
      const result = await runDelete();
      if (!result.data?.deleteMyAccount) {
        setDeleteError(t("deleteAccountFailed"));
        setDeleting(false);
        return;
      }
      // The account no longer exists; clear the Supabase session and leave the
      // app. A signOut failure is non-fatal — the account is gone regardless, so
      // log it and redirect anyway (the middleware treats the next request as
      // anonymous once the cookie is cleared or the user row is missing).
      const supabase = createSupabaseBrowserClient();
      const { error } = await supabase.auth.signOut();
      if (error) {
        // Redact error.message — Supabase auth errors can carry user-identifying
        // content (see .claude/rules/frontend-rsc-error-handling.md). Log the
        // stable error name only.
        console.warn("[profile] signOut after deleteMyAccount failed", {
          name: error instanceof Error ? error.name : "unknown",
        });
      }
      router.replace("/login");
      router.refresh();
      // No further state updates — this component unmounts on navigation.
    } catch (err) {
      console.warn("[profile] deleteMyAccount rejected", { codes: liftGraphQLCodes(err) });
      setDeleteError(
        mutationAuthBanner(err, {
          forbidden: t("deleteAccountForbidden"),
          unauthenticated: t("deleteAccountSessionExpired"),
          fallback: t("deleteAccountFailed"),
        }),
      );
      setDeleting(false);
    }
  }

  return (
    <section
      className="space-y-4 border-t border-destructive/30 pt-6"
      aria-labelledby="profile-danger-zone-heading"
    >
      <div>
        <h2 id="profile-danger-zone-heading" className="text-xl font-semibold text-destructive">
          {t("dangerZoneHeading")}
        </h2>
        <p className="text-sm text-muted-foreground">{t("dangerZoneDescription")}</p>
      </div>

      <AlertDialog open={open} onOpenChange={handleOpenChange}>
        <AlertDialogTrigger asChild>
          <Button type="button" variant="destructiveGhost" data-testid="delete-account-trigger">
            <Trash2 aria-hidden="true" className="h-4 w-4" />
            {t("deleteAccountButton")}
          </Button>
        </AlertDialogTrigger>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t("deleteAccountDialogTitle")}</AlertDialogTitle>
            <AlertDialogDescription>{t("deleteAccountDialogDescription")}</AlertDialogDescription>
          </AlertDialogHeader>

          <div className="space-y-2">
            <label htmlFor="delete-account-confirm" className="block text-sm">
              {t("deleteAccountTypeToConfirm", { phrase: confirmPhrase })}
            </label>
            <input
              id="delete-account-confirm"
              type="text"
              value={confirmInput}
              onChange={(event) => setConfirmInput(event.target.value)}
              placeholder={confirmPhrase}
              autoComplete="off"
              className="w-full rounded-md border border-input bg-background px-3 py-2 text-sm focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
              data-testid="delete-account-confirm-input"
              aria-label={t("deleteAccountConfirmInputLabel")}
            />
          </div>

          {deleteError && (
            <ErrorBanner data-testid="delete-account-error">{deleteError}</ErrorBanner>
          )}

          <AlertDialogFooter>
            <AlertDialogCancel disabled={deleting}>{tCommon("cancel")}</AlertDialogCancel>
            {/*
              A plain destructive Button (not AlertDialogAction) drives the
              confirm so the dialog stays open during the async mutation and while
              a FORBIDDEN (last-admin) error is shown.
            */}
            <Button
              type="button"
              variant="destructive"
              onClick={handleConfirmDelete}
              disabled={!confirmed || deleting}
              data-testid="delete-account-confirm"
            >
              {deleting ? t("deleteAccountDeleting") : t("deleteAccountConfirmButton")}
            </Button>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </section>
  );
}

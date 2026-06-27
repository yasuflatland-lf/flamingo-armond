"use client";

import { useMutation } from "@apollo/client/react";
import { useForm, useStore } from "@tanstack/react-form";
import { Trash2 } from "lucide-react";
import { useTranslations } from "next-intl";
import { useEffect, useMemo, useRef, useState } from "react";
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
import { FormSheet, useFormSheetClose } from "@/components/ui/form-sheet";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import { mutationAuthBanner } from "@/lib/apollo/errors";
import { liftGraphQLCodes } from "@/lib/apollo/graphql-errors";
import { FieldError } from "@/lib/forms/field-error";
import { submitFormHandler } from "@/lib/forms/submit-handler";
import { updateProfileSchema } from "@/schemas/profile";
import type { AdminUserListItem, AdminUserRole } from "./admin-user-row";
import { AdminEditUserMutation } from "./queries";

// The displayName/bio block is validated through the shared profile schema —
// the same source of truth the /profile and onboarding forms use — so the
// admin sheet never duplicates the field rules inline.
const displayNameFieldSchema = updateProfileSchema.shape.displayName;
const bioFieldSchema = updateProfileSchema.shape.bio;

/**
 * Values owned by the TanStack form (the profile block). Roles stage separately.
 * `bio` is `string | undefined` so it matches the optional `bio` field schema's
 * input type — the same shape the /profile form uses.
 */
type ProfileFieldValues = { displayName: string; bio: string | undefined };

/**
 * Thin wrapper around {@link useForm} that pins the value type so the form
 * instance can be passed to {@link AdminUserProfileSheetBody} with a concrete
 * type (TanStack's `useForm` exposes ~12 generics that are awkward to spell out
 * by hand).
 */
function useProfileFieldsForm(opts: {
  defaultValues: ProfileFieldValues;
  onSubmit: (ctx: {
    value: ProfileFieldValues;
    formApi: { state: { isDirty: boolean } };
  }) => Promise<void>;
}) {
  return useForm(opts);
}

type ProfileFieldsForm = ReturnType<typeof useProfileFieldsForm>;

type Props = {
  open: boolean;
  user: AdminUserListItem | null;
  allRoles: AdminUserRole[];
  loading: boolean;
  queryError: string | null;
  onDismiss: () => void;
  onSaved: () => void;
  onReloadRequested?: () => void;
  /**
   * Deletes the user by id. Resolves on success (the parent closes the sheet and
   * surfaces a toast); rejects with a GraphQL error on failure so the danger
   * zone can render the reason. Absent when deletion is not available.
   */
  onDelete?: (id: string) => Promise<void>;
};

function sameSet(a: ReadonlySet<string>, b: ReadonlySet<string>): boolean {
  if (a.size !== b.size) return false;
  for (const value of a) {
    if (!b.has(value)) return false;
  }
  return true;
}

export function AdminUserProfileSheet({
  open,
  user,
  allRoles,
  loading,
  queryError,
  onDismiss,
  onSaved,
  onReloadRequested,
  onDelete,
}: Props) {
  const t = useTranslations("Admin");
  const tCommon = useTranslations("Common");
  const [saveError, setSaveError] = useState("");
  const [stagedRoleIds, setStagedRoleIds] = useState<Set<string>>(() => new Set());
  const [runEdit, { loading: saving, reset: resetEdit }] = useMutation(AdminEditUserMutation);
  // Tracks the id whose data the form last synced to. A ConcurrentUpdateError
  // auto-triggers a parent refetch that re-delivers the *same* user with fresh
  // server values (possibly across several object-identity swaps, since the list
  // refetch and detail reload both rewrite the shared cache entity). We must keep
  // the conflict banner across those same-id reloads and only clear it when the
  // form switches to a *different* user.
  const lastSyncedId = useRef<string | null>(null);

  const form = useProfileFieldsForm({
    defaultValues: {
      displayName: user?.displayName ?? "",
      bio: user?.bio ?? "",
    },
    onSubmit: async ({ value, formApi }) => {
      if (!user) return;

      // Only displayName/bio that actually changed are sent; roles are always
      // sent as the final staged set.
      const profileDirty = formApi.state.isDirty;
      setSaveError("");
      try {
        const result = await runEdit({
          variables: {
            id: user.id,
            expectedVersion: user.version,
            ...(profileDirty
              ? { displayName: value.displayName.trim(), bio: value.bio || null }
              : {}),
            roleIds: Array.from(stagedRoleIds),
          },
        });
        const payload = result.data?.adminEditUser;
        const typename = payload?.__typename ?? null;
        switch (payload?.__typename) {
          case "AdminEditUserSuccess":
            onSaved();
            return;
          case "InputValidationError":
          case "CannotRevokeOwnAdminRoleError":
            setSaveError(payload.message);
            return;
          case "ConcurrentUpdateError":
            // The banner survives the same-id reload triggered here — see the
            // form-sync effect's last-synced-id guard.
            setSaveError(t("concurrentError"));
            onReloadRequested?.();
            return;
          default:
            console.warn("[admin/users] unexpected save payload", {
              userId: user.id,
              typename,
            });
            setSaveError(tCommon("somethingWentWrong"));
        }
      } catch (err) {
        const codes = liftGraphQLCodes(err);
        console.warn("[admin/users] adminEditUser rejected", {
          userId: user.id,
          name: err instanceof Error ? err.name : "unknown",
          codes,
        });
        setSaveError(
          mutationAuthBanner(err, {
            forbidden: t("forbidden"),
            unauthenticated: t("unauthenticated"),
            fallback: t("unexpectedError"),
          }),
        );
      }
    },
  });

  const profileDirty = useStore(form.store, (state) => state.isDirty);
  const initialRoleIds = useMemo(() => new Set(user?.roles.map((role) => role.id) ?? []), [user]);
  const rolesDirty = !sameSet(stagedRoleIds, initialRoleIds);
  const dirty = profileDirty || rolesDirty;

  useEffect(() => {
    if (!user) {
      // Reset on close so reopening the *same* user is treated as a fresh sync
      // (id differs from null) and clears any leftover banner.
      lastSyncedId.current = null;
      return;
    }
    form.reset({ displayName: user.displayName ?? "", bio: user.bio ?? "" });
    setStagedRoleIds(new Set(user.roles.map((role) => role.id)));
    // Only clear the banner when switching to a different user. A same-id reload
    // (the concurrent-update refresh) keeps the conflict banner visible.
    if (lastSyncedId.current !== user.id) {
      setSaveError("");
    }
    lastSyncedId.current = user.id;
    resetEdit();
  }, [user, resetEdit, form]);

  function handleOpenChange(nextOpen: boolean) {
    if (nextOpen) return;
    setSaveError("");
    resetEdit();
    onDismiss();
  }

  function toggleRole(roleId: string): void {
    setStagedRoleIds((current) => {
      const next = new Set(current);
      if (next.has(roleId)) {
        next.delete(roleId);
      } else {
        next.add(roleId);
      }
      return next;
    });
    setSaveError("");
  }

  return (
    <FormSheet
      open={open}
      onOpenChange={handleOpenChange}
      title={t("editUserTitle")}
      size="md"
      submitting={saving}
      dirty={dirty}
      confirmOnDismiss
    >
      <AdminUserProfileSheetBody
        allRoles={allRoles}
        form={form}
        loading={loading}
        open={open}
        queryError={queryError}
        saveError={saveError}
        saving={saving}
        stagedRoleIds={stagedRoleIds}
        user={user}
        onRoleToggle={toggleRole}
        onDelete={onDelete}
      />
    </FormSheet>
  );
}

function AdminUserProfileSheetBody({
  allRoles,
  form,
  loading,
  open,
  queryError,
  saveError,
  saving,
  stagedRoleIds,
  user,
  onRoleToggle,
  onDelete,
}: {
  allRoles: AdminUserRole[];
  form: ProfileFieldsForm;
  loading: boolean;
  open: boolean;
  queryError: string | null;
  saveError: string;
  saving: boolean;
  stagedRoleIds: ReadonlySet<string>;
  user: AdminUserListItem | null;
  onRoleToggle: (roleId: string) => void;
  onDelete?: (id: string) => Promise<void>;
}) {
  const t = useTranslations("Admin");
  const tNav = useTranslations("Nav");
  const tCommon = useTranslations("Common");
  const close = useFormSheetClose();
  const [deleteDialogOpen, setDeleteDialogOpen] = useState(false);
  const [deleting, setDeleting] = useState(false);
  const [deleteError, setDeleteError] = useState("");

  async function handleConfirmDelete() {
    if (!user || !onDelete) return;
    setDeleteError("");
    setDeleting(true);
    try {
      await onDelete(user.id);
      // Success: the parent closes the sheet and shows a toast; this component
      // unmounts, so no further state updates are needed here.
    } catch (err) {
      const codes = liftGraphQLCodes(err);
      console.warn("[admin/users] adminDeleteUser rejected", {
        userId: user.id,
        codes,
      });
      setDeleteError(
        mutationAuthBanner(err, {
          forbidden: t("deleteUserForbidden"),
          unauthenticated: t("unauthenticated"),
          fallback: t("deleteUserFailed"),
        }),
      );
      setDeleting(false);
    }
  }

  return (
    <div className="space-y-6">
      {loading && (
        <p className="text-sm text-muted-foreground" data-testid="admin-user-sheet-loading">
          {t("loadingUser")}
        </p>
      )}

      {queryError && <ErrorBanner>{queryError}</ErrorBanner>}

      {open && !loading && !queryError && !user && <ErrorBanner>{t("userNotFound")}</ErrorBanner>}

      {saveError && <ErrorBanner>{saveError}</ErrorBanner>}

      {user && (
        <>
          <form onSubmit={submitFormHandler(form)} className="space-y-6">
            <form.Field
              name="displayName"
              validators={{
                onChange: displayNameFieldSchema,
                onBlur: displayNameFieldSchema,
                onSubmit: displayNameFieldSchema,
              }}
            >
              {(field) => (
                <div className="space-y-2">
                  <Label htmlFor={field.name}>{t("displayNameLabel")}</Label>
                  <Input
                    id={field.name}
                    name={field.name}
                    value={field.state.value}
                    onBlur={field.handleBlur}
                    onChange={(event) => field.handleChange(event.target.value)}
                  />
                  <FieldError zodErrors={field.state.meta.errors} />
                </div>
              )}
            </form.Field>

            <form.Field
              name="bio"
              validators={{
                onChange: bioFieldSchema,
                onBlur: bioFieldSchema,
                onSubmit: bioFieldSchema,
              }}
            >
              {(field) => (
                <div className="space-y-2">
                  <Label htmlFor={field.name}>{t("bioLabel")}</Label>
                  <Textarea
                    id={field.name}
                    name={field.name}
                    value={field.state.value ?? ""}
                    onBlur={field.handleBlur}
                    onChange={(event) => field.handleChange(event.target.value)}
                    rows={4}
                    placeholder={t("bioPlaceholder")}
                  />
                  <FieldError zodErrors={field.state.meta.errors} />
                </div>
              )}
            </form.Field>

            <div className="space-y-3">
              <p className="text-sm font-medium">{tNav("roles")}</p>
              {allRoles.length === 0 ? (
                <p className="text-xs text-muted-foreground">{t("noRolesAvailable")}</p>
              ) : (
                <div className="grid gap-2">
                  {allRoles.map((role) => {
                    const checkboxId = `admin-user-sheet-role-${user.id}-${role.id}`;
                    return (
                      <label
                        key={role.id}
                        htmlFor={checkboxId}
                        className="flex items-center gap-2 text-sm"
                      >
                        <input
                          id={checkboxId}
                          type="checkbox"
                          checked={stagedRoleIds.has(role.id)}
                          onChange={() => onRoleToggle(role.id)}
                          className="h-4 w-4 rounded border-input accent-brand"
                        />
                        <span>{role.name}</span>
                      </label>
                    );
                  })}
                </div>
              )}
            </div>

            <div className="flex items-center gap-2">
              <Button type="submit" variant="brand" disabled={saving}>
                {saving ? tCommon("saving") : t("saveChanges")}
              </Button>
              <Button type="button" variant="outline" onClick={close}>
                {tCommon("cancel")}
              </Button>
            </div>
          </form>

          {onDelete && (
            <div className="space-y-3 border-t border-destructive/30 pt-6">
              <div>
                <h3 className="text-sm font-semibold text-destructive">{t("deleteUserHeading")}</h3>
                <p className="text-xs text-muted-foreground">{t("deleteUserDescription")}</p>
              </div>

              {deleteError && (
                <ErrorBanner data-testid="admin-delete-user-error">{deleteError}</ErrorBanner>
              )}

              <AlertDialog open={deleteDialogOpen} onOpenChange={setDeleteDialogOpen}>
                <AlertDialogTrigger asChild>
                  <Button
                    type="button"
                    variant="destructiveGhost"
                    data-testid="admin-delete-user-trigger"
                  >
                    <Trash2 aria-hidden="true" className="h-4 w-4" />
                    {t("deleteUserButton")}
                  </Button>
                </AlertDialogTrigger>
                <AlertDialogContent>
                  <AlertDialogHeader>
                    <AlertDialogTitle>{t("deleteUserDialogTitle")}</AlertDialogTitle>
                    <AlertDialogDescription>
                      {t("deleteUserDialogDescription")}
                    </AlertDialogDescription>
                  </AlertDialogHeader>
                  <AlertDialogFooter>
                    <AlertDialogCancel disabled={deleting}>{tCommon("cancel")}</AlertDialogCancel>
                    {/*
                      A plain destructive Button (not AlertDialogAction) drives the
                      confirm so the dialog stays open during the async mutation and
                      while a FORBIDDEN error is shown. AlertDialogAction auto-closes
                      the dialog on click, which would hide the in-progress / error UI.
                    */}
                    <Button
                      type="button"
                      variant="destructive"
                      onClick={handleConfirmDelete}
                      disabled={deleting}
                      data-testid="admin-delete-user-confirm"
                    >
                      {deleting ? t("deleteUserDeleting") : t("deleteUserConfirmButton")}
                    </Button>
                  </AlertDialogFooter>
                </AlertDialogContent>
              </AlertDialog>
            </div>
          )}
        </>
      )}
    </div>
  );
}

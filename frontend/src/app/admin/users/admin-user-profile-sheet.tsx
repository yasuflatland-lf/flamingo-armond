"use client";

import { useMutation } from "@apollo/client/react";
import { useTranslations } from "next-intl";
import { useEffect, useMemo, useRef, useState } from "react";
import { Button } from "@/components/ui/button";
import { FormSheet, useFormSheetClose } from "@/components/ui/form-sheet";
import { liftGraphQLCodes } from "@/lib/apollo/graphql-errors";
import type { AdminUserListItem, AdminUserRole } from "./admin-user-row";
import { AdminEditUserMutation } from "./queries";

type Props = {
  open: boolean;
  user: AdminUserListItem | null;
  allRoles: AdminUserRole[];
  loading: boolean;
  queryError: string | null;
  onDismiss: () => void;
  onSaved: () => void;
  onReloadRequested?: () => void;
};

const DISPLAY_NAME_MAX = 50;
const BIO_MAX = 500;

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
}: Props) {
  const t = useTranslations("Admin");
  const tCommon = useTranslations("Common");
  const [displayName, setDisplayName] = useState("");
  const [bio, setBio] = useState("");
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

  const initialRoleIds = useMemo(() => new Set(user?.roles.map((role) => role.id) ?? []), [user]);
  const displayNameDirty = displayName !== (user?.displayName ?? "");
  const bioDirty = bio !== (user?.bio ?? "");
  const profileDirty = displayNameDirty || bioDirty;
  const rolesDirty = !sameSet(stagedRoleIds, initialRoleIds);
  const dirty = profileDirty || rolesDirty;

  useEffect(() => {
    if (!user) {
      // Reset on close so reopening the *same* user is treated as a fresh sync
      // (id differs from null) and clears any leftover banner.
      lastSyncedId.current = null;
      return;
    }
    setDisplayName(user.displayName ?? "");
    setBio(user.bio ?? "");
    setStagedRoleIds(new Set(user.roles.map((role) => role.id)));
    // Only clear the banner when switching to a different user. A same-id reload
    // (the concurrent-update refresh) keeps the conflict banner visible.
    if (lastSyncedId.current !== user.id) {
      setSaveError("");
    }
    lastSyncedId.current = user.id;
    resetEdit();
  }, [user, resetEdit]);

  function clearSaveStatus(): void {
    setSaveError("");
  }

  function handleOpenChange(nextOpen: boolean) {
    if (nextOpen) return;
    setSaveError("");
    resetEdit();
    onDismiss();
  }

  function validate(): string {
    if (!profileDirty) return "";
    const trimmed = displayName.trim();
    if (trimmed.length < 1) return t("displayNameRequired");
    if (trimmed.length > DISPLAY_NAME_MAX) {
      return t("displayNameTooLong", { max: DISPLAY_NAME_MAX });
    }
    if (bio.length > BIO_MAX) return t("bioTooLong", { max: BIO_MAX });
    return "";
  }

  async function handleSave() {
    if (!user) return;

    const validationError = validate();
    if (validationError) {
      setSaveError(validationError);
      return;
    }

    clearSaveStatus();
    try {
      const result = await runEdit({
        variables: {
          id: user.id,
          expectedVersion: user.version,
          ...(profileDirty ? { displayName: displayName.trim(), bio: bio || null } : {}),
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
      const msg = codes.includes("FORBIDDEN")
        ? t("forbidden")
        : codes.includes("UNAUTHENTICATED")
          ? t("unauthenticated")
          : t("unexpectedError");
      setSaveError(msg);
    }
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
    clearSaveStatus();
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
        bio={bio}
        displayName={displayName}
        loading={loading}
        open={open}
        queryError={queryError}
        saveError={saveError}
        saving={saving}
        stagedRoleIds={stagedRoleIds}
        user={user}
        onBioChange={(nextBio) => {
          setBio(nextBio);
          clearSaveStatus();
        }}
        onDisplayNameChange={(nextDisplayName) => {
          setDisplayName(nextDisplayName);
          clearSaveStatus();
        }}
        onRoleToggle={toggleRole}
        onSave={handleSave}
      />
    </FormSheet>
  );
}

function AdminUserProfileSheetBody({
  allRoles,
  bio,
  displayName,
  loading,
  open,
  queryError,
  saveError,
  saving,
  stagedRoleIds,
  user,
  onBioChange,
  onDisplayNameChange,
  onRoleToggle,
  onSave,
}: {
  allRoles: AdminUserRole[];
  bio: string;
  displayName: string;
  loading: boolean;
  open: boolean;
  queryError: string | null;
  saveError: string;
  saving: boolean;
  stagedRoleIds: ReadonlySet<string>;
  user: AdminUserListItem | null;
  onBioChange: (bio: string) => void;
  onDisplayNameChange: (displayName: string) => void;
  onRoleToggle: (roleId: string) => void;
  onSave: () => void;
}) {
  const t = useTranslations("Admin");
  const tNav = useTranslations("Nav");
  const tCommon = useTranslations("Common");
  const close = useFormSheetClose();

  return (
    <div className="space-y-6">
      {loading && (
        <p className="text-sm text-muted-foreground" data-testid="admin-user-sheet-loading">
          {t("loadingUser")}
        </p>
      )}

      {queryError && (
        <div className="rounded-md bg-destructive/10 p-3 text-sm text-destructive" role="alert">
          {queryError}
        </div>
      )}

      {open && !loading && !queryError && !user && (
        <div className="rounded-md bg-destructive/10 p-3 text-sm text-destructive" role="alert">
          {t("userNotFound")}
        </div>
      )}

      {saveError && (
        <div className="rounded-md bg-destructive/10 p-3 text-sm text-destructive" role="alert">
          {saveError}
        </div>
      )}

      {user && (
        <>
          <div className="space-y-2">
            <label htmlFor="admin-user-display-name" className="block text-sm font-medium">
              {t("displayNameLabel")}
            </label>
            <input
              id="admin-user-display-name"
              type="text"
              value={displayName}
              onChange={(event) => {
                onDisplayNameChange(event.target.value);
              }}
              maxLength={DISPLAY_NAME_MAX}
              className="w-full rounded-md border border-input bg-background px-3 py-2 text-sm focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
            />
            <p className="text-xs text-muted-foreground">
              {displayName.trim().length}/{DISPLAY_NAME_MAX}
            </p>
          </div>

          <div className="space-y-2">
            <label htmlFor="admin-user-bio" className="block text-sm font-medium">
              {t("bioLabel")}
            </label>
            <textarea
              id="admin-user-bio"
              value={bio}
              onChange={(event) => {
                onBioChange(event.target.value);
              }}
              rows={4}
              maxLength={BIO_MAX}
              className="w-full rounded-md border border-input bg-background px-3 py-2 text-sm focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
              placeholder={t("bioPlaceholder")}
            />
            <p className="text-xs text-muted-foreground">
              {bio.length}/{BIO_MAX}
            </p>
          </div>

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
            <Button type="button" variant="brand" onClick={onSave} disabled={saving}>
              {saving ? tCommon("saving") : t("saveChanges")}
            </Button>
            <Button type="button" variant="outline" onClick={close}>
              {tCommon("cancel")}
            </Button>
          </div>
        </>
      )}
    </div>
  );
}

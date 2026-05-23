"use client";

import { useMutation } from "@apollo/client/react";
import { useEffect, useState } from "react";
import { Button } from "@/components/ui/button";
import { FormSheet, useFormSheetClose } from "@/components/ui/form-sheet";
import { liftGraphQLCodes } from "@/lib/apollo/graphql-errors";
import type { AdminUserListItem } from "./admin-user-role-row";
import { AdminUpdateUserMutation } from "./queries";

type Props = {
  open: boolean;
  user: AdminUserListItem | null;
  loading: boolean;
  queryError: string | null;
  onDismiss: () => void;
  onSaved: () => void;
};

const DISPLAY_NAME_MAX = 50;
const BIO_MAX = 500;

const ERR_FORBIDDEN = "You do not have permission.";
const ERR_UNAUTHENTICATED = "Your session has expired. Sign in again.";
const ERR_UNEXPECTED = "An unexpected error occurred. Please try again.";
const ERR_SOMETHING_WRONG = "Something went wrong. Please try again.";

/** Pick the user-facing message for a thrown mutation error. */
function pickAuthErrorMessage(codes: readonly string[]): string {
  if (codes.includes("FORBIDDEN")) return ERR_FORBIDDEN;
  if (codes.includes("UNAUTHENTICATED")) return ERR_UNAUTHENTICATED;
  return ERR_UNEXPECTED;
}

export function AdminUserProfileSheet({
  open,
  user,
  loading,
  queryError,
  onDismiss,
  onSaved,
}: Props) {
  const [displayName, setDisplayName] = useState("");
  const [bio, setBio] = useState("");
  const [saveError, setSaveError] = useState("");
  const [runUpdate, { loading: saving, reset: resetUpdate }] = useMutation(AdminUpdateUserMutation);

  useEffect(() => {
    if (!user) return;
    setDisplayName(user.displayName ?? "");
    setBio(user.bio ?? "");
    setSaveError("");
    resetUpdate();
  }, [user, resetUpdate]);

  function clearSaveStatus(): void {
    setSaveError("");
  }

  function handleOpenChange(nextOpen: boolean) {
    if (nextOpen) return;
    setSaveError("");
    resetUpdate();
    onDismiss();
  }

  function validate(): string {
    const trimmed = displayName.trim();
    if (trimmed.length < 1) return "Display name is required.";
    if (trimmed.length > DISPLAY_NAME_MAX) {
      return `Display name must be at most ${DISPLAY_NAME_MAX} characters.`;
    }
    if (bio.length > BIO_MAX) return `Bio must be at most ${BIO_MAX} characters.`;
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
      const result = await runUpdate({
        variables: {
          id: user.id,
          input: { displayName: displayName.trim(), bio: bio || null },
        },
      });
      const payload = result.data?.adminUpdateUser;
      const saveTypename = payload?.__typename ?? null;
      if (payload?.__typename === "InputValidationError") {
        setSaveError(payload.message);
        return;
      }
      if (payload?.__typename === "AdminUpdateUserSuccess") {
        onSaved();
        return;
      }
      console.warn("[admin/users] unexpected save payload", {
        userId: user.id,
        typename: saveTypename,
      });
      setSaveError(ERR_SOMETHING_WRONG);
    } catch (err) {
      const codes = liftGraphQLCodes(err);
      console.warn("[admin/users] adminUpdateUser rejected", {
        userId: user.id,
        name: err instanceof Error ? err.name : "unknown",
        codes,
      });
      setSaveError(pickAuthErrorMessage(codes));
    }
  }

  return (
    <FormSheet
      open={open}
      onOpenChange={handleOpenChange}
      title="Edit user"
      size="md"
      submitting={saving}
      confirmOnDismiss={false}
    >
      <AdminUserProfileSheetBody
        bio={bio}
        displayName={displayName}
        loading={loading}
        queryError={queryError}
        saveError={saveError}
        saving={saving}
        user={user}
        onBioChange={(nextBio) => {
          setBio(nextBio);
          clearSaveStatus();
        }}
        onDisplayNameChange={(nextDisplayName) => {
          setDisplayName(nextDisplayName);
          clearSaveStatus();
        }}
        onSave={handleSave}
      />
    </FormSheet>
  );
}

function AdminUserProfileSheetBody({
  bio,
  displayName,
  loading,
  queryError,
  saveError,
  saving,
  user,
  onBioChange,
  onDisplayNameChange,
  onSave,
}: {
  bio: string;
  displayName: string;
  loading: boolean;
  queryError: string | null;
  saveError: string;
  saving: boolean;
  user: AdminUserListItem | null;
  onBioChange: (bio: string) => void;
  onDisplayNameChange: (displayName: string) => void;
  onSave: () => void;
}) {
  const close = useFormSheetClose();

  return (
    <div className="space-y-6">
      {loading && (
        <p className="text-sm text-muted-foreground" data-testid="admin-user-sheet-loading">
          Loading user...
        </p>
      )}

      {queryError && (
        <div className="rounded-md bg-destructive/10 p-3 text-sm text-destructive" role="alert">
          {queryError}
        </div>
      )}

      {!loading && !queryError && !user && (
        <div className="rounded-md bg-destructive/10 p-3 text-sm text-destructive" role="alert">
          User not found.
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
              Display name
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
              Bio
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
              placeholder="Optional bio"
            />
            <p className="text-xs text-muted-foreground">
              {bio.length}/{BIO_MAX}
            </p>
          </div>

          <div className="flex items-center gap-2">
            <Button type="button" variant="brand" onClick={onSave} disabled={saving}>
              {saving ? "Saving..." : "Save changes"}
            </Button>
            <Button type="button" variant="outline" onClick={close}>
              Cancel
            </Button>
          </div>
        </>
      )}
    </div>
  );
}

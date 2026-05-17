"use client";

import { useMutation } from "@apollo/client/react";
import Link from "next/link";
import { useState } from "react";
import { Button } from "@/components/ui/button";
import type { AdminRoleFieldsFragment } from "@/generated/graphql";
import { liftGraphQLCodes } from "@/lib/apollo/graphql-errors";
import {
  AdminAssignRoleMutation,
  AdminRevokeRoleMutation,
  AdminUpdateUserMutation,
} from "../../queries";

/** Plain serializable types matching the runtime JSON shape from gqlFetch. */
export type UserForEdit = {
  id: string;
  displayName?: string | null;
  bio?: string | null;
  avatarUrl?: string | null;
  roles: Array<{ id: string; name: string }>;
};

export type RoleOption = {
  id: string;
  name: string;
};

/**
 * Props for AdminUserEditClient.
 * Data is always pre-fetched by the server component; no client-side queries.
 */
type Props = {
  /** Pre-fetched user from the server component. */
  user: UserForEdit;
  /** Pre-fetched roles list from the server component. */
  allRoles: RoleOption[];
};

/** Maximum grapheme clusters for displayName (mirrors usecase/user.go displayNameMax). */
const DISPLAY_NAME_MAX = 50;

/** Maximum grapheme clusters for bio (mirrors usecase/user.go bioMax). */
const BIO_MAX = 500;

export function AdminUserEditClient({ user, allRoles }: Props) {
  const [displayName, setDisplayName] = useState(user.displayName ?? "");
  const [bio, setBio] = useState(user.bio ?? "");
  const [saveError, setSaveError] = useState("");
  const [saveBanner, setSaveBanner] = useState("");

  // Per-role error banners (keyed by role id) and in-flight guard so consecutive
  // clicks on the same role cannot double-fire.
  const [roleBanners, setRoleBanners] = useState<Record<string, string>>({});
  const [roleInflight, setRoleInflight] = useState<Record<string, boolean>>({});
  // Server-truth role ids — initialised from SSR props; updated on mutation success.
  const [userRoleIds, setUserRoleIds] = useState<Set<string>>(
    () => new Set(user.roles.map((r) => r.id)),
  );

  const [runUpdate, { loading: saving }] = useMutation(AdminUpdateUserMutation);
  const [runAssign] = useMutation(AdminAssignRoleMutation);
  const [runRevoke] = useMutation(AdminRevokeRoleMutation);

  function clearSaveStatus(): void {
    setSaveError("");
    setSaveBanner("");
  }

  /** Client-side validation mirror of server-side constraints. Returns error or "". */
  function validate(): string {
    const trimmed = displayName.trim();
    if (trimmed.length < 1) return "Display name is required.";
    if (trimmed.length > DISPLAY_NAME_MAX)
      return `Display name must be at most ${DISPLAY_NAME_MAX} characters.`;
    if (bio.length > BIO_MAX) return `Bio must be at most ${BIO_MAX} characters.`;
    return "";
  }

  async function handleSave() {
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
      // Capture typename before narrowing so the unknown-variant branch still
      // has access to it (TypeScript narrows to `never` after the known cases).
      const saveTypename = payload?.__typename ?? null;
      if (payload?.__typename === "InputValidationError") {
        setSaveError(payload.message);
        return;
      }
      if (payload?.__typename === "AdminUpdateUserSuccess") {
        setSaveBanner("Changes saved.");
        return;
      }
      // Unknown variant: null payload, partial-response null bubble, or a
      // future union variant the client was not regenerated against.
      console.warn("[admin/users/:id/edit] unexpected save payload", {
        typename: saveTypename,
      });
      setSaveError("Something went wrong. Please try again.");
    } catch (err) {
      const codes = liftGraphQLCodes(err);
      setSaveError(
        codes.includes("FORBIDDEN")
          ? "You do not have permission."
          : codes.includes("UNAUTHENTICATED")
            ? "Your session has expired. Sign in again."
            : "An unexpected error occurred. Please try again.",
      );
    }
  }

  async function handleRoleToggle(roleId: string, currentlyAssigned: boolean) {
    setRoleInflight((prev) => ({ ...prev, [roleId]: true }));
    setRoleBanners((prev) => ({ ...prev, [roleId]: "" }));
    try {
      const variables = { userId: user.id, roleId };
      const result = currentlyAssigned
        ? (await runRevoke({ variables })).data?.revokeRole
        : (await runAssign({ variables })).data?.assignRole;
      // Capture typename before narrowing so the unknown-variant branch still
      // has access to it (TypeScript narrows to `never` after the known cases).
      const roleTypename = result?.__typename ?? null;
      if (result?.__typename === "InputValidationError") {
        setRoleBanners((prev) => ({ ...prev, [roleId]: result.message }));
        return;
      }
      if (result?.__typename === "CannotRevokeOwnAdminRoleError") {
        setRoleBanners((prev) => ({ ...prev, [roleId]: result.message }));
        return;
      }
      if (
        result?.__typename === "RevokeRoleSuccess" ||
        result?.__typename === "AssignRoleSuccess"
      ) {
        // Fragment masking is compile-time only; cast to read role ids.
        const serverRoles = result.user.roles as AdminRoleFieldsFragment[];
        setUserRoleIds(new Set(serverRoles.map((r) => r.id)));
        return;
      }
      // Unknown variant: null payload / partial-response null bubble / future variant.
      console.warn("[admin/users/:id/edit] unexpected role-toggle payload", {
        typename: roleTypename,
      });
      setRoleBanners((prev) => ({
        ...prev,
        [roleId]: "Something went wrong. Please try again.",
      }));
    } catch (err) {
      const codes = liftGraphQLCodes(err);
      setRoleBanners((prev) => ({
        ...prev,
        [roleId]: codes.includes("FORBIDDEN")
          ? "You do not have permission."
          : codes.includes("UNAUTHENTICATED")
            ? "Your session has expired. Sign in again."
            : "An unexpected error occurred. Please try again.",
      }));
    } finally {
      setRoleInflight((prev) => ({ ...prev, [roleId]: false }));
    }
  }

  return (
    <main className="p-8">
      <h1 className="mb-6 text-2xl font-semibold">Edit User</h1>

      {/* Save success banner */}
      {saveBanner && !saveError && (
        <div className="mb-4 rounded-md bg-green-50 p-3 text-sm text-green-800" role="status">
          {saveBanner}
        </div>
      )}

      {/* Save error banner */}
      {saveError && (
        <div
          className="mb-4 rounded-md bg-destructive/10 p-3 text-sm text-destructive"
          role="alert"
        >
          {saveError}
        </div>
      )}

      <div className="space-y-6">
        {/* Display name */}
        <div className="space-y-2">
          <label htmlFor="display-name" className="block text-sm font-medium">
            Display name
          </label>
          <input
            id="display-name"
            type="text"
            value={displayName}
            onChange={(e) => {
              setDisplayName(e.target.value);
              clearSaveStatus();
            }}
            maxLength={DISPLAY_NAME_MAX}
            className="w-full rounded-md border border-input bg-background px-3 py-2 text-sm focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
          />
          <p className="text-xs text-muted-foreground">
            {displayName.trim().length}/{DISPLAY_NAME_MAX}
          </p>
        </div>

        {/* Bio */}
        <div className="space-y-2">
          <label htmlFor="bio" className="block text-sm font-medium">
            Bio
          </label>
          <textarea
            id="bio"
            value={bio}
            onChange={(e) => {
              setBio(e.target.value);
              clearSaveStatus();
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

        {/* Save / Cancel actions */}
        <div className="flex items-center gap-2">
          <Button type="button" variant="brand" onClick={handleSave} disabled={saving}>
            {saving ? "Saving..." : "Save changes"}
          </Button>
          <Button asChild variant="outline">
            <Link href="/admin/users">Cancel</Link>
          </Button>
        </div>

        {/* Role multi-select */}
        <section className="space-y-3">
          <h2 className="text-sm font-medium">Roles</h2>
          <p className="text-xs text-muted-foreground">
            Role changes take effect at the user&apos;s next sign-in or token refresh.
          </p>
          {allRoles.length === 0 ? (
            <p className="text-sm text-muted-foreground">No roles available.</p>
          ) : (
            <ul className="space-y-2">
              {allRoles.map((role) => {
                const isAssigned = userRoleIds.has(role.id);
                const isInflight = roleInflight[role.id] ?? false;
                const roleBanner = roleBanners[role.id];
                const checkboxId = `role-checkbox-${role.id}`;
                return (
                  <li key={role.id}>
                    <div className="flex items-center gap-3">
                      <input
                        id={checkboxId}
                        type="checkbox"
                        checked={isAssigned}
                        disabled={isInflight}
                        onChange={() => handleRoleToggle(role.id, isAssigned)}
                        className="h-4 w-4 rounded border-input accent-brand"
                        aria-label={role.name}
                      />
                      <label htmlFor={checkboxId} className="cursor-pointer text-sm">
                        {role.name}
                      </label>
                      {isInflight && (
                        <span className="text-xs text-muted-foreground">Updating...</span>
                      )}
                    </div>
                    {roleBanner && (
                      <div
                        className="mt-1 rounded-md bg-destructive/10 px-3 py-2 text-sm text-destructive"
                        role="alert"
                      >
                        {roleBanner}
                      </div>
                    )}
                  </li>
                );
              })}
            </ul>
          )}
        </section>
      </div>
    </main>
  );
}

"use client";

import { CombinedGraphQLErrors } from "@apollo/client/errors";
import { useMutation } from "@apollo/client/react";
import Link from "next/link";
import { useState } from "react";
import { Button } from "@/components/ui/button";
import type { AdminRoleFieldsFragment } from "@/generated/graphql";
import { getBackendErrorBanner } from "@/lib/apollo/errors";
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

/**
 * Classify a raw Apollo error into a user-facing banner string.
 * FORBIDDEN errors surface the server message verbatim (e.g. self-demotion guard).
 */
function classifyError(err: unknown): string {
  if (!err) return "";
  if (CombinedGraphQLErrors.is(err)) {
    for (const ge of err.errors) {
      if (ge.extensions?.code === "FORBIDDEN") {
        return ge.message;
      }
    }
  }
  return getBackendErrorBanner(err) ?? "An unexpected error occurred. Please try again.";
}

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
      await runUpdate({
        variables: {
          id: user.id,
          input: {
            displayName: displayName.trim(),
            bio: bio || null,
          },
        },
      });
      setSaveBanner("Changes saved.");
    } catch (err) {
      setSaveError(classifyError(err));
    }
  }

  async function handleRoleToggle(roleId: string, currentlyAssigned: boolean) {
    setRoleInflight((prev) => ({ ...prev, [roleId]: true }));
    setRoleBanners((prev) => ({ ...prev, [roleId]: "" }));

    try {
      const variables = { userId: user.id, roleId };
      // Update local role state from the server-truth response. Fragment masking
      // is compile-time only; at runtime the shape is the plain object.
      const serverRoles = (
        currentlyAssigned
          ? (await runRevoke({ variables })).data?.revokeRole?.roles
          : (await runAssign({ variables })).data?.assignRole?.roles
      ) as AdminRoleFieldsFragment[] | undefined;
      if (serverRoles) {
        setUserRoleIds(new Set(serverRoles.map((r) => r.id)));
      }
    } catch (err) {
      // On failure (including FORBIDDEN), surface the banner.
      // No optimistic writes were made, so local state already reflects server truth.
      setRoleBanners((prev) => ({
        ...prev,
        [roleId]: classifyError(err),
      }));
    } finally {
      setRoleInflight((prev) => ({ ...prev, [roleId]: false }));
    }
  }

  return (
    <main className="p-8">
      {/* Navigation */}
      <div className="mb-6 flex items-center gap-4">
        <Link href="/admin/users" className="text-sm text-muted-foreground hover:underline">
          &larr; Back to users
        </Link>
        <h1 className="text-2xl font-semibold">Edit User</h1>
      </div>

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

        {/* Save button */}
        <Button type="button" onClick={handleSave} disabled={saving}>
          {saving ? "Saving..." : "Save changes"}
        </Button>

        {/* Role multi-select */}
        <section className="space-y-3">
          <h2 className="text-sm font-medium">Roles</h2>
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
                        className="h-4 w-4 rounded border-input accent-primary"
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

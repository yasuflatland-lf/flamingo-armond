"use client";

import { CombinedGraphQLErrors } from "@apollo/client/errors";
import { useMutation, useQuery } from "@apollo/client/react";
import Link from "next/link";
import { useState } from "react";
import { Button } from "@/components/ui/button";
import { useFragment } from "@/generated/fragment-masking";
import type {
  AdminRolesQuery as AdminRolesQueryType,
  AdminUserQuery as AdminUserQueryType,
} from "@/generated/graphql";
import { AdminRoleFieldsFragmentDoc, AdminUserFieldsFragmentDoc } from "@/generated/graphql";
import { getBackendErrorBanner } from "@/lib/apollo/errors";
import {
  AdminAssignRoleMutation,
  AdminRevokeRoleMutation,
  AdminRolesQuery,
  AdminUpdateUserMutation,
  AdminUserQuery,
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
 *
 * Two usage modes:
 *  - SSR mode: pass `user` and `allRoles` from a server component (no client queries fired).
 *  - Client mode: pass `userId` and the component fetches its own data (used in tests).
 */
type Props =
  | {
      /** Pre-fetched user from the server component. */
      user: UserForEdit;
      /** Pre-fetched roles list from the server component. */
      allRoles: RoleOption[];
      userId?: never;
      callerId?: string;
    }
  | {
      /** User ID; the component fetches AdminUserQuery and AdminRolesQuery itself. */
      userId: string;
      user?: never;
      allRoles?: never;
      callerId?: string;
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

export function AdminUserEditClient(props: Props) {
  const isClientMode = "userId" in props && props.userId !== undefined;

  // Client-side queries — only active in client mode (skip when SSR props provided).
  const { data: queriedUserData, loading: userLoading } = useQuery(AdminUserQuery, {
    variables: { id: isClientMode ? props.userId : "" },
    skip: !isClientMode,
  });
  const { data: queriedRolesData, loading: rolesLoading } = useQuery(AdminRolesQuery, {
    skip: !isClientMode,
  });

  // Resolve user data: prefer SSR prop, fall back to query result.
  const rawUser = isClientMode
    ? queriedUserData?.adminUser
    : (props.user as unknown as NonNullable<AdminUserQueryType["adminUser"]>);
  const userFields = useFragment(AdminUserFieldsFragmentDoc, rawUser);

  const rawAllRoles = isClientMode
    ? (queriedRolesData?.roles ?? [])
    : ((props.allRoles ?? []) as unknown as AdminRolesQueryType["roles"]);
  const allRoles = useFragment(AdminRoleFieldsFragmentDoc, rawAllRoles);

  const rawUserRoles = rawUser?.roles ?? [];
  const userRoles = useFragment(AdminRoleFieldsFragmentDoc, rawUserRoles);

  const resolvedUserId = userFields?.id ?? (isClientMode ? props.userId : "");

  const [displayName, setDisplayName] = useState(userFields?.displayName ?? "");
  const [bio, setBio] = useState(userFields?.bio ?? "");
  const [saveError, setSaveError] = useState("");
  const [saveBanner, setSaveBanner] = useState("");

  // Per-role error banners (keyed by role id).
  const [roleBanners, setRoleBanners] = useState<Record<string, string>>({});
  // Per-role in-flight guard so consecutive clicks cannot double-fire.
  const [roleInflight, setRoleInflight] = useState<Record<string, boolean>>({});

  const [runUpdate, { loading: saving }] = useMutation(AdminUpdateUserMutation);
  const [runAssign] = useMutation(AdminAssignRoleMutation);
  const [runRevoke] = useMutation(AdminRevokeRoleMutation);

  // Derive assigned role ids. Apollo entity normalization propagates
  // assign/revoke mutation results back into the cache automatically.
  const userRoleIds = new Set(userRoles.map((r) => r.id));

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
    setSaveError("");
    setSaveBanner("");

    try {
      await runUpdate({
        variables: {
          id: resolvedUserId,
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

    // Build the optimistic roles list so the checkbox flips immediately.
    const optimisticRoles = currentlyAssigned
      ? userRoles
          .filter((r) => r.id !== roleId)
          .map((r) => ({ __typename: "Role" as const, id: r.id, name: r.name }))
      : [
          ...userRoles.map((r) => ({
            __typename: "Role" as const,
            id: r.id,
            name: r.name,
          })),
          {
            __typename: "Role" as const,
            id: roleId,
            name: allRoles.find((r) => r.id === roleId)?.name ?? "",
          },
        ];

    const optimisticUser = {
      __typename: "User" as const,
      id: resolvedUserId,
      displayName: userFields?.displayName ?? null,
      bio: userFields?.bio ?? null,
      avatarUrl: userFields?.avatarUrl ?? null,
      roles: optimisticRoles,
    };

    try {
      if (currentlyAssigned) {
        await runRevoke({
          variables: { userId: resolvedUserId, roleId },
          optimisticResponse: { revokeRole: optimisticUser },
        });
      } else {
        await runAssign({
          variables: { userId: resolvedUserId, roleId },
          optimisticResponse: { assignRole: optimisticUser },
        });
      }
    } catch (err) {
      setRoleBanners((prev) => ({
        ...prev,
        [roleId]: classifyError(err),
      }));
    } finally {
      setRoleInflight((prev) => ({ ...prev, [roleId]: false }));
    }
  }

  if (isClientMode && (userLoading || rolesLoading)) {
    return <div>Loading...</div>;
  }

  if (!userFields && !isClientMode) {
    return <div role="alert">User not found.</div>;
  }

  return (
    <main className="mx-auto max-w-2xl p-8">
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
              setSaveError("");
              setSaveBanner("");
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
              setSaveError("");
              setSaveBanner("");
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

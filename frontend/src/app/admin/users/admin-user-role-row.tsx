"use client";

import { useMutation } from "@apollo/client/react";
import { Pencil } from "lucide-react";
import Image from "next/image";
import { useEffect, useMemo, useRef, useState } from "react";
import { Button } from "@/components/ui/button";
import type { AdminRoleFieldsFragment } from "@/generated/graphql";
import { liftGraphQLCodes } from "@/lib/apollo/graphql-errors";
import { AdminAssignRoleMutation, AdminRevokeRoleMutation } from "./queries";

export type AdminUserRole = {
  id: string;
  name: string;
};

export type AdminUserListItem = {
  id: string;
  displayName?: string | null;
  bio?: string | null;
  avatarUrl?: string | null;
  roles: AdminUserRole[];
};

type Props = {
  user: AdminUserListItem;
  allRoles: AdminUserRole[];
  rolesLoading?: boolean;
  onEdit: (id: string) => void;
};

const ERR_FORBIDDEN = "You do not have permission.";
const ERR_UNAUTHENTICATED = "Your session has expired. Sign in again.";
const ERR_UNEXPECTED = "An unexpected error occurred. Please try again.";
const ERR_SOMETHING_WRONG = "Something went wrong. Please try again.";

export function AdminUserRoleRow({ user, allRoles, rolesLoading = false, onEdit }: Props) {
  const [roleBanners, setRoleBanners] = useState<Record<string, string>>({});
  const [roleInflight, setRoleInflight] = useState<Record<string, boolean>>({});
  const roleInflightRef = useRef<Set<string>>(new Set());
  const [userRoleIds, setUserRoleIds] = useState<Set<string>>(
    () => new Set(user.roles.map((role) => role.id)),
  );

  const [runAssign] = useMutation(AdminAssignRoleMutation);
  const [runRevoke] = useMutation(AdminRevokeRoleMutation);

  useEffect(() => {
    setUserRoleIds(new Set(user.roles.map((role) => role.id)));
  }, [user.roles]);

  const avatarFallback = useMemo(
    () => (user.displayName ?? "?").charAt(0).toUpperCase(),
    [user.displayName],
  );

  function setRoleBanner(roleId: string, message: string): void {
    setRoleBanners((prev) => ({ ...prev, [roleId]: message }));
  }

  async function handleRoleToggle(roleId: string, currentlyAssigned: boolean) {
    if (roleInflightRef.current.has(roleId)) return;

    roleInflightRef.current.add(roleId);
    setRoleInflight((prev) => ({ ...prev, [roleId]: true }));
    setRoleBanner(roleId, "");
    try {
      const variables = { userId: user.id, roleId };
      const result = currentlyAssigned
        ? (await runRevoke({ variables })).data?.revokeRole
        : (await runAssign({ variables })).data?.assignRole;
      const roleTypename = result?.__typename ?? null;
      if (
        result?.__typename === "InputValidationError" ||
        result?.__typename === "CannotRevokeOwnAdminRoleError"
      ) {
        setRoleBanner(roleId, result.message);
        return;
      }
      if (
        result?.__typename === "RevokeRoleSuccess" ||
        result?.__typename === "AssignRoleSuccess"
      ) {
        const serverRoles = result.user.roles as AdminRoleFieldsFragment[];
        setUserRoleIds(new Set(serverRoles.map((role) => role.id)));
        return;
      }
      console.warn("[admin/users] unexpected role-toggle payload", {
        typename: roleTypename,
      });
      setRoleBanner(roleId, ERR_SOMETHING_WRONG);
    } catch (err) {
      const codes = liftGraphQLCodes(err);
      console.warn("[admin/users] role-toggle rejected", {
        name: err instanceof Error ? err.name : "unknown",
        codes,
      });
      setRoleBanner(
        roleId,
        codes.includes("FORBIDDEN")
          ? ERR_FORBIDDEN
          : codes.includes("UNAUTHENTICATED")
            ? ERR_UNAUTHENTICATED
            : ERR_UNEXPECTED,
      );
    } finally {
      roleInflightRef.current.delete(roleId);
      setRoleInflight((prev) => ({ ...prev, [roleId]: false }));
    }
  }

  return (
    <li
      className="rounded-md border border-border transition-colors hover:bg-accent"
      data-testid={`admin-user-row-${user.id}`}
    >
      <div className="flex flex-col gap-4 px-4 py-3 md:flex-row md:items-start">
        <div className="flex min-w-0 flex-1 items-start gap-4">
          {user.avatarUrl ? (
            <Image
              src={user.avatarUrl}
              alt={user.displayName ?? "User avatar"}
              width={40}
              height={40}
              className="h-10 w-10 shrink-0 rounded-full object-cover"
            />
          ) : (
            <div
              className="flex h-10 w-10 shrink-0 items-center justify-center rounded-full bg-muted text-sm font-medium text-muted-foreground"
              aria-hidden="true"
            >
              {avatarFallback}
            </div>
          )}

          <div className="min-w-0 flex-1 space-y-1">
            <p className="text-sm font-medium">
              {user.displayName ?? <span className="italic text-muted-foreground">No name</span>}
            </p>
            {user.bio && <p className="truncate text-sm text-muted-foreground">{user.bio}</p>}
          </div>
        </div>

        <div className="flex flex-col gap-3 md:min-w-72">
          <div className="flex flex-wrap items-center gap-3">
            {rolesLoading ? (
              <span className="text-xs text-muted-foreground">Loading roles...</span>
            ) : allRoles.length === 0 ? (
              <span className="text-xs text-muted-foreground">No roles available.</span>
            ) : (
              allRoles.map((role) => {
                const isAssigned = userRoleIds.has(role.id);
                const isInflight = roleInflight[role.id] ?? false;
                const checkboxId = `role-checkbox-${user.id}-${role.id}`;
                return (
                  <div key={role.id} className="flex items-center gap-2">
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
                );
              })
            )}
          </div>
          {allRoles.map((role) => {
            const roleBanner = roleBanners[role.id];
            return roleBanner ? (
              <div
                key={role.id}
                className="rounded-md bg-destructive/10 px-3 py-2 text-sm text-destructive"
                role="alert"
              >
                {roleBanner}
              </div>
            ) : null;
          })}
        </div>

        <Button
          type="button"
          variant="outline"
          size="sm"
          className="self-start"
          onClick={() => onEdit(user.id)}
          aria-label={`Edit ${user.displayName ?? "user"}`}
        >
          <Pencil aria-hidden="true" />
          <span>Edit</span>
        </Button>
      </div>
    </li>
  );
}

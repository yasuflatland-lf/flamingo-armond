"use client";

import { Pencil } from "lucide-react";
import Image from "next/image";
import { useTranslations } from "next-intl";
import { useMemo } from "react";
import { Button } from "@/components/ui/button";

export type AdminUserRole = {
  id: string;
  name: string;
};

/**
 * Display shape for an admin user row. Nullable string fields are `string |
 * null` (required, nullable), not `string | null | undefined` — see
 * docs/frontend/typescript-conventions/required-string-null-over-optional-string-null.md.
 */
export type AdminUserListItem = {
  id: string;
  version: number;
  displayName: string | null;
  bio: string | null;
  avatarUrl: string | null;
  roles: AdminUserRole[];
};

type Props = {
  user: AdminUserListItem;
  onEdit: (id: string) => void;
};

export function AdminUserRow({ user, onEdit }: Props) {
  const t = useTranslations("Admin");
  const tCommon = useTranslations("Common");
  const avatarFallback = useMemo(
    () => (user.displayName ?? "?").charAt(0).toUpperCase(),
    [user.displayName],
  );

  return (
    <li
      className="rounded-md border border-border transition-colors hover:bg-accent"
      data-testid={`admin-user-row-${user.id}`}
    >
      <div className="flex items-center gap-4 px-4 py-3">
        <div className="flex min-w-0 flex-1 items-center gap-4">
          {user.avatarUrl ? (
            <Image
              src={user.avatarUrl}
              alt={user.displayName ?? t("userAvatarAlt")}
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
              {user.displayName ?? (
                <span className="italic text-muted-foreground">{t("noName")}</span>
              )}
            </p>
            {user.bio && <p className="truncate text-sm text-muted-foreground">{user.bio}</p>}
          </div>
        </div>

        <Button
          type="button"
          variant="outline"
          size="sm"
          className="shrink-0"
          onClick={() => onEdit(user.id)}
          aria-label={t("editAriaLabel", { name: user.displayName ?? t("noName") })}
        >
          <Pencil aria-hidden="true" />
          <span>{tCommon("edit")}</span>
        </Button>
      </div>
    </li>
  );
}

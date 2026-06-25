"use client";

import { Pencil } from "lucide-react";
import Link from "next/link";
import { useTranslations } from "next-intl";
import { MasterStatusBadge } from "./master-status-badge";

type MasterStatus = "DRAFT" | "PUBLISHED";

/**
 * Display shape for an admin master row. Nullable string fields are
 * `string | null` (required, nullable), not optional — see
 * docs/frontend/typescript-conventions/required-string-null-over-optional-string-null.md.
 */
export type AdminMasterListItem = {
  id: string;
  version: number;
  name: string;
  description: string | null;
  isDefaultStarter: boolean;
  sortOrder: number;
  status: MasterStatus;
  cardCount: number;
};

type Props = { master: AdminMasterListItem };

/**
 * List row for a master deck. The whole row is a single Link to the edit page;
 * the pencil + label are a non-interactive visual cue (no nested interactive
 * element). The status badge stays visible so publish state shows in the list.
 */
export function AdminMasterRow({ master }: Props) {
  const t = useTranslations("AdminMasters");
  const tCommon = useTranslations("Common");
  const published = master.status === "PUBLISHED";

  return (
    <li
      className="rounded-md border border-border transition-colors hover:bg-accent"
      data-testid={`master-catalog-row-${master.id}`}
    >
      <Link
        href={`/admin/masters/${master.id}/edit`}
        className="flex flex-wrap items-center gap-x-3 gap-y-3 p-4 sm:py-3"
        data-testid="master-row-edit"
        aria-label={t("editAriaLabel", { name: master.name })}
      >
        <p
          className="order-1 w-full min-w-0 truncate text-sm font-medium tracking-tight sm:order-2 sm:w-auto sm:flex-1"
          title={master.name}
        >
          {master.name}
        </p>

        <div className="order-2 flex min-w-0 flex-1 items-center gap-2 sm:contents">
          <MasterStatusBadge
            published={published}
            data-testid="master-row-status-badge"
            className="shrink-0 sm:order-1"
          />
          <span aria-hidden="true" className="text-muted-foreground sm:hidden">
            ·
          </span>
          <span className="shrink-0 text-xs tabular-nums text-muted-foreground sm:order-3">
            {t("cardCount", { count: master.cardCount })}
          </span>
        </div>

        <span className="order-3 inline-flex shrink-0 items-center gap-1 text-sm text-muted-foreground sm:order-4">
          <Pencil aria-hidden="true" className="h-4 w-4" />
          <span>{tCommon("edit")}</span>
        </span>
      </Link>
    </li>
  );
}

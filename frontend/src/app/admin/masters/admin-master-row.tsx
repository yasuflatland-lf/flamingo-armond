"use client";

import { Pencil } from "lucide-react";
import { useTranslations } from "next-intl";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";

type MasterStatus = "DRAFT" | "PUBLISHED";

/**
 * Display shape for an admin master row + edit form. Nullable string fields are
 * `string | null` (required, nullable), not optional — see
 * docs/frontend/typescript-conventions/required-string-null-over-optional-string-null.md.
 */
export type AdminMasterListItem = {
  id: string;
  version: number;
  name: string;
  description: string | null;
  language: string | null;
  level: string | null;
  category: string | null;
  coverImageUrl: string | null;
  source: string | null;
  isDefaultStarter: boolean;
  sortOrder: number;
  status: MasterStatus;
  cardCount: number;
};

type Props = {
  master: AdminMasterListItem;
  onEdit: (id: string) => void;
};

/**
 * List row for a master deck. The status badge stays here so the publish state
 * is always visible in the list, but the publish/unpublish *action* lives in the
 * Edit panel (see AdminMasterForm) — a deck's lifecycle transition is an
 * edit-context operation, not a one-click list affordance.
 */
export function AdminMasterRow({ master, onEdit }: Props) {
  const t = useTranslations("AdminMasters");
  const tCommon = useTranslations("Common");

  const published = master.status === "PUBLISHED";

  return (
    <li
      className="rounded-md border border-border transition-colors hover:bg-accent"
      data-testid={`master-catalog-row-${master.id}`}
    >
      <div className="flex flex-wrap items-center gap-3 px-4 py-3">
        <Badge
          variant={published ? "default" : "secondary"}
          role="status"
          data-testid="master-row-status-badge"
        >
          {published ? t("statusPublished") : t("statusDraft")}
        </Badge>

        <div className="min-w-0 flex-1">
          <p className="truncate text-sm font-medium" title={master.name}>
            {master.name}
          </p>
        </div>

        <span className="shrink-0 text-xs text-muted-foreground">
          {t("cardCount", { count: master.cardCount })}
        </span>

        <Button
          type="button"
          variant="outline"
          size="sm"
          className="shrink-0"
          data-testid="master-row-edit"
          onClick={() => onEdit(master.id)}
          aria-label={t("editAriaLabel", { name: master.name })}
        >
          <Pencil aria-hidden="true" />
          <span>{tCommon("edit")}</span>
        </Button>
      </div>
    </li>
  );
}

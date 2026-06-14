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
      {/*
       * Mobile (default): two tiers — the name as a full-width title line, then a
       * meta line carrying [badge] · count on the left and Edit on the right.
       * Desktop (sm+): a single inline row [badge][name][count][Edit]. One DOM
       * serves both: `w-full` + `order-*` force the mobile line break, and
       * `sm:contents` dissolves the meta wrapper on desktop so the badge and count
       * rejoin the inline row in their own order. Spacing rhythm 16 / 12 / 8.
       */}
      <div className="flex flex-wrap items-center gap-x-3 gap-y-3 p-4 sm:py-3">
        <p
          className="order-1 w-full min-w-0 truncate text-sm font-medium tracking-tight sm:order-2 sm:w-auto sm:flex-1"
          title={master.name}
        >
          {master.name}
        </p>

        <div className="order-2 flex min-w-0 flex-1 items-center gap-2 sm:contents">
          <Badge
            variant={published ? "default" : "secondary"}
            role="status"
            data-testid="master-row-status-badge"
            className="shrink-0 sm:order-1"
          >
            {published ? t("statusPublished") : t("statusDraft")}
          </Badge>
          <span aria-hidden="true" className="text-muted-foreground sm:hidden">
            ·
          </span>
          <span className="shrink-0 text-xs tabular-nums text-muted-foreground sm:order-3">
            {t("cardCount", { count: master.cardCount })}
          </span>
        </div>

        <Button
          type="button"
          variant="outline"
          size="sm"
          className="order-3 shrink-0 sm:order-4"
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

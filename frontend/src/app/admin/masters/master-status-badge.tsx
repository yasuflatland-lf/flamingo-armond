"use client";

import { useTranslations } from "next-intl";
import { Badge } from "@/components/ui/badge";
import { cn } from "@/lib/utils";

type Props = {
  /** Whether the master deck is published; drives the dot color and the label. */
  published: boolean;
  /** Extra classes merged onto the badge (layout-only, e.g. `shrink-0 sm:order-1`). */
  className?: string;
  "data-testid"?: string;
};

/**
 * Read-only publish-status chip for a master deck: an outline pill with a
 * leading status dot — green (`--success`) when published, muted when draft —
 * followed by the localized status label. Shared by the master list row and the
 * master-edit header so every status surface reads identically (Linear/Vercel-
 * style: the container recedes and the dot carries the "live" signal). The label
 * always renders, so the state is never communicated by color alone.
 *
 * The mobile publish-toggle in the edit header composes the same visual
 * vocabulary with a disclosure chevron; this component is read-only and is
 * never a button.
 */
export function MasterStatusBadge({ published, className, "data-testid": dataTestid }: Props) {
  const t = useTranslations("AdminMasters");
  return (
    <Badge
      variant="outline"
      role="status"
      data-testid={dataTestid}
      className={cn("gap-1.5", className)}
    >
      <span
        aria-hidden="true"
        className={cn("h-2 w-2 rounded-full", published ? "bg-success" : "bg-muted-foreground")}
      />
      {published ? t("statusPublished") : t("statusDraft")}
    </Badge>
  );
}

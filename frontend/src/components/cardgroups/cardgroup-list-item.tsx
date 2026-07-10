"use client";

import Link from "next/link";
import { useLocale, useTranslations } from "next-intl";
import { HoverRevealDeleteButton } from "@/components/cardgroups/hover-reveal-delete-button";
import { SwipeableRow } from "@/components/cardgroups/swipeable-row";
import { formatMediumDate } from "@/lib/format";

export type CardgroupListItemProps = {
  id: string;
  name: string;
  updatedAt: string;
  busy?: boolean;
  onDelete: (id: string, name: string) => void;
};

export function CardgroupListItem({
  id,
  name,
  updatedAt,
  busy = false,
  onDelete,
}: CardgroupListItemProps) {
  const locale = useLocale();
  const t = useTranslations("Cardgroups");
  const deleteLabel = t("deleteAriaLabel", { name });
  const requestDelete = () => onDelete(id, name);
  return (
    <SwipeableRow onDelete={requestDelete} disabled={busy}>
      <li className="group flex items-center gap-2 rounded-lg border border-border pr-2 hover:bg-accent active:bg-accent transition-colors bg-background">
        <Link href={`/cardgroups/${id}/edit`} className="flex min-w-0 flex-1 flex-col gap-1 p-4">
          <span className="truncate font-medium text-foreground">{name}</span>
          <span className="text-sm text-muted-foreground">
            {t("updatedAt", { date: formatMediumDate(updatedAt, locale) })}
          </span>
        </Link>
        <HoverRevealDeleteButton
          className="pointer-events-none sm:group-hover:pointer-events-auto sm:group-hover:opacity-100 motion-reduce:pointer-events-auto"
          onDelete={requestDelete}
          disabled={busy}
          ariaLabel={deleteLabel}
        />
      </li>
    </SwipeableRow>
  );
}

import Link from "next/link";
import { CardgroupRowActions } from "./cardgroup-row-actions";
import { formatMediumDate } from "@/lib/format";

export type CardgroupListItemProps = {
  id: string;
  name: string;
  updatedAt: string;
};

export function CardgroupListItem({ id, name, updatedAt }: CardgroupListItemProps) {
  return (
    <li className="flex items-center gap-1 rounded-lg border border-border hover:bg-accent transition-colors">
      <Link
        href={`/cardgroups/${id}/cards`}
        className="flex flex-1 flex-col gap-1 p-4 min-w-0"
      >
        <span className="font-medium text-foreground truncate">{name}</span>
        <span className="text-sm text-muted-foreground">Updated {formatMediumDate(updatedAt)}</span>
      </Link>
      <CardgroupRowActions id={id} name={name} />
    </li>
  );
}

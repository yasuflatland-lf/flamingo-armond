import Link from "next/link";
import { formatMediumDate } from "@/lib/format";

export type CardgroupListItemProps = {
  id: string;
  name: string;
  updatedAt: string;
};

export function CardgroupListItem({ id, name, updatedAt }: CardgroupListItemProps) {
  return (
    <li>
      <Link
        href={`/cardgroups/${id}`}
        className="flex flex-col gap-1 rounded-lg border border-border p-4 hover:bg-accent transition-colors"
      >
        <span className="font-medium text-foreground">{name}</span>
        <span className="text-sm text-muted-foreground">Updated {formatMediumDate(updatedAt)}</span>
      </Link>
    </li>
  );
}

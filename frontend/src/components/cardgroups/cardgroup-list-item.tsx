import Link from "next/link";

export type CardgroupListItemProps = {
  id: string;
  name: string;
  updatedAt: string;
};

function formatDate(iso: string): string {
  return new Intl.DateTimeFormat("en-US", { dateStyle: "medium" }).format(new Date(iso));
}

export function CardgroupListItem({ id, name, updatedAt }: CardgroupListItemProps) {
  return (
    <li>
      <Link
        href={`/cardgroups/${id}`}
        className="flex flex-col gap-1 rounded-lg border border-border p-4 hover:bg-accent transition-colors"
      >
        <span className="font-medium text-foreground">{name}</span>
        <span className="text-sm text-muted-foreground">Updated {formatDate(updatedAt)}</span>
      </Link>
    </li>
  );
}

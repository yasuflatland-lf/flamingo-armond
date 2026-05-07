import { Pencil } from "lucide-react";
import Link from "next/link";
import { DeleteCardgroupButton } from "@/components/cardgroups/delete-cardgroup-button";
import { Button } from "@/components/ui/button";
import { formatMediumDate } from "@/lib/format";

export type CardgroupListItemProps = {
  id: string;
  name: string;
  updatedAt: string;
};

export function CardgroupListItem({ id, name, updatedAt }: CardgroupListItemProps) {
  return (
    <li className="flex items-center gap-2 rounded-lg border border-border pr-2 hover:bg-accent transition-colors">
      <Link href={`/cardgroups/${id}`} className="flex min-w-0 flex-1 flex-col gap-1 p-4">
        <span className="truncate font-medium text-foreground">{name}</span>
        <span className="text-sm text-muted-foreground">Updated {formatMediumDate(updatedAt)}</span>
      </Link>
      <Button asChild variant="outline" size="icon" aria-label={`Edit cardgroup ${name}`}>
        <Link href={`/cardgroups/${id}/edit`}>
          <Pencil className="h-4 w-4" />
        </Link>
      </Button>
      <DeleteCardgroupButton id={id} name={name} />
    </li>
  );
}

import Link from "next/link";
import { Trash2 } from "lucide-react";
import { SwipeableRow } from "@/components/cardgroups/swipeable-row";
import { Button } from "@/components/ui/button";
import { formatMediumDate } from "@/lib/format";

export type CardgroupListItemProps = {
  id: string;
  name: string;
  updatedAt: string;
  busy?: boolean;
  onDelete: (id: string, name: string) => void;
};

export function CardgroupListItem({ id, name, updatedAt, busy = false, onDelete }: CardgroupListItemProps) {
  const deleteLabel = `Delete cardgroup ${name}`;
  const requestDelete = () => onDelete(id, name);
  return (
    <SwipeableRow onDelete={requestDelete} disabled={busy} ariaLabel={deleteLabel}>
      <li className="group flex items-center gap-2 rounded-lg border border-border pr-2 hover:bg-accent active:bg-accent transition-colors bg-background">
        <Link href={`/cardgroups/${id}/edit`} className="flex min-w-0 flex-1 flex-col gap-1 p-4">
          <span className="truncate font-medium text-foreground">{name}</span>
          <span className="text-sm text-muted-foreground">Updated {formatMediumDate(updatedAt)}</span>
        </Link>
        <Button
          variant="outline"
          size="icon"
          className="opacity-0 sm:group-hover:opacity-100 motion-reduce:opacity-100 transition-opacity"
          onClick={requestDelete}
          disabled={busy}
          aria-label={deleteLabel}
        >
          <Trash2 className="h-4 w-4" />
        </Button>
      </li>
    </SwipeableRow>
  );
}

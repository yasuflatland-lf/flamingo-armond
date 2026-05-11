import { Plus } from "lucide-react";
import Link from "next/link";
import { Button } from "@/components/ui/button";

interface LearnAddCardFloatingProps {
  cardgroupId: string;
  cardgroupName: string;
}

export function LearnAddCardFloating({ cardgroupId, cardgroupName }: LearnAddCardFloatingProps) {
  const encodedId = encodeURIComponent(cardgroupId);
  const href = `/cards/new?cardgroup=${encodedId}&return=/learn/${encodedId}`;

  return (
    <Button
      asChild
      variant="ghost"
      size="icon"
      className="fixed top-1 right-2 z-40 inline-flex h-10 w-10 items-center justify-center rounded-md text-foreground active:bg-muted/70 md:top-4 md:right-6"
      aria-label={`Add a new card to ${cardgroupName}`}
    >
      <Link href={href}>
        <Plus className="h-5 w-5" aria-hidden="true" />
      </Link>
    </Button>
  );
}

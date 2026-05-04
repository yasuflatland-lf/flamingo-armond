import { Plus } from "lucide-react";
import Link from "next/link";
import { Button } from "@/components/ui/button";

interface LearnAddCardFloatingProps {
  cardgroupId: string;
  cardgroupName: string;
}

export function LearnAddCardFloating({ cardgroupId, cardgroupName }: LearnAddCardFloatingProps) {
  const href = `/cards/new?cardgroup=${cardgroupId}&return=/learn/${cardgroupId}`;

  return (
    <Button
      asChild
      variant="brand"
      size="icon"
      className="fixed right-6 top-6 z-40 hidden h-10 w-10 rounded-full md:flex"
      aria-label={`Add a new card to ${cardgroupName}`}
    >
      <Link href={href}>
        <Plus className="h-5 w-5" aria-hidden="true" />
      </Link>
    </Button>
  );
}

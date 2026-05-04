"use client";

import { MoreHorizontal, Pencil, Play } from "lucide-react";
import Link from "next/link";
import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";

interface CardgroupRowActionsProps {
  id: string;
  name: string;
}

// `aria-label` includes the cardgroup name so screen readers can disambiguate
// between repeated `⋯` triggers (one per row). The trigger is positioned as a
// sibling of the row's primary Link in `cardgroup-list-item.tsx` — nesting a
// <button> inside an <a> would be invalid HTML and break event delegation.
export function CardgroupRowActions({ id, name }: CardgroupRowActionsProps) {
  const encoded = encodeURIComponent(id);
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button
          variant="ghost"
          size="icon"
          className="mr-2 h-8 w-8 shrink-0"
          aria-label={`Actions for ${name}`}
        >
          <MoreHorizontal className="h-4 w-4" aria-hidden="true" />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end">
        <DropdownMenuItem asChild>
          <Link href={`/learn/${encoded}`}>
            <Play className="mr-2 h-4 w-4" aria-hidden="true" />
            Start learning
          </Link>
        </DropdownMenuItem>
        <DropdownMenuItem asChild>
          <Link href={`/cardgroups/${encoded}/edit`}>
            <Pencil className="mr-2 h-4 w-4" aria-hidden="true" />
            Rename
          </Link>
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

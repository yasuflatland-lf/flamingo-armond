"use client";

import { ChevronDown } from "lucide-react";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";

type CardgroupChipProps = {
  name: string | null;
  onChangeRequested: () => void;
};

export function CardgroupChip({ name, onChangeRequested }: CardgroupChipProps) {
  // Truthy coalescing (||) intentionally treats "" the same as null — see test
  // "renders placeholder for empty-string name (treated as no name)".
  const displayName = name || "Select cardgroup";
  const isMuted = !name;

  return (
    <Button
      type="button"
      variant="outline"
      onClick={onChangeRequested}
      aria-label={
        name ? `Change cardgroup (currently "${name}")` : "Select cardgroup"
      }
      className="inline-flex items-center gap-2"
    >
      <span
        className={cn(
          "truncate max-w-[12ch] sm:max-w-[20ch] text-sm",
          isMuted && "text-muted-foreground",
        )}
      >
        {displayName}
      </span>
      <ChevronDown size={16} className="flex-shrink-0" />
    </Button>
  );
}

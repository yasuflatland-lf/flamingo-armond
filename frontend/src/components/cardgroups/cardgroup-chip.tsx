"use client";

import { ChevronDown } from "lucide-react";
import { Button } from "@/components/ui/button";

type CardgroupChipProps = {
  name: string | null;
  onChangeRequested: () => void;
};

export function CardgroupChip({ name, onChangeRequested }: CardgroupChipProps) {
  const displayName = name ?? "Select cardgroup";
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
        className={`truncate max-w-[12ch] sm:max-w-[20ch] text-sm ${
          isMuted ? "text-muted-foreground" : ""
        }`}
      >
        {displayName}
      </span>
      <ChevronDown size={16} className="flex-shrink-0" />
    </Button>
  );
}

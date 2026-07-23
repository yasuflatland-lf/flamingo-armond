"use client";

import { ChevronDown } from "lucide-react";
import { useTranslations } from "next-intl";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";

type CardgroupChipProps = {
  name: string | null;
  onChangeRequested: () => void;
};

export function CardgroupChip({ name, onChangeRequested }: CardgroupChipProps) {
  const t = useTranslations("Cardgroups");
  // Truthy coalescing (||) intentionally treats "" the same as null — see test
  // "renders placeholder for empty-string name (treated as no name)".
  const displayName = name || t("pickerTitle");
  const isMuted = !name;

  return (
    <Button
      type="button"
      variant="outline"
      onClick={onChangeRequested}
      aria-label={name ? t("chipChangeAriaLabel", { name }) : t("pickerTitle")}
      data-testid="cardgroup-chip"
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

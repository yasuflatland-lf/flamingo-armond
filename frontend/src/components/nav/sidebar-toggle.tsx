"use client";

import { PanelLeft } from "lucide-react";
import { Button } from "@/components/ui/button";

interface SidebarToggleProps {
  expanded: boolean;
  onToggle: () => void;
}

export function SidebarToggle({ expanded, onToggle }: SidebarToggleProps) {
  return (
    <Button
      type="button"
      variant="ghost"
      size="icon"
      onClick={onToggle}
      aria-expanded={expanded}
      aria-label="Toggle navigation rail"
    >
      <PanelLeft aria-hidden="true" />
    </Button>
  );
}

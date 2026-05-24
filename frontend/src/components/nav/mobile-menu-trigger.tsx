"use client";

import { Menu } from "lucide-react";
import { SheetTrigger } from "@/components/ui/sheet";

export function MobileMenuTrigger() {
  return (
    <SheetTrigger asChild>
      <button
        type="button"
        aria-label="Open menu"
        className="rounded-md p-2 hover:bg-accent focus:outline-none focus:ring-2 focus:ring-ring"
      >
        <Menu className="h-5 w-5" aria-hidden="true" />
      </button>
    </SheetTrigger>
  );
}

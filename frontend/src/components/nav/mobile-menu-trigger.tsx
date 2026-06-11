"use client";

import { Menu } from "lucide-react";
import { useTranslations } from "next-intl";
import { SheetTrigger } from "@/components/ui/sheet";

export function MobileMenuTrigger() {
  const t = useTranslations("Nav");
  return (
    <SheetTrigger asChild>
      <button
        type="button"
        aria-label={t("openMenu")}
        className="rounded-md p-2 hover:bg-accent focus:outline-none focus:ring-2 focus:ring-ring"
      >
        <Menu className="h-5 w-5" aria-hidden="true" />
      </button>
    </SheetTrigger>
  );
}

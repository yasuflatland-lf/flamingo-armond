"use client";

import { Plus } from "lucide-react";
import Link from "next/link";
import { usePathname } from "next/navigation";

export function HeaderAddCardLink() {
  const pathname = usePathname();
  const onCardsNew = pathname === "/cards/new";
  return (
    <Link
      href="/cards/new"
      aria-current={onCardsNew ? "page" : undefined}
      className={`flex items-center gap-1.5 rounded-md bg-brand-primary px-3 py-1.5 text-sm font-medium text-brand-primary-foreground transition-opacity ${
        onCardsNew ? "pointer-events-none opacity-60" : "hover:opacity-90"
      }`}
    >
      <Plus className="h-4 w-4" aria-hidden="true" />
      Card
    </Link>
  );
}

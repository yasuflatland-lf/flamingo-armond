"use client";

import { BookOpen, Menu, ShieldCheck, User } from "lucide-react";
import Link from "next/link";
import { LogoutButton } from "@/app/_components/logout-button";
import {
  Sheet,
  SheetClose,
  SheetContent,
  SheetHeader,
  SheetTitle,
  SheetTrigger,
} from "@/components/ui/sheet";

interface HamburgerDrawerProps {
  isAdmin: boolean;
  userEmail?: string | null;
}

const NAV_LINK_CLASS =
  "flex items-center gap-3 rounded-md px-3 py-2 text-sm font-medium hover:bg-accent hover:text-accent-foreground";

export function HamburgerDrawer({ isAdmin, userEmail }: HamburgerDrawerProps) {
  return (
    <Sheet>
      <SheetTrigger asChild>
        <button
          type="button"
          aria-label="Open menu"
          className="rounded-md p-2 hover:bg-accent focus:outline-none focus:ring-2 focus:ring-ring"
        >
          <Menu className="h-5 w-5" />
        </button>
      </SheetTrigger>

      <SheetContent side="left" className="flex flex-col">
        <SheetHeader>
          <SheetTitle className="sr-only">Menu</SheetTitle>
        </SheetHeader>

        {userEmail && <p className="mb-4 truncate text-xs text-muted-foreground">{userEmail}</p>}

        <nav className="flex flex-col gap-1">
          <SheetClose asChild>
            <Link href="/cardgroups" className={NAV_LINK_CLASS}>
              <BookOpen className="h-4 w-4 shrink-0" />
              Cardgroups
            </Link>
          </SheetClose>

          <SheetClose asChild>
            <Link href="/profile" className={NAV_LINK_CLASS}>
              <User className="h-4 w-4 shrink-0" />
              Profile
            </Link>
          </SheetClose>
        </nav>

        {isAdmin && (
          <>
            <hr className="my-3 border-t" />
            <nav className="flex flex-col gap-1">
              <SheetClose asChild>
                <Link
                  href="/admin"
                  className="flex items-center gap-3 rounded-md border border-brand-tint-border bg-brand-tint px-3 py-2 text-sm font-medium text-brand-tint-foreground hover:opacity-90"
                >
                  <ShieldCheck className="h-4 w-4 shrink-0" />
                  Admin
                </Link>
              </SheetClose>
            </nav>
          </>
        )}

        <hr className="my-3 border-t" />

        <div>
          <LogoutButton />
        </div>
      </SheetContent>
    </Sheet>
  );
}

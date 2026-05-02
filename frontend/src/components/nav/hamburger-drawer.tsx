"use client";

import Link from "next/link";
import { BookOpen, Menu, ShieldCheck, User } from "lucide-react";
import {
  Sheet,
  SheetClose,
  SheetContent,
  SheetHeader,
  SheetTitle,
  SheetTrigger,
} from "@/components/ui/sheet";
import { LogoutButton } from "@/app/_components/logout-button";

interface HamburgerDrawerProps {
  isAdmin: boolean;
  userEmail?: string | null;
}

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

        {userEmail && (
          <p className="mb-4 truncate text-xs text-muted-foreground">
            {userEmail}
          </p>
        )}

        {/* Group 1: Navigation */}
        <nav className="flex flex-col gap-1">
          <SheetClose asChild>
            <Link
              href="/cardgroups"
              className="flex items-center gap-3 rounded-md px-3 py-2 text-sm font-medium hover:bg-accent hover:text-accent-foreground"
            >
              <BookOpen className="h-4 w-4 shrink-0" />
              Cardgroups
            </Link>
          </SheetClose>

          <SheetClose asChild>
            <Link
              href="/profile"
              className="flex items-center gap-3 rounded-md px-3 py-2 text-sm font-medium hover:bg-accent hover:text-accent-foreground"
            >
              <User className="h-4 w-4 shrink-0" />
              Profile
            </Link>
          </SheetClose>
        </nav>

        {/* Divider before admin group (always rendered to preserve layout when admin) */}
        {isAdmin && (
          <>
            <hr className="my-3 border-t" />

            {/* Group 2: Admin */}
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

        {/* Divider before sign-out */}
        <hr className="my-3 border-t" />

        {/* Group 3: Sign out */}
        <div>
          <LogoutButton />
        </div>
      </SheetContent>
    </Sheet>
  );
}
